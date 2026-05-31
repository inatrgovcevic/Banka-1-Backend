package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

// HeaderCorrelationID is the canonical name of the per-request id header.
const HeaderCorrelationID = "X-Correlation-Id"

type correlationKey struct{}

// CorrelationFromContext returns the correlation id stored on ctx, or empty.
func CorrelationFromContext(ctx context.Context) string {
	value, _ := ctx.Value(correlationKey{}).(string)
	return value
}

// WithCorrelation stores id on ctx. Used internally and by gRPC interceptors.
func WithCorrelation(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationKey{}, id)
}

// NewCorrelationID returns a 16-byte hex id. Falls back to a timestamp if the
// crypto/rand call ever fails (should never happen on supported platforms).
func NewCorrelationID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(buf)
}

// CorrelationMiddleware preserves an inbound X-Correlation-Id or generates
// one. It always echoes the value back in the response header and injects it
// into the request context for downstream consumers.
func CorrelationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get(HeaderCorrelationID))
		if id == "" {
			id = NewCorrelationID()
		}
		w.Header().Set(HeaderCorrelationID, id)
		next.ServeHTTP(w, r.WithContext(WithCorrelation(r.Context(), id)))
	})
}
