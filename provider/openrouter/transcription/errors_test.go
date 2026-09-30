package transcription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
)

// Error body and error_type values: https://openrouter.ai/docs/api-reference/errors
const rateLimitBody = `{
    "error": {
      "code": 429,
      "message": "Rate limit exceeded",
      "metadata": {
        "error_type": "rate_limit_exceeded",
        "provider_code": "rate_limited"
      }
    }
  }`

const secretKey = "or-secret-7f3a"

func TestHTTPErrorIsAPIError(t *testing.T) {
	calls := map[string]func(*Provider) error{
		"ListModels": func(p *Provider) error {
			_, err := p.ListModels(context.Background())
			return err
		},
		"DoTranscribe": func(p *Provider) error {
			_, err := p.DoTranscribe(context.Background(), sdk.TranscriptionParams{Audio: []byte("a"), Filename: "a.wav"})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(rateLimitBody))
			}))
			defer srv.Close()
			p := New(WithAPIKey(secretKey), WithBaseURL(srv.URL))
			err := call(p)
			apiErr := providertest.WantAPIError(t, err, providerName, http.StatusTooManyRequests, sdk.KindRateLimited)
			if apiErr.Type != "rate_limit_exceeded" || apiErr.Code != "429" || apiErr.Message != "Rate limit exceeded" {
				t.Errorf("Type, Code, Message = %q, %q, %q", apiErr.Type, apiErr.Code, apiErr.Message)
			}
			if apiErr.RequestID != "" {
				t.Errorf("RequestID = %q, want empty", apiErr.RequestID)
			}
			if text := err.Error(); strings.Contains(text, secretKey) || strings.Contains(text, rateLimitBody) {
				t.Errorf("error text %q leaks the key or the body", text)
			}
		})
	}
}
