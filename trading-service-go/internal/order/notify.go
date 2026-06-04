package order

import (
	"context"
	"log/slog"

	"banka1/trading-service-go/internal/api"

	"banka1/go-platform/rabbitmq"
)

// Notifier publishes order notifications to the employee.events exchange. Mirrors
// order-service OrderNotificationProducer + OrderEventNotifier:
//   - supervisor decisions: order.approved / order.declined
//   - lifecycle events: order.created / order.done / order.partial_fill /
//     order.auto_cancelled
//
// The service calls it AFTER the relevant transaction commits (publish-after-
// commit), so a rolled-back change never emits a notification — safer than Java's
// inline convertAndSend, and it matches OrderEventNotifier's afterCommit
// synchronization (exactly one publish per durably-persisted event, no duplicate
// FCM pushes on execution retry).
type Notifier interface {
	OrderApproved(ctx context.Context, payload api.OrderNotificationPayload)
	OrderDeclined(ctx context.Context, payload api.OrderNotificationPayload)
	OrderCreated(ctx context.Context, payload api.OrderNotificationPayload)
	OrderDone(ctx context.Context, payload api.OrderNotificationPayload)
	OrderPartialFill(ctx context.Context, payload api.OrderNotificationPayload)
	OrderAutoCancelled(ctx context.Context, payload api.OrderNotificationPayload)
	RecurringOrderSkipped(ctx context.Context, payload api.RecurringOrderSkippedNotification)
}

const (
	routingOrderApproved         = "order.approved"
	routingOrderDeclined         = "order.declined"
	routingOrderCreated          = "order.created"
	routingOrderDone             = "order.done"
	routingOrderPartialFill      = "order.partial_fill"
	routingOrderAutoCancelled    = "order.auto_cancelled"
	routingOrderRecurringSkipped = "order.recurring_skipped"
)

// Order lifecycle event names — mirror OrderEventNotifier.OrderEventType. Carried
// in the notification templateVariables under the "event" key.
const (
	eventCreated       = "CREATED"
	eventDone          = "DONE"
	eventPartialFill   = "PARTIAL_FILL"
	eventAutoCancelled = "AUTO_CANCELLED"
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

func (n *RabbitNotifier) OrderCreated(ctx context.Context, payload api.OrderNotificationPayload) {
	n.publish(ctx, routingOrderCreated, payload)
}

func (n *RabbitNotifier) OrderDone(ctx context.Context, payload api.OrderNotificationPayload) {
	n.publish(ctx, routingOrderDone, payload)
}

func (n *RabbitNotifier) OrderPartialFill(ctx context.Context, payload api.OrderNotificationPayload) {
	n.publish(ctx, routingOrderPartialFill, payload)
}

func (n *RabbitNotifier) OrderAutoCancelled(ctx context.Context, payload api.OrderNotificationPayload) {
	n.publish(ctx, routingOrderAutoCancelled, payload)
}

// RecurringOrderSkipped mirrors OrderNotificationProducer.sendRecurringOrderSkipped
// (payload shape RecurringOrderSkippedNotification, caught by the notification
// consumer's order.# binding).
func (n *RabbitNotifier) RecurringOrderSkipped(ctx context.Context, payload api.RecurringOrderSkippedNotification) {
	if err := n.pub.Publish(ctx, routingOrderRecurringSkipped, payload); err != nil {
		n.logger.Warn("order notification publish failed", "routingKey", routingOrderRecurringSkipped, "error", err)
	}
}

func (n *RabbitNotifier) publish(ctx context.Context, routingKey string, payload api.OrderNotificationPayload) {
	if err := n.pub.Publish(ctx, routingKey, payload); err != nil {
		n.logger.Warn("order notification publish failed", "routingKey", routingKey, "orderId", payload.OrderID, "error", err)
	}
}

// NoopNotifier discards notifications. Used when no publisher is configured.
type NoopNotifier struct{}

func (NoopNotifier) OrderApproved(context.Context, api.OrderNotificationPayload)                  {}
func (NoopNotifier) OrderDeclined(context.Context, api.OrderNotificationPayload)                  {}
func (NoopNotifier) OrderCreated(context.Context, api.OrderNotificationPayload)                   {}
func (NoopNotifier) OrderDone(context.Context, api.OrderNotificationPayload)                      {}
func (NoopNotifier) OrderPartialFill(context.Context, api.OrderNotificationPayload)               {}
func (NoopNotifier) OrderAutoCancelled(context.Context, api.OrderNotificationPayload)             {}
func (NoopNotifier) RecurringOrderSkipped(context.Context, api.RecurringOrderSkippedNotification) {}
