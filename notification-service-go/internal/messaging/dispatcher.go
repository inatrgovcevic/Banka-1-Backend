package messaging

import (
	"context"
	"fmt"
	"log/slog"

	"Banka1Back/notification-service-go/internal/dto"
	"Banka1Back/notification-service-go/internal/model"
)

// DeliveryHandler is the business-layer port interface that the Dispatcher
// calls after resolving the routing key to a NotificationType.
// Implemented by *service.NotificationService.
type DeliveryHandler interface {
	HandleIncoming(ctx context.Context, req *dto.NotificationRequest, notificationType model.NotificationType) error
	PersistUnsupportedAudit(ctx context.Context, req *dto.NotificationRequest, routingKey string) error
}

// Dispatcher implements MessageHandler. It:
//  1. Resolves the AMQP routing key to a NotificationType using the static map.
//  2. For unknown routing keys: persists an audit record and ACKs the message.
//  3. For known routing keys: validates the payload, then delegates to DeliveryHandler.
type Dispatcher struct {
	delivery DeliveryHandler
	log      *slog.Logger
}

func NewDispatcher(delivery DeliveryHandler, log *slog.Logger) *Dispatcher {
	return &Dispatcher{delivery: delivery, log: log}
}

var _ MessageHandler = (*Dispatcher)(nil)

// Handle is called by the Consumer for every decoded AMQP message.
func (d *Dispatcher) Handle(
	ctx context.Context,
	req *dto.NotificationRequest,
	routingKey string,
) error {
	notificationType, ok := model.ResolveNotificationType(routingKey)
	if !ok {
		d.log.Warn("unsupported AMQP routing key — persisting failed audit",
			"routing_key", routingKey,
		)
		if err := d.delivery.PersistUnsupportedAudit(ctx, req, routingKey); err != nil {
			return fmt.Errorf("persist unsupported audit for key %q: %w", routingKey, err)
		}
		return nil
	}

	if err := req.Validate(); err != nil {
		d.log.Error("invalid notification payload — discarding message",
			"routing_key", routingKey,
			"notification_type", notificationType,
			"error", err,
		)
		return fmt.Errorf("payload validation failed for key %q: %w", routingKey, err)
	}

	d.log.Debug("dispatching notification message",
		"routing_key", routingKey,
		"notification_type", notificationType,
		"recipient", req.UserEmail,
	)

	if err := d.delivery.HandleIncoming(ctx, req, notificationType); err != nil {
		return fmt.Errorf("handle incoming [%s → %s]: %w", routingKey, notificationType, err)
	}
	return nil
}
