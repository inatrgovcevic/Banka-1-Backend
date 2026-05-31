package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"Banka1Back/notification-service-go/internal/config"
	"Banka1Back/notification-service-go/internal/dto"
)

// MessageHandler is the port interface through which the Consumer delegates
// each decoded message to the business layer.
//
// ACK contract:
//   - Return nil   → Consumer calls d.Ack(false)
//   - Return error → Consumer calls d.Nack(false, false) (no requeue)
type MessageHandler interface {
	Handle(ctx context.Context, req *dto.NotificationRequest, routingKey string) error
}

// Consumer manages the full AMQP lifecycle: connection, channel, topology
// declaration, QoS, and a fixed-size worker pool for concurrent processing.
//
// Reconnection: on any connection or channel closure, the consumer re-dials with
// exponential backoff (1s → 2s → … → 30s cap).
type Consumer struct {
	cfg     config.Config
	handler MessageHandler
	log     *slog.Logger
}

func NewConsumer(cfg config.Config, handler MessageHandler, log *slog.Logger) *Consumer {
	return &Consumer{cfg: cfg, handler: handler, log: log}
}

// Run starts the consumer loop and blocks until ctx is cancelled.
// Automatically reconnects on connection/channel errors using exponential backoff.
func (c *Consumer) Run(ctx context.Context) error {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		err := c.runOnce(ctx)
		if err == nil {
			return nil
		}

		c.log.Error("AMQP consumer error — will reconnect",
			"error", err,
			"backoff", backoff,
		)

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (c *Consumer) runOnce(ctx context.Context) error {
	conn, err := amqp091.DialConfig(c.cfg.AMQP.URL(), amqp091.Config{
		Heartbeat: 10 * time.Second,
		Locale:    "en_US",
	})
	if err != nil {
		return fmt.Errorf("dial %s: %w", c.cfg.AMQP.Host, err)
	}
	defer func() { _ = conn.Close() }()

	c.log.Info("AMQP connection established", "host", c.cfg.AMQP.Host)

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	if err := DeclareTopology(ch, c.cfg.Rabbit); err != nil {
		return fmt.Errorf("declare topology: %w", err)
	}
	c.log.Info("AMQP topology declared",
		"exchange", c.cfg.Rabbit.Exchange,
		"queue", c.cfg.Rabbit.Queue,
	)

	if err := ch.Qos(c.cfg.Rabbit.Prefetch, 0, false); err != nil {
		return fmt.Errorf("set QoS prefetch=%d: %w", c.cfg.Rabbit.Prefetch, err)
	}

	deliveries, err := ch.Consume(
		c.cfg.Rabbit.Queue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("start consume on queue %q: %w", c.cfg.Rabbit.Queue, err)
	}

	c.log.Info("AMQP consumer started",
		"queue", c.cfg.Rabbit.Queue,
		"prefetch", c.cfg.Rabbit.Prefetch,
		"workers", c.cfg.Rabbit.Workers,
	)

	return c.drainLoop(ctx, conn, deliveries)
}

func (c *Consumer) drainLoop(
	ctx context.Context,
	conn *amqp091.Connection,
	deliveries <-chan amqp091.Delivery,
) error {
	work := make(chan amqp091.Delivery, c.cfg.Rabbit.Prefetch)

	var wg sync.WaitGroup
	for i := 0; i < c.cfg.Rabbit.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for d := range work {
				c.process(ctx, d)
			}
		}()
	}

	defer func() {
		close(work)
		wg.Wait()
	}()

	connClose := conn.NotifyClose(make(chan *amqp091.Error, 1))

	for {
		select {
		case <-ctx.Done():
			c.log.Info("AMQP consumer shutting down gracefully")
			return nil

		case amqpErr, ok := <-connClose:
			if !ok || amqpErr == nil {
				return fmt.Errorf("AMQP connection closed unexpectedly")
			}
			return fmt.Errorf("AMQP connection closed: %w", amqpErr)

		case d, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("AMQP delivery channel closed")
			}
			work <- d
		}
	}
}

// process deserialises one AMQP delivery and calls the configured handler.
//
// ACK semantics (matches Spring Boot's default-requeue-rejected=false):
//   - Handler returns nil    → d.Ack(false)
//   - JSON decode fails      → d.Nack(false, false)
//   - Handler returns error  → d.Nack(false, false)
func (c *Consumer) process(ctx context.Context, d amqp091.Delivery) {
	routingKey := d.RoutingKey

	var req dto.NotificationRequest
	if err := json.Unmarshal(d.Body, &req); err != nil {
		c.log.Error("failed to deserialise AMQP payload — discarding message",
			"routing_key", routingKey,
			"error", err,
			"body_preview", truncate(string(d.Body), 200),
		)
		if nackErr := d.Nack(false, false); nackErr != nil {
			c.log.Warn("Nack failed", "error", nackErr)
		}
		return
	}

	msgCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := c.handler.Handle(msgCtx, &req, routingKey); err != nil {
		c.log.Error("message handler returned error — discarding message",
			"routing_key", routingKey,
			"recipient", req.UserEmail,
			"error", err,
		)
		if nackErr := d.Nack(false, false); nackErr != nil {
			c.log.Warn("Nack failed", "error", nackErr)
		}
		return
	}

	if ackErr := d.Ack(false); ackErr != nil {
		c.log.Warn("Ack failed — message may be redelivered",
			"routing_key", routingKey,
			"error", ackErr,
		)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
