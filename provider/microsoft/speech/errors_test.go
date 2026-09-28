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

// The text to speech REST API documents status codes and reasons but no
// error body or request ID header:
// https://learn.microsoft.com/azure/ai-services/speech-service/rest-text-to-speech#http-status-codes
// The APIError therefore carries the status Kind and the raw body only.
func TestHTTPErrorIsAPIError(t *testing.T) {
	const secretKey = "azure-secret-7f3a"
	cases := []struct {
		status int
		kind   sdk.ErrorKind
	}{
		{http.StatusUnauthorized, sdk.KindAuthentication},
		{http.StatusTooManyRequests, sdk.KindRateLimited},
		{http.StatusBadGateway, sdk.KindServerError},
		{http.StatusBadRequest, sdk.KindUnknown},
	}
	for _, c := range cases {
		t.Run(http.StatusText(c.status), func(t *testing.T) {
			body := "reason for " + http.StatusText(c.status)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			p := New(WithAPIKey(secretKey), WithBaseURL(srv.URL))
			for name, call := range map[string]func() error{
				"DoSynthesize": func() error {
					_, err := p.DoSynthesize(context.Background(), sdk.SpeechParams{Text: "x"})
					return err
				},
				"DoStream": func() error {
					_, err := p.DoStream(context.Background(), sdk.SpeechParams{Text: "x"})
					return err
				},
			} {
				err := call()
				apiErr := providertest.WantAPIError(t, err, providerName, c.status, c.kind)
				if string(apiErr.Body) != body || apiErr.Code != "" || apiErr.RequestID != "" {
					t.Errorf("%s: Body, Code, RequestID = %q, %q, %q", name, apiErr.Body, apiErr.Code, apiErr.RequestID)
				}
				if text := err.Error(); strings.Contains(text, secretKey) || strings.Contains(text, body) {
					t.Errorf("%s: error text %q leaks the key or the body", name, text)
				}
			}
		})
	}
}
