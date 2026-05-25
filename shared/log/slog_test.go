package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestNew_JSONHandler_IncludesFieldsAndLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(LogConfig{Level: slog.LevelInfo, JSON: true, Output: &buf})
	logger.InfoContext(context.Background(), "hello", "key", "value")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["level"] != "INFO" {
		t.Errorf("level: got %v, want INFO", got["level"])
	}
	if got["msg"] != "hello" {
		t.Errorf("msg: got %v, want hello", got["msg"])
	}
	if got["key"] != "value" {
		t.Errorf("key: got %v, want value", got["key"])
	}
}

func TestNew_TextHandler_RespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(LogConfig{Level: slog.LevelWarn, JSON: false, Output: &buf})
	logger.InfoContext(context.Background(), "should be filtered")
	logger.WarnContext(context.Background(), "should appear")

	out := buf.String()
	if len(out) == 0 {
		t.Fatal("expected output")
	}
	if bytes.Contains(buf.Bytes(), []byte("should be filtered")) {
		t.Errorf("info message should be filtered at WARN level, got: %s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte("should appear")) {
		t.Errorf("warn message should appear, got: %s", out)
	}
}

func TestNew_NilOutputDefaultsToStdout(t *testing.T) {
	// Just verify the function does not panic with nil Output.
	logger := New(LogConfig{Level: slog.LevelInfo, JSON: true, Output: nil})
	if logger == nil {
		t.Fatal("logger nil")
	}
}
