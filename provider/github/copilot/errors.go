package copilot

import (
	"cmp"
	"encoding/json"
	"strings"

	"github.com/felinics/twilight/sdk"
)

// errorFields is a Copilot API error. It arrives either wrapped as
// {"error":{...}} or as the top-level object.
type errorFields struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// decodeError fills e from a Copilot API error body. The request ID is the
// server's x-github-request-id, or the x-request-id it echoes.
func decodeError(e *sdk.APIError) {
	var body struct {
		errorFields
		Error *errorFields `json:"error"`
	}
	if json.Unmarshal(e.Body, &body) == nil {
		fields := body.errorFields
		if body.Error != nil {
			fields = *body.Error
		}
		e.Type, e.Code, e.Message = fields.Type, fields.Code, fields.Message
	}
	e.RequestID = cmp.Or(e.Header.Get("x-github-request-id"), e.Header.Get("x-request-id"))
	if k := kindFor(e.Code); k != sdk.KindUnknown {
		e.Kind = k
	}
}

// kindFor classifies a Copilot error code by its part before any ":".
func kindFor(code string) sdk.ErrorKind {
	prefix, _, _ := strings.Cut(code, ":")
	switch prefix {
	case "quota_exceeded", "free_quota_exceeded", "overage_limit_reached",
		"billing_not_configured", "additional_spend_limit_reached":
		return sdk.KindQuotaExhausted
	case "rate_limited", "user_model_rate_limited", "user_global_rate_limited",
		"integration_rate_limited", "agent_mode_limit_exceeded":
		return sdk.KindRateLimited
	case "model_overloaded":
		return sdk.KindServerError
	default:
		return sdk.KindUnknown
	}
}
