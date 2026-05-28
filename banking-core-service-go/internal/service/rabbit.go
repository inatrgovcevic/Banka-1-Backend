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

	mu                sync.Mutex
	conn              *amqp.Connection
	channel           *amqp.Channel
	declaredExchanges map[string]bool
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
	publishCtx := ctx
	if publishCtx == nil {
		publishCtx = context.Background()
	}
	if _, ok := publishCtx.Deadline(); !ok {
		var cancel context.CancelFunc
		publishCtx, cancel = context.WithTimeout(publishCtx, 5*time.Second)
		defer cancel()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.ensureChannelLocked(exchange); err != nil {
		return err
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

func (p *RabbitPublisher) Check() error {
	if p == nil {
		return nil
	}
	exchange := p.cfg.NotificationExchange
	if exchange == "" {
		exchange = p.cfg.TransferRetryExchange
	}
	return p.ensureChannel(exchange)
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
	return p.ensureChannelLocked(exchange)
}

func (p *RabbitPublisher) ensureChannelLocked(exchange string) error {
	if p.conn != nil && !p.conn.IsClosed() && p.channel != nil {
		return p.ensureExchangeLocked(exchange)
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
	p.conn = conn
	p.channel = channel
	p.declaredExchanges = map[string]bool{}
	if err := p.ensureExchangeLocked(exchange); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		p.channel = nil
		p.conn = nil
		p.declaredExchanges = nil
		return err
	}
	return nil
}

func (p *RabbitPublisher) ensureExchangeLocked(exchange string) error {
	if exchange == "" {
		return nil
	}
	if p.declaredExchanges == nil {
		p.declaredExchanges = map[string]bool{}
	}
	if p.declaredExchanges[exchange] {
		return nil
	}
	if err := p.channel.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	p.declaredExchanges[exchange] = true
	return nil
}
