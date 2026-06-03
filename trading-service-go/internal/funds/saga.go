package funds

import (
	"context"
	"encoding/json"
	"log/slog"

	"banka1/go-platform/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/shopspring/decimal"
)

// Saga routing key constants. Publish keys land on the SAGA_EVENTS_EXCHANGE
// (saga.events by default); subscribe keys are consumed from the
// SAGA_RESULTS_EXCHANGE (saga.exchange by default). These match the Java
// listeners' @RabbitListener key= annotations byte-for-byte (saga-orchestrator-
// service binds to them).
const (
	RoutingFundSubscribeRequested       = "fund.subscribe.requested"
	RoutingFundRedeemRequested          = "fund.redeem.requested"
	RoutingFundRedeemLiquidationRequest = "fund.redeem.with-liquidation.requested"

	RoutingFundSubscribeSuccess         = "saga.FUND_SUBSCRIBE.STEP_1.fund.success"
	RoutingFundSubscribeFailure         = "saga.FUND_SUBSCRIBE.STEP_1.fund.failure"
	RoutingFundRedeemSuccess            = "saga.FUND_REDEEM.STEP_1.fund.success"
	RoutingFundRedeemFailure            = "saga.FUND_REDEEM.STEP_1.fund.failure"
	RoutingFundRedeemLiquidationSuccess = "saga.FUND_LIQUIDATION_FOR_REDEMPTION.STEP_2.fund.success"
	RoutingFundRedeemLiquidationFailure = "saga.FUND_LIQUIDATION_FOR_REDEMPTION.STEP_X.fund.failure"
)

// Saga queue names. Must match Java @Queue value= annotations exactly — these
// are durable, broker-owned queues; binding the same queue name from Java AND
// Go would round-robin deliveries → half-processed sagas. That is why
// FUND_SAGA_CONSUMERS_ENABLED is OFF by default during coexistence.
const (
	QueueFundSubscribeSuccess         = "trading.fund.subscribe.success"
	QueueFundSubscribeFailure         = "trading.fund.subscribe.failure"
	QueueFundRedeemSuccess            = "trading.fund.redeem.success"
	QueueFundRedeemFailure            = "trading.fund.redeem.failure"
	QueueFundRedeemLiquidationSuccess = "trading.fund.redeem-liquidation.success"
	QueueFundRedeemLiquidationFailure = "trading.fund.redeem-liquidation.failure"
)

// SagaPublisher publishes the three saga-request keys onto the SAGA_EVENTS
// exchange. The Service depends on this interface so a Noop implementation
// (NoopSagaPublisher) can fall in when the broker is unreachable.
type SagaPublisher interface {
	PublishSubscribeRequested(ctx context.Context, e FundSubscribeRequestedEvent) error
	PublishRedeemRequested(ctx context.Context, e FundRedeemRequestedEvent) error
}

// RabbitSagaPublisher wraps a go-platform Publisher bound to the SAGA_EVENTS
// exchange. PublishRedeemRequested routes to fund.redeem.requested OR
// fund.redeem.with-liquidation.requested based on the LiquidEnough flag,
// mirroring InvestmentFundService.redeem.
type RabbitSagaPublisher struct {
	pub    rabbitmq.Publisher
	logger *slog.Logger
}

// NewRabbitSagaPublisher wraps a publisher already bound to SAGA_EVENTS_EXCHANGE.
// Callers should build the publisher with rabbitmq.NewPublisher and a Config
// that has Exchange = cfg.SagaEventsExchange.
func NewRabbitSagaPublisher(pub rabbitmq.Publisher, logger *slog.Logger) *RabbitSagaPublisher {
	return &RabbitSagaPublisher{pub: pub, logger: logger}
}

func (p *RabbitSagaPublisher) PublishSubscribeRequested(ctx context.Context, e FundSubscribeRequestedEvent) error {
	return p.pub.Publish(ctx, RoutingFundSubscribeRequested, e)
}

func (p *RabbitSagaPublisher) PublishRedeemRequested(ctx context.Context, e FundRedeemRequestedEvent) error {
	key := RoutingFundRedeemRequested
	if !e.LiquidEnough {
		key = RoutingFundRedeemLiquidationRequest
	}
	return p.pub.Publish(ctx, key, e)
}

// NoopSagaPublisher discards publishes. Used when the broker is unreachable —
// the local transaction still commits, the client_fund_transaction stays
// PENDING, supervisor sees the stuck row.
type NoopSagaPublisher struct {
	logger *slog.Logger
}

// NewNoopSagaPublisher returns a publisher that logs and drops every event.
func NewNoopSagaPublisher(logger *slog.Logger) *NoopSagaPublisher {
	return &NoopSagaPublisher{logger: logger}
}

func (p *NoopSagaPublisher) PublishSubscribeRequested(_ context.Context, e FundSubscribeRequestedEvent) error {
	p.logger.Warn("saga publisher noop: dropping fund.subscribe.requested",
		"transactionId", e.TransactionID, "fundId", e.FundID, "amount", e.Amount)
	return nil
}

func (p *NoopSagaPublisher) PublishRedeemRequested(_ context.Context, e FundRedeemRequestedEvent) error {
	key := RoutingFundRedeemRequested
	if !e.LiquidEnough {
		key = RoutingFundRedeemLiquidationRequest
	}
	p.logger.Warn("saga publisher noop: dropping fund redeem", "routingKey", key,
		"transactionId", e.TransactionID, "fundId", e.FundID, "amount", e.Amount)
	return nil
}

// ============================== consumers =================================

// ConsumerSet is the pair (queue → routing-key) plus the handler kind. We map
// every Java @RabbitListener to one row and start one consumer per row.
type consumerBinding struct {
	queue      string
	bindingKey string
	onSuccess  bool
	label      string
}

// fundConsumerBindings enumerates the six durable queues the funds domain
// owns (3 sagas × {success, failure}).
var fundConsumerBindings = []consumerBinding{
	{QueueFundSubscribeSuccess, RoutingFundSubscribeSuccess, true, "fund.subscribe.success"},
	{QueueFundSubscribeFailure, RoutingFundSubscribeFailure, false, "fund.subscribe.failure"},
	{QueueFundRedeemSuccess, RoutingFundRedeemSuccess, true, "fund.redeem.success"},
	{QueueFundRedeemFailure, RoutingFundRedeemFailure, false, "fund.redeem.failure"},
	{QueueFundRedeemLiquidationSuccess, RoutingFundRedeemLiquidationSuccess, true, "fund.redeem-liquidation.success"},
	{QueueFundRedeemLiquidationFailure, RoutingFundRedeemLiquidationFailure, false, "fund.redeem-liquidation.failure"},
}

// StartSagaConsumers wires the six saga-result consumers against the results
// exchange. Each consumer dispatches into Service.CompleteInvest /
// CompleteRedeem / FailTransaction depending on the saga family and result
// kind. Cancel ctx to stop them. Spawned goroutines log fatal-channel errors;
// the service keeps running (matches Java listener-container behavior).
//
// Gated by cfg.FundSagaConsumersEnabled at the caller (NewApp). When OFF, this
// is never called.
func StartSagaConsumers(ctx context.Context, rmqCfg rabbitmq.Config, svc *Service, logger *slog.Logger) ([]*rabbitmq.Consumer, error) {
	out := make([]*rabbitmq.Consumer, 0, len(fundConsumerBindings))
	for _, b := range fundConsumerBindings {
		b := b
		handler := buildSagaHandler(svc, b, logger)
		opts := rabbitmq.ConsumerOpts{
			Queue:       b.queue,
			BindingKeys: []string{b.bindingKey},
			Concurrency: 1,
		}
		cons, err := rabbitmq.NewConsumer(ctx, rmqCfg, opts, handler, logger)
		if err != nil {
			// roll back previously started consumers so the caller does not
			// keep half-connected channels.
			for _, c := range out {
				_ = c.Close()
			}
			return nil, err
		}
		go func() {
			if err := cons.Run(ctx); err != nil && ctx.Err() == nil {
				logger.Error("fund saga consumer stopped",
					"queue", b.queue, "key", b.bindingKey, "error", err)
			}
		}()
		logger.Info("fund saga consumer started", "queue", b.queue, "key", b.bindingKey)
		out = append(out, cons)
	}
	return out, nil
}

// buildSagaHandler returns a rabbitmq.Handler matching the binding. The handler
// always Acks — there is no automatic redelivery for a logically-bad event
// (the Service-layer "no-op when already terminal" guarantees idempotency, so
// a duplicate delivery is harmless). A panic is recovered by the consumer
// scaffold and Rejects (configured upstream).
func buildSagaHandler(svc *Service, b consumerBinding, logger *slog.Logger) rabbitmq.Handler {
	return func(ctx context.Context, env rabbitmq.Envelope, raw amqp.Delivery) rabbitmq.HandlerResult {
		var evt SagaResultEvent
		if err := decodeSagaEvent(env.Body, &evt); err != nil {
			logger.Error("fund saga: decode failed", "queue", b.queue, "error", err)
			return rabbitmq.Reject
		}
		txID := flexibleInt64Or(evt.TransactionID, 0)
		if txID == 0 {
			logger.Warn("fund saga: missing transactionId — skip", "queue", b.queue, "key", raw.RoutingKey)
			return rabbitmq.Ack
		}
		if b.onSuccess {
			clientID := int64Or(evt.ClientID, 0)
			fundID := int64Or(evt.FundID, 0)
			var amount decimal.Decimal
			if evt.Amount != nil {
				amount = *evt.Amount
			}
			if isInvestSaga(b.queue) {
				if err := svc.CompleteInvest(ctx, txID, clientID, fundID, amount); err != nil {
					logger.Error("fund saga: completeInvest failed", "txId", txID, "error", err)
					return rabbitmq.Reject
				}
			} else {
				if err := svc.CompleteRedeem(ctx, txID, clientID, fundID, amount); err != nil {
					logger.Error("fund saga: completeRedeem failed", "txId", txID, "error", err)
					return rabbitmq.Reject
				}
			}
			return rabbitmq.Ack
		}
		reason := "unknown"
		if evt.FailureReason != nil {
			reason = *evt.FailureReason
		}
		if err := svc.FailTransaction(ctx, txID, reason); err != nil {
			logger.Error("fund saga: failTransaction failed", "txId", txID, "error", err)
			return rabbitmq.Reject
		}
		return rabbitmq.Ack
	}
}

func isInvestSaga(queue string) bool {
	return queue == QueueFundSubscribeSuccess
}

// decodeSagaEvent permissively decodes a saga result body. The Java publisher
// emits Map<String,Object> so numeric ids may be int64 or float64; we tolerate
// both via SagaResultEvent's pointer/optional fields.
func decodeSagaEvent(body []byte, dst *SagaResultEvent) error {
	if len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, dst)
}

func int64Or(p *int64, def int64) int64 {
	if p == nil {
		return def
	}
	return *p
}

func flexibleInt64Or(p *FlexibleInt64, def int64) int64 {
	if p == nil {
		return def
	}
	return p.Int64()
}
