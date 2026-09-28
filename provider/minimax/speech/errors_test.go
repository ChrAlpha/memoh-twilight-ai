package speech

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
)

// The envelope and trace_id are those of the T2A response example at
// https://platform.minimax.io/docs/api-reference/speech-t2a-http; the code and
// message are from https://platform.minimax.io/docs/api-reference/errorcode.
const (
	invalidKeyBody = `{"trace_id":"01b8bf9bb7433cc75c18eee6cfa8fe21","base_resp":{"status_code":2049,"status_msg":"invalid API Key"}}`
	balanceBody    = `{"data":null,"trace_id":"04ece790375f3ca2edbb44e8c4c200bf","base_resp":{"status_code":1008,"status_msg":"insufficient balance"}}`
	secretKey      = "minimax-secret-7f3a"
)

func TestDecodeError(t *testing.T) {
	providertest.RunErrorCases(t, providerName, decodeError, []providertest.ErrorCase{
		{
			Name:   "invalid key",
			Status: http.StatusUnauthorized,
			Body:   invalidKeyBody,
			Want: sdk.APIError{
				StatusCode: http.StatusUnauthorized,
				Code:       "2049",
				Message:    "invalid API Key",
				RequestID:  "01b8bf9bb7433cc75c18eee6cfa8fe21",
				Kind:       sdk.KindAuthentication,
			},
		},
		{
			Name:   "usage window",
			Status: http.StatusTooManyRequests,
			Body:   `{"trace_id":"t1","base_resp":{"status_code":2056,"status_msg":"usage limit exceeded"}}`,
			Want: sdk.APIError{
				StatusCode: http.StatusTooManyRequests,
				Code:       "2056",
				Message:    "usage limit exceeded",
				RequestID:  "t1",
				Kind:       sdk.KindQuotaExhausted,
			},
		},
		{
			Name:   "not json",
			Status: http.StatusBadGateway,
			Body:   "<html>502 Bad Gateway</html>",
			Want:   sdk.APIError{StatusCode: http.StatusBadGateway, Kind: sdk.KindServerError},
		},
	})
}

func TestHTTPErrorIsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(invalidKeyBody))
	}))
	defer srv.Close()

	p := New(WithAPIKey(secretKey), WithBaseURL(srv.URL))
	_, err := p.DoSynthesize(context.Background(), sdk.SpeechParams{Text: "hi"})
	apiErr := providertest.WantAPIError(t, err, providerName, http.StatusUnauthorized, sdk.KindAuthentication)
	if apiErr.Code != "2049" || apiErr.RequestID != "01b8bf9bb7433cc75c18eee6cfa8fe21" {
		t.Errorf("Code, RequestID = %q, %q", apiErr.Code, apiErr.RequestID)
	}
	assertNoLeak(t, err, invalidKeyBody)
}

// MiniMax reports most failures in base_resp of a 200 response.
func TestBodyErrorIsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(balanceBody))
	}))
	defer srv.Close()

	p := New(WithAPIKey(secretKey), WithBaseURL(srv.URL))
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
			apiErr := providertest.WantAPIError(t, err, providerName, 0, sdk.KindQuotaExhausted)
			if apiErr.Code != "1008" || apiErr.Message != "insufficient balance" || apiErr.RequestID != "04ece790375f3ca2edbb44e8c4c200bf" {
				t.Errorf("Code, Message, RequestID = %q, %q, %q", apiErr.Code, apiErr.Message, apiErr.RequestID)
			}
			assertNoLeak(t, err, balanceBody)
		})
	}
}

func assertNoLeak(t *testing.T, err error, body string) {
	t.Helper()
	if text := err.Error(); strings.Contains(text, secretKey) || strings.Contains(text, body) {
		t.Errorf("error text %q leaks the key or the body", text)
	}
}
