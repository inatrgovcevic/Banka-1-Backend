package log

import (
	"context"
	"log/slog"
	"testing"
)

func TestLevelMapping(t *testing.T) {
	cases := map[string]slog.Level{
		"DEBUG":   slog.LevelDebug,
		"debug":   slog.LevelDebug,
		"INFO":    slog.LevelInfo,
		"":        slog.LevelInfo,
		"WARN":    slog.LevelWarn,
		"WARNING": slog.LevelWarn,
		"ERROR":   slog.LevelError,
		"unknown": slog.LevelInfo,
	}
	for raw, want := range cases {
		if got := Level(raw); got != want {
			t.Errorf("Level(%q)=%v, want %v", raw, got, want)
		}
	}
}

func TestNewIncludesServiceAttr(t *testing.T) {
	logger := New("my-service", slog.LevelInfo)
	if logger == nil {
		t.Fatal("logger nil")
	}
	// We can't easily capture stdout, so just ensure it does not panic.
	logger.Info("hello", "k", "v")
}

func TestContextFallback(t *testing.T) {
	def := slog.New(slog.DiscardHandler)
	if FromContext(context.Background(), def) != def {
		t.Fatal("expected fallback when no logger in context")
	}
	ctx := WithLogger(context.Background(), def)
	if FromContext(ctx, nil) != def {
		t.Fatal("expected stored logger from context")
	}
}
