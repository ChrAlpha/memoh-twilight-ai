package errorformat_test

import (
	"net/http"
	"testing"

	"github.com/felinics/twilight/internal/errorformat"
	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
)

func reqID(name, value string) http.Header {
	h := http.Header{}
	h.Set(name, value)
	return h
}

// OpenAI error codes and statuses: https://platform.openai.com/docs/guides/error-codes
// (mirrored at https://github.com/openai/openai-cookbook/blob/6dc6324fb9ed780b32b787f23fad336e9f1eff15/examples/data/oai_docs/error-codes.txt).
// The 401 body is verbatim from the official .NET SDK's recording:
// https://github.com/openai/openai-dotnet/blob/4e5ae90621089e0b1f5571546a75cea6b0cc713f/tests/SessionRecords/ChatTests/AuthFailure.json
func TestDecodeOpenAI(t *testing.T) {
	providertest.RunErrorCases(t, "openai-completions", errorformat.DecodeOpenAI, []providertest.ErrorCase{
		{
			Name:   "invalid api key",
			Status: http.StatusUnauthorized,
			Header: reqID("x-request-id", "req_401"),
			Body:   `{"error":{"message":"Incorrect API key provided: not-a-re**************************ized. You can find your API key at https://platform.openai.com/account/api-keys.","type":"invalid_request_error","param":null,"code":"invalid_api_key"}}`,
			Want: sdk.APIError{
				Type: "invalid_request_error", Code: "invalid_api_key", RequestID: "req_401", Kind: sdk.KindAuthentication,
				Message: "Incorrect API key provided: not-a-re**************************ized. You can find your API key at https://platform.openai.com/account/api-keys.",
			},
		},
		{
			// The code wins over the 429 status.
			Name:   "insufficient quota",
			Status: http.StatusTooManyRequests,
			Body:   `{"error":{"message":"You exceeded your current quota, please check your plan and billing details.","type":"insufficient_quota","param":null,"code":"insufficient_quota"}}`,
			Want: sdk.APIError{
				Type: "insufficient_quota", Code: "insufficient_quota", Kind: sdk.KindQuotaExhausted,
				Message: "You exceeded your current quota, please check your plan and billing details.",
			},
		},
		{
			Name:   "rate limit",
			Status: http.StatusTooManyRequests,
			Body:   `{"error":{"message":"Rate limit reached for requests","type":"requests","param":null,"code":"rate_limit_exceeded"}}`,
			Want:   sdk.APIError{Type: "requests", Code: "rate_limit_exceeded", Message: "Rate limit reached for requests", Kind: sdk.KindRateLimited},
		},
		{
			Name:   "server error",
			Status: http.StatusInternalServerError,
			Body:   `{"error":{"message":"The server had an error while processing your request.","type":"server_error","param":null,"code":null}}`,
			Want:   sdk.APIError{Type: "server_error", Message: "The server had an error while processing your request.", Kind: sdk.KindServerError},
		},
		{
			Name:   "unsupported region falls back to status",
			Status: http.StatusForbidden,
			Body:   `{"error":{"message":"Country, region, or territory not supported","type":"request_forbidden","param":null,"code":"unsupported_country_region_territory"}}`,
			Want: sdk.APIError{
				Type: "request_forbidden", Code: "unsupported_country_region_territory",
				Message: "Country, region, or territory not supported", Kind: sdk.KindPermissionDenied,
			},
		},
		{
			Name:   "unknown code and status",
			Status: http.StatusBadRequest,
			Body:   `{"error":{"message":"bad","type":"invalid_request_error","param":"model","code":"model_not_found"}}`,
			Want:   sdk.APIError{Type: "invalid_request_error", Code: "model_not_found", Message: "bad", Kind: sdk.KindUnknown},
		},
		{
			// OpenAI-compatible APIs (OpenRouter) send a numeric code.
			Name:   "numeric code",
			Status: http.StatusPaymentRequired,
			Body:   `{"error":{"code":402,"message":"Insufficient credits"}}`,
			Want:   sdk.APIError{Code: "402", Message: "Insufficient credits", Kind: sdk.KindQuotaExhausted},
		},
		{
			Name:   "bedrock request id",
			Status: http.StatusServiceUnavailable,
			Header: reqID("x-amzn-requestid", "0b6c3e1a-bedrock"),
			Body:   `not json`,
			Want:   sdk.APIError{RequestID: "0b6c3e1a-bedrock", Kind: sdk.KindServerError},
		},
	})
}

// The "error" event's top-level fields follow openai-go's ResponseErrorEvent:
// https://github.com/openai/openai-go/blob/d7fd0c65cc247957d5b247ad42283fc8e4061868/responses/response.go
// The nested shape is the one the Codex CLI reads:
// https://github.com/openai/codex/blob/1b1835f751ebdc0cfc50b3fe55d4571dbb294563/codex-rs/codex-api/src/sse/responses.rs
func TestDecodeOpenAIErrorEvent(t *testing.T) {
	providertest.RunErrorCases(t, "openai-responses", errorformat.DecodeOpenAIErrorEvent, []providertest.ErrorCase{
		{
			Name:   "top-level code and message",
			Header: reqID("x-request-id", "req_ev"),
			Body:   `{"type":"error","code":"server_error","message":"The server had an error while processing your request.","param":null,"sequence_number":7}`,
			Want: sdk.APIError{
				Code: "server_error", Message: "The server had an error while processing your request.",
				RequestID: "req_ev", Kind: sdk.KindServerError,
			},
		},
		{
			Name: "nested error object",
			Body: `{"type":"error","sequence_number":2,"error":{"type":"invalid_request_error","code":"rate_limit_exceeded","message":"Rate limit reached"}}`,
			Want: sdk.APIError{Type: "invalid_request_error", Code: "rate_limit_exceeded", Message: "Rate limit reached", Kind: sdk.KindRateLimited},
		},
		{
			Name: "unknown code",
			Body: `{"type":"error","code":"made_up","message":"boom"}`,
			Want: sdk.APIError{Code: "made_up", Message: "boom", Kind: sdk.KindUnknown},
		},
		{
			Name: "not json",
			Body: `not json`,
			Want: sdk.APIError{Kind: sdk.KindUnknown},
		},
	})
}

// The body is verbatim from the Codex CLI's response.failed fixture:
// https://github.com/openai/codex/blob/1b1835f751ebdc0cfc50b3fe55d4571dbb294563/codex-rs/codex-api/src/sse/responses.rs
func TestDecodeOpenAIFailedEvent(t *testing.T) {
	const message = "Rate limit reached for gpt-5.1 in organization org-AAA on tokens per min (TPM): Limit 30000, Used 22999, Requested 12528. Please try again in 11.054s. Visit https://platform.openai.com/account/rate-limits to learn more."
	providertest.RunErrorCases(t, "openai-responses", errorformat.DecodeOpenAIFailedEvent, []providertest.ErrorCase{
		{
			Name: "rate limit",
			Body: `{"type":"response.failed","sequence_number":3,"response":{"id":"resp_689bcf18d7f08194bf3440ba62fe05d803fee0cdac429894","object":"response","created_at":1755041560,"status":"failed","background":false,"error":{"code":"rate_limit_exceeded","message":"` + message + `"}, "usage":null,"user":null,"metadata":{}}}`,
			Want: sdk.APIError{Code: "rate_limit_exceeded", Message: message, Kind: sdk.KindRateLimited},
		},
		{
			Name: "no error object",
			Body: `{"type":"response.failed","response":{"status":"failed"}}`,
			Want: sdk.APIError{Kind: sdk.KindUnknown},
		},
	})
}

// Canonical statuses: https://cloud.google.com/apis/design/errors. ErrorInfo
// reasons: https://github.com/googleapis/googleapis/blob/563e22e733b315aca418482f644c878997f328ea/google/api/error_reason.proto
// The RESOURCE_EXHAUSTED body is the design guide's example, verbatim; the
// others put the proto's ErrorInfo examples inside the same envelope.
func TestDecodeGoogle(t *testing.T) {
	providertest.RunErrorCases(t, "google-generative-ai", errorformat.DecodeGoogle, []providertest.ErrorCase{
		{
			// The reason wins over the 400 status.
			Name:   "api key invalid",
			Status: http.StatusBadRequest,
			Body:   `{"error":{"code":400,"message":"API key not valid. Please pass a valid API key.","status":"INVALID_ARGUMENT","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"API_KEY_INVALID","domain":"googleapis.com","metadata":{"service":"generativelanguage.googleapis.com"}}]}}`,
			Want: sdk.APIError{
				Type: "INVALID_ARGUMENT", Code: "API_KEY_INVALID",
				Message: "API key not valid. Please pass a valid API key.", Kind: sdk.KindAuthentication,
			},
		},
		{
			Name:   "billing disabled",
			Status: http.StatusForbidden,
			Body:   `{"error":{"code":403,"message":"Billing disabled","status":"PERMISSION_DENIED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"BILLING_DISABLED","domain":"googleapis.com","metadata":{"consumer":"projects/123","service":"pubsub.googleapis.com"}}]}}`,
			Want:   sdk.APIError{Type: "PERMISSION_DENIED", Code: "BILLING_DISABLED", Message: "Billing disabled", Kind: sdk.KindQuotaExhausted},
		},
		{
			Name:   "permission denied",
			Status: http.StatusForbidden,
			Body:   `{"error":{"code":403,"message":"denied","status":"PERMISSION_DENIED"}}`,
			Want:   sdk.APIError{Type: "PERMISSION_DENIED", Message: "denied", Kind: sdk.KindPermissionDenied},
		},
		{
			Name:   "resource exhausted",
			Status: http.StatusTooManyRequests,
			Body:   `{"error":{"code":429,"message":"The zone 'us-east1-a' does not have enough resources available to fulfill the request. Try a different zone, or try again later.","status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"RESOURCE_AVAILABILITY","domain":"compute.googleapis.com"}]}}`,
			Want: sdk.APIError{
				Type: "RESOURCE_EXHAUSTED", Code: "RESOURCE_AVAILABILITY", Kind: sdk.KindRateLimited,
				Message: "The zone 'us-east1-a' does not have enough resources available to fulfill the request. Try a different zone, or try again later.",
			},
		},
		{
			Name:   "unavailable",
			Status: http.StatusServiceUnavailable,
			Body:   `{"error":{"code":503,"message":"The model is overloaded. Please try again later.","status":"UNAVAILABLE"}}`,
			Want:   sdk.APIError{Type: "UNAVAILABLE", Message: "The model is overloaded. Please try again later.", Kind: sdk.KindServerError},
		},
		{
			Name:   "not found",
			Status: http.StatusNotFound,
			Body:   `{"error":{"code":404,"message":"models/x is not found","status":"NOT_FOUND"}}`,
			Want:   sdk.APIError{Type: "NOT_FOUND", Message: "models/x is not found", Kind: sdk.KindUnknown},
		},
	})
}

// Error body and error_type values: https://openrouter.ai/docs/api-reference/errors
func TestDecodeOpenRouter(t *testing.T) {
	cases := []providertest.ErrorCase{
		{Name: "typed rate limit", Status: 429,
			Body: `{
    "error": {
      "code": 429,
      "message": "Rate limit exceeded",
      "metadata": {
        "error_type": "rate_limit_exceeded",
        "provider_code": "rate_limited"
      }
    }
  }`,
			Want: sdk.APIError{Type: "rate_limit_exceeded", Code: "429", Message: "Rate limit exceeded", Kind: sdk.KindRateLimited}},
		{Name: "credits", Status: 402,
			Body: `{"error":{"code":402,"message":"Insufficient credits","metadata":{"error_type":"payment_required"}}}`,
			Want: sdk.APIError{Type: "payment_required", Code: "402", Message: "Insufficient credits", Kind: sdk.KindQuotaExhausted}},
		{Name: "untyped falls back to status", Status: 401,
			Body: `{"error":{"code":401,"message":"No auth credentials found"}}`,
			Want: sdk.APIError{Code: "401", Message: "No auth credentials found", Kind: sdk.KindAuthentication}},
		{Name: "provider overloaded", Status: 503,
			Body: `{"error":{"code":503,"message":"busy","metadata":{"error_type":"provider_overloaded"}}}`,
			Want: sdk.APIError{Type: "provider_overloaded", Code: "503", Message: "busy", Kind: sdk.KindServerError}},
	}
	providertest.RunErrorCases(t, "openrouter-videos", errorformat.DecodeOpenRouter, cases)
}

// Error body: https://help.aliyun.com/zh/model-studio/text-to-image-v2-api-reference
// (异常响应). Codes and statuses: https://help.aliyun.com/zh/model-studio/error-code
func TestDecodeDashScope(t *testing.T) {
	cases := []providertest.ErrorCase{
		{Name: "no api key", Status: 401,
			Body: `{
 "code": "InvalidApiKey",
 "message": "No API-key provided.",
 "request_id": "7438d53d-6eb8-4596-8835-xxxxxx"
}`,
			Want: sdk.APIError{Code: "InvalidApiKey", Message: "No API-key provided.", RequestID: "7438d53d-6eb8-4596-8835-xxxxxx", Kind: sdk.KindAuthentication}},
		{Name: "arrearage is 400", Status: 400,
			Body: `{"request_id":"r1","code":"Arrearage","message":"Access denied, please make sure your account is in good standing."}`,
			Want: sdk.APIError{Code: "Arrearage", Message: "Access denied, please make sure your account is in good standing.", RequestID: "r1", Kind: sdk.KindQuotaExhausted}},
		{Name: "free tier only is 403", Status: 403,
			Body: `{"request_id":"r2","code":"AllocationQuota.FreeTierOnly","message":"The free tier of the model has been exhausted."}`,
			Want: sdk.APIError{Code: "AllocationQuota.FreeTierOnly", Message: "The free tier of the model has been exhausted.", RequestID: "r2", Kind: sdk.KindQuotaExhausted}},
		{Name: "rate quota", Status: 429,
			Body: `{"request_id":"r3","code":"Throttling.RateQuota","message":"Requests rate limit exceeded, please try again later."}`,
			Want: sdk.APIError{Code: "Throttling.RateQuota", Message: "Requests rate limit exceeded, please try again later.", RequestID: "r3", Kind: sdk.KindRateLimited}},
		{Name: "invalid parameter", Status: 400,
			Body: `{"request_id":"a4d78a5f-655f-9639-8437-xxxxxx","code":"InvalidParameter","message":"n must be 1"}`,
			Want: sdk.APIError{Code: "InvalidParameter", Message: "n must be 1", RequestID: "a4d78a5f-655f-9639-8437-xxxxxx", Kind: sdk.KindUnknown}},
		{Name: "unknown code falls back to status", Status: 500,
			Body: `{"request_id":"r4","code":"InternalError","message":"boom"}`,
			Want: sdk.APIError{Code: "InternalError", Message: "boom", RequestID: "r4", Kind: sdk.KindServerError}},
		{Name: "failed task in a 2xx body",
			Body: `{"request_id":"r5","output":{"task_id":"t1","task_status":"FAILED","code":"DataInspectionFailed","message":"Input data may contain inappropriate content."}}`,
			Want: sdk.APIError{Code: "DataInspectionFailed", Message: "Input data may contain inappropriate content.", RequestID: "r5", Kind: sdk.KindUnknown}},
		{Name: "business code in a 2xx body",
			Body: `{"request_id":"r6","code":"Arrearage","message":"Access denied, please make sure your account is in good standing."}`,
			Want: sdk.APIError{Code: "Arrearage", Message: "Access denied, please make sure your account is in good standing.", RequestID: "r6", Kind: sdk.KindQuotaExhausted}},
		{Name: "non-JSON body keeps the status kind", Status: 503,
			Body: "upstream unavailable",
			Want: sdk.APIError{Kind: sdk.KindServerError}},
	}
	providertest.RunErrorCases(t, "alibabacloud-images", errorformat.DecodeDashScope, cases)
}

func TestKindWithoutStatus(t *testing.T) {
	// Stream events carry no status; the type or code alone must classify.
	if got := errorformat.OpenAIKind("", "insufficient_quota"); got != sdk.KindQuotaExhausted {
		t.Errorf("OpenAIKind = %q", got)
	}
	if got := errorformat.OpenAIKind("", "made_up"); got != sdk.KindUnknown {
		t.Errorf("OpenAIKind unknown = %q", got)
	}
	if got := errorformat.GoogleKind("UNAVAILABLE", ""); got != sdk.KindServerError {
		t.Errorf("GoogleKind = %q", got)
	}
}
