package transcription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/felinics/twilight/provider/providertest"
	sdk "github.com/felinics/twilight/sdk"
)

func TestProvider_ListModels(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-2.5-flash","supportedGenerationMethods":["generateContent"]}]}`))
	}))
	defer srv.Close()

	p := New(WithAPIKey("key"), WithBaseURL(srv.URL))
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 || models[0].ID != "gemini-2.5-flash" {
		t.Fatalf("unexpected models: %+v", models)
	}
}

func TestProvider_DoTranscribe(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/gemini-2.5-flash:generateContent" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"hello from gemini"}]}}]}`))
	}))
	defer srv.Close()

	p := New(WithAPIKey("key"), WithBaseURL(srv.URL))
	result, err := p.DoTranscribe(context.Background(), sdk.TranscriptionParams{
		Model:       p.TranscriptionModel("gemini-2.5-flash"),
		Audio:       []byte("audio"),
		Filename:    "test.wav",
		ContentType: "audio/wav",
	})
	if err != nil {
		t.Fatalf("DoTranscribe: %v", err)
	}
	if result.Text != "hello from gemini" {
		t.Fatalf("text = %q", result.Text)
	}
}

func TestListModelsHTTPErrorIsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"message":"API key not valid. Please pass a valid API key.","status":"INVALID_ARGUMENT","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"API_KEY_INVALID","domain":"googleapis.com"}]}}`))
	}))
	defer srv.Close()
	p := New(WithAPIKey("key"), WithBaseURL(srv.URL))
	_, err := p.ListModels(context.Background())
	apiErr := providertest.WantAPIError(t, err, providerName, http.StatusBadRequest, sdk.KindAuthentication)
	if apiErr.Code != "API_KEY_INVALID" {
		t.Errorf("Code = %q", apiErr.Code)
	}
}

// The body is google.rpc.Status as the Gemini API sends it for an exhausted
// quota: https://ai.google.dev/gemini-api/docs/troubleshooting (429
// RESOURCE_EXHAUSTED) and https://cloud.google.com/apis/design/errors. The
// API documents no request ID header.
func TestTranscribeHTTPErrorIsAPIError(t *testing.T) {
	const body = `{"error":{"code":429,"message":"You exceeded your current quota, please check your plan and billing details.","status":"RESOURCE_EXHAUSTED"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	p := New(WithAPIKey("secret-7f3a"), WithBaseURL(srv.URL))
	_, err := p.DoTranscribe(context.Background(), sdk.TranscriptionParams{Audio: []byte("a")})
	apiErr := providertest.WantAPIError(t, err, providerName, http.StatusTooManyRequests, sdk.KindRateLimited)
	if apiErr.Type != "RESOURCE_EXHAUSTED" || apiErr.Message != "You exceeded your current quota, please check your plan and billing details." {
		t.Errorf("Type, Message = %q, %q", apiErr.Type, apiErr.Message)
	}
	if text := err.Error(); strings.Contains(text, "secret-7f3a") || strings.Contains(text, body) {
		t.Errorf("error text %q leaks the key or the body", text)
	}
}
