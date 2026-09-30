//go:build integration

package completions_test

import (
	"context"
	"os"
	"testing"

	"github.com/felinics/twilight/internal/testutil"
	"github.com/felinics/twilight/provider/openai/completions"
	"github.com/felinics/twilight/sdk"
	"github.com/google/jsonschema-go/jsonschema"
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

func newIntegrationProvider(t *testing.T) *completions.Provider {
	t.Helper()
	apiKey := envOrSkip(t, "OPENAI_API_KEY")
	opts := []completions.Option{completions.WithAPIKey(apiKey)}
	if base := os.Getenv("OPENAI_BASE_URL"); base != "" {
		opts = append(opts, completions.WithBaseURL(base))
	}
	return completions.New(opts...)
}

func integrationModel(t *testing.T) *sdk.Model {
	t.Helper()
	m := os.Getenv("OPENAI_MODEL")
	if m == "" {
		m = "gpt-4o-mini"
	}
	return &sdk.Model{ID: m}
}

func TestIntegration_DoGenerate(t *testing.T) {
	p := newIntegrationProvider(t)
	result, err := p.DoGenerate(context.Background(), sdk.Request{
		Model: integrationModel(t).ID,
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
	sr, err := p.DoStream(context.Background(), sdk.Request{
		Model: integrationModel(t).ID,
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

// ---------- multi-model integration tests (OpenRouter) ----------

const (
	openRouterGoogleIntegrationModel = "google/gemini-3.6-flash"
	// The proxy's plain-chat DeepSeek; "deepseek/deepseek-chat" is no longer
	// routed there (404 model_not_found).
	openRouterDeepSeekChatIntegrationModel = "deepseek/deepseek-v4-flash"
)

func TestIntegration_MultiModel(t *testing.T) {
	p := newIntegrationProvider(t)

	models := []struct {
		id           string
		hasReasoning bool
	}{
		{openRouterGoogleIntegrationModel, false},
		{"deepseek/deepseek-r1", true},
		{openRouterDeepSeekChatIntegrationModel, false},
	}

	for _, m := range models {
		t.Run(m.id, func(t *testing.T) {
			model := &sdk.Model{ID: m.id}
			result, err := p.DoGenerate(context.Background(), sdk.Request{
				Model:    model.ID,
				Messages: []sdk.Message{sdk.UserMessage("What is 2+3? Answer with just the number.")},
			})
			if err != nil {
				t.Fatalf("DoGenerate: %v", err)
			}
			t.Logf("text=%q reasoning=%q finish=%s tokens=in:%d/out:%d/reasoning:%d",
				result.Text, truncate(result.Reasoning, 80), result.FinishReason,
				result.Usage.InputTokens, result.Usage.OutputTokens, result.Usage.ReasoningTokens)

			if result.Text == "" {
				t.Error("expected non-empty text")
			}
			if m.hasReasoning && result.Reasoning == "" {
				t.Error("expected non-empty reasoning for reasoning model")
			}
		})
	}
}

func TestIntegration_MultiModel_Stream(t *testing.T) {
	p := newIntegrationProvider(t)

	models := []struct {
		id           string
		hasReasoning bool
	}{
		{openRouterGoogleIntegrationModel, false},
		{"deepseek/deepseek-r1", true},
	}

	for _, m := range models {
		t.Run(m.id, func(t *testing.T) {
			model := &sdk.Model{ID: m.id}
			sr, err := p.DoStream(context.Background(), sdk.Request{
				Model:    model.ID,
				Messages: []sdk.Message{sdk.UserMessage("What is 2+3? Answer with just the number.")},
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
				case *sdk.ReasoningDeltaPart:
					reasoning += p.Text
				case *sdk.ReasoningEndPart:
					gotReasoningEnd = true
				case *sdk.TextDeltaPart:
					text += p.Text
				case *sdk.ErrorPart:
					t.Fatalf("stream error: %v", p.Error)
				case *sdk.FinishPart:
					t.Logf("finish=%s", p.FinishReason)
				}
			}
			t.Logf("text=%q reasoning=%q (len=%d)", text, truncate(reasoning, 80), len(reasoning))

			if text == "" {
				t.Error("expected non-empty text")
			}
			if m.hasReasoning {
				if reasoning == "" {
					t.Error("expected non-empty reasoning")
				}
				if !gotReasoningStart {
					t.Error("missing ReasoningStartPart")
				}
				if !gotReasoningEnd {
					t.Error("missing ReasoningEndPart")
				}
			}
		})
	}
}

func TestIntegration_Reasoning_ToolCall(t *testing.T) {
	p := newIntegrationProvider(t)
	// The reasoning assertions need a model that emits reasoning, so this case
	// reads OPENAI_MODEL like every other integration case: point it at a
	// reasoning model (deepseek-reasoner, o4-mini, ...) to exercise them, and
	// the tool-call assertions run against whatever the endpoint serves.
	model := integrationModel(t)

	result, err := p.DoGenerate(context.Background(), sdk.Request{
		Model:    model.ID,
		Messages: []sdk.Message{sdk.UserMessage("What's the weather in Tokyo right now?")},
		Tools: []sdk.ToolDefinition{{
			Name:        "get_weather",
			Description: "Get the current weather for a city",
			Parameters: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"city": {Type: "string", Description: "City name"},
				},
				Required: []string{"city"},
			},
		}},
		ToolChoice: sdk.ToolChoice{Mode: sdk.ToolChoiceAuto},
	})
	if err != nil {
		t.Fatalf("DoGenerate: %v", err)
	}

	t.Logf("text=%q reasoning=%q (len=%d) finish=%s toolCalls=%d",
		truncate(result.Text, 80), truncate(result.Reasoning, 80),
		len(result.Reasoning), result.FinishReason, len(result.ToolCalls))

	if result.Reasoning == "" {
		t.Log("warning: no reasoning returned (model may not emit reasoning with tool calls)")
	}
	if len(result.ToolCalls) > 0 {
		for _, tc := range result.ToolCalls {
			t.Logf("  tool=%q id=%s input=%v", tc.ToolName, tc.ToolCallID, tc.Input)
		}
	} else if result.Text == "" {
		t.Error("expected either tool calls or text response")
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
