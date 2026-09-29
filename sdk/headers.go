package sdk

import (
	"context"

	"github.com/felinics/twilight/internal/reqheaders"
)

// WithRequestHeaders returns a context carrying HTTP headers for provider
// requests. It copies the supplied map and merges it with inherited headers;
// the new values override inherited values, ignoring header name casing.
// Request headers override provider-level WithHeaders and default headers.
// Transport-required headers (SSE, JSON and multipart content types) and
// signing are applied last.
//
// Use a separate context per conversation for session identifiers, and pass the
// same context to every call in that conversation, including the calls that
// answer tool results. The context can be passed to generation, streaming,
// model listing and probes. Every provider called with the context receives
// these headers, so keep credentials in provider options.
//
// Supported by Anthropic Messages, all OpenAI providers, Google Generative AI,
// GitHub Copilot and OpenCode Go. Other providers may ignore these headers.
func WithRequestHeaders(ctx context.Context, headers map[string]string) context.Context {
	return reqheaders.WithContext(ctx, headers)
}
