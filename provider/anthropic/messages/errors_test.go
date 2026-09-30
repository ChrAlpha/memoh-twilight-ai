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

// Every error type of the ErrorResponse schema, with the schema's default
// message, in
// https://github.com/anthropics/anthropic-sdk-go/blob/ad865dfa3d1a8d2f4a7ad0d072011e811e9957a9/scripts/mock-spec.json.gz
// (MIT, Copyright 2023 Anthropic, PBC.), and the two types that only
// https://platform.claude.com/docs/en/api/errors lists, conflict_error and
// request_too_large. The statuses are the documentation's. The documentation
// gives no message for those two, so theirs are placeholders.
func TestDecodeErrorTypes(t *testing.T) {
	types := []struct {
		status       int
		typ, message string
		kind         sdk.ErrorKind
	}{
		{400, "invalid_request_error", "Invalid request", sdk.KindUnknown},
		{401, "authentication_error", "Authentication error", sdk.KindAuthentication},
		{402, "billing_error", "Billing error", sdk.KindQuotaExhausted},
		{403, "permission_error", "Permission denied", sdk.KindPermissionDenied},
		{404, "not_found_error", "Not found", sdk.KindUnknown},
		{409, "conflict_error", "Conflict", sdk.KindUnknown},
		{413, "request_too_large", "Request too large", sdk.KindUnknown},
		{429, "rate_limit_error", "Rate limited", sdk.KindRateLimited},
		{500, "api_error", "Internal server error", sdk.KindServerError},
		{504, "timeout_error", "Request timeout", sdk.KindServerError},
		{529, "overloaded_error", "Overloaded", sdk.KindServerError},
	}
	cases := make([]providertest.ErrorCase, 0, len(types))
	for _, et := range types {
		cases = append(cases, providertest.ErrorCase{
			Name:   et.typ,
			Status: et.status,
			Body:   `{"type":"error","error":{"type":"` + et.typ + `","message":"` + et.message + `"},"request_id":"req_body"}`,
			Want:   sdk.APIError{Type: et.typ, Message: et.message, RequestID: "req_body", Kind: et.kind},
		})
	}
	providertest.RunErrorCases(t, "anthropic-messages", decodeError, cases)
}
