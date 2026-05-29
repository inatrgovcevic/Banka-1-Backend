package otc

import (
	"context"
	"encoding/json"
	"log/slog"

	"banka1/go-platform/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Saga routing keys. Both publish and consume keys live on the SAGA_EVENTS
// exchange (saga.events) — note this differs from funds, whose RESULT keys live
// on saga.exchange. These match the Java OtcService publishes + the
// @RabbitListener key= annotations byte-for-byte (saga-orchestrator-service binds
// to them).
const (
	RoutingPremiumTransferRequested = "otc.premium.transfer.requested"
	RoutingExerciseRequested        = "otc.exercise.requested"

	RoutingPremiumTransferCompleted = "otc.premium.transfer.completed"
	RoutingPremiumTransferFailed    = "otc.premium.transfer.failed"
	RoutingExerciseCompleted        = "otc.exercise.completed"
)

// Saga queue names. Must match the Java @Queue value= annotations exactly — they
// are durable, broker-owned queues. Binding the same name from Java AND Go would
// round-robin deliveries → half-processed contracts; that is why
// OTC_SAGA_CONSUMERS_ENABLED is OFF by default during coexistence.
const (
	QueuePremiumCompleted  = "trading.otc.premium.completed"
	QueuePremiumFailed     = "trading.otc.premium.failed"
	QueueExerciseCompleted = "trading.otc.exercise.completed"
)

// SagaPublisher publishes the two OTC saga-request keys onto the saga.events
// exchange. The Service depends on this interface so NoopSagaPublisher can fall in
// when the broker is unreachable.
type SagaPublisher interface {
	PublishPremiumTransferRequested(ctx context.Context, e PremiumTransferRequestedEvent) error
	PublishExerciseRequested(ctx context.Context, e ExerciseRequestedEvent) error
}

// RabbitSagaPublisher wraps the go-platform Publisher already bound to the
// saga.events exchange (the same publisher the P5 funds saga uses — shared
// connection, no funds code touched).
type RabbitSagaPublisher struct {
	pub    rabbitmq.Publisher
	logger *slog.Logger
}

// NewRabbitSagaPublisher wraps a publisher already bound to SAGA_EVENTS_EXCHANGE.
func NewRabbitSagaPublisher(pub rabbitmq.Publisher, logger *slog.Logger) *RabbitSagaPublisher {
	return &RabbitSagaPublisher{pub: pub, logger: logger}
}

func (p *RabbitSagaPublisher) PublishPremiumTransferRequested(ctx context.Context, e PremiumTransferRequestedEvent) error {
	return p.pub.Publish(ctx, RoutingPremiumTransferRequested, e)
}

func (p *RabbitSagaPublisher) PublishExerciseRequested(ctx context.Context, e ExerciseRequestedEvent) error {
	return p.pub.Publish(ctx, RoutingExerciseRequested, e)
}

// NoopSagaPublisher discards publishes. Used when the broker is unreachable — the
// local transaction still commits, the contract stays PENDING_PREMIUM (premium)
// or ACTIVE+exercised_at (exercise), and a supervisor sees the stuck row.
type NoopSagaPublisher struct {
	logger *slog.Logger
}

// NewNoopSagaPublisher returns a publisher that logs and drops every event.
func NewNoopSagaPublisher(logger *slog.Logger) *NoopSagaPublisher {
	return &NoopSagaPublisher{logger: logger}
}

func (p *NoopSagaPublisher) PublishPremiumTransferRequested(_ context.Context, e PremiumTransferRequestedEvent) error {
	p.logger.Warn("otc saga publisher noop: dropping otc.premium.transfer.requested",
		"contractId", e.ContractID, "premium", e.Premium)
	return nil
}

func (p *NoopSagaPublisher) PublishExerciseRequested(_ context.Context, e ExerciseRequestedEvent) error {
	p.logger.Warn("otc saga publisher noop: dropping otc.exercise.requested",
		"contractId", e.ContractID, "ticker", e.StockTicker, "amount", e.Amount)
	return nil
}

// ============================== consumers =================================

type sagaHandlerKind int

const (
	kindPremiumCompleted sagaHandlerKind = iota
	kindPremiumFailed
	kindExerciseCompleted
)

type consumerBinding struct {
	queue      string
	bindingKey string
	kind       sagaHandlerKind
	label      string
}

// otcConsumerBindings enumerates the three durable queues the OTC domain owns,
// each mapping a Java @RabbitListener (queue ← routing key on saga.events).
var otcConsumerBindings = []consumerBinding{
	{QueuePremiumCompleted, RoutingPremiumTransferCompleted, kindPremiumCompleted, "otc.premium.completed"},
	{QueuePremiumFailed, RoutingPremiumTransferFailed, kindPremiumFailed, "otc.premium.failed"},
	{QueueExerciseCompleted, RoutingExerciseCompleted, kindExerciseCompleted, "otc.exercise.completed"},
}

// StartSagaConsumers wires the three OTC saga-result consumers against the
// saga.events exchange (rmqCfg.Exchange must be SAGA_EVENTS_EXCHANGE). Each
// consumer dispatches into Service.CompletePremiumTransfer / FailPremiumTransfer /
// CompleteExercise. Cancel ctx to stop them. Gated by cfg.OtcSagaConsumersEnabled
// at the caller (NewApp); when OFF, this is never called.
func StartSagaConsumers(ctx context.Context, rmqCfg rabbitmq.Config, svc *Service, logger *slog.Logger) ([]*rabbitmq.Consumer, error) {
	out := make([]*rabbitmq.Consumer, 0, len(otcConsumerBindings))
	for _, b := range otcConsumerBindings {
		b := b
		handler := buildSagaHandler(svc, b, logger)
		opts := rabbitmq.ConsumerOpts{
			Queue:       b.queue,
			BindingKeys: []string{b.bindingKey},
			Concurrency: 1,
		}
		cons, err := rabbitmq.NewConsumer(ctx, rmqCfg, opts, handler, logger)
		if err != nil {
			for _, c := range out {
				_ = c.Close()
			}
			return nil, err
		}
		go func() {
			if err := cons.Run(ctx); err != nil && ctx.Err() == nil {
				logger.Error("otc saga consumer stopped", "queue", b.queue, "key", b.bindingKey, "error", err)
			}
		}()
		logger.Info("otc saga consumer started", "queue", b.queue, "key", b.bindingKey)
		out = append(out, cons)
	}
	return out, nil
}

// buildSagaHandler returns the rabbitmq.Handler for one binding. The Service-layer
// transitions are idempotent (no-op once the contract left the expected source
// state), so a duplicate delivery is harmless and the handler always Acks on
// success. A decode failure or a service error Rejects without requeue (matches
// the funds consumer + the Java listener-container behavior).
func buildSagaHandler(svc *Service, b consumerBinding, logger *slog.Logger) rabbitmq.Handler {
	return func(ctx context.Context, env rabbitmq.Envelope, raw amqp.Delivery) rabbitmq.HandlerResult {
		var evt sagaContractEvent
		if len(env.Body) > 0 {
			if err := json.Unmarshal(env.Body, &evt); err != nil {
				logger.Error("otc saga: decode failed", "queue", b.queue, "error", err)
				return rabbitmq.Reject
			}
		}
		if evt.ContractID == nil {
			logger.Warn("otc saga: missing contractId — skip", "queue", b.queue, "key", raw.RoutingKey)
			return rabbitmq.Ack
		}
		contractID := *evt.ContractID
		var err error
		switch b.kind {
		case kindPremiumCompleted:
			err = svc.CompletePremiumTransfer(ctx, contractID)
		case kindPremiumFailed:
			reason := ""
			if evt.Reason != nil {
				reason = *evt.Reason
			}
			err = svc.FailPremiumTransfer(ctx, contractID, reason)
		case kindExerciseCompleted:
			err = svc.CompleteExercise(ctx, contractID)
		}
		if err != nil {
			logger.Error("otc saga: handler failed", "queue", b.queue, "contractId", contractID, "error", err)
			return rabbitmq.Reject
		}
		return rabbitmq.Ack
	}
}
