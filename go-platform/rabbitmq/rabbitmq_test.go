package rabbitmq

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

)

func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

func TestConfigURL(t *testing.T) {
	cfg := Config{Host: "rabbit", Port: "5672", Username: "u", Password: "p"}
	if cfg.URL() != "amqp://u:p@rabbit:5672/" {
		t.Fatalf("unexpected URL: %s", cfg.URL())
	}
}

func TestEncodePayloadEmitsRawJSON(t *testing.T) {
	raw, err := encodePayload(map[string]string{"email": "a@b"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var payload map[string]string
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["email"] != "a@b" {
		t.Fatalf("payload corrupted: %v", payload)
	}
	// Critical: must be the raw payload, NOT wrapped in an envelope. The
	// existing Java listeners expect the body shape unchanged.
	if !json.Valid(raw) {
		t.Fatal("expected valid JSON body")
	}
	if string(raw)[0] != '{' || string(raw)[1] != '"' {
		t.Fatalf("body should be raw JSON object, got %s", raw)
	}
}

func TestEncodePayloadRejectsNil(t *testing.T) {
	if _, err := encodePayload(nil); err == nil {
		t.Fatal("expected error on nil payload")
	}
}

func TestNoopPublisherSwallowsCalls(t *testing.T) {
	p := &noopPublisher{logger: discard()}
	if err := p.PublishWithID(context.Background(), "rk", "id", struct{}{}); err != nil {
		t.Fatalf("noop should not fail: %v", err)
	}
	if err := p.Publish(context.Background(), "rk", "data"); err != nil {
		t.Fatalf("noop publish failed: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("noop close: %v", err)
	}
}

func TestNewPublisherUsesNoopWhenBrokerUnreachableAndAllowed(t *testing.T) {
	cfg := Config{
		Host: "127.0.0.1", Port: "1", Username: "x", Password: "x",
		Exchange: "test.events", AllowNoop: true,
		MaxDialAttempts: 2, DialBackoff: 10 * time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pub, err := NewPublisher(ctx, cfg, discard())
	if err != nil {
		t.Fatalf("expected noop fallback, got error: %v", err)
	}
	if _, isNoop := pub.(*noopPublisher); !isNoop {
		t.Fatalf("expected noop publisher, got %T", pub)
	}
	_ = pub.Close()
}

func TestNewPublisherFailsWhenBrokerUnreachableAndNoopDisallowed(t *testing.T) {
	cfg := Config{
		Host: "127.0.0.1", Port: "1", Username: "x", Password: "x",
		Exchange: "test.events", AllowNoop: false,
		MaxDialAttempts: 1, DialBackoff: 10 * time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := NewPublisher(ctx, cfg, discard())
	if err == nil {
		t.Fatal("expected error when noop disabled")
	}
	if !strings.Contains(err.Error(), "dial") {
		t.Fatalf("expected dial error, got %v", err)
	}
}
