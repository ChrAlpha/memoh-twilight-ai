package copilot

import (
	"net/http"
	"testing"

	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
)

// GitHub publishes no error format for the Copilot API. The envelope, the
// codes and the request ID headers are the ones the Copilot Chat client reads:
// https://github.com/microsoft/vscode/blob/10b02313064d1c1978691b43178542ca20bc8202/extensions/copilot/src/extension/prompt/node/chatMLFetcher.ts
// https://github.com/microsoft/vscode/blob/10b02313064d1c1978691b43178542ca20bc8202/extensions/copilot/src/platform/networking/common/fetch.ts
func TestDecodeError(t *testing.T) {
	cases := []providertest.ErrorCase{
		{Name: "quota", Status: 402, Header: http.Header{"X-Github-Request-Id": {"GH:1"}, "X-Request-Id": {"echo"}},
			Body: `{"error":{"message":"quota","code":"quota_exceeded"}}`,
			Want: sdk.APIError{Code: "quota_exceeded", Message: "quota", RequestID: "GH:1", Kind: sdk.KindQuotaExhausted}},
		{Name: "rate limited with suffix", Status: 429, Header: http.Header{"X-Request-Id": {"echo"}},
			Body: `{"error":{"message":"slow","code":"user_model_rate_limited:gpt-4o"}}`,
			Want: sdk.APIError{Code: "user_model_rate_limited:gpt-4o", Message: "slow", RequestID: "echo", Kind: sdk.KindRateLimited}},
		{Name: "top-level object", Status: 429,
			Body: `{"message":"blocked","code":"extension_blocked","type":"rate_limit_error"}`,
			Want: sdk.APIError{Type: "rate_limit_error", Code: "extension_blocked", Message: "blocked", Kind: sdk.KindRateLimited}},
		{Name: "overloaded", Status: 429,
			Body: `{"error":{"message":"busy","code":"model_overloaded"}}`,
			Want: sdk.APIError{Code: "model_overloaded", Message: "busy", Kind: sdk.KindServerError}},
		{Name: "bad credentials", Status: 401,
			Body: `{"error":{"message":"Bad credentials","type":"invalid_request_error"}}`,
			Want: sdk.APIError{Type: "invalid_request_error", Message: "Bad credentials", Kind: sdk.KindAuthentication}},
	}
	providertest.RunErrorCases(t, "github-copilot", decodeError, cases)
}
