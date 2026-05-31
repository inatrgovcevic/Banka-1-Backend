// Package log provides the structured JSON logger every Go service should use.
//
// Logs are written to stdout in JSON, with a top-level `service` attribute so
// promtail/Loki can route them per service. Other middleware (httpx,
// grpcx, otel) decorate the logger via context so trace/correlation IDs
// appear on every line.
package log

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// Level resolves a log level from common string forms (DEBUG, INFO, WARN,
// ERROR) used by the Java services' application.properties. Unknown values
// default to INFO.
func Level(raw string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// New returns a slog.Logger configured for production: JSON output, the
// supplied service name attached as `service`, and the given minimum level.
func New(service string, level slog.Level) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler).With(slog.String("service", service))
}

type loggerKey struct{}

// WithLogger stores a logger on ctx for downstream code to retrieve via
// FromContext.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// FromContext returns the logger attached to ctx, or fallback when none.
// Pass slog.Default() as fallback if you do not have a more specific one.
func FromContext(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if value, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && value != nil {
		return value
	}
	return fallback
}
