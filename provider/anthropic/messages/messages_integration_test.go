//go:build integration

package messages_test

import (
	"context"
	"os"
	"testing"

	"github.com/felinics/twilight/internal/testutil"
	"github.com/felinics/twilight/provider/anthropic/messages"
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

func baseOpts(t *testing.T) []messages.Option {
	t.Helper()
	apiKey := envOrSkip(t, "ANTHROPIC_API_KEY")
	opts := []messages.Option{}
	if os.Getenv("ANTHROPIC_AUTH_MODE") == "bearer" {
		opts = append(opts, messages.WithAuthToken(apiKey))
	} else {
		opts = append(opts, messages.WithAPIKey(apiKey))
	}
	if base := os.Getenv("ANTHROPIC_BASE_URL"); base != "" {
		opts = append(opts, messages.WithBaseURL(base))
	}
	return opts
}

func newIntegrationProvider(t *testing.T) *messages.Provider {
	t.Helper()
	return messages.New(baseOpts(t)...)
}

func newReasoningProvider(t *testing.T) *messages.Provider {
	t.Helper()
	opts := baseOpts(t)
	opts = append(opts, messages.WithThinking(messages.ThinkingConfig{
		Type:         "enabled",
		BudgetTokens: 4000,
	}))
	return messages.New(opts...)
}

func integrationModel(t *testing.T) *sdk.Model {
	t.Helper()
	m := os.Getenv("ANTHROPIC_MODEL")
	if m == "" {
		m = "claude-sonnet-4-20250514"
	}
	return &sdk.Model{ID: m}
}

func reasoningModel(t *testing.T) *sdk.Model {
	t.Helper()
	m := os.Getenv("ANTHROPIC_REASONING_MODEL")
	if m == "" {
		t.Skip("skipping: ANTHROPIC_REASONING_MODEL not set")
	}
	return &sdk.Model{ID: m}
}

func TestIntegration_DoGenerate(t *testing.T) {
	p := newIntegrationProvider(t)
	maxTokens := 100
	result, err := p.DoGenerate(context.Background(), sdk.Request{
		Model:     integrationModel(t).ID,
		MaxTokens: &maxTokens,
		Messages: []sdk.Message{{
			Role:    sdk.MessageRoleUser,
			Content: []sdk.MessagePart{sdk.TextPart{Text: "Say hello in one word."}},
		}},
	})
	if err != nil {
		t.Fatalf("DoGenerate: %v", err)
	}
	t.Logf("text=%q finish=%s tokens=%d/%d", result.Text, result.FinishReason,
		result.Usage.InputTokens, result.Usage.OutputTokens)

	if result.Text == "" {
		t.Error("expected non-empty text")
	}
}

func TestIntegration_DoStream(t *testing.T) {
	p := newIntegrationProvider(t)
	maxTokens := 100
	sr, err := p.DoStream(context.Background(), sdk.Request{
		Model:     integrationModel(t).ID,
		MaxTokens: &maxTokens,
		Messages: []sdk.Message{{
			Role:    sdk.MessageRoleUser,
			Content: []sdk.MessagePart{sdk.TextPart{Text: "Count from 1 to 5."}},
		}},
	})
	if err != nil {
		t.Fatalf("DoStream: %v", err)
	}

	var text string
	for part := range sr {
		switch p := part.(type) {
		case *sdk.TextDeltaPart:
			text += p.Text
			t.Logf("text delta: %q", p.Text)
		case *sdk.ErrorPart:
			t.Fatalf("stream error: %v", p.Error)
		case *sdk.FinishPart:
			t.Logf("finish=%s", p.FinishReason)
		}
	}
	t.Logf("streamed text: %q", text)
	if text == "" {
		t.Error("expected non-empty streamed text")
	}
}

func TestIntegration_DoGenerate_Reasoning(t *testing.T) {
	p := newReasoningProvider(t)
	model := reasoningModel(t)
	maxTokens := 8000
	result, err := p.DoGenerate(context.Background(), sdk.Request{
		Model:     model.ID,
		MaxTokens: &maxTokens,
		Messages: []sdk.Message{{
			Role:    sdk.MessageRoleUser,
			Content: []sdk.MessagePart{sdk.TextPart{Text: "What is 15 * 37? Think step by step."}},
		}},
	})
	if err != nil {
		t.Fatalf("DoGenerate: %v", err)
	}
	t.Logf("model=%s", model.ID)
	t.Logf("text=%q", result.Text)
	t.Logf("reasoning=%q", result.Reasoning)
	t.Logf("finish=%s tokens=%d/%d", result.FinishReason,
		result.Usage.InputTokens, result.Usage.OutputTokens)

	if result.Text == "" {
		t.Error("expected non-empty text")
	}
	if result.Reasoning == "" {
		t.Error("expected non-empty reasoning from thinking model")
	}
}

func TestIntegration_DoStream_Reasoning(t *testing.T) {
	p := newReasoningProvider(t)
	model := reasoningModel(t)
	maxTokens := 8000
	sr, err := p.DoStream(context.Background(), sdk.Request{
		Model:     model.ID,
		MaxTokens: &maxTokens,
		Messages: []sdk.Message{{
			Role:    sdk.MessageRoleUser,
			Content: []sdk.MessagePart{sdk.TextPart{Text: "What is 15 * 37? Think step by step."}},
		}},
	})
	if err != nil {
		t.Fatalf("DoStream: %v", err)
	}

	var text, reasoning string
	var gotReasoningStart, gotReasoningEnd bool
	for part := range sr {
		switch p := part.(type) {
		case *sdk.ReasoningStartPart:
			gotReasoningStart = true
			t.Log("--- reasoning start ---")
		case *sdk.ReasoningDeltaPart:
			reasoning += p.Text
		case *sdk.ReasoningEndPart:
			gotReasoningEnd = true
			t.Logf("--- reasoning end (len=%d) ---", len(reasoning))
		case *sdk.TextDeltaPart:
			text += p.Text
		case *sdk.ErrorPart:
			t.Fatalf("stream error: %v", p.Error)
		case *sdk.FinishPart:
			t.Logf("finish=%s total_usage=%+v", p.FinishReason, p.TotalUsage)
		}
	}

	t.Logf("model=%s", model.ID)
	t.Logf("streamed text: %q", text)
	t.Logf("reasoning length: %d chars", len(reasoning))

	if text == "" {
		t.Error("expected non-empty streamed text")
	}
	if reasoning == "" {
		t.Error("expected non-empty reasoning from thinking model")
	}
	if !gotReasoningStart {
		t.Error("missing ReasoningStartPart")
	}
	if !gotReasoningEnd {
		t.Error("missing ReasoningEndPart")
	}
}
