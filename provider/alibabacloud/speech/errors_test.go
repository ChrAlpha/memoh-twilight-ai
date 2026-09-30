package speech

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
	"github.com/gorilla/websocket"
)

const secretKey = "sk-secret-7f3a"

// The handshake body is DashScope's documented error shape for a bad key:
// https://help.aliyun.com/zh/model-studio/error-code
const invalidKeyBody = `{"code":"InvalidApiKey","message":"Invalid API-key provided.","request_id":"fb53c4ec-1c12-4fc4-a580-cdb7c3261fc1"}`

func TestHandshakeHTTPErrorIsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(invalidKeyBody))
	}))
	defer srv.Close()
	p := New(WithAPIKey(secretKey), WithBaseURL("ws"+strings.TrimPrefix(srv.URL, "http")))

	_, err := p.DoSynthesize(context.Background(), sdk.SpeechParams{Text: "x"})
	apiErr := providertest.WantAPIError(t, err, providerName, http.StatusUnauthorized, sdk.KindAuthentication)
	if apiErr.Code != "InvalidApiKey" || apiErr.RequestID != "fb53c4ec-1c12-4fc4-a580-cdb7c3261fc1" {
		t.Errorf("Code, RequestID = %q, %q", apiErr.Code, apiErr.RequestID)
	}
	assertNoLeak(t, err, invalidKeyBody)
}

// failingHandler accepts the session and answers run-task with frame.
func failingHandler(t *testing.T, frame func(taskID string) []byte) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg struct {
			Header struct {
				TaskID string `json:"task_id"`
			} `json:"header"`
		}
		_ = json.Unmarshal(data, &msg)
		_ = conn.WriteMessage(websocket.TextMessage, frame(msg.Header.TaskID))
		_, _, _ = conn.ReadMessage()
	})
}

// taskFailedFrame follows the task-failed event documented at
// https://help.aliyun.com/zh/model-studio/cosyvoice-websocket-api
func taskFailedFrame(taskID string) []byte {
	return []byte(`{"header":{"task_id":"` + taskID + `","event":"task-failed","error_code":"InvalidParameter","error_message":"The request is missing required parameters or in a wrong format, please check the parameters that you send.","attributes":{}},"payload":{}}`)
}

func wantTaskFailed(t *testing.T, err error) {
	t.Helper()
	apiErr := providertest.WantAPIError(t, err, providerName, 0, sdk.KindUnknown)
	if apiErr.Code != "InvalidParameter" {
		t.Errorf("Code = %q, want InvalidParameter", apiErr.Code)
	}
	if !strings.HasPrefix(apiErr.Message, "The request is missing required parameters") {
		t.Errorf("Message = %q", apiErr.Message)
	}
	if apiErr.RequestID == "" {
		t.Error("RequestID is empty, want the task ID")
	}
	assertNoLeak(t, err, string(apiErr.Body))
}

func TestTaskFailedIsAPIError(t *testing.T) {
	srv := httptest.NewServer(failingHandler(t, taskFailedFrame))
	defer srv.Close()
	p := New(WithAPIKey(secretKey), WithBaseURL("ws"+strings.TrimPrefix(srv.URL, "http")))

	t.Run("DoSynthesize", func(t *testing.T) {
		_, err := p.DoSynthesize(context.Background(), sdk.SpeechParams{Text: "x"})
		wantTaskFailed(t, err)
	})
	t.Run("DoStream", func(t *testing.T) {
		result, err := p.DoStream(context.Background(), sdk.SpeechParams{Text: "x"})
		if err != nil {
			t.Fatalf("DoStream: %v", err)
		}
		for range result.Stream {
			t.Error("unexpected audio chunk")
		}
		wantTaskFailed(t, result.Err())
	})
}

func TestTaskFailedKind(t *testing.T) {
	frame := func(taskID string) []byte {
		return []byte(`{"header":{"task_id":"` + taskID + `","event":"task-failed","error_code":"Throttling.RateQuota","error_message":"Requests rate limit exceeded, please try again later.","attributes":{}},"payload":{}}`)
	}
	srv := httptest.NewServer(failingHandler(t, frame))
	defer srv.Close()
	p := New(WithAPIKey(secretKey), WithBaseURL("ws"+strings.TrimPrefix(srv.URL, "http")))
	_, err := p.DoSynthesize(context.Background(), sdk.SpeechParams{Text: "x"})
	providertest.WantAPIError(t, err, providerName, 0, sdk.KindRateLimited)
}

func assertNoLeak(t *testing.T, err error, body string) {
	t.Helper()
	if text := err.Error(); strings.Contains(text, secretKey) || strings.Contains(text, body) {
		t.Errorf("error text %q leaks the key or the body", text)
	}
}
