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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-request-id", "req_img")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	p := New(WithAPIKey("sk-secret-7f3a"), WithBaseURL(srv.URL))
	_, err := p.DoGenerate(context.Background(), &sdk.ImageGenerationParams{Model: p.GenerationModel("gpt-image-1"), Prompt: "x"})
	apiErr := providertest.WantAPIError(t, err, providerName, http.StatusTooManyRequests, sdk.KindQuotaExhausted)
	if apiErr.RequestID != "req_img" || apiErr.Code != "insufficient_quota" {
		t.Errorf("RequestID, Code = %q, %q", apiErr.RequestID, apiErr.Code)
	}
	if text := err.Error(); strings.Contains(text, "sk-secret-7f3a") || strings.Contains(text, body) {
		t.Errorf("error text %q leaks the key or the body", text)
	}
}
