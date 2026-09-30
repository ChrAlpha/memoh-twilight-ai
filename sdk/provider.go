package sdk

import (
	"context"
	"errors"
	"net/http"
)

// ModelTestResult holds the result of a model support check.
type ModelTestResult struct {
	Supported bool
	Message   string
}

// ClassifyProbe maps the outcome of a minimal generation request to a
// ModelTestResult. Providers use it as a fallback when the models listing API
// (GET /models/{id}) is unavailable. err is what the probe request returned:
// nil for a 2xx response, a *APIError for any other status, or the transport
// error.
//
// A 2xx, 400, 422 or 429 response means the provider accepted the model, and
// a 404 means it does not know it; both return a result and a nil error. Any
// other outcome returns err unchanged, so a rejected credential is a *APIError
// whose Kind is KindAuthentication or KindPermissionDenied.
func ClassifyProbe(err error) (*ModelTestResult, error) {
	if err == nil {
		return &ModelTestResult{Supported: true, Message: "supported"}, nil
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return nil, err
	}
	switch apiErr.StatusCode {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusTooManyRequests:
		return &ModelTestResult{Supported: true, Message: "supported"}, nil
	case http.StatusNotFound:
		return &ModelTestResult{Supported: false, Message: "model not found"}, nil
	default:
		return nil, err
	}
}

// Provider is the interface that AI backends must implement.
//
// Test checks that the provider is reachable and, where the provider's check
// needs them, that it accepts the configured credentials. nil means the check
// passed; a provider whose check calls a public endpoint documents that nil only
// establishes reachability. A *APIError in the returned chain means the provider
// was reached and rejected the check; its Kind tells a rejected credential
// (KindAuthentication, KindPermissionDenied) from other failures. Any other
// error means the check did not reach the provider, for example a transport
// failure or an ended context, which stays in the chain.
//
// DoGenerate and DoStream are the seam between the SDK and a backend, and they
// speak the single-call boundary: a Request goes in, and a ModelResult or a
// channel of StreamParts comes out. Tool schemas arrive in Request.Tools as
// JSON Schema.
//
// DoStream must close the returned channel, must respect ctx while producing and
// while sending, and reports a mid-stream failure as an ErrorPart rather than by
// dropping the failure. Assembling the parts into a ModelResult is the SDK's
// job, not the provider's, so that streamed and generated results cannot drift.
type Provider interface {
	Name() string
	ListModels(ctx context.Context) ([]Model, error)
	Test(ctx context.Context) error
	TestModel(ctx context.Context, modelID string) (*ModelTestResult, error)
	DoGenerate(ctx context.Context, req Request) (ModelResult, error)
	DoStream(ctx context.Context, req Request) (<-chan StreamPart, error)
}
