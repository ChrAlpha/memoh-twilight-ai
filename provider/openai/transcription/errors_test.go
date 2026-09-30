package transcription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
)

// The body is verbatim from the official .NET SDK's recording of a 401:
// https://github.com/openai/openai-dotnet/blob/4e5ae90621089e0b1f5571546a75cea6b0cc713f/tests/SessionRecords/ChatTests/AuthFailure.json
// The request ID header is documented at
// https://platform.openai.com/docs/api-reference/debugging-requests.
const authBody = `{"error":{"message":"Incorrect API key provided: not-a-re**************************ized. You can find your API key at https://platform.openai.com/account/api-keys.","type":"invalid_request_error","param":null,"code":"invalid_api_key"}}`

const secretKey = "sk-secret-7f3a"

func TestHTTPErrorIsAPIError(t *testing.T) {
	calls := map[string]func(*Provider) error{
		"ListModels": func(p *Provider) error {
			_, err := p.ListModels(context.Background())
			return err
		},
		"DoTranscribe": func(p *Provider) error {
			_, err := p.DoTranscribe(context.Background(), sdk.TranscriptionParams{Audio: []byte("a"), Filename: "a.wav"})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("x-request-id", "req_stt")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(authBody))
			}))
			defer srv.Close()
			p := New(WithAPIKey(secretKey), WithBaseURL(srv.URL))
			err := call(p)
			apiErr := providertest.WantAPIError(t, err, providerName, http.StatusUnauthorized, sdk.KindAuthentication)
			if apiErr.Type != "invalid_request_error" || apiErr.Code != "invalid_api_key" || apiErr.RequestID != "req_stt" {
				t.Errorf("Type, Code, RequestID = %q, %q, %q", apiErr.Type, apiErr.Code, apiErr.RequestID)
			}
			if !strings.HasPrefix(apiErr.Message, "Incorrect API key provided") {
				t.Errorf("Message = %q", apiErr.Message)
			}
			if text := err.Error(); strings.Contains(text, secretKey) || strings.Contains(text, authBody) {
				t.Errorf("error text %q leaks the key or the body", text)
			}
		})
	}
}
