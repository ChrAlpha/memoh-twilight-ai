package copilot_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felinics/twilight/provider/github/copilot"
	"github.com/felinics/twilight/sdk"
)

func testToken() string {
	return "ghu_test_token"
}

func TestDoGenerate_AutoModelOmitsModelField(t *testing.T) {
	token := testToken()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Fatalf("unexpected auth header: %s", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if _, ok := body["model"]; ok {
			t.Fatalf("auto model should omit model field, got %#v", body["model"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "copilotcmpl-test",
			"object":  "chat.completion",
			"created": 1700000000,
			"model":   "github-managed-model",
			"choices": []map[string]any{{
				"index":         0,
				"finish_reason": "stop",
				"message":       map[string]any{"role": "assistant", "content": "Hello from Copilot!"},
			}},
			"usage": map[string]any{
				"prompt_tokens":     5,
				"completion_tokens": 4,
				"total_tokens":      9,
			},
		})
	}))
	defer srv.Close()

	p := copilot.New(
		copilot.WithGitHubToken(token),
		copilot.WithBaseURL(srv.URL),
	)

	result, err := p.DoGenerate(context.Background(), sdk.Request{
		Model:    copilot.AutoModel,
		Messages: []sdk.Message{sdk.UserMessage("Hi")},
	})
	if err != nil {
		t.Fatalf("DoGenerate failed: %v", err)
	}

	if result.Text != "Hello from Copilot!" {
		t.Fatalf("unexpected text: %q", result.Text)
	}
	if result.Response.ModelID != "github-managed-model" {
		t.Fatalf("unexpected response model: %q", result.Response.ModelID)
	}
}

func TestDoStream(t *testing.T) {
	token := testToken()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("server does not support flushing")
		}

		chunks := []string{
			`{"id":"chunk-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`,
			`{"id":"chunk-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":" Copilot"},"finish_reason":null}]}`,
			`{"id":"chunk-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`,
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", c)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	p := copilot.New(
		copilot.WithGitHubToken(token),
		copilot.WithBaseURL(srv.URL),
	)

	sr, err := p.DoStream(context.Background(), sdk.Request{
		Model:    copilot.AutoModel,
		Messages: []sdk.Message{sdk.UserMessage("Hi")},
	})
	if err != nil {
		t.Fatalf("DoStream failed: %v", err)
	}

	var collected string
	var gotStart, gotFinish bool
	for part := range sr {
		switch p := part.(type) {
		case *sdk.StartPart:
			gotStart = true
		case *sdk.TextDeltaPart:
			collected += p.Text
		case *sdk.FinishPart:
			gotFinish = true
			if p.FinishReason != sdk.FinishReasonStop {
				t.Fatalf("unexpected finish reason: %q", p.FinishReason)
			}
		}
	}

	if !gotStart {
		t.Fatal("missing StartPart")
	}
	if !gotFinish {
		t.Fatal("missing FinishPart")
	}
	if collected != "Hello Copilot" {
		t.Fatalf("unexpected collected text: %q", collected)
	}
}

func TestTestModel_ExplicitModelProbe(t *testing.T) {
	token := testToken()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["model"] != "gpt-4.1" {
			t.Fatalf("unexpected model: %#v", body["model"])
		}
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	p := copilot.New(
		copilot.WithGitHubToken(token),
		copilot.WithBaseURL(srv.URL),
	)

	result, err := p.TestModel(context.Background(), "gpt-4.1")
	if err != nil {
		t.Fatalf("TestModel failed: %v", err)
	}
	if !result.Supported {
		t.Fatalf("expected explicit model probe to be treated as supported")
	}
}

func TestListModels(t *testing.T) {
	p := copilot.New(copilot.WithGitHubToken(testToken()))
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("unexpected model count: %d", len(models))
	}
	if models[0].ID != copilot.AutoModel {
		t.Fatalf("unexpected first model: %q", models[0].ID)
	}
}
