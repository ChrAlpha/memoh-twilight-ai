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

// The Edge read-aloud service is undocumented. A rejected handshake (it
// answers 403 when the Sec-MS-GEC token is stale) carries no decodable body,
// so the APIError comes from the status alone.
func TestHandshakeHTTPErrorIsAPIError(t *testing.T) {
	const body = "Forbidden"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	p := New(WithBaseURL("ws" + strings.TrimPrefix(srv.URL, "http") + "/edge/v1"))

	check := func(t *testing.T, err error) {
		t.Helper()
		apiErr := providertest.WantAPIError(t, err, providerName, http.StatusForbidden, sdk.KindPermissionDenied)
		if string(apiErr.Body) != body {
			t.Errorf("Body = %q, want %q", apiErr.Body, body)
		}
		if strings.Contains(err.Error(), body) {
			t.Errorf("error text %q leaks the body", err)
		}
	}
	t.Run("DoSynthesize", func(t *testing.T) {
		_, err := p.DoSynthesize(context.Background(), sdk.SpeechParams{Text: "x"})
		check(t, err)
	})
	t.Run("DoStream", func(t *testing.T) {
		result, err := p.DoStream(context.Background(), sdk.SpeechParams{Text: "x"})
		if err == nil {
			for range result.Stream {
				t.Error("unexpected audio chunk")
			}
			err = result.Err()
		}
		check(t, err)
	})
}
