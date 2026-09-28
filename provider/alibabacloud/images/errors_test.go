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

func TestHTTPErrorIsAPIError(t *testing.T) {
	const body = `{"code":"InvalidApiKey","message":"Invalid API-key provided.","request_id":"r1"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	p := New(WithAPIKey("sk-secret-7f3a"), WithBaseURL(srv.URL))
	_, err := p.DoGenerate(context.Background(), &sdk.ImageGenerationParams{Model: p.GenerationModel("wan2.2-t2i-flash"), Prompt: "x"})
	apiErr := providertest.WantAPIError(t, err, providerName, http.StatusUnauthorized, sdk.KindAuthentication)
	if string(apiErr.Body) != body {
		t.Errorf("Body = %q", apiErr.Body)
	}
	if text := err.Error(); strings.Contains(text, "sk-secret-7f3a") || strings.Contains(text, body) {
		t.Errorf("error text %q leaks the key or the body", text)
	}
}
