package generativeai_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/felinics/twilight/provider/google/generativeai"
	"github.com/felinics/twilight/provider/providertest"
	"github.com/felinics/twilight/sdk"
)

// This file runs the seam conformance suite (provider/providertest) against the
// Google Generative AI (Gemini) wire format. The suite reaches the provider
// only through sdk.Generate and sdk.Stream, so these fixtures stay valid across
// a change to the provider interface itself.
//
// The canned wire data below mirrors shapes already proven by
// generativeai_test.go: candidates[].content.parts[] for generateContent
// (generativeai_test.go:37-50, :178-197) and an SSE stream of the same
// candidate objects for streamGenerateContent (generativeai_test.go:587-595,
// :647-649). The model travels in the URL path
// ("/models/<id>:generateContent", generativeai_test.go:22 and :574); the suite
// counts the URL as part of what reached the wire (providertest.go:129-132).

const (
	conformanceModel    = "gemini-2.0-flash"
	conformanceAPIKey   = "test-key"
	conformanceToolName = "providertest_tool_marker_2d91"
)

func conformanceProvider(baseURL string) sdk.Provider {
	return generativeai.New(
		generativeai.WithAPIKey(conformanceAPIKey),
		generativeai.WithBaseURL(baseURL),
	)
}

// assertGoogleRequest pins the transport the provider must use: the model in
// the URL path, the API key header, and alt=sse on the streaming endpoint
// (generativeai_test.go:574-579). t.Errorf is safe from the server goroutine.
func assertGoogleRequest(t *testing.T, r *http.Request, method string) {
	t.Helper()
	if want := "/models/" + conformanceModel + ":" + method; r.URL.Path != want {
		t.Errorf("google request path = %q, want %q", r.URL.Path, want)
	}
	if got := r.Header.Get("x-goog-api-key"); got != conformanceAPIKey {
		t.Errorf("x-goog-api-key = %q, want %q", got, conformanceAPIKey)
	}
	if method == "streamGenerateContent" && r.URL.Query().Get("alt") != "sse" {
		t.Errorf("alt query = %q, want %q", r.URL.Query().Get("alt"), "sse")
	}
}

// googleJSON answers with a non-streaming generateResponse.
func googleJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

// googleSSE writes the SSE framing utils.FetchSSE reads: one bare
// "data: <candidate object>" per event and no event name
// (generativeai_test.go:592-595).
func googleSSE(w http.ResponseWriter, chunks ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	for _, chunk := range chunks {
		_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

// googleErrorBody is the google.rpc.Status example from Google's API design
// guide, verbatim: https://cloud.google.com/apis/design/errors (Error model).
const googleErrorBody = `{
  "error": {
    "code": 429,
    "message": "The zone 'us-east1-a' does not have enough resources available to fulfill the request. Try a different zone, or try again later.",
    "status": "RESOURCE_EXHAUSTED",
    "details": [
      {
        "@type": "type.googleapis.com/google.rpc.ErrorInfo",
        "reason": "RESOURCE_AVAILABILITY",
        "domain": "compute.googleapis.com",
        "metadata": {
          "zone": "us-east1-a",
          "vmType": "e2-medium",
          "attachment": "local-ssd=3,nvidia-t4=2",
          "zonesWithCapacity": "us-central1-f,us-central1-c"
        }
      }
    ]
  }
}`

// googleErrorHandler answers both the generate and the stream request with
// the google.rpc.Status error body, which utils.FetchJSON and utils.FetchSSE
// turn into an *sdk.APIError.
func googleErrorHandler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		method := "generateContent"
		if strings.HasSuffix(r.URL.Path, ":streamGenerateContent") {
			method = "streamGenerateContent"
		}
		assertGoogleRequest(t, r, method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(googleErrorBody))
	}
}

// googleWantError is what googleErrorBody decodes to. The ErrorInfo reason is
// not one the SDK maps, so Kind comes from the RESOURCE_EXHAUSTED status. The
// Gemini API documents no request ID header.
func googleWantError() *sdk.APIError {
	return &sdk.APIError{
		Provider:   "google-generative-ai",
		StatusCode: http.StatusTooManyRequests,
		Type:       "RESOURCE_EXHAUSTED",
		Code:       "RESOURCE_AVAILABILITY",
		Message:    "The zone 'us-east1-a' does not have enough resources available to fulfill the request. Try a different zone, or try again later.",
		Kind:       sdk.KindRateLimited,
	}
}

// textFixture answers with plain text on both paths. The streamed answer
// arrives split across two data events, so this also covers delta
// accumulation.
func textFixture(t *testing.T) providertest.Fixture {
	return providertest.Fixture{
		NewProvider: conformanceProvider,
		ModelID:     conformanceModel,
		Options:     json.RawMessage(`{"generationConfig":{"temperature":0.42}}`),
		Reply: func(w http.ResponseWriter, r *http.Request) {
			assertGoogleRequest(t, r, "generateContent")
			googleJSON(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"conformance text"}]},"finishReason":"STOP"}],`+
				`"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7}}`)
		},
		ReplyStream: func(w http.ResponseWriter, r *http.Request) {
			assertGoogleRequest(t, r, "streamGenerateContent")
			googleSSE(w,
				`{"candidates":[{"content":{"role":"model","parts":[{"text":"conformance "}]}}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":1,"totalTokenCount":6}}`,
				`{"candidates":[{"content":{"role":"model","parts":[{"text":"text"}]}}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7}}`,
				`{"candidates":[{"content":{"role":"model","parts":[{"text":""}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7}}`,
			)
		},
		ReplyError: googleErrorHandler(t),
		WantError:  googleWantError(),
		Secret:     conformanceAPIKey,
		Want: providertest.Want{
			Text:         "conformance text",
			FinishReason: sdk.FinishReasonStop,
			TotalTokens:  7,
		},
	}
}

// toolCallFixture answers with one function call on both paths.
//
// The reply carries no tool-call id because the Google wire cannot: contentPart
// models functionCall as {name,args} only (types.go:39-42), so the provider
// mints an id per response with generateID (generativeai.go:522 for generate,
// :699 for stream, :832-833 for the generator). Want therefore leaves
// ToolCallID empty, which tells the suite to require a generated non-empty id
// instead of comparing one (providertest.go:305-309).
func toolCallFixture(t *testing.T) providertest.Fixture {
	const toolReply = `{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"` + conformanceToolName + `","args":{"city":"Paris"}}}]},"finishReason":"STOP"}],` +
		`"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":10,"totalTokenCount":30}}`
	return providertest.Fixture{
		NewProvider: conformanceProvider,
		ModelID:     conformanceModel,
		Reply: func(w http.ResponseWriter, r *http.Request) {
			assertGoogleRequest(t, r, "generateContent")
			googleJSON(w, toolReply)
		},
		ReplyStream: func(w http.ResponseWriter, r *http.Request) {
			assertGoogleRequest(t, r, "streamGenerateContent")
			// Google streams one whole functionCall part per event and the
			// provider emits its tool call from that single part, so this is
			// the same body generativeai_test.go:648 feeds it.
			googleSSE(w, toolReply)
		},
		Want: providertest.Want{
			FinishReason: sdk.FinishReasonToolCalls,
			TotalTokens:  30,
			ToolCalls: []sdk.ToolCall{{
				ToolName: conformanceToolName,
				Input:    sdk.ParseToolArguments(`{"city":"Paris"}`),
			}},
		},
	}
}

// Caps is the zero value for both fixtures: the Google wire carries
// finishReason and usageMetadata on both paths, and neither path carries
// response metadata (parseResponse never fills Response, and the stream's
// FinishStepPart sends an empty sdk.ResponseMetadata), so Want.Response stays
// zero and the suite compares only that both paths leave it zero.
func TestSeamConformance(t *testing.T) {
	t.Run("text", func(t *testing.T) { providertest.Run(t, textFixture) })
	t.Run("tool-call", func(t *testing.T) { providertest.Run(t, toolCallFixture) })
}
