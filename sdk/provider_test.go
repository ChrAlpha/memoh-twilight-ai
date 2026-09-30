package sdk

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestClassifyProbe(t *testing.T) {
	transport := errors.New("dial tcp: connection refused")
	status := func(code int) error {
		return fmt.Errorf("probe: %w", &APIError{Provider: "p", StatusCode: code, Kind: KindUnknown})
	}
	for _, c := range []struct {
		name      string
		err       error
		supported bool
		wantErr   bool
	}{
		{"2xx", nil, true, false},
		{"400", status(http.StatusBadRequest), true, false},
		{"422", status(http.StatusUnprocessableEntity), true, false},
		{"429", status(http.StatusTooManyRequests), true, false},
		{"404", status(http.StatusNotFound), false, false},
		{"401", status(http.StatusUnauthorized), false, true},
		{"403", status(http.StatusForbidden), false, true},
		{"500", status(http.StatusInternalServerError), false, true},
		{"transport", transport, false, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := ClassifyProbe(c.err)
			if c.wantErr {
				if !errors.Is(err, c.err) {
					t.Fatalf("ClassifyProbe() error = %v, want the input error returned unchanged", err)
				}
				if got != nil {
					t.Fatalf("ClassifyProbe() result = %+v, want nil", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ClassifyProbe() error = %v, want nil", err)
			}
			if got == nil || got.Supported != c.supported {
				t.Fatalf("ClassifyProbe() = %+v, want Supported %v", got, c.supported)
			}
		})
	}
}

func TestClassifyProbeKeepsAuthenticationKind(t *testing.T) {
	in := &APIError{Provider: "p", StatusCode: http.StatusUnauthorized, Kind: KindAuthentication}
	_, err := ClassifyProbe(in)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr != in {
		t.Fatalf("ClassifyProbe() error = %v, want the *APIError it was given", err)
	}
	if KindOf(err) != KindAuthentication {
		t.Fatalf("KindOf = %q, want %q", KindOf(err), KindAuthentication)
	}
}
