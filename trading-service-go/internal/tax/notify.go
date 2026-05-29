package tax

import (
	"context"
	"log/slog"

	"banka1/trading-service-go/internal/api"

	"banka1/go-platform/rabbitmq"
)

// Notifier publishes tax.collected notifications to the employee.events exchange.
// Mirrors order-service OrderNotificationProducer.sendTaxCollected (routing key
// tax.collected). The service calls it AFTER the tax debit succeeds
// (publish-after-charge), so a failed/deleted reservation never emits one.
type Notifier interface {
	TaxCollected(ctx context.Context, payload api.TaxCollectedPayload)
}

const routingTaxCollected = "tax.collected"

// RabbitNotifier publishes over a go-platform rabbitmq.Publisher. Publish errors
// are logged, never fatal — matching the order notifier and Java's best-effort
// convertAndSend (a notification must not roll back a collected tax).
type RabbitNotifier struct {
	pub    rabbitmq.Publisher
	logger *slog.Logger
}

// NewRabbitNotifier wraps a publisher.
func NewRabbitNotifier(pub rabbitmq.Publisher, logger *slog.Logger) *RabbitNotifier {
	return &RabbitNotifier{pub: pub, logger: logger}
}

func (n *RabbitNotifier) TaxCollected(ctx context.Context, payload api.TaxCollectedPayload) {
	if err := n.pub.Publish(ctx, routingTaxCollected, payload); err != nil {
		n.logger.Warn("tax notification publish failed", "routingKey", routingTaxCollected, "error", err)
	}
}

// NoopNotifier discards notifications. Used when no publisher is configured.
type NoopNotifier struct{}

func (NoopNotifier) TaxCollected(context.Context, api.TaxCollectedPayload) {}
