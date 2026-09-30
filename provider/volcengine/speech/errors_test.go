package speech

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
)

// Status codes and texts are from https://www.volcengine.com/docs/6489/72012;
// the envelope is the invoke response of https://www.volcengine.com/docs/6489/71999.
const (
	illegalTokenBody = `{"task_id":"00000000-0000-0000-0000-000000000000","status_code":40200002,"status_text":"DeniedAccess:IllegalToken"}`
	qpsBody          = `{"task_id":"1f2e3d4c-0000-0000-0000-000000000000","namespace":"TTS","status_code":40200011,"status_text":"ExceededQPSQuota"}`
	secretToken      = "sami-token-7f3a"
	secretKey        = "volc-secret-7f3a"
)

// lackPolicyBody is the SAMI FAQ's LackPolicy error
// (https://www.volcengine.com/docs/6489/77815) in the OpenAPI common error
// envelope (https://www.volcengine.com/docs/6369/80336).
const lackPolicyBody = `{"ResponseMetadata":{"RequestId":"20230604110420B2B3C4D5E6F7A8B9C0","Action":"GetToken","Version":"2021-07-27","Service":"sami","Region":"cn-north-1","Error":{"CodeN":100012,"Code":"LackPolicy","Message":"Request was rejected because of lack of policy."}}}`

func TestDecodeInvokeError(t *testing.T) {
	providertest.RunErrorCases(t, providerName, decodeInvokeError, []providertest.ErrorCase{
		{Name: "illegal token", Status: http.StatusUnauthorized, Body: illegalTokenBody,
			Want: sdk.APIError{StatusCode: http.StatusUnauthorized, Code: "40200002", Message: "DeniedAccess:IllegalToken",
				RequestID: "00000000-0000-0000-0000-000000000000", Kind: sdk.KindAuthentication}},
		{Name: "malformed request json is not auth", Status: http.StatusBadRequest,
			Body: `{"task_id":"t1","status_code":40200002,"status_text":"DeniedAccess:json: cannot unmarshal object"}`,
			Want: sdk.APIError{StatusCode: http.StatusBadRequest, Code: "40200002", Message: "DeniedAccess:json: cannot unmarshal object",
				RequestID: "t1", Kind: sdk.KindUnknown}},
		{Name: "trial quota", Status: http.StatusBadRequest,
			Body: `{"task_id":"t2","status_code":40200010,"status_text":"ExceededQuota"}`,
			Want: sdk.APIError{StatusCode: http.StatusBadRequest, Code: "40200010", Message: "ExceededQuota",
				RequestID: "t2", Kind: sdk.KindQuotaExhausted}},
		{Name: "gateway timeout html uses logid", Status: http.StatusGatewayTimeout,
			Header: http.Header{"X-Tt-Logid": {"20230604110420logid"}},
			Body:   "<html>\n<head><title>504 Gateway Time-out</title></head>\n</html>",
			Want:   sdk.APIError{StatusCode: http.StatusGatewayTimeout, RequestID: "20230604110420logid", Kind: sdk.KindServerError}},
	})
}

func TestDecodeTokenError(t *testing.T) {
	providertest.RunErrorCases(t, providerName, decodeTokenError, []providertest.ErrorCase{
		{Name: "openapi envelope", Status: http.StatusForbidden, Body: lackPolicyBody,
			Want: sdk.APIError{StatusCode: http.StatusForbidden, Code: "LackPolicy",
				Message: "Request was rejected because of lack of policy.", RequestID: "20230604110420B2B3C4D5E6F7A8B9C0",
				Kind: sdk.KindPermissionDenied}},
		{Name: "signature", Status: http.StatusUnauthorized,
			Body: `{"ResponseMetadata":{"RequestId":"r1","Error":{"CodeN":100010,"Code":"SignatureDoesNotMatch","Message":"The request signature we calculated does not match the signature you provided."}}}`,
			Want: sdk.APIError{StatusCode: http.StatusUnauthorized, Code: "SignatureDoesNotMatch",
				Message: "The request signature we calculated does not match the signature you provided.", RequestID: "r1",
				Kind: sdk.KindAuthentication}},
		// The FAQ does not give the HTTP status of this body.
		{Name: "faq code msg", Status: http.StatusBadRequest, Body: `{"code":420,"msg":"generate token failed"}`,
			Want: sdk.APIError{StatusCode: http.StatusBadRequest, Code: "420", Message: "generate token failed", Kind: sdk.KindUnknown}},
	})
}

func TestInvokeHTTPErrorIsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(illegalTokenBody))
	}))
	defer srv.Close()

	p := New(WithBaseURL(srv.URL), WithToken(secretToken), WithAppKey("app"))
	_, err := p.DoSynthesize(context.Background(), sdk.SpeechParams{Text: "hi"})
	apiErr := providertest.WantAPIError(t, err, providerName, http.StatusUnauthorized, sdk.KindAuthentication)
	if apiErr.Code != "40200002" || apiErr.RequestID != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("Code, RequestID = %q, %q", apiErr.Code, apiErr.RequestID)
	}
	assertNoLeak(t, err, secretToken, illegalTokenBody)
}

// A transport failure must not print the invoke URL, whose query carries the
// token and app key.
func TestInvokeTransportErrorHidesToken(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()

	p := New(WithBaseURL(srv.URL), WithToken(secretToken), WithAppKey("app-secret-key"))
	_, err := p.DoSynthesize(context.Background(), sdk.SpeechParams{Text: "hi"})
	if err == nil {
		t.Fatal("DoSynthesize succeeded against a closed server")
	}
	if text := err.Error(); strings.Contains(text, secretToken) || strings.Contains(text, "app-secret-key") {
		t.Errorf("error text %q leaks the token or app key", text)
	}
}

// SAMI can report a failure in the body of a 200 response.
func TestInvokeBodyErrorIsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(qpsBody))
	}))
	defer srv.Close()

	p := New(WithBaseURL(srv.URL), WithToken(secretToken), WithAppKey("app"))
	calls := map[string]func() error{
		"DoSynthesize": func() error {
			_, err := p.DoSynthesize(context.Background(), sdk.SpeechParams{Text: "hi"})
			return err
		},
		"DoStream": func() error {
			_, err := p.DoStream(context.Background(), sdk.SpeechParams{Text: "hi"})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call()
			apiErr := providertest.WantAPIError(t, err, providerName, 0, sdk.KindRateLimited)
			if apiErr.Code != "40200011" || apiErr.Message != "ExceededQPSQuota" || apiErr.RequestID != "1f2e3d4c-0000-0000-0000-000000000000" {
				t.Errorf("Code, Message, RequestID = %q, %q, %q", apiErr.Code, apiErr.Message, apiErr.RequestID)
			}
			assertNoLeak(t, err, secretToken, qpsBody)
		})
	}
}

// redirect sends every request to srv, standing in for open.volcengineapi.com.
type redirect struct{ target *url.URL }

func (r redirect) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = r.target.Scheme
	req.URL.Host = r.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

func tokenProvider(t *testing.T, status int, body string) *Provider {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return New(WithAccessKey("AKLT-test"), WithSecretKey(secretKey), WithAppKey("app"),
		WithHTTPClient(&http.Client{Transport: redirect{target}}))
}

func TestTokenHTTPErrorIsAPIError(t *testing.T) {
	p := tokenProvider(t, http.StatusForbidden, lackPolicyBody)
	_, err := p.DoSynthesize(context.Background(), sdk.SpeechParams{Text: "hi"})
	apiErr := providertest.WantAPIError(t, err, providerName, http.StatusForbidden, sdk.KindPermissionDenied)
	if apiErr.Code != "LackPolicy" || apiErr.RequestID != "20230604110420B2B3C4D5E6F7A8B9C0" {
		t.Errorf("Code, RequestID = %q, %q", apiErr.Code, apiErr.RequestID)
	}
	assertNoLeak(t, err, secretKey, lackPolicyBody)
}

func TestTokenBodyErrorIsAPIError(t *testing.T) {
	const body = `{"status_code":40200001,"status_text":"InvalidAccess","task_id":"t3"}`
	p := tokenProvider(t, http.StatusOK, body)
	_, err := p.DoSynthesize(context.Background(), sdk.SpeechParams{Text: "hi"})
	apiErr := providertest.WantAPIError(t, err, providerName, 0, sdk.KindAuthentication)
	if apiErr.Code != "40200001" || apiErr.Message != "InvalidAccess" || apiErr.RequestID != "t3" {
		t.Errorf("Code, Message, RequestID = %q, %q, %q", apiErr.Code, apiErr.Message, apiErr.RequestID)
	}
	assertNoLeak(t, err, secretKey, body)
}

func assertNoLeak(t *testing.T, err error, secret, body string) {
	t.Helper()
	if text := err.Error(); strings.Contains(text, secret) || strings.Contains(text, body) {
		t.Errorf("error text %q leaks the credential or the body", text)
	}
}
