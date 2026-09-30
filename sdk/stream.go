package sdk

import "errors"

type StreamPartType string

const (
	StreamPartTypeTextStart      StreamPartType = "text-start"
	StreamPartTypeTextDelta      StreamPartType = "text-delta"
	StreamPartTypeTextEnd        StreamPartType = "text-end"
	StreamPartTypeReasoningStart StreamPartType = "reasoning-start"
	StreamPartTypeReasoningDelta StreamPartType = "reasoning-delta"
	StreamPartTypeReasoningEnd   StreamPartType = "reasoning-end"
	StreamPartTypeToolInputStart StreamPartType = "tool-input-start"
	StreamPartTypeToolInputDelta StreamPartType = "tool-input-delta"
	StreamPartTypeToolInputEnd   StreamPartType = "tool-input-end"
	StreamPartTypeToolCall       StreamPartType = "tool-call"
	StreamPartTypeSource         StreamPartType = "source"
	StreamPartTypeFile           StreamPartType = "file"
	StreamPartTypeStart          StreamPartType = "start"
	StreamPartTypeFinish         StreamPartType = "finish"
	StreamPartTypeStartStep      StreamPartType = "start-step"
	StreamPartTypeFinishStep     StreamPartType = "finish-step"
	StreamPartTypeError          StreamPartType = "error"
)

// StreamPart is the interface implemented by all stream chunk types.
// Consumers should use a type switch to handle specific part types.
type StreamPart interface {
	Type() StreamPartType
}

// --- Text ---

type TextStartPart struct {
	ID               string
	ProviderMetadata ProviderMetadata
}

func (p *TextStartPart) Type() StreamPartType { return StreamPartTypeTextStart }

type TextDeltaPart struct {
	ID               string
	Text             string
	ProviderMetadata ProviderMetadata
}

func (p *TextDeltaPart) Type() StreamPartType { return StreamPartTypeTextDelta }

type TextEndPart struct {
	ID               string
	ProviderMetadata ProviderMetadata
}

func (p *TextEndPart) Type() StreamPartType { return StreamPartTypeTextEnd }

// --- Reasoning ---

type ReasoningStartPart struct {
	ID               string
	Model            string
	Format           ReasoningFormat
	ProviderMetadata ProviderMetadata
}

func (p *ReasoningStartPart) Type() StreamPartType { return StreamPartTypeReasoningStart }

type ReasoningDeltaPart struct {
	ID               string
	Model            string
	Text             string
	Format           ReasoningFormat
	ProviderMetadata ProviderMetadata
}

func (p *ReasoningDeltaPart) Type() StreamPartType { return StreamPartTypeReasoningDelta }

type ReasoningEndPart struct {
	ID               string
	Model            string
	Format           ReasoningFormat
	ProviderMetadata ProviderMetadata
}

func (p *ReasoningEndPart) Type() StreamPartType { return StreamPartTypeReasoningEnd }

// --- Tool Input ---

type ToolInputStartPart struct {
	ID               string
	ToolName         string
	ProviderMetadata ProviderMetadata
}

func (p *ToolInputStartPart) Type() StreamPartType { return StreamPartTypeToolInputStart }

type ToolInputDeltaPart struct {
	ID               string
	Delta            string
	ProviderMetadata ProviderMetadata
}

func (p *ToolInputDeltaPart) Type() StreamPartType { return StreamPartTypeToolInputDelta }

type ToolInputEndPart struct {
	ID               string
	ProviderMetadata ProviderMetadata
}

func (p *ToolInputEndPart) Type() StreamPartType { return StreamPartTypeToolInputEnd }

// --- Tool Call ---

type StreamToolCallPart struct {
	ToolCallID       string
	ToolName         string
	Input            ToolArguments
	ProviderMetadata ProviderMetadata
}

func (p *StreamToolCallPart) Type() StreamPartType { return StreamPartTypeToolCall }

// --- Source & File ---

type StreamSourcePart struct {
	Source Source
}

func (p *StreamSourcePart) Type() StreamPartType { return StreamPartTypeSource }

type StreamFilePart struct {
	File GeneratedFile
}

func (p *StreamFilePart) Type() StreamPartType { return StreamPartTypeFile }

// --- Lifecycle ---

type StartPart struct{}

func (p *StartPart) Type() StreamPartType { return StreamPartTypeStart }

type FinishPart struct {
	FinishReason    FinishReason
	RawFinishReason string
	TotalUsage      Usage
}

func (p *FinishPart) Type() StreamPartType { return StreamPartTypeFinish }

type StartStepPart struct{}

func (p *StartStepPart) Type() StreamPartType { return StreamPartTypeStartStep }

type FinishStepPart struct {
	FinishReason     FinishReason
	RawFinishReason  string
	Usage            Usage
	Response         ResponseMetadata
	ProviderMetadata ProviderMetadata
}

func (p *FinishStepPart) Type() StreamPartType { return StreamPartTypeFinishStep }

// ErrorPart reports that the stream failed. A stream carries at most one
// ErrorPart. The FinishPart that closes a failed stream follows it, with
// FinishReasonError and the usage reported before the failure.
//
// Error is an *APIError when the provider reported the failure, including in
// an error event after the stream started (StatusCode 0), and wraps
// ErrStreamIncomplete when the stream ended before its terminal event.
type ErrorPart struct {
	Error error
}

func (p *ErrorPart) Type() StreamPartType { return StreamPartTypeError }

// ErrStreamIncomplete reports that a provider's stream ended without a
// transport error but before the event that marks a complete response, such
// as Anthropic's message_stop or the Responses API's response.completed. The
// parts received before it are a truncated response. Test for it with
// errors.Is.
var ErrStreamIncomplete = errors.New("twilightai: stream ended before its terminal event")
