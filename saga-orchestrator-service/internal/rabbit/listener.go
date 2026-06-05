package rabbit

import (
	"context"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Decoder decodes raw AMQP message bytes into a typed event value.
// Returns an error if the payload cannot be decoded.
type Decoder func(body []byte) (any, error)

// Handler processes a decoded event. Returning an error causes the message
// to be nack'd (once, without requeue) and logged at ERROR level.
type Handler func(ctx context.Context, payload any) error

// ConsumeChannel is the subset of amqp.Channel used by Listener.
// Using an interface allows unit tests to inject a fake without a broker.
// *amqp.Channel satisfies this interface.
type ConsumeChannel interface {
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
}

// Listener subscribes to a single queue and dispatches messages to a handler.
// It ack's on success, nack's-without-requeue on handler or decode error (the
// broker then dead-letters the message per the queue's x-dead-letter-* args).
type Listener struct {
	ch      ConsumeChannel
	queue   string
	decoder Decoder
	handler Handler
	log     *slog.Logger
}

// NewListener constructs a Listener. ch must already be set up (channel open,
// topology declared). Each Listener should have its own dedicated channel.
func NewListener(ch ConsumeChannel, queue string, decoder Decoder, handler Handler, log *slog.Logger) *Listener {
	return &Listener{
		ch:      ch,
		queue:   queue,
		decoder: decoder,
		handler: handler,
		log:     log,
	}
}

// Run starts consuming from the queue and blocks until ctx is cancelled or the
// channel is closed. It returns a non-nil error only on fatal setup failures
// (e.g., QueueConsume error); normal shutdown returns nil.
func (l *Listener) Run(ctx context.Context) error {
	msgs, err := l.ch.Consume(
		l.queue,
		"",    // consumer tag — broker auto-generates
		false, // autoAck — we ack/nack manually
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,
	)
	if err != nil {
		return fmt.Errorf("rabbit listener %q: consume: %w", l.queue, err)
	}

	l.log.Info("saga listener started", "queue", l.queue)

	for {
		select {
		case <-ctx.Done():
			l.log.Info("saga listener stopping", "queue", l.queue)
			return nil
		case msg, ok := <-msgs:
			if !ok {
				l.log.Warn("saga listener channel closed", "queue", l.queue)
				return nil
			}
			l.dispatch(ctx, msg)
		}
	}
}

// dispatch decodes and handles a single delivery, managing ack/nack.
func (l *Listener) dispatch(ctx context.Context, msg amqp.Delivery) {
	payload, err := l.decoder(msg.Body)
	if err != nil {
		l.log.Error("saga listener: decode failed; nack without requeue",
			"queue", l.queue,
			"routing_key", msg.RoutingKey,
			"error", err,
		)
		_ = msg.Nack(false, false) // single, no requeue
		return
	}

	if err := l.handler(ctx, payload); err != nil {
		l.log.Error("saga listener: handler failed; nack without requeue",
			"queue", l.queue,
			"routing_key", msg.RoutingKey,
			"error", err,
		)
		_ = msg.Nack(false, false)
		return
	}

	if err := msg.Ack(false); err != nil {
		l.log.Warn("saga listener: ack failed",
			"queue", l.queue,
			"error", err,
		)
	}
}
