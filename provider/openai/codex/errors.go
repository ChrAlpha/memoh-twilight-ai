package codex

import (
	"cmp"

	"github.com/felinics/twilight/internal/errorformat"
	"github.com/felinics/twilight/sdk"
)

// decodeError fills e from the Codex backend's error body, which uses
// OpenAI's envelope. The request ID headers are the ones the Codex CLI reads.
func decodeError(e *sdk.APIError) {
	errorformat.ParseOpenAI(e)
	e.RequestID = cmp.Or(e.Header.Get("x-request-id"), e.Header.Get("x-oai-request-id"))
	if k := kindFor(e.Type, e.Code); k != sdk.KindUnknown {
		e.Kind = k
	}
}

// kindFor classifies a Codex error type and code: the ChatGPT plan limits
// first, then OpenAI's own codes.
func kindFor(typ, code string) sdk.ErrorKind {
	switch typ {
	case "usage_limit_reached":
		return sdk.KindQuotaExhausted
	case "usage_not_included":
		return sdk.KindPermissionDenied
	}
	return errorformat.OpenAIKind(typ, code)
}
