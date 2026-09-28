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

// Codes, types and statuses: https://www.volcengine.com/docs/82379/1299023.
// The envelope and its field names are the Ark Go SDK's model.ErrorResponse:
// https://github.com/volcengine/volcengine-go-sdk/blob/be628166a0bada0efa7646ce6d0e9e484f0af163/service/arkruntime/model/error.go
func TestDecodeError(t *testing.T) {
	body := func(status, code, msg string) string {
		return `{"error":{"code":"` + code + `","message":"` + msg + `","param":"","type":"` + status + `"}}`
	}
	cases := []providertest.ErrorCase{
		{Name: "authentication", Status: 401, Body: body("Unauthorized", "AuthenticationError", "The API key or AK/SK in the request is missing or invalid. Request ID: 1."),
			Want: sdk.APIError{Type: "Unauthorized", Code: "AuthenticationError", Message: "The API key or AK/SK in the request is missing or invalid. Request ID: 1.", Kind: sdk.KindAuthentication}},
		{Name: "overdue is 403", Status: 403, Body: body("Forbidden", "AccountOverdueError", "overdue"),
			Want: sdk.APIError{Type: "Forbidden", Code: "AccountOverdueError", Message: "overdue", Kind: sdk.KindQuotaExhausted}},
		{Name: "access denied", Status: 403, Body: body("Forbidden", "AccessDenied", "denied"),
			Want: sdk.APIError{Type: "Forbidden", Code: "AccessDenied", Message: "denied", Kind: sdk.KindPermissionDenied}},
		{Name: "set limit is 429", Status: 429, Body: body("TooManyRequests", "SetLimitExceeded", "paused"),
			Want: sdk.APIError{Type: "TooManyRequests", Code: "SetLimitExceeded", Message: "paused", Kind: sdk.KindQuotaExhausted}},
		{Name: "overloaded is 429", Status: 429, Body: body("TooManyRequests", "ServerOverloaded", "busy"),
			Want: sdk.APIError{Type: "TooManyRequests", Code: "ServerOverloaded", Message: "busy", Kind: sdk.KindServerError}},
		{Name: "rpm", Status: 429, Body: body("TooManyRequests", "ModelAccountRpmRateLimitExceeded", "rpm"),
			Want: sdk.APIError{Type: "TooManyRequests", Code: "ModelAccountRpmRateLimitExceeded", Message: "rpm", Kind: sdk.KindRateLimited}},
		{Name: "internal", Status: 500, Body: body("InternalServerError", "InternalServiceError", "retry"),
			Want: sdk.APIError{Type: "InternalServerError", Code: "InternalServiceError", Message: "retry", Kind: sdk.KindServerError}},
	}
	providertest.RunErrorCases(t, providerName, decodeError, cases)
}

func TestHTTPErrorIsAPIError(t *testing.T) {
	const body = `{"error":{"code":"AccountOverdueError","message":"overdue","param":"","type":"Forbidden"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	p := New(WithAPIKey("ark-secret-7f3a"), WithBaseURL(srv.URL))
	_, err := p.DoGet(context.Background(), p.VideoModel("m"), "task-1")
	apiErr := providertest.WantAPIError(t, err, providerName, http.StatusForbidden, sdk.KindQuotaExhausted)
	if string(apiErr.Body) != body {
		t.Errorf("Body = %q", apiErr.Body)
	}
	if text := err.Error(); strings.Contains(text, "ark-secret-7f3a") || strings.Contains(text, body) {
		t.Errorf("error text %q leaks the key or the body", text)
	}
}
