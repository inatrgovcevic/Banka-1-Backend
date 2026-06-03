package market

import (
	"context"

	"banka1/market-service-go/internal/platform"
)

const PriceAlertRoutingKey = "price.alert_triggered"

type RabbitPriceAlertPublisher struct {
	pub platform.EventPublisher
}

func NewRabbitPriceAlertPublisher(pub platform.EventPublisher) *RabbitPriceAlertPublisher {
	return &RabbitPriceAlertPublisher{pub: pub}
}

func (p *RabbitPriceAlertPublisher) PublishPriceAlertTriggered(ctx context.Context, payload PriceAlertNotificationPayload) error {
	return p.pub.Publish(ctx, PriceAlertRoutingKey, payload)
}
