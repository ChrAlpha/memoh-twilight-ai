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

func TestGenerateVideoFailedJobIsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(videoResponse{ID: "job-1", Status: "pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(videoResponse{ID: "job-1", Status: "failed", Error: "content policy violation"})
	}))
	defer server.Close()

	prov := New(WithAPIKey("test-key"), WithBaseURL(server.URL))
	result, err := sdk.GenerateVideo(context.Background(),
		sdk.WithVideoModel(prov.VideoModel("google/veo-3.1")),
		sdk.WithVideoPrompt("cinematic ocean"),
		sdk.WithVideoPollInterval(time.Millisecond),
		sdk.WithVideoPollTimeout(time.Second),
	)
	var apiErr *sdk.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("GenerateVideo error = %v (%T), want *sdk.APIError", err, err)
	}
	if apiErr.Provider != providerName || apiErr.StatusCode != 0 ||
		apiErr.Message != "content policy violation" || apiErr.Kind != sdk.KindUnknown {
		t.Fatalf("APIError = %+v, want provider %s, status 0, the job's message, kind unknown", apiErr, providerName)
	}
	if result == nil || result.Job.Status != sdk.VideoJobFailed {
		t.Fatalf("result = %+v, want the failed job", result)
	}
}

func TestDoDownloadNon2xxIsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer server.Close()

	prov := New(WithAPIKey("test-key"), WithBaseURL(server.URL))
	_, _, err := prov.DoDownload(context.Background(), nil, sdk.VideoOutput{URL: server.URL + "/out.mp4"})
	var apiErr *sdk.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("DoDownload error = %v (%T), want *sdk.APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound || apiErr.Provider != providerName || string(apiErr.Body) != "not found" {
		t.Fatalf("APIError = %+v, want 404 from %s with the body kept", apiErr, providerName)
	}
}
