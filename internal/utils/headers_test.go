package utils

import (
	"context"
	"maps"
	"testing"

	"github.com/felinics/twilight/internal/reqheaders"
)

func TestAddClientRequestID(t *testing.T) {
	ctx := reqheaders.WithContext(context.Background(), map[string]string{"x-client-request-id": "stale"})
	withID := reqheaders.WithClientRequestID(ctx, "client-id")
	// A name in any casing overrides the context header of that name.
	got := AddClientRequestID(withID, RequestHeaders(withID, nil, nil), "x-client-request-id")
	if want := map[string]string{"X-Client-Request-Id": "client-id"}; !maps.Equal(got, want) {
		t.Errorf("with an ID, headers = %v, want %v", got, want)
	}
	got = AddClientRequestID(ctx, RequestHeaders(ctx, nil, nil), ClientRequestIDHeader)
	if want := map[string]string{"X-Client-Request-Id": "stale"}; !maps.Equal(got, want) {
		t.Errorf("without an ID, headers = %v, want %v", got, want)
	}
}
