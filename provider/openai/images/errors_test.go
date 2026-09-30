package images

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
)

// The JSON generation and edit requests go through utils.FetchJSON, so a
// non-2xx response surfaces as an *sdk.APIError decoded with OpenAI's format.
func TestHTTPErrorIsAPIError(t *testing.T) {
	const body = `{"error":{"message":"You exceeded your current quota, please check your plan and billing details.","type":"insufficient_quota","param":null,"code":"insufficient_quota"}}`
	calls := map[string]func(*Provider) error{
		"/images/generations": func(p *Provider) error {
			_, err := p.DoGenerate(context.Background(), &sdk.ImageGenerationParams{Model: p.GenerationModel("gpt-image-1"), Prompt: "x"})
			return err
		},
		"/images/edits": func(p *Provider) error {
			_, err := p.DoEdit(context.Background(), &sdk.ImageEditParams{
				Model: p.EditModel("gpt-image-1"), Prompt: "x", Images: []sdk.ImageInput{{FileID: "file-abc"}},
			})
			return err
		},
	}
	for path, call := range calls {
		t.Run(path, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != path {
					t.Errorf("path = %q, want %q", r.URL.Path, path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("x-request-id", "req_img")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			err := call(New(WithAPIKey("sk-secret-7f3a"), WithBaseURL(srv.URL)))
			apiErr := providertest.WantAPIError(t, err, providerName, http.StatusTooManyRequests, sdk.KindQuotaExhausted)
			if apiErr.RequestID != "req_img" || apiErr.Code != "insufficient_quota" {
				t.Errorf("RequestID, Code = %q, %q", apiErr.RequestID, apiErr.Code)
			}
			if text := err.Error(); strings.Contains(text, "sk-secret-7f3a") || strings.Contains(text, body) {
				t.Errorf("error text %q leaks the key or the body", text)
			}
		})
	}
}

// Raw-file edits are sent as multipart/form-data outside utils.FetchJSON, and
// must surface the same APIError. The body is OpenAI's documented 400 for a
// request that trips the safety system:
// https://platform.openai.com/docs/guides/error-codes (mirrored at
// https://github.com/openai/openai-cookbook/blob/6dc6324fb9ed780b32b787f23fad336e9f1eff15/examples/data/oai_docs/error-codes.txt).
func TestMultipartEditHTTPErrorIsAPIError(t *testing.T) {
	const body = `{"error":{"message":"Your request was rejected as a result of our safety system.","type":"invalid_request_error","param":null,"code":"moderation_blocked"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("Content-Type = %q, want multipart", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-request-id", "req_edit")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	p := New(WithAPIKey("sk-secret-7f3a"), WithBaseURL(srv.URL))
	_, err := p.DoEdit(context.Background(), &sdk.ImageEditParams{
		Model:  p.EditModel("gpt-image-1"),
		Prompt: "x",
		Images: []sdk.ImageInput{{Data: []byte("png")}},
	})
	apiErr := providertest.WantAPIError(t, err, providerName, http.StatusBadRequest, sdk.KindUnknown)
	if apiErr.Type != "invalid_request_error" || apiErr.Code != "moderation_blocked" || apiErr.RequestID != "req_edit" {
		t.Errorf("Type, Code, RequestID = %q, %q, %q", apiErr.Type, apiErr.Code, apiErr.RequestID)
	}
	if apiErr.Message != "Your request was rejected as a result of our safety system." {
		t.Errorf("Message = %q", apiErr.Message)
	}
	if text := err.Error(); strings.Contains(text, "sk-secret-7f3a") || strings.Contains(text, body) {
		t.Errorf("error text %q leaks the key or the body", text)
	}
}
