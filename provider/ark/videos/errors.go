package videos

import (
	"github.com/felinics/twilight/internal/errorformat"
	"github.com/felinics/twilight/sdk"
)

// providerName identifies this package in APIError.Provider.
const providerName = "ark-videos"

// decodeError fills e from an Ark error body, which uses OpenAI's envelope:
// {"error":{"code":"AuthenticationError","message":"...","param":"","type":"Unauthorized"}}.
func decodeError(e *sdk.APIError) {
	errorformat.ParseOpenAI(e)
	if k := kindFor(e.Code); k != sdk.KindUnknown {
		e.Kind = k
	}
}

// kindFor classifies an Ark error code. Overdue accounts arrive as 403 and
// server overload as 429, so the status alone would misfile them.
func kindFor(code string) sdk.ErrorKind {
	switch code {
	case "AuthenticationError":
		return sdk.KindAuthentication
	case "AccessDenied":
		return sdk.KindPermissionDenied
	case "AccountOverdueError", "OperationDenied.ServiceOverdue", "SetLimitExceeded",
		"QuotaExceeded.DoubaoSearchFreeQuotaExceeded", "QuotaExceeded.AgentPlanQuotaExceeded":
		return sdk.KindQuotaExhausted
	case "ServerOverloaded":
		return sdk.KindServerError
	default:
		return sdk.KindUnknown
	}
}
