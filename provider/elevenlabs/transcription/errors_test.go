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

// Error bodies follow https://elevenlabs.io/docs/eleven-api/resources/errors.
// The validation body is the page's example verbatim; the others fill the
// documented fields with rows of its error-code table.
const (
	validationBody = `{"detail":{"type":"validation_error","code":"invalid_parameters","message":"The 'keyterms' parameter is only supported with the 'scribe_v2' model. You specified 'scribe_v1'.","status":"invalid_parameters","request_id":"3c807fc4c3a1705f9638ecc764a91c01","param":"keyterms"}}`
	creditsBody    = `{"detail":{"type":"payment_required","code":"insufficient_credits","message":"Your account does not have enough credits for this operation.","status":"insufficient_credits","request_id":"req_402"}}`
	secretKey      = "xi-secret-7f3a"
)

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
				w.Header().Set("request-id", "req_402")
				w.WriteHeader(http.StatusPaymentRequired)
				_, _ = w.Write([]byte(creditsBody))
			}))
			defer srv.Close()
			err := call(New(WithAPIKey(secretKey), WithBaseURL(srv.URL)))
			apiErr := providertest.WantAPIError(t, err, providerName, http.StatusPaymentRequired, sdk.KindQuotaExhausted)
			if apiErr.Code != "insufficient_credits" || apiErr.RequestID != "req_402" {
				t.Errorf("Code, RequestID = %q, %q", apiErr.Code, apiErr.RequestID)
			}
			if text := err.Error(); strings.Contains(text, secretKey) || strings.Contains(text, creditsBody) {
				t.Errorf("error text %q leaks the key or the body", text)
			}
		})
	}
}
