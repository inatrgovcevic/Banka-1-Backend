package httpx

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"banka1/go-platform/log"
)

// RecoverMiddleware catches panics, logs them with the active correlation
// id, and returns a 500 JSON error containing the correlation id so the
// client can quote it back when reporting incidents.
func RecoverMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					corrID := CorrelationFromContext(r.Context())
					lg := log.FromContext(r.Context(), logger)
					lg.LogAttrs(r.Context(), slog.LevelError, "panic recovered",
						slog.String("correlationId", corrID),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.Any("panic", recovered),
						slog.String("stack", string(debug.Stack())),
					)
					InternalError(w, r, "Internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
