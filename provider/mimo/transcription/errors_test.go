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

// MiMo publishes no error reference. The body is the 401 that
// api.xiaomimimo.com returned for an invalid key, as quoted in
// https://github.com/XiaomiMiMo/MiMo-Code/issues/306. code is a string and
// type is not one of OpenAI's, so Kind comes from the status.
const authBody = `{"error":{"message":"Invalid API Key","param":"Please provide valid API Key","code":"401","type":"invalid_key"}}`

const secretKey = "mimo-secret-7f3a"

func TestHTTPErrorIsAPIError(t *testing.T) {
	calls := map[string]func(*Provider) error{
		"DoTranscribe": func(p *Provider) error {
			_, err := p.DoTranscribe(context.Background(), sdk.TranscriptionParams{Audio: []byte("a"), Filename: "a.wav"})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(authBody))
			}))
			defer srv.Close()
			p := New(WithAPIKey(secretKey), WithBaseURL(srv.URL+"/v1"))
			err := call(p)
			apiErr := providertest.WantAPIError(t, err, providerName, http.StatusUnauthorized, sdk.KindAuthentication)
			if apiErr.Type != "invalid_key" || apiErr.Code != "401" || apiErr.Message != "Invalid API Key" {
				t.Errorf("Type, Code, Message = %q, %q, %q", apiErr.Type, apiErr.Code, apiErr.Message)
			}
			if apiErr.RequestID != "" {
				t.Errorf("RequestID = %q, want empty", apiErr.RequestID)
			}
			if text := err.Error(); strings.Contains(text, secretKey) || strings.Contains(text, authBody) {
				t.Errorf("error text %q leaks the key or the body", text)
			}
		})
	}
}
