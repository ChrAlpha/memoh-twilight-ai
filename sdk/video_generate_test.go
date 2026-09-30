package sdk

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/felinics/twilight/internal/reqheaders"
)

func TestCreateVideoValidation(t *testing.T) {
	_, err := CreateVideo(context.Background(), WithVideoPrompt("hello"))
	if err == nil || !strings.Contains(err.Error(), "video model is required") {
		t.Fatalf("expected missing model error, got %v", err)
	}

	_, err = CreateVideo(context.Background(), WithVideoModel(&VideoModel{ID: "m"}), WithVideoPrompt("hello"))
	if err == nil || !strings.Contains(err.Error(), "has no provider") {
		t.Fatalf("expected missing provider error, got %v", err)
	}

	_, err = CreateVideo(context.Background(), WithVideoModel(testVideoModel(&fakeVideoProvider{})))
	if err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("expected missing prompt error, got %v", err)
	}
}

func TestGenerateVideoPollsUntilSucceeded(t *testing.T) {
	prov := &fakeVideoProvider{
		createJob: &VideoJob{ID: "job-1", Status: VideoJobQueued},
		getJobs: []*VideoJob{
			{ID: "job-1", Status: VideoJobRunning},
			{ID: "job-1", Status: VideoJobSucceeded, Outputs: []VideoOutput{{URL: "https://example.com/out.mp4"}}},
		},
		downloadData: []byte("video"),
	}

	result, err := GenerateVideo(context.Background(),
		WithVideoModel(testVideoModel(prov)),
		WithVideoPrompt("make a clip"),
		WithVideoPollInterval(time.Millisecond),
		WithVideoPollTimeout(time.Second),
		WithVideoDownload(true),
	)
	if err != nil {
		t.Fatalf("GenerateVideo returned error: %v", err)
	}
	if result.Job.Status != VideoJobSucceeded {
		t.Fatalf("status = %s, want succeeded", result.Job.Status)
	}
	if prov.getCalls != 2 {
		t.Fatalf("get calls = %d, want 2", prov.getCalls)
	}
	if string(result.Data) != "video" || result.ContentType != "video/mp4" {
		t.Fatalf("unexpected download result: %q %q", result.Data, result.ContentType)
	}
}

func TestGenerateVideoSendsClientRequestIDOnlyWithCreate(t *testing.T) {
	prov := &fakeVideoProvider{
		createJob:    &VideoJob{ID: "job-1", Status: VideoJobQueued},
		getJobs:      []*VideoJob{{ID: "job-1", Status: VideoJobSucceeded, Outputs: []VideoOutput{{URL: "https://example.com/out.mp4"}}}},
		downloadData: []byte("video"),
	}
	ctx := WithClientRequestID(context.Background(), "client-id")
	if _, err := GenerateVideo(ctx,
		WithVideoModel(testVideoModel(prov)),
		WithVideoPrompt("make a clip"),
		WithVideoPollInterval(time.Millisecond),
		WithVideoPollTimeout(time.Second),
		WithVideoDownload(true),
	); err != nil {
		t.Fatalf("GenerateVideo returned error: %v", err)
	}
	// Create, one poll, download.
	if want := []string{"client-id", "", ""}; !slices.Equal(prov.clientRequestIDs, want) {
		t.Fatalf("client request IDs = %q, want %q", prov.clientRequestIDs, want)
	}
}

func TestGenerateVideoTimeout(t *testing.T) {
	prov := &fakeVideoProvider{
		createJob: &VideoJob{ID: "job-1", Status: VideoJobQueued},
		getJobs:   []*VideoJob{{ID: "job-1", Status: VideoJobRunning}},
	}

	_, err := GenerateVideo(context.Background(),
		WithVideoModel(testVideoModel(prov)),
		WithVideoPrompt("make a clip"),
		WithVideoPollInterval(time.Millisecond),
		WithVideoPollTimeout(3*time.Millisecond),
	)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected an error wrapping context.DeadlineExceeded, got %v", err)
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Fatalf("a poll timeout is not a provider failure, got APIError %+v", apiErr)
	}
}

func TestGenerateVideoCanceled(t *testing.T) {
	prov := &fakeVideoProvider{
		createJob: &VideoJob{ID: "job-1", Status: VideoJobQueued},
		getJobs:   []*VideoJob{{ID: "job-1", Status: VideoJobRunning}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := GenerateVideo(ctx,
		WithVideoModel(testVideoModel(prov)),
		WithVideoPrompt("make a clip"),
		WithVideoPollInterval(time.Hour),
		WithVideoPollTimeout(time.Hour),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected an error wrapping context.Canceled, got %v", err)
	}
}

func TestGenerateVideoFailedStatus(t *testing.T) {
	prov := &fakeVideoProvider{
		createJob: &VideoJob{ID: "job-1", Status: VideoJobQueued},
		getJobs: []*VideoJob{
			{ID: "job-1", Status: VideoJobFailed, Error: &VideoError{Code: "QuotaExceeded", Message: "blocked", Kind: KindQuotaExhausted}},
		},
	}

	result, err := GenerateVideo(context.Background(),
		WithVideoModel(testVideoModel(prov)),
		WithVideoPrompt("make a clip"),
		WithVideoPollInterval(time.Millisecond),
		WithVideoPollTimeout(time.Second),
	)
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected failed status error, got %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T %v", err, err)
	}
	if apiErr.Provider != "fake-videos" || apiErr.StatusCode != 0 || apiErr.Code != "QuotaExceeded" ||
		apiErr.Message != "blocked" || apiErr.Kind != KindQuotaExhausted {
		t.Fatalf("APIError = %+v, want provider fake-videos, status 0, code QuotaExceeded, message blocked, kind quota_exhausted", apiErr)
	}
	if result == nil || result.Job.Status != VideoJobFailed {
		t.Fatalf("expected failed result, got %#v", result)
	}
}

func TestGenerateVideoFailedWithoutPayload(t *testing.T) {
	prov := &fakeVideoProvider{
		createJob: &VideoJob{ID: "job-1", Status: VideoJobQueued},
		getJobs:   []*VideoJob{{ID: "job-1", Status: VideoJobFailed}},
	}

	_, err := GenerateVideo(context.Background(),
		WithVideoModel(testVideoModel(prov)),
		WithVideoPrompt("make a clip"),
		WithVideoPollInterval(time.Millisecond),
		WithVideoPollTimeout(time.Second),
	)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T %v", err, err)
	}
	if apiErr.StatusCode != 0 || apiErr.Kind != KindUnknown {
		t.Fatalf("APIError = %+v, want status 0 and kind unknown", apiErr)
	}
}

func TestGenerateVideoCanceledStatusIsNotAPIError(t *testing.T) {
	prov := &fakeVideoProvider{
		createJob: &VideoJob{ID: "job-1", Status: VideoJobQueued},
		getJobs:   []*VideoJob{{ID: "job-1", Status: VideoJobCanceled}},
	}

	_, err := GenerateVideo(context.Background(),
		WithVideoModel(testVideoModel(prov)),
		WithVideoPrompt("make a clip"),
		WithVideoPollInterval(time.Millisecond),
		WithVideoPollTimeout(time.Second),
	)
	var apiErr *APIError
	if err == nil || errors.As(err, &apiErr) {
		t.Fatalf("expected a plain error for a canceled job, got %v", err)
	}
}

func testVideoModel(prov VideoProvider) *VideoModel {
	return &VideoModel{ID: "model-1", Provider: prov}
}

type fakeVideoProvider struct {
	createJob    *VideoJob
	getJobs      []*VideoJob
	getCalls     int
	downloadData []byte
	// clientRequestIDs records the client request ID each call's context
	// carried, in call order.
	clientRequestIDs []string
}

func (p *fakeVideoProvider) Name() string { return "fake-videos" }

func (p *fakeVideoProvider) ListModels(context.Context) ([]*VideoModel, error) {
	return nil, nil
}

func (p *fakeVideoProvider) DoCreate(ctx context.Context, _ VideoParams) (*VideoJob, error) {
	p.clientRequestIDs = append(p.clientRequestIDs, reqheaders.ClientRequestID(ctx))
	return p.createJob, nil
}

func (p *fakeVideoProvider) DoGet(ctx context.Context, _ *VideoModel, _ string) (*VideoJob, error) {
	p.clientRequestIDs = append(p.clientRequestIDs, reqheaders.ClientRequestID(ctx))
	if len(p.getJobs) == 0 {
		return nil, errors.New("no jobs configured")
	}
	idx := p.getCalls
	if idx >= len(p.getJobs) {
		idx = len(p.getJobs) - 1
	}
	p.getCalls++
	return p.getJobs[idx], nil
}

func (p *fakeVideoProvider) DoCancel(context.Context, *VideoModel, string) error {
	return nil
}

func (p *fakeVideoProvider) DoDownload(ctx context.Context, _ *VideoModel, _ VideoOutput) ([]byte, string, error) {
	p.clientRequestIDs = append(p.clientRequestIDs, reqheaders.ClientRequestID(ctx))
	return p.downloadData, "video/mp4", nil
}
