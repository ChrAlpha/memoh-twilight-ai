package codex

import (
	"cmp"

	"github.com/felinics/twilight/internal/errorformat"
	"github.com/felinics/twilight/sdk"
)

// decodeError fills e from the Codex backend's error body, which uses
// OpenAI's envelope.
func decodeError(e *sdk.APIError) {
	errorformat.ParseOpenAI(e)
	classify(e)
}

// decodeErrorEvent fills e from a stream's "error" event, in either of the
// shapes errorformat.ParseOpenAIErrorEvent accepts.
func decodeErrorEvent(e *sdk.APIError) {
	errorformat.ParseOpenAIErrorEvent(e)
	classify(e)
}

// decodeFailedEvent fills e from a stream's "response.failed" event, whose
// response carries the error object the Codex CLI reads
// (https://github.com/openai/codex/blob/1b1835f751ebdc0cfc50b3fe55d4571dbb294563/codex-rs/codex-api/src/sse/responses_error.rs).
func decodeFailedEvent(e *sdk.APIError) {
	errorformat.ParseOpenAIFailedEvent(e)
	classify(e)
}

// classify sets the request ID and the Kind of an APIError whose type and code
// are parsed. The request ID headers are the ones the Codex CLI reads.
func classify(e *sdk.APIError) {
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
