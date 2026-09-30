package speech

import (
	"cmp"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/felinics/twilight/sdk"
)

// providerName identifies this package in APIError.Provider.
const providerName = "minimax-speech"

// errorBody is the part of a MiniMax response that reports failure:
// {"trace_id":"...","base_resp":{"status_code":1004,"status_msg":"..."}}.
// status_code 0 is success. The shape is documented with the T2A response at
// https://platform.minimax.io/docs/api-reference/speech-t2a-http and the codes
// at https://platform.minimax.io/docs/api-reference/errorcode. trace_id is the
// ID MiniMax asks for in support. Error bodies often omit it; the Trace-Id
// response header carries the same value.
type errorBody struct {
	TraceID  string `json:"trace_id"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

// decodeError fills e from a MiniMax body. Code is status_code as decimal
// text and RequestID is trace_id, or the Trace-Id header without one.
func decodeError(e *sdk.APIError) {
	e.RequestID = e.Header.Get("Trace-Id")
	var body errorBody
	if json.Unmarshal(e.Body, &body) != nil {
		return
	}
	e.RequestID = cmp.Or(body.TraceID, e.RequestID)
	if body.BaseResp.StatusCode == 0 {
		return
	}
	e.Code = strconv.Itoa(body.BaseResp.StatusCode)
	e.Message = body.BaseResp.StatusMsg
	if k := kindFor(body.BaseResp.StatusCode); k != sdk.KindUnknown {
		e.Kind = k
	}
}

// bodyError returns the APIError for a 200 response whose base_resp reports
// failure. StatusCode is 0 because the HTTP exchange itself succeeded.
func bodyError(header http.Header, raw []byte) *sdk.APIError {
	e := &sdk.APIError{Provider: providerName, Kind: sdk.KindUnknown, Header: header, Body: raw}
	decodeError(e)
	return e
}

// kindFor classifies a MiniMax status_code. 2056 is a usage cap that resets
// in the next five-hour window, so it is quota rather than rate.
func kindFor(code int) sdk.ErrorKind {
	switch code {
	case 1004, 2049:
		return sdk.KindAuthentication
	case 2042:
		return sdk.KindPermissionDenied
	case 1008, 2056:
		return sdk.KindQuotaExhausted
	case 1002, 1039, 1041, 2045:
		return sdk.KindRateLimited
	case 1000, 1001, 1024, 1033:
		return sdk.KindServerError
	default:
		return sdk.KindUnknown
	}
}
