package saga

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/events"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/rabbit"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

const sagaTypeFundRedeem = "FUND_REDEEM"

// HandleFundRedeem runs the single-step fund redeem saga (fast path).
//
// Used when the fund has sufficient liquid cash to cover the redemption
// directly — no liquidation required. Transfers amount from the fund's
// account to the client's account.
//
// On success publishes saga.FUND_REDEEM.STEP_1.fund.success.
// On failure publishes saga.FUND_REDEEM.STEP_1.fund.failure.
//
// correlationID = transactionId.
func (o *Orchestrator) HandleFundRedeem(ctx context.Context, evt events.FundRedeemRequested) error {
	correlationID := evt.TransactionID

	payloadBytes, _ := json.Marshal(evt)
	inst, isNew, err := o.findOrInitialize(ctx, sagaTypeFundRedeem, correlationID, 1, payloadBytes)
	if err != nil {
		return fmt.Errorf("FUND_REDEEM findOrInitialize: %w", err)
	}
	if !isNew {
		if inst.IsTerminal() {
			o.log.Info("FUND_REDEEM already terminal — skipping",
				"correlationId", correlationID, "state", inst.State)
			return nil
		}
		o.log.Warn("FUND_REDEEM in unexpected state; skipping",
			"correlationId", correlationID, "state", inst.State)
		return nil
	}

	if err := o.advanceState(ctx, inst); err != nil {
		return fmt.Errorf("FUND_REDEEM advanceState: %w", err)
	}
	inst.CurrentStep = 1

	transferID, err := o.bc.InternalTransfer(
		ctx,
		evt.FromAccountNumber, evt.ToAccountNumber,
		evt.Amount, correlationID,
	)
	if err != nil {
		compLog := map[string]string{"failureReason": err.Error()}
		_ = o.finalize(ctx, inst, store.SagaStateFailed, compLog)
		o.publishJSON(ctx, rabbit.RKFundRedeemFailure, events.FundRedeemFailed{
			TransactionID: evt.TransactionID,
			FundID:        evt.FundID,
			FailureReason: err.Error(),
		})
		o.log.Error("FUND_REDEEM FAILED", "correlationId", correlationID, "error", err)
		return err
	}

	compLog := map[string]string{"step1_transferId": transferID}
	if err := o.finalize(ctx, inst, store.SagaStateCompleted, compLog); err != nil {
		o.log.Error("FUND_REDEEM finalize error", "correlationId", correlationID, "error", err)
	}

	o.publishJSON(ctx, rabbit.RKFundRedeemSuccess, events.FundRedeemCompleted{
		TransactionID:   evt.TransactionID,
		Amount:          evt.Amount,
		FundID:          evt.FundID,
		ToAccountNumber: evt.ToAccountNumber,
		TransferID:      transferID,
	})
	o.log.Info("FUND_REDEEM COMPLETED", "correlationId", correlationID, "transferId", transferID)
	return nil
}
