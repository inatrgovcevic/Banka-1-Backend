package saga

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/events"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/rabbit"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

const sagaTypeFundLiquidation = "FUND_LIQUIDATION_FOR_REDEMPTION"

// HandleFundRedeemWithLiquidation runs the 2-step fund redemption-with-liquidation saga.
//
// Used when the fund does NOT have enough liquid cash: it must sell holdings
// first, then transfer the proceeds to the client.
//
//	Step 1: trading-service liquidates fund holdings to cover amount
//	Step 2: banking-core transfers amount from fund account to client account
//
// Compensation notes:
//   - Step 1 (liquidation) is NOT reversible — asset prices have moved by the
//     time we'd need to undo; we log an alert and require manual intervention.
//   - Step 2 (transfer) failure is also non-compensatable after liquidation.
//
// On success publishes saga.FUND_LIQUIDATION_FOR_REDEMPTION.STEP_2.fund.success.
// On failure publishes saga.FUND_LIQUIDATION_FOR_REDEMPTION.STEP_X.fund.failure.
//
// correlationID = transactionId.
func (o *Orchestrator) HandleFundRedeemWithLiquidation(ctx context.Context, evt events.FundRedeemWithLiquidationRequested) error {
	correlationID := evt.TransactionID

	payloadBytes, _ := json.Marshal(evt)
	inst, isNew, err := o.findOrInitialize(ctx, sagaTypeFundLiquidation, correlationID, 2, payloadBytes)
	if err != nil {
		return fmt.Errorf("FUND_LIQUIDATION findOrInitialize: %w", err)
	}
	if !isNew {
		if inst.IsTerminal() {
			o.log.Info("FUND_LIQUIDATION already terminal — skipping",
				"correlationId", correlationID, "state", inst.State)
			return nil
		}
		// Crash recovery: re-run (all downstream calls are idempotent per correlationID).
		// advanceState's optimistic lock rejects any true concurrent duplicate.
		o.log.Warn("FUND_LIQUIDATION crash recovery: re-running IN_PROGRESS saga",
			"correlationId", correlationID, "state", inst.State)
	}

	if err := o.advanceState(ctx, inst); err != nil {
		return fmt.Errorf("FUND_LIQUIDATION advanceState: %w", err)
	}

	compLog := make(map[string]string)

	// -----------------------------------------------------------------------
	// Step 1: Liquidate fund holdings to cover the redemption amount.
	// -----------------------------------------------------------------------
	inst.CurrentStep = 1
	liquidationID, liqErr := o.td.LiquidateForFund(ctx, evt.FundID, evt.Amount, correlationID)
	if liqErr != nil {
		compLog["failureReason"] = liqErr.Error()
		compLog["failedAtStep"] = "1"
		_ = o.finalize(ctx, inst, store.SagaStateFailed, compLog)
		o.publishJSON(ctx, rabbit.RKFundLiquidationFailure, events.FundLiquidationFailed{
			TransactionID: evt.TransactionID,
			FundID:        evt.FundID,
			FailureReason: liqErr.Error(),
			FailedAtStep:  1,
		})
		o.log.Error("FUND_LIQUIDATION step 1 FAILED", "correlationId", correlationID, "error", liqErr)
		return liqErr
	}
	compLog["step1_liquidationId"] = liquidationID
	if err := o.saveLog(ctx, inst, compLog); err != nil {
		// Cannot safely proceed — but liquidation already happened. Log alert.
		o.log.Error("FUND_LIQUIDATION saveLog after step 1 failed — liquidation completed but state not persisted; manual intervention required",
			"correlationId", correlationID, "liquidationId", liquidationID, "error", err)
		return fmt.Errorf("FUND_LIQUIDATION saveLog step 1: %w", err)
	}
	o.log.Info("FUND_LIQUIDATION step 1 OK", "correlationId", correlationID, "liquidationId", liquidationID)

	// -----------------------------------------------------------------------
	// Step 2: Transfer amount from fund account to client account.
	// -----------------------------------------------------------------------
	inst.CurrentStep = 2
	transferID, err := o.bc.InternalTransfer(
		ctx,
		evt.FundAccountNumber, evt.ToAccountNumber,
		evt.Amount, correlationID,
	)
	if err != nil {
		// Step 1 already completed (assets sold) — cannot reverse. Alert required.
		compLog["failureReason"] = err.Error()
		compLog["failedAtStep"] = "2"
		compLog["alertRequired"] = "Liquidation completed but transfer failed — manual intervention needed; liquidationId=" + liquidationID
		_ = o.finalize(ctx, inst, store.SagaStateFailed, compLog)
		o.publishJSON(ctx, rabbit.RKFundLiquidationFailure, events.FundLiquidationFailed{
			TransactionID: evt.TransactionID,
			FundID:        evt.FundID,
			FailureReason: err.Error(),
			FailedAtStep:  2,
		})
		o.log.Error("FUND_LIQUIDATION step 2 FAILED (manual intervention required)",
			"correlationId", correlationID,
			"liquidationId", liquidationID,
			"error", err,
		)
		return err
	}
	compLog["step2_transferId"] = transferID
	if err := o.finalize(ctx, inst, store.SagaStateCompleted, compLog); err != nil {
		o.log.Error("FUND_LIQUIDATION finalize error", "correlationId", correlationID, "error", err)
	}

	o.publishJSON(ctx, rabbit.RKFundLiquidationSuccess, events.FundLiquidationCompleted{
		TransactionID:   evt.TransactionID,
		Amount:          evt.Amount,
		FundID:          evt.FundID,
		ToAccountNumber: evt.ToAccountNumber,
		TransferID:      transferID,
		LiquidationID:   liquidationID,
	})
	o.log.Info("FUND_LIQUIDATION COMPLETED",
		"correlationId", correlationID,
		"liquidationId", liquidationID,
		"transferId", transferID,
	)
	return nil
}
