package platform

import (
	"context"
	"log/slog"

	gprabbit "banka1/go-platform/rabbitmq"
)

type EventPublisher interface {
	Publish(ctx context.Context, routingKey string, payload any) error
	Close()
}

type rabbitPublisher struct {
	pub gprabbit.Publisher
}

func NewRabbitPublisher(ctx context.Context, logger *slog.Logger) (EventPublisher, error) {
	cfg := gprabbit.LoadConfig()
	cfg.AllowNoop = true
	cfg.MaxDialAttempts = 1
	pub, err := gprabbit.NewPublisher(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	return &rabbitPublisher{pub: pub}, nil
}

func (p *rabbitPublisher) Publish(ctx context.Context, routingKey string, payload any) error {
	return p.pub.Publish(ctx, routingKey, payload)
}

func (p *rabbitPublisher) Close() {
	_ = p.pub.Close()
}
