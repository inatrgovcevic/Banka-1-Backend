package rabbitmq

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// HandlerResult signals what the consumer should do with the delivery.
type HandlerResult int

const (
	// Ack acknowledges the delivery; broker discards it.
	Ack HandlerResult = iota
	// Reject discards the delivery without requeue. Use for poison messages.
	Reject
	// Requeue puts the delivery back on the queue. Use sparingly to avoid loops.
	Requeue
)

// Handler processes one delivery. Decode envelope.Payload yourself with
// json.Unmarshal(envelope.Payload, &myType).
type Handler func(ctx context.Context, envelope Envelope, raw amqp.Delivery) HandlerResult

// IdempotencyStore tracks which (routingKey, eventID) pairs have already been
// processed. Implementations may use Redis, Postgres, etc. Return true when
// the message is a known duplicate.
type IdempotencyStore interface {
	SeenAndMark(ctx context.Context, key string) (bool, error)
}

// ConsumerOpts configures a NewConsumer.
type ConsumerOpts struct {
	Queue         string
	BindingKeys   []string
	Concurrency   int
	HandleTimeout time.Duration
	Idempotency   IdempotencyStore
}

// Consumer connects to a queue, applies a handler with manual ack and panic
// recovery, and supports an optional idempotency hook.
type Consumer struct {
	logger   *slog.Logger
	conn     *amqp.Connection
	ch       *amqp.Channel
	exchange string
	opts     ConsumerOpts
	handler  Handler
}

// NewConsumer dials the broker, declares and binds the queue to the topic
// exchange. The caller must call Run to start consuming.
func NewConsumer(ctx context.Context, cfg Config, opts ConsumerOpts, handler Handler, logger *slog.Logger) (*Consumer, error) {
	if opts.Queue == "" {
		return nil, errors.New("rabbitmq: queue name is required")
	}
	if len(opts.BindingKeys) == 0 {
		return nil, errors.New("rabbitmq: at least one binding key is required")
	}
	if opts.Concurrency < 1 {
		opts.Concurrency = 1
	}
	if opts.HandleTimeout <= 0 {
		opts.HandleTimeout = 30 * time.Second
	}
	conn, err := dialWithBackoff(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := ch.ExchangeDeclare(cfg.Exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	if _, err := ch.QueueDeclare(opts.Queue, true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	for _, key := range opts.BindingKeys {
		if err := ch.QueueBind(opts.Queue, key, cfg.Exchange, false, nil); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return nil, err
		}
	}
	if err := ch.Qos(opts.Concurrency, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	return &Consumer{logger: logger, conn: conn, ch: ch, exchange: cfg.Exchange, opts: opts, handler: handler}, nil
}

// Run blocks delivering messages until ctx is cancelled or the channel closes.
// Acknowledgments are manual; panics inside the handler are recovered and the
// delivery is rejected without requeue (avoid poison loops).
func (c *Consumer) Run(ctx context.Context) error {
	deliveries, err := c.ch.Consume(c.opts.Queue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("rabbitmq: delivery channel closed")
			}
			c.process(ctx, d)
		}
	}
}

func (c *Consumer) process(ctx context.Context, d amqp.Delivery) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("rabbitmq handler panicked",
				"routingKey", d.RoutingKey,
				"panic", r,
				"stack", string(debug.Stack()),
			)
			_ = d.Reject(false)
		}
	}()
	env := Envelope{
		EventID:   d.MessageId,
		EmittedAt: d.Timestamp,
		Body:      d.Body,
	}
	if d.Headers != nil {
		if v, ok := d.Headers["x-correlation-id"].(string); ok {
			env.CorrelationID = v
		}
	}
	if c.opts.Idempotency != nil && env.EventID != "" {
		seen, err := c.opts.Idempotency.SeenAndMark(ctx, d.RoutingKey+":"+env.EventID)
		if err != nil {
			c.logger.Warn("rabbitmq: idempotency check failed; requeueing", "error", err.Error())
			_ = d.Nack(false, true)
			return
		}
		if seen {
			_ = d.Ack(false)
			return
		}
	}
	handleCtx, cancel := context.WithTimeout(ctx, c.opts.HandleTimeout)
	defer cancel()
	switch c.handler(handleCtx, env, d) {
	case Ack:
		_ = d.Ack(false)
	case Reject:
		_ = d.Reject(false)
	case Requeue:
		_ = d.Nack(false, true)
	default:
		_ = d.Reject(false)
	}
}

// Close releases the channel and connection.
func (c *Consumer) Close() error {
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
