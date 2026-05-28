package service

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"banka1/banking-core-service-go/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitPublisher struct {
	cfg config.Config

	mu      sync.Mutex
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewRabbitPublisher(cfg config.Config) *RabbitPublisher {
	return &RabbitPublisher{cfg: cfg}
}

func (p *RabbitPublisher) PublishJSON(ctx context.Context, exchange, routingKey string, payload any) error {
	if p == nil || exchange == "" || routingKey == "" {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := p.ensureChannel(exchange); err != nil {
		return err
	}
	publishCtx := ctx
	if publishCtx == nil {
		publishCtx = context.Background()
	}
	if _, ok := publishCtx.Deadline(); !ok {
		var cancel context.CancelFunc
		publishCtx, cancel = context.WithTimeout(publishCtx, 5*time.Second)
		defer cancel()
	}
	return p.channel.PublishWithContext(
		publishCtx,
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now().UTC(),
			Body:         body,
		},
	)
}

func (p *RabbitPublisher) PublishJSONBestEffort(ctx context.Context, exchange, routingKey string, payload any) {
	if p == nil {
		return
	}
	_ = p.PublishJSON(ctx, exchange, routingKey, payload)
}

func (p *RabbitPublisher) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var err error
	if p.channel != nil {
		err = p.channel.Close()
		p.channel = nil
	}
	if p.conn != nil && !p.conn.IsClosed() {
		if closeErr := p.conn.Close(); err == nil {
			err = closeErr
		}
		p.conn = nil
	}
	return err
}

func (p *RabbitPublisher) ensureChannel(exchange string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil && !p.conn.IsClosed() && p.channel != nil {
		return nil
	}
	conn, err := amqp.Dial(p.cfg.RabbitURL())
	if err != nil {
		return err
	}
	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}
	if err := channel.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return err
	}
	p.conn = conn
	p.channel = channel
	return nil
}
