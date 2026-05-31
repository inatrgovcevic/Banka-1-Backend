package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"banka1/go-platform/httpx"
	"banka1/go-platform/log"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Envelope captures the per-message metadata the consumer surfaces alongside
// the raw body. The on-wire AMQP body itself is the caller's payload JSON
// (no extra wrapping) so the existing Java listeners decoding the raw shape
// keep working. EventID rides on AMQP MessageId; CorrelationID rides on the
// "x-correlation-id" header.
type Envelope struct {
	EventID       string
	CorrelationID string
	EmittedAt     time.Time
	Body          []byte
}

// Publisher publishes JSON messages to a topic exchange. Always emit through
// this interface so business code is independent of the active backend.
type Publisher interface {
	Publish(ctx context.Context, routingKey string, payload any) error
	PublishWithID(ctx context.Context, routingKey, eventID string, payload any) error
	Close() error
}

// amqpPublisher is the live broker-backed implementation.
type amqpPublisher struct {
	logger   *slog.Logger
	exchange string
	conn     *amqp.Connection
	ch       *amqp.Channel
}

// NewPublisher dials the broker (with retry), declares the topic exchange
// idempotently, and returns a Publisher. If the dial ultimately fails:
//   - returns NoopPublisher when cfg.AllowNoop is true
//   - returns an error otherwise (caller must decide service-fatal vs not)
func NewPublisher(ctx context.Context, cfg Config, logger *slog.Logger) (Publisher, error) {
	conn, err := dialWithBackoff(ctx, cfg, logger)
	if err != nil {
		if cfg.AllowNoop {
			logger.Warn("rabbitmq unreachable; using NoopPublisher", "error", err.Error())
			return &noopPublisher{logger: logger}, nil
		}
		return nil, fmt.Errorf("rabbitmq: dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq: channel: %w", err)
	}
	if err := ch.ExchangeDeclare(cfg.Exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq: declare exchange %q: %w", cfg.Exchange, err)
	}
	logger.Info("rabbitmq publisher ready", "exchange", cfg.Exchange)
	return &amqpPublisher{logger: logger, exchange: cfg.Exchange, conn: conn, ch: ch}, nil
}

func dialWithBackoff(ctx context.Context, cfg Config, logger *slog.Logger) (*amqp.Connection, error) {
	attempts := cfg.MaxDialAttempts
	if attempts < 1 {
		attempts = 1
	}
	backoff := cfg.DialBackoff
	if backoff <= 0 {
		backoff = time.Second
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		conn, err := amqp.Dial(cfg.URL())
		if err == nil {
			return conn, nil
		}
		lastErr = err
		logger.Warn("rabbitmq dial failed; will retry", "attempt", i+1, "error", err.Error())
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > 10*time.Second {
			backoff = 10 * time.Second
		}
	}
	return nil, lastErr
}

// Publish sends payload to routingKey, wrapping it in an Envelope. EventID is
// blank; callers that need dedupe should use PublishWithID.
func (p *amqpPublisher) Publish(ctx context.Context, routingKey string, payload any) error {
	return p.PublishWithID(ctx, routingKey, "", payload)
}

// PublishWithID is like Publish but stamps the message with eventID so
// downstream consumers can dedupe retries.
func (p *amqpPublisher) PublishWithID(ctx context.Context, routingKey, eventID string, payload any) error {
	if payload == nil {
		return errors.New("rabbitmq: payload is nil")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("rabbitmq: marshal payload: %w", err)
	}
	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	headers := amqp.Table{}
	if id := httpx.CorrelationFromContext(ctx); id != "" {
		headers["x-correlation-id"] = id
	}
	pub := amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		MessageId:    eventID,
		Headers:      headers,
		Body:         body,
	}
	if err := p.ch.PublishWithContext(pubCtx, p.exchange, routingKey, false, false, pub); err != nil {
		log.FromContext(ctx, p.logger).Warn("rabbitmq publish failed", "routingKey", routingKey, "error", err.Error())
		return err
	}
	return nil
}

func (p *amqpPublisher) Close() error {
	if p.ch != nil {
		_ = p.ch.Close()
	}
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

// encodePayload exists for tests that want to verify the on-wire bytes match
// the raw payload (no envelope wrapping).
func encodePayload(payload any) ([]byte, error) {
	if payload == nil {
		return nil, errors.New("rabbitmq: payload is nil")
	}
	return json.Marshal(payload)
}

// noopPublisher discards every Publish call. Only used when cfg.AllowNoop is
// true and the broker was unreachable at startup.
type noopPublisher struct {
	logger *slog.Logger
}

func (p *noopPublisher) Publish(ctx context.Context, routingKey string, payload any) error {
	return p.PublishWithID(ctx, routingKey, "", payload)
}
func (p *noopPublisher) PublishWithID(ctx context.Context, routingKey, eventID string, payload any) error {
	log.FromContext(ctx, p.logger).Warn("rabbitmq noop publish",
		"routingKey", routingKey,
		"eventId", eventID,
		"correlationId", httpx.CorrelationFromContext(ctx),
	)
	return nil
}
func (p *noopPublisher) Close() error { return nil }
