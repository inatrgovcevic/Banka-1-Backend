// Package saga contains the saga orchestrator and all five saga handler
// implementations. Each handler lives in its own file for readability but
// they all operate through the Orchestrator struct which owns the shared
// dependencies (store, clients, publisher, logger).
//
// # Design
//
// The orchestrator uses imperative "Model B" style (no generic state machine):
// each saga is a plain Go method that sequences outbound HTTP calls, updates
// the saga_instance row between steps for crash-recovery, and runs a LIFO
// compensation chain on failure.
//
// Idempotency: Before starting, every handler checks whether a saga with the
// same (sagaType, correlationID) already exists in a terminal state.  If yes,
// it returns nil immediately — the RabbitMQ ack happens and the message is
// discarded.  This means at-least-once delivery is fine; the worst case is
// one redundant DB lookup.
//
// Optimistic locking: UpdateOptimistic uses version = current_version in the
// WHERE predicate.  If a concurrent consumer has already updated the row, the
// method returns ErrOptimisticLockConflict which is treated as a transient
// error — the listener nack's without requeue so RabbitMQ drops to DLQ.
//
// Step idempotency note: if a REST call succeeds but the subsequent
// UpdateOptimistic fails (e.g. process crash), the saga is replayed from the
// beginning on the next delivery.  All downstream REST endpoints are assumed
// to be idempotent per correlationID.  The Java reference code has the same
// assumption.
package saga

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

// BankingCoreActions defines the banking-core operations needed by sagas.
type BankingCoreActions interface {
	ReserveFunds(ctx context.Context, ownerID int64, amount decimal.Decimal, corrID string) (string, error)
	ReleaseFunds(ctx context.Context, reservationID, corrID string) error
	CommitReservation(ctx context.Context, reservationID, corrID string) error
	InternalTransfer(ctx context.Context, fromAccount, toAccount string, amount decimal.Decimal, corrID string) (string, error)
	ReverseTransfer(ctx context.Context, transferID, corrID string) error
	ResolveDefaultAccountNumber(ctx context.Context, ownerID int64) (string, error)
}

// TradingActions defines the trading-service operations needed by sagas.
type TradingActions interface {
	ReserveStocks(ctx context.Context, ownerID int64, ticker string, amount int, corrID string) (string, error)
	ReleaseStocks(ctx context.Context, reservationID, corrID string) error
	TransferOwnership(ctx context.Context, reservationID string, buyerID int64, corrID string) (string, error)
	ReverseOwnership(ctx context.Context, ownershipTransferID, corrID string) error
	LiquidateForFund(ctx context.Context, fundID int64, targetAmount decimal.Decimal, corrID string) (liquidationID string, err error)
}

// MarketActions defines the market-service operations needed by sagas.
type MarketActions interface {
	ConvertCurrencyNoCommission(ctx context.Context, fromCurrency, toCurrency string, amount decimal.Decimal) (decimal.Decimal, error)
}

// EventPublisher publishes saga result events back onto the bus.
type EventPublisher interface {
	Publish(ctx context.Context, routingKey string, body []byte) error
}

// SagaStore is the persistence interface needed by the orchestrator.
// The concrete implementation is *store.SagaInstanceStore; the interface
// exists so that in-memory fakes can be injected in unit tests.
type SagaStore interface {
	FindByTypeAndCorrelation(ctx context.Context, sagaType, correlationID string) (*store.SagaInstance, error)
	Insert(ctx context.Context, inst *store.SagaInstance) error
	UpdateOptimistic(ctx context.Context, inst *store.SagaInstance) error
}

// Orchestrator is the central saga coordinator. Construct it with NewOrchestrator
// and call the Handle* methods from RabbitMQ listener goroutines.
type Orchestrator struct {
	store     SagaStore
	bc        BankingCoreActions
	td        TradingActions
	mk        MarketActions
	publisher EventPublisher
	log       *slog.Logger
}

// NewOrchestrator creates an Orchestrator with all dependencies wired.
func NewOrchestrator(
	st SagaStore,
	bc BankingCoreActions,
	td TradingActions,
	mk MarketActions,
	publisher EventPublisher,
	log *slog.Logger,
) *Orchestrator {
	return &Orchestrator{
		store:     st,
		bc:        bc,
		td:        td,
		mk:        mk,
		publisher: publisher,
		log:       log,
	}
}

// NewOrchestratorForTest creates an Orchestrator that accepts any SagaStore
// implementation. This allows test code to inject in-memory fakes without
// depending on a real PostgreSQL connection pool.
func NewOrchestratorForTest(
	st SagaStore,
	bc BankingCoreActions,
	td TradingActions,
	mk MarketActions,
	publisher EventPublisher,
	log *slog.Logger,
) *Orchestrator {
	return NewOrchestrator(st, bc, td, mk, publisher, log)
}

// ---------------------------------------------------------------------------
// Internal helpers shared across all saga implementations
// ---------------------------------------------------------------------------

// findOrInitialize looks up an existing saga instance by (sagaType, correlationID).
// Returns:
//   - existing instance if found in any state, plus isNew=false
//   - newly inserted STARTED instance if not found, plus isNew=true
//   - (nil, false, err) on database error
//
// The caller must check existing.IsTerminal() and existing.State == IN_PROGRESS
// before proceeding.
func (o *Orchestrator) findOrInitialize(
	ctx context.Context,
	sagaType, correlationID string,
	totalSteps int,
	payloadBytes []byte,
) (*store.SagaInstance, bool, error) {
	existing, err := o.store.FindByTypeAndCorrelation(ctx, sagaType, correlationID)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, nil
	}
	inst := &store.SagaInstance{
		SagaType:      sagaType,
		CorrelationID: correlationID,
		CurrentStep:   0,
		TotalSteps:    totalSteps,
		State:         store.SagaStateStarted,
		Payload:       payloadBytes,
	}
	if err := o.store.Insert(ctx, inst); err != nil {
		if err == store.ErrOptimisticLockConflict {
			// Concurrent insert won; re-read and return.
			inst, err = o.store.FindByTypeAndCorrelation(ctx, sagaType, correlationID)
			return inst, false, err
		}
		return nil, false, err
	}
	return inst, true, nil
}

// advanceState marks the saga IN_PROGRESS and persists. Returns the updated instance.
func (o *Orchestrator) advanceState(ctx context.Context, inst *store.SagaInstance) error {
	inst.State = store.SagaStateInProgress
	return o.store.UpdateOptimistic(ctx, inst)
}

// marshalLog marshals a compensation log map to JSON bytes (nil on empty map).
func marshalLog(m map[string]string) []byte {
	if len(m) == 0 {
		return nil
	}
	b, _ := json.Marshal(m)
	return b
}

// unmarshalLog parses a JSONB compensation log into a map.
func unmarshalLog(b []byte) map[string]string {
	if len(b) == 0 {
		return make(map[string]string)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return make(map[string]string)
	}
	return m
}

// publishJSON marshals v and calls publisher.Publish. Errors are logged but
// not propagated — a publish failure after a completed saga must not re-trigger
// compensation.
func (o *Orchestrator) publishJSON(ctx context.Context, routingKey string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		o.log.Error("saga publish marshal error", "rk", routingKey, "error", err)
		return
	}
	if err := o.publisher.Publish(ctx, routingKey, b); err != nil {
		o.log.Error("saga publish error", "rk", routingKey, "error", err)
	}
}

// saveLog saves the current compensation log to the instance and persists.
func (o *Orchestrator) saveLog(ctx context.Context, inst *store.SagaInstance, log map[string]string) error {
	inst.CompensationLog = marshalLog(log)
	return o.store.UpdateOptimistic(ctx, inst)
}

// finalize marks the saga COMPLETED or FAILED and saves.
func (o *Orchestrator) finalize(ctx context.Context, inst *store.SagaInstance, state string, compLog map[string]string) error {
	inst.State = state
	inst.CompensationLog = marshalLog(compLog)
	return o.store.UpdateOptimistic(ctx, inst)
}
