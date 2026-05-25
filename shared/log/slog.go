// Package log builds a structured slog.Logger configured from a single struct.
package log

import (
	"io"
	"log/slog"
	"os"
)

// LogConfig controls the slog handler.
type LogConfig struct {
	Level  slog.Level
	JSON   bool      // true → JSONHandler; false → TextHandler
	Output io.Writer // nil → os.Stdout
}

// New constructs an slog.Logger from the given config.
func New(cfg LogConfig) *slog.Logger {
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}
	opts := &slog.HandlerOptions{Level: cfg.Level}
	var h slog.Handler
	if cfg.JSON {
		h = slog.NewJSONHandler(out, opts)
	} else {
		h = slog.NewTextHandler(out, opts)
	}
	return slog.New(h)
}
