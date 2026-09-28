package codex

import (
	"net/http"
	"testing"

	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
)

// Types, codes and request ID headers are the ones the Codex CLI maps:
// https://github.com/openai/codex/blob/44fe510ce3ee61c8ef623adcbf89b901c73ddd61/codex-rs/codex-api/src/api_bridge.rs
func TestDecodeError(t *testing.T) {
	cases := []providertest.ErrorCase{
		{Name: "usage limit", Status: 429, Header: http.Header{"X-Request-Id": {"req_1"}},
			Body: `{"error":{"type":"usage_limit_reached","plan_type":"pro"}}`,
			Want: sdk.APIError{Type: "usage_limit_reached", RequestID: "req_1", Kind: sdk.KindQuotaExhausted}},
		{Name: "usage not included", Status: 429, Header: http.Header{"X-Oai-Request-Id": {"req_oai"}},
			Body: `{"error":{"type":"usage_not_included"}}`,
			Want: sdk.APIError{Type: "usage_not_included", RequestID: "req_oai", Kind: sdk.KindPermissionDenied}},
		{Name: "spend limit code", Status: 429,
			Body: `{"error":{"code":"project_spend_limit_exceeded","message":"limit"}}`,
			Want: sdk.APIError{Code: "project_spend_limit_exceeded", Message: "limit", Kind: sdk.KindQuotaExhausted}},
		{Name: "server overloaded", Status: 503,
			Body: `{"error":{"code":"server_is_overloaded","message":"busy"}}`,
			Want: sdk.APIError{Code: "server_is_overloaded", Message: "busy", Kind: sdk.KindServerError}},
		{Name: "slow down", Status: 503,
			Body: `{"error":{"code":"slow_down","message":"wait"}}`,
			Want: sdk.APIError{Code: "slow_down", Message: "wait", Kind: sdk.KindRateLimited}},
		{Name: "plain 429 is a rate limit", Status: 429, Body: `{}`,
			Want: sdk.APIError{Kind: sdk.KindRateLimited}},
	}
	providertest.RunErrorCases(t, "openai-codex", decodeError, cases)
}
