//go:build integration

package responses_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/felinics/twilight/internal/testutil"
	"github.com/felinics/twilight/provider/openai/responses"
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

const openRouterResponsesReasoningModel = "openai/gpt-5.4-mini"

func envOrSkip(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("skipping: %s not set", key)
	}
	return v
}

func newResponsesIntegrationProvider(t *testing.T) *responses.Provider {
	t.Helper()
	apiKey := envOrSkip(t, "OPENAI_API_KEY")
	opts := []responses.Option{responses.WithAPIKey(apiKey)}
	if base := os.Getenv("OPENAI_BASE_URL"); base != "" {
		opts = append(opts, responses.WithBaseURL(base))
	}
	return responses.New(opts...)
}

func responsesIntegrationModel(t *testing.T, p *responses.Provider) *sdk.Model {
	t.Helper()
	m := os.Getenv("OPENAI_MODEL")
	if m == "" {
		m = "gpt-4o-mini"
	}
	return p.ChatModel(m)
}

func newBedrockBearerIntegrationProvider(t *testing.T) *responses.Provider {
	t.Helper()

	token := envOrSkip(t, "AWS_BEARER_TOKEN_BEDROCK")
	baseURLs, source := bedrockBearerBaseURLs()
	t.Logf("resolving Bedrock base URL from %s", source)

	for _, baseURL := range baseURLs {
		p := responses.New(
			responses.WithAPIKey(token),
			responses.WithBaseURL(baseURL),
		)

		models, err := p.ListModels(context.Background())
		if err == nil {
			t.Logf("using Bedrock base URL %q", baseURL)
			if len(models) == 0 {
				t.Fatalf("Bedrock base URL %q returned zero models", baseURL)
			}
			return p
		}

		if strings.Contains(err.Error(), "valid region") {
			t.Logf("Bedrock base URL %q rejected token region, trying next endpoint", baseURL)
			continue
		}
		if strings.Contains(err.Error(), "Signature expired") {
			t.Skipf("skipping Bedrock integration due to expired signature: %v", err)
		}

		t.Fatalf("Bedrock ListModels via %q: %v", baseURL, err)
	}

	t.Fatal("unable to find a valid Bedrock region for AWS_BEARER_TOKEN_BEDROCK; set AWS_BEDROCK_BASE_URL or AWS_REGION explicitly")
	return nil
}

func bedrockBearerBaseURLs() ([]string, string) {
	if baseURL := os.Getenv("AWS_BEDROCK_BASE_URL"); baseURL != "" {
		return []string{baseURL}, "AWS_BEDROCK_BASE_URL"
	}

	if region := os.Getenv("AWS_REGION"); region != "" {
		return []string{fmt.Sprintf("https://bedrock-mantle.%s.api.aws/v1", region)}, "AWS_REGION"
	}
	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		return []string{fmt.Sprintf("https://bedrock-mantle.%s.api.aws/v1", region)}, "AWS_DEFAULT_REGION"
	}

	regions := []string{
		"us-east-1",
		"us-east-2",
		"us-west-2",
		"ap-northeast-1",
		"ap-south-1",
		"ap-southeast-3",
		"eu-central-1",
		"eu-west-1",
		"eu-west-2",
		"eu-south-1",
		"eu-north-1",
		"sa-east-1",
	}

	baseURLs := make([]string, 0, len(regions))
	for _, region := range regions {
		baseURLs = append(baseURLs, fmt.Sprintf("https://bedrock-mantle.%s.api.aws/v1", region))
	}
	return baseURLs, "built-in region fallback"
}

func TestIntegration_ResponsesDoGenerate(t *testing.T) {
	p := newResponsesIntegrationProvider(t)
	model := responsesIntegrationModel(t, p)
	result, err := p.DoGenerate(context.Background(), sdk.Request{
		Model:    model.ID,
		Messages: []sdk.Message{sdk.UserMessage("Say hello in one word.")},
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

func TestIntegration_BedrockBearer_ListModelsAndStartSession(t *testing.T) {
	p := newBedrockBearerIntegrationProvider(t)

	models, err := p.ListModels(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "Signature expired") {
			t.Skipf("skipping due to expired Bedrock signature: %v", err)
		}
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) == 0 {
		t.Fatal("expected at least one Bedrock model")
	}

	t.Logf("listed %d models", len(models))

	model, result, err := runBedrockResponsesProbe(t, p, models)
	if err != nil {
		t.Fatalf("Bedrock responses probe: %v", err)
	}

	t.Logf("response_id=%q model=%q finish=%s text=%q", result.Response.ID, model.ID, result.FinishReason, result.Text)

	if result.Response.ID == "" {
		t.Error("expected non-empty response id for Bedrock session")
	}
	if result.Text == "" {
		t.Error("expected non-empty text")
	}
}

func runBedrockResponsesProbe(t *testing.T, p *responses.Provider, models []sdk.Model) (*sdk.Model, sdk.ModelResult, error) {
	t.Helper()

	candidates := make([]string, 0, len(models))
	preferred := os.Getenv("AWS_BEDROCK_MODEL")
	if preferred != "" {
		candidates = append(candidates, preferred)
	}

	seen := map[string]bool{}
	for _, model := range models {
		if !seen[model.ID] {
			candidates = append(candidates, model.ID)
			seen[model.ID] = true
		}
	}

	var lastErr error
	for _, modelID := range candidates {
		model := p.ChatModel(modelID)
		result, err := p.DoGenerate(context.Background(), sdk.Request{
			Model:    model.ID,
			Messages: []sdk.Message{sdk.UserMessage("Reply with exactly: ok")},
		})
		if err == nil {
			t.Logf("selected Bedrock responses model %q", modelID)
			return model, result, nil
		}

		if strings.Contains(err.Error(), "does not support the '/v1/responses' API") {
			t.Logf("skipping model %q: %v", modelID, err)
			lastErr = err
			continue
		}

		return nil, sdk.ModelResult{}, err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no Bedrock models returned from ListModels")
	}
	return nil, sdk.ModelResult{}, lastErr
}

func TestIntegration_ResponsesDoStream(t *testing.T) {
	p := newResponsesIntegrationProvider(t)
	model := responsesIntegrationModel(t, p)
	sr, err := p.DoStream(context.Background(), sdk.Request{
		Model:    model.ID,
		Messages: []sdk.Message{sdk.UserMessage("Count from 1 to 5.")},
	})
	if err != nil {
		t.Fatalf("DoStream: %v", err)
	}

	var text string
	for part := range sr {
		switch p := part.(type) {
		case *sdk.TextDeltaPart:
			text += p.Text
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

func TestIntegration_ResponsesDoGenerate_Reasoning(t *testing.T) {
	p := newResponsesIntegrationProvider(t)
	model := p.ChatModel(openRouterResponsesReasoningModel)
	effort := "low"
	summary := "auto"
	result, err := p.DoGenerate(context.Background(), sdk.Request{
		Model:            model.ID,
		Messages:         []sdk.Message{sdk.UserMessage("What is 15 * 37? Think step by step.")},
		ReasoningEffort:  &effort,
		ReasoningSummary: &summary,
	})
	if err != nil {
		t.Fatalf("DoGenerate: %v", err)
	}
	t.Logf("text=%q", result.Text)
	t.Logf("reasoning=%q", result.Reasoning)
	t.Logf("finish=%s tokens=%d/%d reasoning_tokens=%d",
		result.FinishReason, result.Usage.InputTokens, result.Usage.OutputTokens,
		result.Usage.ReasoningTokens)

	if result.Text == "" {
		t.Error("expected non-empty text")
	}
}

func TestIntegration_ResponsesDoStream_Reasoning(t *testing.T) {
	p := newResponsesIntegrationProvider(t)
	model := p.ChatModel(openRouterResponsesReasoningModel)
	effort := "low"
	summary := "auto"
	sr, err := p.DoStream(context.Background(), sdk.Request{
		Model:            model.ID,
		Messages:         []sdk.Message{sdk.UserMessage("What is 15 * 37? Think step by step.")},
		ReasoningEffort:  &effort,
		ReasoningSummary: &summary,
	})
	if err != nil {
		t.Fatalf("DoStream: %v", err)
	}

	var text, reasoning string
	var gotReasoningStart, gotReasoningEnd bool
	events := make([]sdk.StreamPartType, 0, 8)
	for part := range sr {
		events = append(events, part.Type())
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
			t.Logf("finish=%s tokens=%d/%d reasoning_tokens=%d",
				p.FinishReason, p.TotalUsage.InputTokens, p.TotalUsage.OutputTokens,
				p.TotalUsage.ReasoningTokens)
		}
	}

	t.Logf("reasoning=%q", reasoning)
	t.Logf("text=%q", text)
	t.Logf("events=%v", events)

	if text == "" {
		t.Error("expected non-empty text")
	}
	if !gotReasoningStart {
		t.Log("WARN: no ReasoningStartPart (model may not emit reasoning summary)")
	}
	if gotReasoningStart && !gotReasoningEnd {
		t.Error("got ReasoningStartPart but no ReasoningEndPart")
	}
}

func TestIntegration_ResponsesDoGenerate_ToolCall(t *testing.T) {
	p := newResponsesIntegrationProvider(t)
	model := responsesIntegrationModel(t, p)
	result, err := p.DoGenerate(context.Background(), sdk.Request{
		Model:    model.ID,
		Messages: []sdk.Message{sdk.UserMessage("What's the weather in Tokyo right now?")},
		Tools: []sdk.ToolDefinition{{
			Name:        "get_weather",
			Description: "Get current weather for a city",
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

	t.Logf("text=%q finish=%s tool_calls=%d", result.Text, result.FinishReason, len(result.ToolCalls))
	for i, tc := range result.ToolCalls {
		t.Logf("  tool_call[%d]: id=%s name=%s input=%v", i, tc.ToolCallID, tc.ToolName, tc.Input)
	}

	if result.FinishReason != sdk.FinishReasonToolCalls {
		t.Errorf("expected tool-calls finish, got %q", result.FinishReason)
	}
	if len(result.ToolCalls) == 0 {
		t.Error("expected at least one tool call")
	}
}

func TestIntegration_ResponsesDoStream_ToolCall(t *testing.T) {
	p := newResponsesIntegrationProvider(t)
	model := responsesIntegrationModel(t, p)
	sr, err := p.DoStream(context.Background(), sdk.Request{
		Model:    model.ID,
		Messages: []sdk.Message{sdk.UserMessage("What's the weather in Tokyo right now?")},
		Tools: []sdk.ToolDefinition{{
			Name:        "get_weather",
			Description: "Get current weather for a city",
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
		t.Fatalf("DoStream: %v", err)
	}

	var toolCalls []sdk.StreamToolCallPart
	events := make([]sdk.StreamPartType, 0, 8)
	for part := range sr {
		events = append(events, part.Type())
		switch p := part.(type) {
		case *sdk.StreamToolCallPart:
			toolCalls = append(toolCalls, *p)
		case *sdk.ErrorPart:
			t.Fatalf("stream error: %v", p.Error)
		case *sdk.FinishPart:
			t.Logf("finish=%s", p.FinishReason)
		}
	}

	t.Logf("events=%v", events)
	for i, tc := range toolCalls {
		t.Logf("  tool_call[%d]: id=%s name=%s input=%v", i, tc.ToolCallID, tc.ToolName, tc.Input)
	}

	if len(toolCalls) == 0 {
		t.Error("expected at least one tool call")
	}
}
