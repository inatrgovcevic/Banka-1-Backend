package audit

import (
	"context"
	"encoding/json"
	"log/slog"

	"banka1/go-platform/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

// handleAuditMessage is the per-delivery handler extracted for unit-testability.
func handleAuditMessage(ctx context.Context, env rabbitmq.Envelope, svc *Service, logger *slog.Logger) rabbitmq.HandlerResult {
	if len(env.Body) == 0 {
		logger.Warn("audit: empty payload — skipping")
		return rabbitmq.Ack
	}
	var ev Event
	if err := json.Unmarshal(env.Body, &ev); err != nil {
		logger.Error("audit: decode failed — rejecting", "error", err)
		return rabbitmq.Reject
	}
	if err := svc.Record(ctx, ev); err != nil {
		logger.Error("audit: persist failed — requeueing", "actionType", ev.ActionType, "error", err)
		return rabbitmq.Requeue
	}
	return rabbitmq.Ack
}

// StartConsumer wires the audit.# consumer (mirrors AuditEventListener's
// @RabbitListener binding: durable queue audit-log-queue bound to audit.# on
// the employee.events topic exchange). Producer services across the stack —
// user-service-go (audit.employee_permissions_changed), the Java order-service
// during coexistence — publish AuditEventDto messages there; this consumer
// persists one audit_log row per event.
//
// Gated by cfg.AuditConsumerEnabled at the caller (NewApp): only one consumer
// (Java trading-service or this one) may own the durable queue at a time.
func StartConsumer(ctx context.Context, rmqCfg rabbitmq.Config, svc *Service, logger *slog.Logger) (*rabbitmq.Consumer, error) {
	opts := rabbitmq.ConsumerOpts{
		Queue:       "audit-log-queue",
		BindingKeys: []string{"audit.#"},
		Concurrency: 1,
	}
	handler := func(ctx context.Context, env rabbitmq.Envelope, raw amqp.Delivery) rabbitmq.HandlerResult {
		return handleAuditMessage(ctx, env, svc, logger)
	}
	cons, err := rabbitmq.NewConsumer(ctx, rmqCfg, opts, handler, logger)
	if err != nil {
		return nil, err
	}
	go func() {
		if err := cons.Run(ctx); err != nil && ctx.Err() == nil {
			logger.Error("audit consumer stopped", "queue", opts.Queue, "error", err)
		}
	}()
	logger.Info("audit consumer started", "queue", opts.Queue, "binding", "audit.#")
	return cons, nil
}
