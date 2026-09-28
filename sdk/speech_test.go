package sdk

import (
	"errors"
	"testing"
)

func TestSpeechStreamResultErr(t *testing.T) {
	failure := errors.New("read failed")
	cases := []struct {
		name    string
		errCh   func() <-chan error
		wantErr error
	}{
		{"closed without error", func() <-chan error {
			ch := make(chan error)
			close(ch)
			return ch
		}, nil},
		{"one error", func() <-chan error {
			ch := make(chan error, 1)
			ch <- failure
			close(ch)
			return ch
		}, failure},
		{"nil channel", func() <-chan error { return nil }, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stream := make(chan []byte, 2)
			stream <- []byte("ab")
			stream <- []byte("c")
			close(stream)
			r := NewSpeechStreamResult(stream, "audio/mpeg", c.errCh())

			audio, err := r.Bytes()
			if string(audio) != "abc" {
				t.Errorf("Bytes audio = %q, want %q", audio, "abc")
			}
			if !errors.Is(err, c.wantErr) {
				t.Errorf("Bytes err = %v, want %v", err, c.wantErr)
			}
			// Err reports the same error on every call, after Bytes has
			// consumed the channel.
			for range 2 {
				if got := r.Err(); !errors.Is(got, c.wantErr) {
					t.Errorf("Err() = %v, want %v", got, c.wantErr)
				}
			}
		})
	}
}

// A caller that ranges over Stream itself learns about truncation from Err.
func TestSpeechStreamResultErrAfterRange(t *testing.T) {
	failure := errors.New("connection reset")
	stream := make(chan []byte)
	errCh := make(chan error, 1)
	go func() {
		stream <- []byte("partial")
		errCh <- failure
		close(stream)
		close(errCh)
	}()
	r := NewSpeechStreamResult(stream, "audio/mpeg", errCh)
	var got []byte
	for chunk := range r.Stream {
		got = append(got, chunk...)
	}
	if string(got) != "partial" {
		t.Errorf("audio = %q", got)
	}
	if err := r.Err(); !errors.Is(err, failure) {
		t.Errorf("Err() = %v, want %v", err, failure)
	}
}
