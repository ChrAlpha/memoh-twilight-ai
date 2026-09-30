package providertest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felinics/twilight/sdk"
)

// testHealth covers Provider.Test. The replies are
// provider-neutral: an empty JSON object decodes as an empty models listing
// and is a 2xx for a probe, and a 401 with an empty object carries no
// provider type or code, so the status alone classifies it.
func testHealth(t *testing.T, f Fixture) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		p, rec := serve(t, f, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		})
		if err := p.Test(ctx); err != nil {
			t.Fatalf("Test() = %v, want nil", err)
		}
		if hits, _ := rec.wire(); hits == 0 {
			t.Fatal("Test() sent no request")
		}
	})
	t.Run("unauthorized", func(t *testing.T) {
		p, _ := serve(t, f, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{}`))
		})
		err := p.Test(ctx)
		var apiErr *sdk.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("Test() = %v (%T), want an error that unwraps to *sdk.APIError", err, err)
		}
		if apiErr.StatusCode != http.StatusUnauthorized {
			t.Errorf("APIError.StatusCode = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
		}
		if apiErr.Kind != sdk.KindAuthentication {
			t.Errorf("APIError.Kind = %q, want %q", apiErr.Kind, sdk.KindAuthentication)
		}
		if apiErr.Provider != p.Name() {
			t.Errorf("APIError.Provider = %q, want %q", apiErr.Provider, p.Name())
		}
	})
	t.Run("unreachable", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		srv.Close()
		if f.NewProvider == nil {
			t.Fatal("fixture has no NewProvider")
		}
		err := f.NewProvider(srv.URL).Test(ctx)
		if err == nil {
			t.Fatal("Test() against a closed server = nil, want an error")
		}
		var apiErr *sdk.APIError
		if errors.As(err, &apiErr) {
			t.Fatalf("Test() against a closed server = APIError %v, want a transport error", apiErr)
		}
	})
}

// testModelProbe covers TestModel's generation probe. A provider either probes
// directly or first looks the model up with a GET and probes only when that
// is a 404, so every GET here answers 404 and the probe, a POST, answers with
// the status under test.
func testModelProbe(t *testing.T, f Fixture) {
	ctx := context.Background()
	probe := func(status int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				w.WriteHeader(http.StatusNotFound)
			} else {
				w.WriteHeader(status)
			}
			_, _ = w.Write([]byte(`{}`))
		}
	}
	for _, c := range []struct {
		status    int
		supported bool
	}{
		{http.StatusBadRequest, true},
		{http.StatusUnprocessableEntity, true},
		{http.StatusTooManyRequests, true},
		{http.StatusNotFound, false},
	} {
		t.Run(http.StatusText(c.status), func(t *testing.T) {
			p, _ := serve(t, f, probe(c.status))
			got, err := p.TestModel(ctx, f.ModelID)
			if err != nil {
				t.Fatalf("TestModel() error = %v, want a result", err)
			}
			if got.Supported != c.supported {
				t.Errorf("TestModel().Supported = %v, want %v", got.Supported, c.supported)
			}
		})
	}
	t.Run("unauthorized", func(t *testing.T) {
		p, _ := serve(t, f, probe(http.StatusUnauthorized))
		got, err := p.TestModel(ctx, f.ModelID)
		if err == nil {
			t.Fatalf("TestModel() = %+v, want an error", got)
		}
		var apiErr *sdk.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("TestModel() error = %v (%T), want one that unwraps to *sdk.APIError", err, err)
		}
		if apiErr.Kind != sdk.KindAuthentication || apiErr.Provider != p.Name() {
			t.Errorf("APIError = %+v, want Kind %q and Provider %q", apiErr, sdk.KindAuthentication, p.Name())
		}
	})
}
