package speech

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
)

// Error bodies are verbatim from https://developers.deepgram.com/docs/errors,
// with the documented "uuid" placeholder replaced by a sample ID.
const (
	invalidAuthBody = `{"err_code":"INVALID_AUTH","err_msg":"Invalid credentials.","request_id":"9a1c5e7f-0000-4000-8000-000000000401"}`
	secretKey       = "dg-secret-7f3a"
)

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
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("dg-request-id", "9a1c5e7f-0000-4000-8000-000000000401")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(invalidAuthBody))
			}))
			defer srv.Close()
			err := call(New(WithAPIKey(secretKey), WithBaseURL(srv.URL)))
			apiErr := providertest.WantAPIError(t, err, providerName, http.StatusUnauthorized, sdk.KindAuthentication)
			if apiErr.Code != "INVALID_AUTH" || apiErr.Message != "Invalid credentials." || apiErr.RequestID != "9a1c5e7f-0000-4000-8000-000000000401" {
				t.Errorf("Code, Message, RequestID = %q, %q, %q", apiErr.Code, apiErr.Message, apiErr.RequestID)
			}
			if text := err.Error(); strings.Contains(text, secretKey) || strings.Contains(text, invalidAuthBody) {
				t.Errorf("error text %q leaks the key or the body", text)
			}
		})
	}
}
