package speech

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
)

// The body is OpenAI's documented 429 for an exhausted quota:
// https://platform.openai.com/docs/guides/error-codes (mirrored at
// https://github.com/openai/openai-cookbook/blob/6dc6324fb9ed780b32b787f23fad336e9f1eff15/examples/data/oai_docs/error-codes.txt).
// The request ID header is documented at
// https://platform.openai.com/docs/api-reference/debugging-requests.
const quotaBody = `{"error":{"message":"You exceeded your current quota, please check your plan and billing details.","type":"insufficient_quota","param":null,"code":"insufficient_quota"}}`

const secretKey = "sk-secret-7f3a"

func quotaServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-request-id", "req_speech")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(quotaBody))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestHTTPErrorIsAPIError(t *testing.T) {
	calls := map[string]func(*Provider) error{
		"ListModels": func(p *Provider) error {
			_, err := p.ListModels(context.Background())
			return err
		},
		"DoSynthesize": func(p *Provider) error {
			_, err := p.DoSynthesize(context.Background(), sdk.SpeechParams{Text: "x"})
			return err
		},
		"DoStream": func(p *Provider) error {
			_, err := p.DoStream(context.Background(), sdk.SpeechParams{Text: "x"})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			p := New(WithAPIKey(secretKey), WithBaseURL(quotaServer(t).URL))
			err := call(p)
			apiErr := providertest.WantAPIError(t, err, providerName, http.StatusTooManyRequests, sdk.KindQuotaExhausted)
			if apiErr.Type != "insufficient_quota" || apiErr.Code != "insufficient_quota" || apiErr.RequestID != "req_speech" {
				t.Errorf("Type, Code, RequestID = %q, %q, %q", apiErr.Type, apiErr.Code, apiErr.RequestID)
			}
			if apiErr.Message != "You exceeded your current quota, please check your plan and billing details." {
				t.Errorf("Message = %q", apiErr.Message)
			}
			if string(apiErr.Body) != quotaBody {
				t.Errorf("Body = %q", apiErr.Body)
			}
			if text := err.Error(); strings.Contains(text, secretKey) || strings.Contains(text, quotaBody) {
				t.Errorf("error text %q leaks the key or the body", text)
			}
		})
	}
}

// A connection failure is a transport error, not an APIError.
func TestConnectionFailureIsNotAPIError(t *testing.T) {
	p := New(WithAPIKey("key"), WithBaseURL("http://127.0.0.1:0"))
	_, err := p.DoSynthesize(context.Background(), sdk.SpeechParams{Text: "x"})
	var apiErr *sdk.APIError
	if err == nil || errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want a non-APIError", err)
	}
}
