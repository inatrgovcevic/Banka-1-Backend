// Package rabbit provides RabbitMQ connection management, topology declaration,
// consumer listener helpers, and a publisher for saga result events.
//
// # Topology
//
// The saga orchestrator uses a single topic exchange ("saga.exchange") and five
// dedicated trigger queues — one per saga type. Each queue is bound with a
// specific routing key so that publisher services (trading-service,
// banking-core) can target exact queues by routing key rather than delivering
// to a generic "saga.events" queue.
//
// Fix vs Java: In the Java reference implementation, OtcExerciseSaga and
// OtcPremiumTransferSaga published result events using rabbitTemplate with the
// exchange set to the queue name "saga.events" — this is a topology bug because
// RabbitMQ treats it as the default exchange and the queue name as a routing key.
// In the Go port, ALL publishers use the exchange "saga.exchange" with proper
// routing keys.
package rabbit

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

// Exchange and queue constants used throughout the saga service.
const (
	// ExchangeSaga is the durable topic exchange used for all saga messages.
	ExchangeSaga = "saga.exchange"

	// DLQName is the dead-letter queue where messages with exhausted retries land.
	DLQName = "saga.dlq"
)

// Trigger queue names — bound to ExchangeSaga with specific routing keys.
// Downstream services publish trigger events directly to ExchangeSaga with the
// matching routing key; the broker delivers to the appropriate queue.
const (
	QueueOtcExercise               = "saga.otc.exercise.queue"
	QueueOtcPremium                = "saga.otc.premium.queue"
	QueueFundSubscribe             = "saga.fund.subscribe.queue"
	QueueFundRedeem                = "saga.fund.redeem.queue"
	QueueFundRedeemWithLiquidation = "saga.fund.redeem.with-liquidation.queue"
)

// Routing keys this service consumes (trigger events from other services).
const (
	RKOtcExerciseRequested               = "otc.exercise.requested"
	RKOtcPremiumRequested                = "otc.premium.transfer.requested"
	RKFundSubscribeRequested             = "fund.subscribe.requested"
	RKFundRedeemRequested                = "fund.redeem.requested"
	RKFundRedeemWithLiquidationRequested = "fund.redeem.with-liquidation.requested"
)

// Result routing keys published by the orchestrator after saga completion.
// These match the Java RabbitTemplate.convertAndSend call sites so that
// existing Java consumers (trading-service, banking-core) continue to receive
// events without modification.
const (
	RKOtcExerciseCompleted    = "saga.OTC_EXERCISE.completed"
	RKOtcExerciseFailed       = "saga.OTC_EXERCISE.failed"
	RKOtcPremiumCompleted     = "saga.OTC_PREMIUM_TRANSFER.completed"
	RKOtcPremiumFailed        = "saga.OTC_PREMIUM_TRANSFER.failed"
	RKFundSubscribeSuccess    = "saga.FUND_SUBSCRIBE.STEP_1.fund.success"
	RKFundSubscribeFailure    = "saga.FUND_SUBSCRIBE.STEP_1.fund.failure"
	RKFundRedeemSuccess       = "saga.FUND_REDEEM.STEP_1.fund.success"
	RKFundRedeemFailure       = "saga.FUND_REDEEM.STEP_1.fund.failure"
	RKFundLiquidationSuccess  = "saga.FUND_LIQUIDATION_FOR_REDEMPTION.STEP_2.fund.success"
	RKFundLiquidationFailure  = "saga.FUND_LIQUIDATION_FOR_REDEMPTION.STEP_X.fund.failure"
)

// triggerBindings maps each trigger queue to its routing key.
var triggerBindings = []struct {
	queue      string
	routingKey string
}{
	{QueueOtcExercise, RKOtcExerciseRequested},
	{QueueOtcPremium, RKOtcPremiumRequested},
	{QueueFundSubscribe, RKFundSubscribeRequested},
	{QueueFundRedeem, RKFundRedeemRequested},
	{QueueFundRedeemWithLiquidation, RKFundRedeemWithLiquidationRequested},
}

// ChannelDeclarer is the subset of amqp.Channel used by DeclareTopology.
// Using an interface allows unit tests to inject a mock without a broker.
type ChannelDeclarer interface {
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error
}

// DeclareTopology declares the saga exchange, DLQ, and all five trigger queues
// with their bindings. Each declaration is idempotent: re-declaring an already-
// existing exchange or queue with the same parameters is a no-op in RabbitMQ.
//
// Call once after establishing a channel, before starting any consumers.
func DeclareTopology(ch ChannelDeclarer) error {
	// 1. Declare the topic exchange (durable, non-auto-delete).
	if err := ch.ExchangeDeclare(
		ExchangeSaga, "topic",
		true, false, false, false, nil,
	); err != nil {
		return err
	}

	// 2. Declare the dead-letter queue (plain durable, no TTL).
	if _, err := ch.QueueDeclare(
		DLQName,
		true, false, false, false, nil,
	); err != nil {
		return err
	}

	// 3. Declare each trigger queue with x-dead-letter-exchange pointing to the
	//    default exchange with DLQName as routing key.
	dlqArgs := amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": DLQName,
	}
	for _, b := range triggerBindings {
		if _, err := ch.QueueDeclare(
			b.queue,
			true, false, false, false, dlqArgs,
		); err != nil {
			return err
		}
		if err := ch.QueueBind(
			b.queue, b.routingKey, ExchangeSaga,
			false, nil,
		); err != nil {
			return err
		}
	}

	return nil
}
