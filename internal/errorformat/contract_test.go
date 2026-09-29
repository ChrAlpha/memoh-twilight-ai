package errorformat_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/felinics/twilight/internal/errorformat"
	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
)

// The tests in this file check the decoders against the providers' published
// contracts. Each enum is copied in full from the pinned spec, so every value
// the provider can send has a listed Kind, and each example is verbatim.

// wantKinds requires kindOf to return the listed Kind for every value.
func wantKinds(t *testing.T, kindOf func(string) sdk.ErrorKind, want map[string]sdk.ErrorKind) {
	t.Helper()
	for value, kind := range want {
		if got := kindOf(value); got != kind {
			t.Errorf("%s: Kind = %q, want %q", value, got, kind)
		}
	}
}

// ResponseErrorCode, the code of a failed Response's error, in
// https://github.com/openai/openai-openapi/blob/b6059fc737ac846e8ba64ad65f4f2c8bc0d15854/openapi.yaml
func TestOpenAIResponseErrorCodes(t *testing.T) {
	wantKinds(t, func(code string) sdk.ErrorKind { return errorformat.OpenAIKind("", code) }, map[string]sdk.ErrorKind{
		"server_error":                   sdk.KindServerError,
		"rate_limit_exceeded":            sdk.KindRateLimited,
		"invalid_prompt":                 sdk.KindUnknown,
		"data_residency_mismatch":        sdk.KindUnknown,
		"bio_policy":                     sdk.KindUnknown,
		"misalignment_policy_violation":  sdk.KindUnknown,
		"vector_store_timeout":           sdk.KindUnknown,
		"invalid_image":                  sdk.KindUnknown,
		"invalid_image_format":           sdk.KindUnknown,
		"invalid_base64_image":           sdk.KindUnknown,
		"invalid_image_url":              sdk.KindUnknown,
		"image_too_large":                sdk.KindUnknown,
		"image_too_small":                sdk.KindUnknown,
		"image_parse_error":              sdk.KindUnknown,
		"image_content_policy_violation": sdk.KindUnknown,
		"invalid_image_mode":             sdk.KindUnknown,
		"image_file_too_large":           sdk.KindUnknown,
		"unsupported_image_media_type":   sdk.KindUnknown,
		"empty_image_file":               sdk.KindUnknown,
		"failed_to_download_image":       sdk.KindUnknown,
		"image_file_not_found":           sdk.KindUnknown,
	})
}

// The ResponseErrorEvent and ResponseFailedEvent examples, verbatim from
// https://github.com/openai/openai-openapi/blob/b6059fc737ac846e8ba64ad65f4f2c8bc0d15854/openapi.yaml
func TestOpenAIStreamEventExamples(t *testing.T) {
	providertest.RunErrorCases(t, "openai-responses", errorformat.DecodeOpenAIErrorEvent, []providertest.ErrorCase{{
		Name: "error event",
		Body: `{
  "type": "error",
  "code": "ERR_SOMETHING",
  "message": "Something went wrong",
  "param": null,
  "sequence_number": 1
}`,
		Want: sdk.APIError{Code: "ERR_SOMETHING", Message: "Something went wrong", Kind: sdk.KindUnknown},
	}})
	providertest.RunErrorCases(t, "openai-responses", errorformat.DecodeOpenAIFailedEvent, []providertest.ErrorCase{{
		Name: "response.failed",
		Body: `{
  "type": "response.failed",
  "response": {
    "id": "resp_123",
    "object": "response",
    "access_programs": null,
    "created_at": 1740855869,
    "status": "failed",
    "completed_at": null,
    "error": {
      "code": "server_error",
      "message": "The model failed to generate a response."
    },
    "incomplete_details": null,
    "instructions": null,
    "max_output_tokens": null,
    "model": "gpt-6-astra",
    "output": [],
    "previous_response_id": null,
    "reasoning_effort": null,
    "store": false,
    "temperature": 1,
    "text": {
      "format": {
        "type": "text"
      }
    },
    "tool_choice": "auto",
    "tools": [],
    "top_p": 1,
    "truncation": "disabled",
    "usage": null,
    "user": null,
    "metadata": {},
    "parallel_tool_calls": true
  },
  "sequence_number": 1
}`,
		Want: sdk.APIError{Code: "server_error", Message: "The model failed to generate a response.", Kind: sdk.KindServerError},
	}})
}

// google.rpc.Code, whose names are the status field of a google.rpc.Status:
// https://github.com/googleapis/googleapis/blob/5d2a5100759be0b6fe5a1d3ce5e025d53d8283f4/google/rpc/code.proto
func TestGoogleCanonicalCodes(t *testing.T) {
	wantKinds(t, func(status string) sdk.ErrorKind { return errorformat.GoogleKind(status, "") }, map[string]sdk.ErrorKind{
		"OK":                  sdk.KindUnknown,
		"CANCELLED":           sdk.KindUnknown,
		"UNKNOWN":             sdk.KindUnknown,
		"INVALID_ARGUMENT":    sdk.KindUnknown,
		"DEADLINE_EXCEEDED":   sdk.KindServerError,
		"NOT_FOUND":           sdk.KindUnknown,
		"ALREADY_EXISTS":      sdk.KindUnknown,
		"PERMISSION_DENIED":   sdk.KindPermissionDenied,
		"UNAUTHENTICATED":     sdk.KindAuthentication,
		"RESOURCE_EXHAUSTED":  sdk.KindRateLimited,
		"FAILED_PRECONDITION": sdk.KindUnknown,
		"ABORTED":             sdk.KindUnknown,
		"OUT_OF_RANGE":        sdk.KindUnknown,
		"UNIMPLEMENTED":       sdk.KindUnknown,
		"INTERNAL":            sdk.KindServerError,
		"UNAVAILABLE":         sdk.KindServerError,
		"DATA_LOSS":           sdk.KindUnknown,
	})
}

// google.api.ErrorReason, the ErrorInfo reasons of the googleapis.com domain:
// https://github.com/googleapis/googleapis/blob/5d2a5100759be0b6fe5a1d3ce5e025d53d8283f4/google/api/error_reason.proto
// Reasons listed as unknown leave the Kind to the canonical status.
func TestGoogleErrorReasons(t *testing.T) {
	wantKinds(t, func(reason string) sdk.ErrorKind { return errorformat.GoogleKind("", reason) }, map[string]sdk.ErrorKind{
		"ERROR_REASON_UNSPECIFIED":            sdk.KindUnknown,
		"SERVICE_DISABLED":                    sdk.KindPermissionDenied,
		"BILLING_DISABLED":                    sdk.KindQuotaExhausted,
		"API_KEY_INVALID":                     sdk.KindAuthentication,
		"API_KEY_SERVICE_BLOCKED":             sdk.KindPermissionDenied,
		"API_KEY_HTTP_REFERRER_BLOCKED":       sdk.KindPermissionDenied,
		"API_KEY_IP_ADDRESS_BLOCKED":          sdk.KindPermissionDenied,
		"API_KEY_ANDROID_APP_BLOCKED":         sdk.KindPermissionDenied,
		"API_KEY_IOS_APP_BLOCKED":             sdk.KindPermissionDenied,
		"RATE_LIMIT_EXCEEDED":                 sdk.KindRateLimited,
		"RESOURCE_QUOTA_EXCEEDED":             sdk.KindUnknown,
		"LOCATION_TAX_POLICY_VIOLATED":        sdk.KindUnknown,
		"USER_PROJECT_DENIED":                 sdk.KindPermissionDenied,
		"CONSUMER_SUSPENDED":                  sdk.KindPermissionDenied,
		"CONSUMER_INVALID":                    sdk.KindUnknown,
		"SECURITY_POLICY_VIOLATED":            sdk.KindUnknown,
		"ACCESS_TOKEN_EXPIRED":                sdk.KindAuthentication,
		"ACCESS_TOKEN_SCOPE_INSUFFICIENT":     sdk.KindPermissionDenied,
		"ACCOUNT_STATE_INVALID":               sdk.KindUnknown,
		"ACCESS_TOKEN_TYPE_UNSUPPORTED":       sdk.KindAuthentication,
		"CREDENTIALS_MISSING":                 sdk.KindAuthentication,
		"RESOURCE_PROJECT_INVALID":            sdk.KindUnknown,
		"SESSION_COOKIE_INVALID":              sdk.KindUnknown,
		"USER_BLOCKED_BY_ADMIN":               sdk.KindUnknown,
		"RESOURCE_USAGE_RESTRICTION_VIOLATED": sdk.KindUnknown,
		"SYSTEM_PARAMETER_UNSUPPORTED":        sdk.KindUnknown,
		"ORG_RESTRICTION_VIOLATION":           sdk.KindUnknown,
		"ORG_RESTRICTION_HEADER_INVALID":      sdk.KindUnknown,
		"SERVICE_NOT_VISIBLE":                 sdk.KindUnknown,
		"GCP_SUSPENDED":                       sdk.KindUnknown,
		"LOCATION_POLICY_VIOLATED":            sdk.KindUnknown,
		"MISSING_ORIGIN":                      sdk.KindUnknown,
		"OVERLOADED_CREDENTIALS":              sdk.KindUnknown,
		"LOCATION_ORG_POLICY_VIOLATED":        sdk.KindUnknown,
		"TLS_ORG_POLICY_VIOLATED":             sdk.KindUnknown,
		"EMULATOR_QUOTA_EXCEEDED":             sdk.KindUnknown,
		"CREDENTIAL_ANDROID_APP_INVALID":      sdk.KindUnknown,
		"IAM_PERMISSION_DENIED":               sdk.KindPermissionDenied,
		"JWT_TOKEN_INVALID":                   sdk.KindAuthentication,
		"CREDENTIAL_TYPE_UNSUPPORTED":         sdk.KindAuthentication,
		"ACCOUNT_TYPE_UNSUPPORTED":            sdk.KindUnknown,
		"ENDPOINT_USAGE_RESTRICTION_VIOLATED": sdk.KindUnknown,
		"TLS_CIPHER_RESTRICTION_VIOLATED":     sdk.KindUnknown,
		"MCP_SERVER_DISABLED":                 sdk.KindUnknown,
	})
}

// ApiErrorType, the metadata.error_type of an OpenRouter error, in the OpenAPI
// document of OpenRouter's generated Go SDK:
// https://github.com/OpenRouterTeam/go-sdk/blob/bc502ea31157ee4fa1f83c333478995e911dfbd1/.speakeasy/in.openapi.yaml
func TestOpenRouterErrorTypes(t *testing.T) {
	wantKinds(t, errorformat.OpenRouterKind, map[string]sdk.ErrorKind{
		"context_length_exceeded":  sdk.KindUnknown,
		"max_tokens_exceeded":      sdk.KindUnknown,
		"token_limit_exceeded":     sdk.KindUnknown,
		"string_too_long":          sdk.KindUnknown,
		"authentication":           sdk.KindAuthentication,
		"permission_denied":        sdk.KindPermissionDenied,
		"payment_required":         sdk.KindQuotaExhausted,
		"rate_limit_exceeded":      sdk.KindRateLimited,
		"provider_overloaded":      sdk.KindServerError,
		"provider_unavailable":     sdk.KindServerError,
		"invalid_request":          sdk.KindUnknown,
		"invalid_prompt":           sdk.KindUnknown,
		"not_found":                sdk.KindUnknown,
		"precondition_failed":      sdk.KindUnknown,
		"payload_too_large":        sdk.KindUnknown,
		"unprocessable":            sdk.KindUnknown,
		"content_policy_violation": sdk.KindUnknown,
		"refusal":                  sdk.KindUnknown,
		"invalid_image":            sdk.KindUnknown,
		"image_too_large":          sdk.KindUnknown,
		"image_too_small":          sdk.KindUnknown,
		"unsupported_image_format": sdk.KindUnknown,
		"image_not_found":          sdk.KindUnknown,
		"image_download_failed":    sdk.KindUnknown,
		"server":                   sdk.KindServerError,
		"timeout":                  sdk.KindUnknown,
		"unmapped":                 sdk.KindUnknown,
	})
}

// The example of every *ResponseErrorData schema in the same document, one per
// status. The examples carry no metadata, so the status decides the Kind.
func TestOpenRouterStatusExamples(t *testing.T) {
	examples := []struct {
		status  int
		message string
		kind    sdk.ErrorKind
	}{
		{400, "Invalid request parameters", sdk.KindUnknown},
		{401, "Missing Authentication header", sdk.KindAuthentication},
		{402, "Insufficient credits. Add more using https://openrouter.ai/credits", sdk.KindQuotaExhausted},
		{403, "Only management keys can perform this operation", sdk.KindPermissionDenied},
		{404, "Resource not found", sdk.KindUnknown},
		{408, "Operation timed out. Please try again later.", sdk.KindUnknown},
		{409, "Resource conflict. Please try again later.", sdk.KindUnknown},
		{410, "The Coinbase APIs used by this endpoint have been deprecated, so the Coinbase Commerce credits API has been removed. Use the web credits purchase flow instead.", sdk.KindUnknown},
		{413, "Request payload too large", sdk.KindUnknown},
		{422, "Invalid argument", sdk.KindUnknown},
		{429, "Rate limit exceeded", sdk.KindRateLimited},
		{500, "Internal Server Error", sdk.KindServerError},
		{502, "Provider returned error", sdk.KindServerError},
		{503, "Service temporarily unavailable", sdk.KindServerError},
		{504, "The operation was aborted due to timeout", sdk.KindServerError},
		{524, "Request timed out. Please try again later.", sdk.KindServerError},
		{529, "Provider returned error", sdk.KindServerError},
	}
	cases := make([]providertest.ErrorCase, 0, len(examples))
	for _, ex := range examples {
		code := strconv.Itoa(ex.status)
		cases = append(cases, providertest.ErrorCase{
			Name:   code + " " + http.StatusText(ex.status),
			Status: ex.status,
			Body:   `{"error":{"code":` + code + `,"message":"` + ex.message + `"}}`,
			Want:   sdk.APIError{Code: code, Message: ex.message, Kind: ex.kind},
		})
	}
	providertest.RunErrorCases(t, "openrouter-videos", errorformat.DecodeOpenRouter, cases)
}
