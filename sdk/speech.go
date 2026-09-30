package sdk

import (
	"context"
	"sync"
)

// SpeechProvider is the interface that speech synthesis backends must implement.
type SpeechProvider interface {
	ListModels(ctx context.Context) ([]*SpeechModel, error)
	DoSynthesize(ctx context.Context, params SpeechParams) (*SpeechResult, error)
	DoStream(ctx context.Context, params SpeechParams) (*SpeechStreamResult, error)
}

// SpeechModel represents a speech model bound to a SpeechProvider.
type SpeechModel struct {
	ID       string
	Provider SpeechProvider
}

// SpeechParams holds the parameters for a speech synthesis request.
// Config is open-ended and provider-specific (e.g. voice, format, speed).
type SpeechParams struct {
	Model  *SpeechModel
	Text   string
	Config map[string]any
}

// SpeechResult holds the result of a non-streaming synthesis.
type SpeechResult struct {
	Audio       []byte
	ContentType string // MIME type, e.g. "audio/mpeg"
}

// SpeechStreamResult holds a channel that yields raw audio chunks.
// The channel is closed when the stream ends. A stream that ends early, because
// reading failed, the provider reported a failure or ctx was cancelled, closes
// the channel too; call Err after it is closed to tell a complete stream from
// a truncated one.
type SpeechStreamResult struct {
	Stream      <-chan []byte
	ContentType string

	errCh   <-chan error
	errOnce sync.Once
	err     error
}

// Err returns the error that ended the stream, or nil if the stream ended
// normally. Call it after Stream is closed; before that it blocks until the
// stream ends. A failure the provider reported is an *APIError; a read failure
// or a cancelled context wraps its cause.
func (r *SpeechStreamResult) Err() error {
	r.errOnce.Do(func() {
		if r.errCh != nil {
			r.err = <-r.errCh
		}
	})
	return r.err
}

// Bytes consumes the entire stream and returns the concatenated audio data,
// along with Err. On error the audio received before the failure is returned
// too.
func (r *SpeechStreamResult) Bytes() ([]byte, error) {
	var out []byte
	for chunk := range r.Stream {
		out = append(out, chunk...)
	}
	return out, r.Err()
}

// NewSpeechStreamResult creates a SpeechStreamResult from data and error
// channels. The provider sends at most one error on errCh and closes both
// channels when the stream ends; errCh may be nil for a stream that cannot
// fail once it has started.
func NewSpeechStreamResult(stream <-chan []byte, contentType string, errCh <-chan error) *SpeechStreamResult {
	return &SpeechStreamResult{
		Stream:      stream,
		ContentType: contentType,
		errCh:       errCh,
	}
}
