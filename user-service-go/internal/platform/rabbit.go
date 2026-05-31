package platform

import (
	"context"
	"log/slog"

	gprabbit "banka1/go-platform/rabbitmq"
)

// EmailNotification is the wire shape the existing Java notification-service
// listener decodes. Stays in user-service-go because the field set is
// domain-specific.
type EmailNotification struct {
	UserEmail         string            `json:"userEmail"`
	Username          string            `json:"username"`
	EmailType         string            `json:"emailType,omitempty"`
	TemplateVariables map[string]string `json:"templateVariables,omitempty"`
}

// NotificationPublisher exposes the email-shaped publish call the service
// layer wants. Under the hood it delegates to the shared Rabbit publisher
// (which keeps the on-wire shape Java listeners expect — raw JSON body, no
// envelope wrapping).
type NotificationPublisher interface {
	PublishEmail(ctx context.Context, routingKey string, payload EmailNotification) error
	Close()
}

type rabbitPublisher struct {
	logger *slog.Logger
	pub    gprabbit.Publisher
}

// NewRabbitPublisher dials the broker via the shared go-platform layer. When
// the broker is unreachable, it returns a NoopPublisher (logs and discards)
// to preserve previous behavior — set RABBITMQ_ALLOW_NOOP=false to make a
// missing broker fatal.
func NewRabbitPublisher(ctx context.Context, cfg Config, logger *slog.Logger) (NotificationPublisher, error) {
	rc := gprabbit.LoadConfig()
	rc.Host = cfg.RabbitMQ.Host
	rc.Port = cfg.RabbitMQ.Port
	rc.Username = cfg.RabbitMQ.Username
	rc.Password = cfg.RabbitMQ.Password
	rc.Exchange = cfg.RabbitMQ.Exchange
	if !envHasKey("RABBITMQ_ALLOW_NOOP") {
		rc.AllowNoop = true
	}
	pub, err := gprabbit.NewPublisher(ctx, rc, logger)
	if err != nil {
		return nil, err
	}
	return &rabbitPublisher{logger: logger, pub: pub}, nil
}

func (p *rabbitPublisher) PublishEmail(ctx context.Context, routingKey string, payload EmailNotification) error {
	return p.pub.Publish(ctx, routingKey, payload)
}

func (p *rabbitPublisher) Close() {
	_ = p.pub.Close()
}
