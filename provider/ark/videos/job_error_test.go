package videos

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/felinics/twilight/sdk"
)

func TestGenerateVideoFailedTaskIsAPIError(t *testing.T) {
	for _, c := range []struct {
		code string
		kind sdk.ErrorKind
	}{
		{"OutputVideoSensitiveContentDetected", sdk.KindUnknown},
		{"SetLimitExceeded", sdk.KindQuotaExhausted},
	} {
		t.Run(c.code, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					_ = json.NewEncoder(w).Encode(map[string]any{"id": "task-1"})
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id":     "task-1",
					"status": "failed",
					"error":  map[string]any{"code": c.code, "message": "task failed"},
				})
			}))
			defer server.Close()

			prov := New(WithAPIKey("ark-key"), WithBaseURL(server.URL))
			result, err := sdk.GenerateVideo(context.Background(),
				sdk.WithVideoModel(prov.VideoModel("doubao-seedance-2-0-260128")),
				sdk.WithVideoPrompt("city at night"),
				sdk.WithVideoPollInterval(time.Millisecond),
				sdk.WithVideoPollTimeout(time.Second),
			)
			var apiErr *sdk.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("GenerateVideo error = %v (%T), want *sdk.APIError", err, err)
			}
			if apiErr.Provider != providerName || apiErr.StatusCode != 0 || apiErr.Code != c.code ||
				apiErr.Message != "task failed" || apiErr.Kind != c.kind {
				t.Fatalf("APIError = %+v, want provider %s, status 0, code %s, kind %s", apiErr, providerName, c.code, c.kind)
			}
			if result == nil || result.Job.Error == nil || result.Job.Error.Code != c.code {
				t.Fatalf("result job error = %+v, want code %s", result, c.code)
			}
		})
	}
}

func TestDoDownloadNon2xxIsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<Error><Code>AccessDenied</Code></Error>"))
	}))
	defer server.Close()

	prov := New(WithAPIKey("ark-key"), WithBaseURL(server.URL))
	_, _, err := prov.DoDownload(context.Background(), nil, sdk.VideoOutput{URL: server.URL + "/out.mp4"})
	var apiErr *sdk.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("DoDownload error = %v (%T), want *sdk.APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusForbidden || apiErr.Provider != providerName || apiErr.Kind != sdk.KindPermissionDenied {
		t.Fatalf("APIError = %+v, want 403 from %s with kind permission_denied", apiErr, providerName)
	}
}
