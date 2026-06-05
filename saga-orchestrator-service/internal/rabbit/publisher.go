package rabbit

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// PublishChannel is the subset of amqp.Channel used by Publisher.
// Using an interface allows unit tests to inject a fake without a broker.
// *amqp.Channel satisfies this interface.
type PublishChannel interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
}

// Publisher publishes saga result events to ExchangeSaga.
// It holds a single amqp.Channel; for production use each goroutine should have
// its own Publisher (channels are not thread-safe in amqp091-go).
type Publisher struct {
	ch PublishChannel
}

// NewPublisher wraps an already-open amqp channel.
func NewPublisher(ch PublishChannel) *Publisher {
	return &Publisher{ch: ch}
}

// Publish sends body to ExchangeSaga with the given routing key.
// The message is persistent (DeliveryMode 2) and content-type application/json.
// The ctx parameter is accepted for interface symmetry and future use (amqp091
// does not yet accept context on Publish).
func (p *Publisher) Publish(_ context.Context, routingKey string, body []byte) error {
	err := p.ch.Publish(
		ExchangeSaga,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("rabbit: publish %q: %w", routingKey, err)
	}
	return nil
}
