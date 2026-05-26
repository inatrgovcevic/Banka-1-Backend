package platform

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type NotificationPublisher interface {
	PublishEmail(ctx context.Context, routingKey string, payload EmailNotification) error
	Close()
}

type EmailNotification struct {
	UserEmail         string            `json:"userEmail"`
	Username          string            `json:"username"`
	EmailType         string            `json:"emailType,omitempty"`
	TemplateVariables map[string]string `json:"templateVariables,omitempty"`
}

type RabbitPublisher struct {
	logger   *slog.Logger
	exchange string
	conn     *amqp.Connection
	ch       *amqp.Channel
}

func NewRabbitPublisher(ctx context.Context, cfg Config, logger *slog.Logger) (NotificationPublisher, error) {
	_ = ctx
	conn, err := amqp.Dial(cfg.RabbitURL())
	if err != nil {
		logger.Warn("rabbitmq unavailable; email events will be logged only", "error", err)
		return NoopPublisher{logger: logger}, nil
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := ch.ExchangeDeclare(cfg.RabbitMQ.Exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	return &RabbitPublisher{logger: logger, exchange: cfg.RabbitMQ.Exchange, conn: conn, ch: ch}, nil
}

func (p *RabbitPublisher) PublishEmail(ctx context.Context, routingKey string, payload EmailNotification) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return p.ch.PublishWithContext(ctx, p.exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Body:         body,
	})
}

func (p *RabbitPublisher) Close() {
	_ = p.ch.Close()
	_ = p.conn.Close()
}

type NoopPublisher struct {
	logger *slog.Logger
}

func (p NoopPublisher) PublishEmail(ctx context.Context, routingKey string, payload EmailNotification) error {
	p.logger.Warn("email event skipped because rabbitmq is unavailable", "routingKey", routingKey, "email", payload.UserEmail)
	return nil
}

func (p NoopPublisher) Close() {}
