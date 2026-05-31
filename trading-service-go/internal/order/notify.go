package order

import (
	"context"
	"log/slog"

	"banka1/trading-service-go/internal/api"

	"banka1/go-platform/rabbitmq"
)

// Notifier publishes supervisor order decisions to the employee.events exchange.
// Mirrors order-service OrderNotificationProducer (routing keys order.approved /
// order.declined). The service calls it AFTER the decision transaction commits
// (publish-after-commit), so a rolled-back decision never emits a notification —
// safer than Java's inline convertAndSend.
type Notifier interface {
	OrderApproved(ctx context.Context, payload api.OrderNotificationPayload)
	OrderDeclined(ctx context.Context, payload api.OrderNotificationPayload)
}

const (
	routingOrderApproved = "order.approved"
	routingOrderDeclined = "order.declined"
)

// RabbitNotifier publishes over a go-platform rabbitmq.Publisher. Publish errors
// are logged, never fatal (notifications are best-effort, matching how a missing
// broker must not block trading).
type RabbitNotifier struct {
	pub    rabbitmq.Publisher
	logger *slog.Logger
}

// NewRabbitNotifier wraps a publisher.
func NewRabbitNotifier(pub rabbitmq.Publisher, logger *slog.Logger) *RabbitNotifier {
	return &RabbitNotifier{pub: pub, logger: logger}
}

func (n *RabbitNotifier) OrderApproved(ctx context.Context, payload api.OrderNotificationPayload) {
	n.publish(ctx, routingOrderApproved, payload)
}

func (n *RabbitNotifier) OrderDeclined(ctx context.Context, payload api.OrderNotificationPayload) {
	n.publish(ctx, routingOrderDeclined, payload)
}

func (n *RabbitNotifier) publish(ctx context.Context, routingKey string, payload api.OrderNotificationPayload) {
	if err := n.pub.Publish(ctx, routingKey, payload); err != nil {
		n.logger.Warn("order notification publish failed", "routingKey", routingKey, "orderId", payload.OrderID, "error", err)
	}
}

// NoopNotifier discards notifications. Used when no publisher is configured.
type NoopNotifier struct{}

func (NoopNotifier) OrderApproved(context.Context, api.OrderNotificationPayload) {}
func (NoopNotifier) OrderDeclined(context.Context, api.OrderNotificationPayload) {}
