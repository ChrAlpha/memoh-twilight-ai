package messages

import (
	"net/http"
	"testing"

	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
)

// Error types, statuses and the request-id header:
// https://platform.claude.com/docs/en/api/errors
func TestDecodeError(t *testing.T) {
	body := func(typ, msg string) string {
		return `{"type":"error","error":{"type":"` + typ + `","message":"` + msg + `"},"request_id":"req_body"}`
	}
	header := http.Header{"Request-Id": {"req_018EeWyXxfu5pfWkrYcMdjWG"}}
	cases := []providertest.ErrorCase{
		{Name: "authentication", Status: 401, Header: header, Body: body("authentication_error", "invalid x-api-key"),
			Want: sdk.APIError{Type: "authentication_error", Message: "invalid x-api-key", RequestID: "req_018EeWyXxfu5pfWkrYcMdjWG", Kind: sdk.KindAuthentication}},
		{Name: "billing", Status: 402, Body: body("billing_error", "billing"),
			Want: sdk.APIError{Type: "billing_error", Message: "billing", RequestID: "req_body", Kind: sdk.KindQuotaExhausted}},
		{Name: "permission", Status: 403, Body: body("permission_error", "denied"),
			Want: sdk.APIError{Type: "permission_error", Message: "denied", RequestID: "req_body", Kind: sdk.KindPermissionDenied}},
		{Name: "rate limit", Status: 429, Body: body("rate_limit_error", "slow down"),
			Want: sdk.APIError{Type: "rate_limit_error", Message: "slow down", RequestID: "req_body", Kind: sdk.KindRateLimited}},
		{Name: "api error", Status: 500, Body: body("api_error", "internal"),
			Want: sdk.APIError{Type: "api_error", Message: "internal", RequestID: "req_body", Kind: sdk.KindServerError}},
		{Name: "overloaded", Status: 529, Body: body("overloaded_error", "Overloaded"),
			Want: sdk.APIError{Type: "overloaded_error", Message: "Overloaded", RequestID: "req_body", Kind: sdk.KindServerError}},
		// A spend limit arrives as 400 invalid_request_error and has no
		// structured marker, so it stays unknown rather than read from text.
		{Name: "invalid request", Status: 400, Body: body("invalid_request_error", "spend limit"),
			Want: sdk.APIError{Type: "invalid_request_error", Message: "spend limit", RequestID: "req_body", Kind: sdk.KindUnknown}},
		{Name: "unknown type falls back to status", Status: 503, Body: body("new_error", "x"),
			Want: sdk.APIError{Type: "new_error", Message: "x", RequestID: "req_body", Kind: sdk.KindServerError}},
		{Name: "proxy html", Status: 502, Header: header, Body: `<html>Bad Gateway</html>`,
			Want: sdk.APIError{RequestID: "req_018EeWyXxfu5pfWkrYcMdjWG", Kind: sdk.KindServerError}},
	}
	providertest.RunErrorCases(t, "anthropic-messages", decodeError, cases)
}
