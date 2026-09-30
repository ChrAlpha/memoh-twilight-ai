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

// WithClientRequestID returns a context that sends id, an identifier the caller
// chooses, with each provider request made with the context. A provider that
// records it can find a request that failed without a response, for example one
// that timed out, so quote the ID in support requests.
//
// The OpenAI providers except Codex send it as X-Client-Request-Id, whichever
// endpoint they are configured for; OpenCode Go sends it for models routed to
// Completions or Responses. Ark video sends X-Client-Request-Id and GitHub
// Copilot sends X-Request-Id. Other providers do not send it. It overrides a
// header of the same name set with WithRequestHeaders. An empty id clears an
// inherited one.
//
// OpenAI expects a unique ID per request, of at most 512 ASCII characters.
// Derive a context with a new ID for every call rather than setting one on a
// context shared by a conversation. GenerateVideo sends it only with the
// request that creates the job.
func WithClientRequestID(ctx context.Context, id string) context.Context {
	return reqheaders.WithClientRequestID(ctx, id)
}
