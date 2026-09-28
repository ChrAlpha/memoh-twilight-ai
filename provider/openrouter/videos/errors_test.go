package videos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
)

func TestHTTPErrorIsAPIError(t *testing.T) {
	const body = `{"error":{"code":402,"message":"Insufficient credits","metadata":{"error_type":"payment_required"}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	p := New(WithAPIKey("or-secret-7f3a"), WithBaseURL(srv.URL))
	_, err := p.DoGet(context.Background(), p.VideoModel("m"), "job-1")
	apiErr := providertest.WantAPIError(t, err, providerName, http.StatusPaymentRequired, sdk.KindQuotaExhausted)
	if string(apiErr.Body) != body {
		t.Errorf("Body = %q", apiErr.Body)
	}
	if text := err.Error(); strings.Contains(text, "or-secret-7f3a") || strings.Contains(text, body) {
		t.Errorf("error text %q leaks the key or the body", text)
	}
}
