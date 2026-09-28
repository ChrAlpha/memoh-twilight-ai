//go:build integration

package copilot_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/felinics/twilight/internal/testutil"
	"github.com/felinics/twilight/provider/github/copilot"
	"github.com/felinics/twilight/sdk"
)

// Live API tests. Compiled only with -tags=integration; each test skips
// when its credentials are not set. Credentials come from the environment
// or a .env file found by walking up from the package directory.

func TestMain(m *testing.M) {
	testutil.LoadEnv()
	os.Exit(m.Run())
}

func envOrSkip(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("skipping: %s not set", key)
	}
	return v
}

func newIntegrationProvider(t *testing.T) *copilot.Provider {
	t.Helper()
	token := envOrSkip(t, "GITHUB_COPILOT_TOKEN")
	opts := []copilot.Option{copilot.WithGitHubToken(token)}
	if base := os.Getenv("GITHUB_COPILOT_BASE_URL"); base != "" {
		opts = append(opts, copilot.WithBaseURL(base))
	}
	return copilot.New(opts...)
}

func integrationModelID() string {
	if v := os.Getenv("GITHUB_COPILOT_MODEL"); v != "" {
		return v
	}
	return "gpt-5-mini"
}

func isModelUnsupported(err error) bool {
	return err != nil && strings.Contains(err.Error(), "model_not_supported")
}

func isEndpointForbidden(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *sdk.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "api error 403") ||
		strings.Contains(msg, "Access to this endpoint is forbidden")
}

func TestIntegration_ListModels(t *testing.T) {
	p := newIntegrationProvider(t)
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	t.Logf("models: %+v", models)
	if len(models) == 0 {
		t.Fatal("expected non-empty model catalog")
	}
	if models[0].ID != copilot.AutoModel {
		t.Fatalf("unexpected first model: %q", models[0].ID)
	}
}

func TestIntegration_DoGenerate_ExplicitModel(t *testing.T) {
	p := newIntegrationProvider(t)
	modelID := integrationModelID()

	result, err := p.DoGenerate(context.Background(), sdk.Request{
		Model:    modelID,
		Messages: []sdk.Message{sdk.UserMessage("Reply with exactly: ok")},
	})
	if err != nil {
		if isModelUnsupported(err) {
			t.Skipf("explicit model %q is not supported by this Copilot endpoint/token: %v", modelID, err)
		}
		if isEndpointForbidden(err) {
			t.Skipf("token is not allowed to call the Copilot chat completions endpoint: %v", err)
		}
		t.Fatalf("DoGenerate failed: %v", err)
	}

	t.Logf("requested_model=%q response_model=%q text=%q finish=%s input=%d output=%d",
		modelID, result.Response.ModelID, result.Text, result.FinishReason,
		result.Usage.InputTokens, result.Usage.OutputTokens)

	if result.Text == "" {
		t.Fatal("expected non-empty text")
	}
}

func TestIntegration_DoGenerate_AutoModel(t *testing.T) {
	p := newIntegrationProvider(t)

	result, err := p.DoGenerate(context.Background(), sdk.Request{
		Model:    copilot.AutoModel,
		Messages: []sdk.Message{sdk.UserMessage("Reply with exactly: ok")},
	})
	if err != nil {
		if isEndpointForbidden(err) {
			t.Skipf("token is not allowed to call the Copilot chat completions endpoint: %v", err)
		}
		t.Fatalf("DoGenerate failed: %v", err)
	}

	t.Logf("response_model=%q text=%q finish=%s input=%d output=%d",
		result.Response.ModelID, result.Text, result.FinishReason,
		result.Usage.InputTokens, result.Usage.OutputTokens)

	if result.Text == "" {
		t.Fatal("expected non-empty text")
	}
}
