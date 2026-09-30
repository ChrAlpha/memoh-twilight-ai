package sdkdiff_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/felinics/twilight/provider/anthropic/messages"
	"github.com/felinics/twilight/sdk"
)

// anthropicTypes are the error types of the ErrorResponse schema in
// https://github.com/anthropics/anthropic-sdk-go/blob/ad865dfa3d1a8d2f4a7ad0d072011e811e9957a9/scripts/mock-spec.json.gz
// (MIT, Copyright 2023 Anthropic, PBC.) with the statuses of
// https://platform.claude.com/docs/en/api/errors.
var anthropicTypes = map[string]int{
	"invalid_request_error": http.StatusBadRequest,
	"authentication_error":  http.StatusUnauthorized,
	"billing_error":         http.StatusPaymentRequired,
	"permission_error":      http.StatusForbidden,
	"not_found_error":       http.StatusNotFound,
	"rate_limit_error":      http.StatusTooManyRequests,
	"api_error":             http.StatusInternalServerError,
	"timeout_error":         http.StatusGatewayTimeout,
	"overloaded_error":      529,
}

func anthropicProvider(url string) sdk.Provider {
	return messages.New(messages.WithAPIKey("sk-ant-test"), messages.WithBaseURL(url))
}

func officialAnthropic(url string) anthropic.Client {
	return anthropic.NewClient(option.WithAPIKey("sk-ant-test"), option.WithBaseURL(url), option.WithMaxRetries(0))
}

var anthropicParams = anthropic.MessageNewParams{
	Model:     "claude-sonnet-4-5",
	MaxTokens: 16,
	Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("hi"))},
}

func officialAnthropicError(t *testing.T, err error) *anthropic.Error {
	t.Helper()
	var official *anthropic.Error
	if !errors.As(err, &official) {
		t.Fatalf("official SDK: error %v (%T) is not *anthropic.Error", err, err)
	}
	return official
}

// The official SDK does not parse the message, so only the status, type and
// request ID are compared. The request-id header differs from the body's
// request_id to show that both read the header.
func TestAnthropicHTTPErrors(t *testing.T) {
	for typ, status := range anthropicTypes {
		t.Run(typ, func(t *testing.T) {
			url := serve(t, reply{
				Status: status,
				Header: http.Header{"Request-Id": {"req_header"}},
				Body:   `{"type":"error","error":{"type":"` + typ + `","message":"m"},"request_id":"req_body"}`,
			})
			got := twilightGenerate(t, anthropicProvider(url), "claude-sonnet-4-5")
			client := officialAnthropic(url)
			_, err := client.Messages.New(context.Background(), anthropicParams)
			official := officialAnthropicError(t, err)
			agree(t,
				field{"StatusCode", got.StatusCode, official.StatusCode},
				field{"Type", got.Type, string(official.Type())},
				field{"RequestID", got.RequestID, official.RequestID},
			)
		})
	}
}

// The official SDK reports the stream's 200 as the status of an error event;
// twilight reports 0 there by design, so the status is not compared.
func TestAnthropicStreamErrors(t *testing.T) {
	const start = `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[],"stop_reason":null,"usage":{"input_tokens":1,"output_tokens":0}}}`
	for typ := range anthropicTypes {
		t.Run(typ, func(t *testing.T) {
			url := serve(t, reply{
				Header: http.Header{"Request-Id": {"req_stream"}},
				Events: [][2]string{
					{"message_start", start},
					{"error", `{"type":"error","error":{"type":"` + typ + `","message":"m"}}`},
				},
			})
			got := twilightStream(t, anthropicProvider(url), "claude-sonnet-4-5")
			client := officialAnthropic(url)
			stream := client.Messages.NewStreaming(context.Background(), anthropicParams)
			for stream.Next() {
			}
			official := officialAnthropicError(t, stream.Err())
			agree(t,
				field{"Type", got.Type, string(official.Type())},
				field{"RequestID", got.RequestID, official.RequestID},
			)
		})
	}
}
