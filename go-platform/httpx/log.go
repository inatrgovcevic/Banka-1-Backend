package httpx

import (
	"log/slog"
	"net/http"
	"time"

	"banka1/go-platform/log"
)

// RequestLogMiddleware emits one structured log line per HTTP request:
//
//	{
//	  "level": "INFO", "msg": "http request",
//	  "service": <from base logger>,
//	  "correlationId": <set by CorrelationMiddleware>,
//	  "traceId": <set by otel middleware when present>,
//	  "spanId":  <set by otel middleware when present>,
//	  "method": "GET", "path": "/x", "status": 200,
//	  "durationMs": 12, "bytes": 1842
//	}
//
// Place after CorrelationMiddleware so the correlation id is in context.
// The traceId/spanId fields are filled by the otel middleware via
// log.WithLogger when tracing is enabled; otherwise they are absent.
func RequestLogMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			lg := log.FromContext(r.Context(), logger)
			lg.LogAttrs(r.Context(), slog.LevelInfo, "http request",
				slog.String("correlationId", CorrelationFromContext(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.Status()),
				slog.Int64("durationMs", time.Since(start).Milliseconds()),
				slog.Int("bytes", rec.bytes),
			)
		})
	}
}
