package opencodego

// Protocol identifies the wire protocol used by an OpenCode Go model.
type Protocol string

const (
	ProtocolCompletions Protocol = "openai-completions"
	ProtocolResponses   Protocol = "openai-responses"
	ProtocolMessages    Protocol = "anthropic-messages"
)

// protocolExceptions lists the models that do not use Chat Completions, taken
// from https://opencode.ai/docs/go/#endpoints on 2026-09-28, less the models
// the live /models endpoint no longer lists. Every other model uses
// Completions, as in OpenCode itself: the model list changes about twice a week
// and most new models are served through Completions, so a full directory
// would be stale more often than this table. WithModelProtocols covers a new
// exception before the SDK is updated.
var protocolExceptions = map[string]Protocol{
	"grok-4.7":                   ProtocolResponses,
	"grok-4.6":                   ProtocolResponses,
	"gpt-6-luna":                 ProtocolResponses,
	"gpt-5.6-luna":               ProtocolResponses,
	"muse-spark-1.3-contributor": ProtocolResponses,
	"muse-spark-1.2-contributor": ProtocolResponses,
	"minimax-m3":                 ProtocolMessages,
	"minimax-m2.7":               ProtocolMessages,
	"qwen3.8-max":                ProtocolMessages,
	"qwen3.8-flash":              ProtocolMessages,
	"qwen3.7-plus":               ProtocolMessages,
}
