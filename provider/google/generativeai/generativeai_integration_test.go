//go:build integration

package generativeai_test

import (
	"context"
	"os"
	"testing"

	"github.com/felinics/twilight/internal/testutil"
	"github.com/felinics/twilight/provider/google/generativeai"
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

func newIntegrationProvider(t *testing.T) *generativeai.Provider {
	t.Helper()
	apiKey := envOrSkip(t, "GOOGLE_GENERATIVE_AI_API_KEY")
	opts := []generativeai.Option{generativeai.WithAPIKey(apiKey)}
	if base := os.Getenv("GOOGLE_GENERATIVE_AI_BASE_URL"); base != "" {
		opts = append(opts, generativeai.WithBaseURL(base))
	}
	return generativeai.New(opts...)
}

func integrationModel(t *testing.T) *sdk.Model {
	t.Helper()
	m := os.Getenv("GOOGLE_GENERATIVE_AI_MODEL")
	if m == "" {
		m = "gemini-2.5-flash"
	}
	return &sdk.Model{ID: m}
}

func TestIntegration_DoGenerate(t *testing.T) {
	p := newIntegrationProvider(t)
	model := integrationModel(t)
	model.Provider = p
	result, err := p.DoGenerate(context.Background(), sdk.Request{
		Model: model.ID,
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
	model := integrationModel(t)
	model.Provider = p
	sr, err := p.DoStream(context.Background(), sdk.Request{
		Model: model.ID,
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

func TestIntegration_ToolCall(t *testing.T) {
	p := newIntegrationProvider(t)
	model := integrationModel(t)
	model.Provider = p
	result, err := p.DoGenerate(context.Background(), sdk.Request{
		Model: model.ID,
		Messages: []sdk.Message{{
			Role:    sdk.MessageRoleUser,
			Content: []sdk.MessagePart{sdk.TextPart{Text: "What's the weather in San Francisco?"}},
		}},
		Tools: []sdk.ToolDefinition{{
			Name:        "get_weather",
			Description: "Get the weather for a location",
			Parameters: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"location": {Type: "string", Description: "City name"},
				},
				Required: []string{"location"},
			},
		}},
		ToolChoice: sdk.ToolChoice{Mode: sdk.ToolChoiceAuto},
	})
	if err != nil {
		t.Fatalf("DoGenerate: %v", err)
	}
	t.Logf("finish=%s toolCalls=%d text=%q", result.FinishReason, len(result.ToolCalls), result.Text)

	if len(result.ToolCalls) == 0 {
		t.Error("expected at least one tool call")
	}
	for _, tc := range result.ToolCalls {
		t.Logf("  tool=%q id=%s input=%v", tc.ToolName, tc.ToolCallID, tc.Input)
	}
}

func TestIntegration_ToolCallWithAdditionalPropertiesSchema(t *testing.T) {
	p := newIntegrationProvider(t)
	model := integrationModel(t)
	model.Provider = p

	result, err := p.DoGenerate(context.Background(), sdk.Request{
		Model: model.ID,
		Messages: []sdk.Message{{
			Role: sdk.MessageRoleUser,
			Content: []sdk.MessagePart{sdk.TextPart{
				Text: "Call get_weather with location San Francisco.",
			}},
		}},
		Tools: []sdk.ToolDefinition{{
			Name:        "get_weather",
			Description: "Get the weather for a location.",
			Parameters: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"location": {Type: "string", Description: "City name"},
				},
				Required:             []string{"location"},
				AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			},
		}},
		ToolChoice: sdk.ToolChoice{Mode: sdk.ToolChoiceRequired},
	})
	if err != nil {
		t.Fatalf("DoGenerate with additionalProperties in tool schema: %v", err)
	}
	t.Logf("finish=%s toolCalls=%d text=%q", result.FinishReason, len(result.ToolCalls), result.Text)

	if result.FinishReason != sdk.FinishReasonToolCalls {
		t.Errorf("finish: got %q, want %q", result.FinishReason, sdk.FinishReasonToolCalls)
	}
	if len(result.ToolCalls) == 0 {
		t.Fatal("expected at least one tool call")
	}
	tc := result.ToolCalls[0]
	t.Logf("  tool=%q id=%s input=%v", tc.ToolName, tc.ToolCallID, tc.Input)
	if tc.ToolName != "get_weather" {
		t.Errorf("tool name: got %q, want get_weather", tc.ToolName)
	}
	var input map[string]any
	if err := tc.Input.Unmarshal(&input); err != nil {
		t.Fatalf("decode input: %v", err)
	}
	if location, ok := input["location"].(string); !ok || location == "" {
		t.Errorf("location input: got %v", input["location"])
	}
}
