// Package errorformat decodes the error bodies that more than one provider
// package receives: OpenAI's format, which OpenAI-compatible APIs copy,
// OpenRouter's extension of it, Google's google.rpc.Status format and the
// DashScope format of Alibaba Cloud Model Studio. A format that only one
// package receives stays in that package.
package errorformat

import (
	"bytes"
	"cmp"
	"encoding/json"
	"strconv"

	"github.com/felinics/twilight/sdk"
)

// openAIBody is OpenAI's error body:
// {"error":{"message":"...","type":"...","param":null,"code":"..."}}.
// OpenAI-compatible APIs keep the envelope but may send code as a number.
type openAIBody struct {
	Error struct {
		Message string          `json:"message"`
		Type    string          `json:"type"`
		Code    json.RawMessage `json:"code"`
	} `json:"error"`
}

// ParseOpenAI copies type, code and message from an OpenAI-format body into
// e. A numeric code is copied as its decimal text.
func ParseOpenAI(e *sdk.APIError) {
	var body openAIBody
	if json.Unmarshal(e.Body, &body) != nil {
		return
	}
	e.Type = body.Error.Type
	e.Code = scalarString(body.Error.Code)
	e.Message = body.Error.Message
}

// DecodeOpenAI is the error decoder for OpenAI's API and the OpenAI-compatible
// endpoints the OpenAI-format packages are pointed at. The request ID comes
// from x-request-id, or from x-amzn-requestid on Amazon Bedrock.
func DecodeOpenAI(e *sdk.APIError) {
	ParseOpenAI(e)
	e.RequestID = cmp.Or(e.Header.Get("x-request-id"), e.Header.Get("x-amzn-requestid"))
	if k := OpenAIKind(e.Type, e.Code); k != sdk.KindUnknown {
		e.Kind = k
	}
}

// OpenAIKind classifies OpenAI's error type and code. It returns KindUnknown
// for values it has no mapping for, including every value an
// OpenAI-compatible API invents.
func OpenAIKind(typ, code string) sdk.ErrorKind {
	switch code {
	case "invalid_api_key":
		return sdk.KindAuthentication
	case "insufficient_quota",
		"credit_balance_exhausted",
		"organization_spend_limit_exceeded",
		"project_spend_limit_exceeded",
		"organization_usage_limit_exceeded":
		return sdk.KindQuotaExhausted
	case "rate_limit_exceeded", "slow_down":
		return sdk.KindRateLimited
	case "server_error", "server_is_overloaded":
		return sdk.KindServerError
	}
	switch typ {
	case "insufficient_quota":
		return sdk.KindQuotaExhausted
	case "server_error":
		return sdk.KindServerError
	}
	return sdk.KindUnknown
}

// googleBody is google.rpc.Status as the JSON APIs send it:
// {"error":{"code":429,"message":"...","status":"RESOURCE_EXHAUSTED","details":[...]}}.
type googleBody struct {
	Error struct {
		Message string `json:"message"`
		Status  string `json:"status"`
		Details []struct {
			Type   string `json:"@type"`
			Reason string `json:"reason"`
		} `json:"details"`
	} `json:"error"`
}

const googleErrorInfo = "type.googleapis.com/google.rpc.ErrorInfo"

// DecodeGoogle is the error decoder for Google APIs, which report errors as a
// google.rpc.Status. Type is the canonical status (error.status) and Code is
// the reason of the google.rpc.ErrorInfo detail, when there is one. The APIs
// document no request ID header, so RequestID stays empty.
func DecodeGoogle(e *sdk.APIError) {
	var body googleBody
	if json.Unmarshal(e.Body, &body) != nil {
		return
	}
	e.Type = body.Error.Status
	e.Message = body.Error.Message
	for _, d := range body.Error.Details {
		if d.Type == googleErrorInfo && d.Reason != "" {
			e.Code = d.Reason
			break
		}
	}
	if k := GoogleKind(e.Type, e.Code); k != sdk.KindUnknown {
		e.Kind = k
	}
}

// GoogleKind classifies a google.rpc.Status by its ErrorInfo reason and, when
// the reason has no mapping, by its canonical status.
func GoogleKind(status, reason string) sdk.ErrorKind {
	switch reason {
	case "API_KEY_INVALID", "API_KEY_MISSING", "CREDENTIALS_MISSING",
		"ACCESS_TOKEN_EXPIRED", "ACCESS_TOKEN_TYPE_UNSUPPORTED":
		return sdk.KindAuthentication
	case "API_KEY_SERVICE_BLOCKED", "API_KEY_HTTP_REFERRER_BLOCKED", "API_KEY_IP_ADDRESS_BLOCKED",
		"ACCESS_TOKEN_SCOPE_INSUFFICIENT", "CONSUMER_SUSPENDED", "SERVICE_DISABLED", "USER_PROJECT_DENIED":
		return sdk.KindPermissionDenied
	case "BILLING_DISABLED":
		return sdk.KindQuotaExhausted
	case "RATE_LIMIT_EXCEEDED", "QUOTA_EXCEEDED", "RESOURCE_EXHAUSTED":
		return sdk.KindRateLimited
	}
	switch status {
	case "UNAUTHENTICATED":
		return sdk.KindAuthentication
	case "PERMISSION_DENIED":
		return sdk.KindPermissionDenied
	case "RESOURCE_EXHAUSTED":
		return sdk.KindRateLimited
	case "INTERNAL", "UNAVAILABLE", "DEADLINE_EXCEEDED":
		return sdk.KindServerError
	}
	return sdk.KindUnknown
}

// scalarString returns a JSON string's value or a JSON number's text, and ""
// for anything else.
func scalarString(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	switch raw[0] {
	case '"':
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		if _, err := strconv.ParseFloat(string(raw), 64); err == nil {
			return string(raw)
		}
	}
	return ""
}

// openRouterBody is the part of OpenRouter's error body that OpenAI's format
// lacks: {"error":{"code":402,"message":"...","metadata":{"error_type":"..."}}}.
type openRouterBody struct {
	Error struct {
		Metadata struct {
			ErrorType string `json:"error_type"`
		} `json:"metadata"`
	} `json:"error"`
}

// DecodeOpenRouter is the error decoder for OpenRouter
// (https://openrouter.ai/docs/api-reference/errors). Code is the numeric code
// as text and Type is metadata.error_type, which OpenRouter documents as the
// field to classify by. No request ID header is documented, so RequestID
// stays empty.
func DecodeOpenRouter(e *sdk.APIError) {
	ParseOpenAI(e)
	var body openRouterBody
	if json.Unmarshal(e.Body, &body) == nil && body.Error.Metadata.ErrorType != "" {
		e.Type = body.Error.Metadata.ErrorType
	}
	if k := OpenRouterKind(e.Type); k != sdk.KindUnknown {
		e.Kind = k
	}
}

// OpenRouterKind classifies an OpenRouter metadata.error_type.
func OpenRouterKind(errorType string) sdk.ErrorKind {
	switch errorType {
	case "authentication":
		return sdk.KindAuthentication
	case "permission_denied":
		return sdk.KindPermissionDenied
	case "payment_required":
		return sdk.KindQuotaExhausted
	case "rate_limit_exceeded":
		return sdk.KindRateLimited
	case "provider_overloaded", "provider_unavailable", "server":
		return sdk.KindServerError
	default:
		return sdk.KindUnknown
	}
}

// dashScopeBody is DashScope's error body:
// {"request_id":"...","code":"InvalidApiKey","message":"..."}.
type dashScopeBody struct {
	RequestID string `json:"request_id"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// DecodeDashScope is the error decoder for Alibaba Cloud Model Studio's
// DashScope API (https://help.aliyun.com/zh/model-studio/error-code). The
// request ID comes from the body.
func DecodeDashScope(e *sdk.APIError) {
	var body dashScopeBody
	if json.Unmarshal(e.Body, &body) != nil {
		return
	}
	e.Code = body.Code
	e.Message = body.Message
	e.RequestID = body.RequestID
	if k := DashScopeKind(e.Code); k != sdk.KindUnknown {
		e.Kind = k
	}
}

// DashScopeKind classifies a DashScope error code. Arrearage and the free-tier
// stop arrive as 400 and 403, so the status alone would misfile them.
func DashScopeKind(code string) sdk.ErrorKind {
	switch code {
	case "InvalidApiKey":
		return sdk.KindAuthentication
	case "Arrearage", "AllocationQuota.FreeTierOnly":
		return sdk.KindQuotaExhausted
	case "Throttling", "Throttling.RateQuota":
		return sdk.KindRateLimited
	default:
		return sdk.KindUnknown
	}
}
