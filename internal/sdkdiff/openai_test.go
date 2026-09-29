package sdkdiff_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"

	"github.com/felinics/twilight/provider/openai/completions"
	twresponses "github.com/felinics/twilight/provider/openai/responses"
	"github.com/felinics/twilight/sdk"
)

// The bodies follow the Error schema of
// https://github.com/openai/openai-openapi/blob/b6059fc737ac846e8ba64ad65f4f2c8bc0d15854/openapi.yaml
// with codes from https://platform.openai.com/docs/guides/error-codes. The
// 401 body is verbatim from the official .NET SDK's recording:
// https://github.com/openai/openai-dotnet/blob/4e5ae90621089e0b1f5571546a75cea6b0cc713f/tests/SessionRecords/ChatTests/AuthFailure.json
var openAIHTTPErrors = map[string]reply{
	"invalid api key": {Status: http.StatusUnauthorized, Header: http.Header{"X-Request-Id": {"req_401"}},
		Body: `{"error":{"message":"Incorrect API key provided: not-a-re**************************ized. You can find your API key at https://platform.openai.com/account/api-keys.","type":"invalid_request_error","param":null,"code":"invalid_api_key"}}`},
	"insufficient quota": {Status: http.StatusTooManyRequests, Header: http.Header{"X-Request-Id": {"req_429"}},
		Body: `{"error":{"message":"You exceeded your current quota, please check your plan and billing details.","type":"insufficient_quota","param":null,"code":"insufficient_quota"}}`},
	"null code": {Status: http.StatusInternalServerError,
		Body: `{"error":{"message":"The server had an error while processing your request.","type":"server_error","param":null,"code":null}}`},
	"not json": {Status: http.StatusBadGateway, Body: `<html>Bad Gateway</html>`},
}

func officialOpenAI(baseURL string) openai.Client {
	return openai.NewClient(option.WithAPIKey("sk-test"), option.WithBaseURL(baseURL), option.WithMaxRetries(0))
}

func wantOpenAIAgree(t *testing.T, got *sdk.APIError, err error) {
	t.Helper()
	var official *openai.Error
	if !errors.As(err, &official) {
		t.Fatalf("official SDK: error %v (%T) is not *openai.Error", err, err)
	}
	agree(t,
		field{"StatusCode", got.StatusCode, official.StatusCode},
		field{"Type", got.Type, official.Type},
		field{"Code", got.Code, official.Code},
		field{"Message", got.Message, official.Message},
		field{"RequestID", got.RequestID, official.Response.Header.Get("x-request-id")},
	)
}

func TestOpenAIChatCompletionsHTTPErrors(t *testing.T) {
	for name, r := range openAIHTTPErrors {
		t.Run(name, func(t *testing.T) {
			url := serve(t, r)
			got := twilightGenerate(t, completions.New(completions.WithAPIKey("sk-test"), completions.WithBaseURL(url)), "gpt-4o")
			client := officialOpenAI(url)
			_, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
				Model:    "gpt-4o",
				Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("hi")},
			})
			wantOpenAIAgree(t, got, err)
		})
	}
}

func TestOpenAIResponsesHTTPErrors(t *testing.T) {
	for name, r := range openAIHTTPErrors {
		t.Run(name, func(t *testing.T) {
			url := serve(t, r)
			got := twilightGenerate(t, twresponses.New(twresponses.WithAPIKey("sk-test"), twresponses.WithBaseURL(url)), "gpt-5")
			client := officialOpenAI(url)
			_, err := client.Responses.New(context.Background(), responses.ResponseNewParams{
				Model: "gpt-5",
				Input: responses.ResponseNewParamsInputUnion{OfString: openai.String("hi")},
			})
			wantOpenAIAgree(t, got, err)
		})
	}
}

// The error event and response.failed data are the ResponseErrorEvent and
// ResponseFailedEvent examples of the pinned openapi.yaml, compacted.
func TestOpenAIResponsesStreamErrors(t *testing.T) {
	const created = `{"type":"response.created","sequence_number":0,"response":{"id":"resp_123","object":"response","created_at":1740855869,"status":"in_progress","model":"gpt-5","output":[]}}`
	events := map[string][2]string{
		"error":           {"error", `{"type":"error","code":"ERR_SOMETHING","message":"Something went wrong","param":null,"sequence_number":1}`},
		"response.failed": {"response.failed", `{"type":"response.failed","response":{"id":"resp_123","object":"response","access_programs":null,"created_at":1740855869,"status":"failed","completed_at":null,"error":{"code":"server_error","message":"The model failed to generate a response."},"incomplete_details":null,"instructions":null,"max_output_tokens":null,"model":"gpt-6-astra","output":[],"previous_response_id":null,"reasoning_effort":null,"store":false,"temperature":1,"text":{"format":{"type":"text"}},"tool_choice":"auto","tools":[],"top_p":1,"truncation":"disabled","usage":null,"user":null,"metadata":{},"parallel_tool_calls":true},"sequence_number":1}`},
	}
	for name, event := range events {
		t.Run(name, func(t *testing.T) {
			url := serve(t, reply{Events: [][2]string{{"response.created", created}, event}})
			got := twilightStream(t, twresponses.New(twresponses.WithAPIKey("sk-test"), twresponses.WithBaseURL(url)), "gpt-5")

			client := officialOpenAI(url)
			stream := client.Responses.NewStreaming(context.Background(), responses.ResponseNewParams{
				Model: "gpt-5",
				Input: responses.ResponseNewParamsInputUnion{OfString: openai.String("hi")},
			})
			var code, message string
			for stream.Next() {
				switch e := stream.Current(); e.Type {
				case "error":
					code, message = e.Code, e.Message
				case "response.failed":
					code, message = string(e.Response.Error.Code), e.Response.Error.Message
				}
			}
			if err := stream.Err(); err != nil {
				t.Fatalf("official SDK: stream: %v", err)
			}
			agree(t,
				field{"StatusCode", got.StatusCode, 0},
				field{"Code", got.Code, code},
				field{"Message", got.Message, message},
			)
		})
	}
}
