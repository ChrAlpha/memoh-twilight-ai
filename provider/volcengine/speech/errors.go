package speech

import (
	"cmp"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/felinics/twilight/sdk"
)

// providerName identifies this package in APIError.Provider.
const providerName = "volcengine-speech"

// statusOK is the SAMI status_code of a successful request.
const statusOK = 20000000

// samiStatus is the status part of a SAMI response. Every response, failed
// or not, is {"task_id":"...","status_code":40200002,"status_text":"...",...};
// see https://www.volcengine.com/docs/6489/71999 for the invoke response and
// https://www.volcengine.com/docs/6489/72012 for the status codes. task_id is
// the ID SAMI asks for in support tickets.
type samiStatus struct {
	TaskID     string `json:"task_id"`
	StatusCode int    `json:"status_code"`
	StatusText string `json:"status_text"`
}

// failed reports whether s carries a SAMI error code.
func (s samiStatus) failed() bool {
	return s.StatusCode != 0 && s.StatusCode != statusOK
}

// fill copies a failing status into e. RequestID falls back to the
// X-Tt-Logid response header, which SAMI documents as the trace ID in
// https://www.volcengine.com/docs/6489/71997.
func (s samiStatus) fill(e *sdk.APIError) {
	e.RequestID = cmp.Or(s.TaskID, e.Header.Get("X-Tt-Logid"))
	if !s.failed() {
		return
	}
	e.Code = strconv.Itoa(s.StatusCode)
	e.Message = s.StatusText
	if k := samiKind(s.StatusCode, s.StatusText); k != sdk.KindUnknown {
		e.Kind = k
	}
}

// decodeInvokeError fills e from a SAMI invoke error body. A body that is
// not JSON, such as the gateway's 504 page, still gets the X-Tt-Logid
// request ID.
func decodeInvokeError(e *sdk.APIError) {
	var s samiStatus
	_ = json.Unmarshal(e.Body, &s)
	s.fill(e)
}

// tokenErrorBody holds the three error shapes GetToken is documented to
// return. The OpenAPI gateway answers signing and policy failures with the
// common envelope {"ResponseMetadata":{"RequestId":"...","Error":{"Code":"...","Message":"..."}}}
// (https://www.volcengine.com/docs/6369/80336). The SAMI FAQ
// (https://www.volcengine.com/docs/6489/77815) quotes {"code":420,"msg":"..."}
// for an unknown appkey and {"code":400,"msg":"..."} for bad parameters. A
// token response that fails in SAMI itself carries status_code and
// status_text (https://www.volcengine.com/docs/6489/71995).
type tokenErrorBody struct {
	ResponseMetadata struct {
		RequestID string `json:"RequestId"`
		Error     struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
	} `json:"ResponseMetadata"`
	Code json.Number `json:"code"`
	Msg  string      `json:"msg"`
	samiStatus
}

// decodeTokenError fills e from a GetToken error body.
func decodeTokenError(e *sdk.APIError) {
	var body tokenErrorBody
	if json.Unmarshal(e.Body, &body) != nil {
		return
	}
	meta := body.ResponseMetadata
	switch {
	case meta.Error.Code != "":
		e.Code = meta.Error.Code
		e.Message = meta.Error.Message
		e.RequestID = meta.RequestID
		if k := openAPIKind(meta.Error.Code); k != sdk.KindUnknown {
			e.Kind = k
		}
	case body.Code != "":
		e.Code = body.Code.String()
		e.Message = body.Msg
	default:
		body.fill(e)
	}
}

// bodyError returns the APIError for a 200 response whose body reports
// failure. StatusCode is 0 because the HTTP exchange itself succeeded.
func bodyError(header http.Header, raw []byte, decode func(*sdk.APIError)) *sdk.APIError {
	e := &sdk.APIError{Provider: providerName, Kind: sdk.KindUnknown, Header: header, Body: raw}
	decode(e)
	return e
}

// samiKind classifies a SAMI status_code. 40200002 is documented both for a
// rejected token and, with a "DeniedAccess:json" status_text, for a request
// body SAMI cannot parse; only the former is an authentication failure.
// 40200014 covers a service that is not enabled or was suspended for arrears.
func samiKind(code int, text string) sdk.ErrorKind {
	switch code {
	case 40200001:
		return sdk.KindAuthentication
	case 40200002:
		if strings.HasPrefix(text, "DeniedAccess:json") {
			return sdk.KindUnknown
		}
		return sdk.KindAuthentication
	case 40200004, 40200014:
		return sdk.KindPermissionDenied
	case 40200010:
		return sdk.KindQuotaExhausted
	case 40200011, 40200012:
		return sdk.KindRateLimited
	case 50000000, 50000001, 50000002, 50000011:
		return sdk.KindServerError
	default:
		return sdk.KindUnknown
	}
}

// openAPIKind classifies a Volcengine OpenAPI common error code, listed at
// https://www.volcengine.com/docs/6369/68677. LackPolicy is the code the SAMI
// FAQ gives for an account without the SAMI access policy.
func openAPIKind(code string) sdk.ErrorKind {
	switch code {
	case "InvalidAccessKey", "SignatureDoesNotMatch", "InvalidSecretToken",
		"InvalidCredential", "InvalidAuthorization":
		return sdk.KindAuthentication
	case "AccessDenied", "LackPolicy":
		return sdk.KindPermissionDenied
	case "FlowLimitExceeded":
		return sdk.KindRateLimited
	case "InternalError", "InternalServiceError", "InternalServiceTimeout", "ServiceUnavailableTemp":
		return sdk.KindServerError
	default:
		return sdk.KindUnknown
	}
}
