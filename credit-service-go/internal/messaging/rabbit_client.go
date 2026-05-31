package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitClient struct {
	conn       *amqp.Connection
	channel    *amqp.Channel
	exchange   string
	routingKey string
	enabled    bool
}

func NewRabbitClient() (*RabbitClient, error) {
	host := getenv("RABBITMQ_HOST", "localhost")
	port := getenv("RABBITMQ_PORT", "5672")
	username := getenv("RABBITMQ_USERNAME", "guest")
	password := getenv("RABBITMQ_PASSWORD", "guest")
	exchange := getenv("RABBITMQ_EXCHANGE", "notification_exchange")
	routingKey := getenv("RABBITMQ_ROUTING_KEY", "notification_routing_key")

	url := fmt.Sprintf("amqp://%s:%s@%s:%s/", username, password, host, port)

	conn, err := amqp.Dial(url)
	if err != nil {
		return &RabbitClient{enabled: false}, err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return &RabbitClient{enabled: false}, err
	}

	return &RabbitClient{
		conn:       conn,
		channel:    ch,
		exchange:   exchange,
		routingKey: routingKey,
		enabled:    true,
	}, nil
}

func (r *RabbitClient) Close() {
	if r == nil {
		return
	}
	if r.channel != nil {
		_ = r.channel.Close()
	}
	if r.conn != nil {
		_ = r.conn.Close()
	}
}

func (r *RabbitClient) PublishJSON(ctx context.Context, payload any) error {
	if r == nil || !r.enabled {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return r.channel.PublishWithContext(
		ctx,
		r.exchange,
		r.routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
