package saga

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/events"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/rabbit"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

const sagaTypeFundSubscribe = "FUND_SUBSCRIBE"

// HandleFundSubscribe runs the single-step fund subscribe saga.
//
// When a client wants to invest in a fund, trading-service publishes
// fund.subscribe.requested. This saga transfers the amount from the client's
// account to the fund's RSD account. On success, it publishes
// saga.FUND_SUBSCRIBE.STEP_1.fund.success so that trading-service can flip the
// ClientFundTransaction to COMPLETED and update ClientFundPosition.
//
// correlationID = transactionId (a string UUID assigned by trading-service).
func (o *Orchestrator) HandleFundSubscribe(ctx context.Context, evt events.FundSubscribeRequested) error {
	correlationID := evt.TransactionID

	payloadBytes, _ := json.Marshal(evt)
	inst, isNew, err := o.findOrInitialize(ctx, sagaTypeFundSubscribe, correlationID, 1, payloadBytes)
	if err != nil {
		return fmt.Errorf("FUND_SUBSCRIBE findOrInitialize: %w", err)
	}
	if !isNew {
		if inst.IsTerminal() {
			o.log.Info("FUND_SUBSCRIBE already terminal — skipping",
				"correlationId", correlationID, "state", inst.State)
			return nil
		}
		// Crash recovery: re-run (all downstream calls are idempotent per correlationID).
		// advanceState's optimistic lock rejects any true concurrent duplicate.
		o.log.Warn("FUND_SUBSCRIBE crash recovery: re-running IN_PROGRESS saga",
			"correlationId", correlationID, "state", inst.State)
	}

	if err := o.advanceState(ctx, inst); err != nil {
		return fmt.Errorf("FUND_SUBSCRIBE advanceState: %w", err)
	}
	inst.CurrentStep = 1

	transferID, err := o.bc.InternalTransfer(
		ctx,
		evt.FromAccountNumber, evt.FundAccountNumber,
		evt.Amount, correlationID,
	)
	if err != nil {
		compLog := map[string]string{"failureReason": err.Error()}
		_ = o.finalize(ctx, inst, store.SagaStateFailed, compLog)
		o.publishJSON(ctx, rabbit.RKFundSubscribeFailure, events.FundSubscribeFailed{
			TransactionID: evt.TransactionID,
			FundID:        evt.FundID,
			FailureReason: err.Error(),
		})
		o.log.Error("FUND_SUBSCRIBE FAILED", "correlationId", correlationID, "error", err)
		return err
	}

	compLog := map[string]string{"step1_transferId": transferID}
	if err := o.finalize(ctx, inst, store.SagaStateCompleted, compLog); err != nil {
		o.log.Error("FUND_SUBSCRIBE finalize error",
			"correlationId", correlationID, "error", err)
	}

	o.publishJSON(ctx, rabbit.RKFundSubscribeSuccess, events.FundSubscribeCompleted{
		TransactionID:     evt.TransactionID,
		Amount:            evt.Amount,
		FromAccountNumber: evt.FromAccountNumber,
		FundAccountNumber: evt.FundAccountNumber,
		FundID:            evt.FundID,
		TransferID:        transferID,
	})
	o.log.Info("FUND_SUBSCRIBE COMPLETED", "correlationId", correlationID, "transferId", transferID)
	return nil
}
