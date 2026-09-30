package images

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
)

func TestHTTPErrorIsAPIError(t *testing.T) {
	const body = `{"code":"InvalidApiKey","message":"Invalid API-key provided.","request_id":"r1"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	p := New(WithAPIKey("sk-secret-7f3a"), WithBaseURL(srv.URL))
	_, err := p.DoGenerate(context.Background(), &sdk.ImageGenerationParams{Model: p.GenerationModel("wan2.2-t2i-flash"), Prompt: "x"})
	apiErr := providertest.WantAPIError(t, err, providerName, http.StatusUnauthorized, sdk.KindAuthentication)
	if string(apiErr.Body) != body {
		t.Errorf("Body = %q", apiErr.Body)
	}
	if text := err.Error(); strings.Contains(text, "sk-secret-7f3a") || strings.Contains(text, body) {
		t.Errorf("error text %q leaks the key or the body", text)
	}
}

// DashScope reports business failures in a 2xx body: a top-level code on the
// request, or a code in the output of a failed task
// (https://help.aliyun.com/zh/model-studio/error-code).
func TestBodyErrorIsAPIError(t *testing.T) {
	const (
		arrearage  = `{"request_id":"r1","code":"Arrearage","message":"Access denied, please make sure your account is in good standing."}`
		failedTask = `{"request_id":"r2","output":{"task_id":"t1","task_status":"FAILED","code":"DataInspectionFailed","message":"Input data may contain inappropriate content."}}`
	)
	for _, c := range []struct {
		name, model string
		reply       func(path string) string
		body        string
		kind        sdk.ErrorKind
	}{
		{"async create", "wan2.2-t2i-flash", func(string) string { return arrearage }, arrearage, sdk.KindQuotaExhausted},
		{"sync multimodal", "qwen-image-plus", func(string) string { return arrearage }, arrearage, sdk.KindQuotaExhausted},
		{"failed task", "wan2.2-t2i-flash", func(path string) string {
			if strings.HasSuffix(path, "/tasks/t1") {
				return failedTask
			}
			return `{"request_id":"r0","output":{"task_id":"t1","task_status":"PENDING"}}`
		}, failedTask, sdk.KindUnknown},
	} {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(c.reply(r.URL.Path)))
			}))
			defer srv.Close()
			p := New(WithAPIKey("sk-secret-7f3a"), WithBaseURL(srv.URL), WithPollInterval(time.Millisecond))
			_, err := p.DoGenerate(context.Background(), &sdk.ImageGenerationParams{Model: p.GenerationModel(c.model), Prompt: "x"})
			apiErr := providertest.WantAPIError(t, err, providerName, 0, c.kind)
			if string(apiErr.Body) != c.body {
				t.Errorf("Body = %q", apiErr.Body)
			}
			if apiErr.Code == "" || apiErr.Message == "" || apiErr.RequestID == "" {
				t.Errorf("Code, Message, RequestID = %q, %q, %q", apiErr.Code, apiErr.Message, apiErr.RequestID)
			}
			if text := err.Error(); strings.Contains(text, "sk-secret-7f3a") || strings.Contains(text, c.body) {
				t.Errorf("error text %q leaks the key or the body", text)
			}
		})
	}
}
