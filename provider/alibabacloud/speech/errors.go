package speech

import (
	"encoding/json"

	"github.com/felinics/twilight/internal/errorformat"
	"github.com/felinics/twilight/sdk"
)

// providerName identifies this package in APIError.Provider.
const providerName = "alibabacloud-speech"

// eventHeader is the header of a DashScope server event. A task-failed event
// carries the failure in error_code and error_message:
// {"header":{"task_id":"...","event":"task-failed","error_code":"...","error_message":"...","attributes":{}},"payload":{}}.
type eventHeader struct {
	TaskID       string `json:"task_id"`
	Event        string `json:"event"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

// parseEvent reads the header of a DashScope server event.
func parseEvent(data []byte) (eventHeader, error) {
	var msg struct {
		Header eventHeader `json:"header"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return eventHeader{}, err
	}
	return msg.Header, nil
}

// taskFailedError turns a task-failed event into an APIError. The session had
// already upgraded, so there is no HTTP status to report and StatusCode stays
// 0. DashScope documents no request ID for the session, so the task ID the
// client chose stands in for it.
func taskFailedError(h eventHeader, frame []byte) *sdk.APIError {
	return &sdk.APIError{
		Provider:  providerName,
		Code:      h.ErrorCode,
		Message:   h.ErrorMessage,
		RequestID: h.TaskID,
		Kind:      errorformat.DashScopeKind(h.ErrorCode),
		Body:      frame,
	}
}
