package sdkdiff_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"google.golang.org/genai"

	"github.com/felinics/twilight/provider/google/generativeai"
)

// The first three bodies are verbatim from generativelanguage.googleapis.com
// on 2026-09-29. The RESOURCE_EXHAUSTED body is the example of
// https://cloud.google.com/apis/design/errors.
var googleHTTPErrors = map[string]reply{
	"api key invalid": {Status: http.StatusBadRequest, Body: `{
  "error": {
    "code": 400,
    "message": "API key not valid. Please pass a valid API key.",
    "status": "INVALID_ARGUMENT",
    "details": [
      {
        "@type": "type.googleapis.com/google.rpc.ErrorInfo",
        "reason": "API_KEY_INVALID",
        "domain": "googleapis.com",
        "metadata": {
          "service": "generativelanguage.googleapis.com"
        }
      },
      {
        "@type": "type.googleapis.com/google.rpc.LocalizedMessage",
        "locale": "en-US",
        "message": "API key not valid. Please pass a valid API key."
      }
    ]
  }
}`},
	"api key missing": {Status: http.StatusForbidden, Body: `{
  "error": {
    "code": 403,
    "message": "Method doesn't allow unregistered callers (callers without established identity). Please use API Key or other form of API consumer identity to call this API.",
    "status": "PERMISSION_DENIED"
  }
}`},
	"not found": {Status: http.StatusNotFound, Body: `{
  "error": {
    "code": 404,
    "message": "models/gemini-does-not-exist is not found for API version v1beta, or is not supported for generateContent. Call ModelService.ListModels to see the list of available models and their supported methods.",
    "status": "NOT_FOUND"
  }
}`},
	"resource exhausted": {Status: http.StatusTooManyRequests,
		Body: `{"error":{"code":429,"message":"The zone 'us-east1-a' does not have enough resources available to fulfill the request. Try a different zone, or try again later.","status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"RESOURCE_AVAILABILITY","domain":"compute.googleapis.com"}]}}`},
}

// go-genai keeps the ErrorInfo inside Details without naming its reason, so
// the reason is read from there for the comparison.
func TestGoogleHTTPErrors(t *testing.T) {
	for name, r := range googleHTTPErrors {
		t.Run(name, func(t *testing.T) {
			url := serve(t, r)
			got := twilightGenerate(t, generativeai.New(generativeai.WithAPIKey("key"), generativeai.WithBaseURL(url)), "gemini-2.5-flash")

			client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
				APIKey: "key", Backend: genai.BackendGeminiAPI, HTTPOptions: genai.HTTPOptions{BaseURL: url},
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Models.GenerateContent(context.Background(), "gemini-2.5-flash", genai.Text("hi"), nil)
			var official genai.APIError
			if !errors.As(err, &official) {
				t.Fatalf("official SDK: error %v (%T) is not genai.APIError", err, err)
			}
			var reason string
			for _, d := range official.Details {
				if d["@type"] == "type.googleapis.com/google.rpc.ErrorInfo" {
					reason, _ = d["reason"].(string)
				}
			}
			agree(t,
				field{"StatusCode", got.StatusCode, official.Code},
				field{"Type", got.Type, official.Status},
				field{"Code", got.Code, reason},
				field{"Message", got.Message, official.Message},
			)
		})
	}
}
