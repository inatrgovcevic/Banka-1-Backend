package messaging

import (
	"fmt"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"Banka1Back/notification-service-go/internal/config"
)

// DeclareTopology ensures the AMQP exchange, queue, and all domain bindings
// exist on the broker before the consumer starts. All declarations are idempotent.
func DeclareTopology(ch *amqp091.Channel, cfg config.RabbitConfig) error {
	if err := ch.ExchangeDeclare(
		cfg.Exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare exchange %q: %w", cfg.Exchange, err)
	}

	if _, err := ch.QueueDeclare(
		cfg.Queue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare queue %q: %w", cfg.Queue, err)
	}

	for _, pattern := range cfg.BindingPatterns {
		if err := ch.QueueBind(
			cfg.Queue,
			pattern,
			cfg.Exchange,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("bind queue %q to exchange %q with pattern %q: %w",
				cfg.Queue, cfg.Exchange, pattern, err)
		}
	}

	return nil
}
