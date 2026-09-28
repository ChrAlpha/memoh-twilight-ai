package utils

import (
	"io"
	"net/http"

	"github.com/felinics/twilight/sdk"
)

// ErrorDecoder fills Type, Code, Message and RequestID of e from e.Body and
// e.Header, using the provider's own error format. It sets Kind only when the
// provider's type or code identifies one; otherwise it leaves the Kind derived
// from the HTTP status in place. Each provider package supplies one.
type ErrorDecoder func(e *sdk.APIError)

// NewHTTPError reads resp.Body and builds the APIError for a non-2xx
// response. The caller still closes resp.Body. Kind starts from the HTTP
// status and decode, when non-nil, refines it from the provider's own type or
// code.
func NewHTTPError(provider string, resp *http.Response, decode ErrorDecoder) *sdk.APIError {
	body, _ := io.ReadAll(resp.Body)
	e := &sdk.APIError{
		Provider:   provider,
		StatusCode: resp.StatusCode,
		Kind:       kindFromStatus(resp.StatusCode),
		Header:     resp.Header,
		Body:       body,
	}
	if decode != nil {
		decode(e)
	}
	return e
}

// kindFromStatus is the classification used when the provider's type or code
// does not identify one.
func kindFromStatus(status int) sdk.ErrorKind {
	switch {
	case status == http.StatusUnauthorized:
		return sdk.KindAuthentication
	case status == http.StatusPaymentRequired:
		return sdk.KindQuotaExhausted
	case status == http.StatusForbidden:
		return sdk.KindPermissionDenied
	case status == http.StatusTooManyRequests:
		return sdk.KindRateLimited
	case status >= 500 && status <= 599:
		return sdk.KindServerError
	default:
		return sdk.KindUnknown
	}
}
