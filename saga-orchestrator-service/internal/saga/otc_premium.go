package saga

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/events"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/rabbit"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

const sagaTypeOtcPremium = "OTC_PREMIUM_TRANSFER"

// HandleOtcPremiumTransfer runs the single-step OTC premium transfer saga.
//
// When a buyer and seller agree on an OTC option contract, the buyer owes the
// seller a premium. This saga transfers the premium from the buyer's default
// (RSD) account to the seller's default (RSD) account via banking-core.
// If the premium is in a foreign currency it is converted to RSD first using
// the market-service no-commission rate.
//
// There is no compensation chain — if the transfer fails the saga is marked
// FAILED and an alert is expected (manual reconciliation). The option contract
// remains active regardless; this follows the Java reference behavior.
//
// Publishes saga.OTC_PREMIUM_TRANSFER.completed or .failed.
func (o *Orchestrator) HandleOtcPremiumTransfer(ctx context.Context, evt events.OtcPremiumTransferRequested) error {
	correlationID := strconv.FormatInt(evt.ContractID, 10)
	transferCorrID := "otc-premium-" + correlationID

	payloadBytes, _ := json.Marshal(evt)
	inst, isNew, err := o.findOrInitialize(ctx, sagaTypeOtcPremium, correlationID, 1, payloadBytes)
	if err != nil {
		return fmt.Errorf("OTC_PREMIUM_TRANSFER findOrInitialize: %w", err)
	}
	if !isNew {
		if inst.IsTerminal() {
			o.log.Info("OTC_PREMIUM_TRANSFER already terminal — skipping",
				"correlationId", correlationID, "state", inst.State)
			return nil
		}
		// Crash recovery: re-run (all downstream calls are idempotent per correlationID).
		// advanceState's optimistic lock rejects any true concurrent duplicate.
		o.log.Warn("OTC_PREMIUM_TRANSFER crash recovery: re-running IN_PROGRESS saga",
			"correlationId", correlationID, "state", inst.State)
	}

	if err := o.advanceState(ctx, inst); err != nil {
		return fmt.Errorf("OTC_PREMIUM_TRANSFER advanceState: %w", err)
	}

	inst.CurrentStep = 1

	// Resolve accounts.
	buyerAccount, err := o.bc.ResolveDefaultAccountNumber(ctx, evt.BuyerID)
	if err != nil {
		return o.otcPremiumFail(ctx, inst, correlationID,
			fmt.Errorf("ResolveDefaultAccountNumber(buyer %d): %w", evt.BuyerID, err))
	}
	sellerAccount, err := o.bc.ResolveDefaultAccountNumber(ctx, evt.SellerID)
	if err != nil {
		return o.otcPremiumFail(ctx, inst, correlationID,
			fmt.Errorf("ResolveDefaultAccountNumber(seller %d): %w", evt.SellerID, err))
	}

	// Convert to RSD if necessary.
	transferAmount := evt.Premium
	currency := evt.Currency
	if currency == "" {
		currency = "USD"
	}
	if currency != "RSD" {
		converted, err := o.mk.ConvertCurrencyNoCommission(ctx, currency, "RSD", evt.Premium)
		if err != nil {
			return o.otcPremiumFail(ctx, inst, correlationID,
				fmt.Errorf("ConvertCurrencyNoCommission(%s->RSD): %w", currency, err))
		}
		transferAmount = converted
	}

	// Single transfer step.
	transferID, err := o.bc.InternalTransfer(ctx, buyerAccount, sellerAccount, transferAmount, transferCorrID)
	if err != nil {
		return o.otcPremiumFail(ctx, inst, correlationID,
			fmt.Errorf("InternalTransfer: %w", err))
	}

	compLog := map[string]string{"step1_transferId": transferID}
	if err := o.finalize(ctx, inst, store.SagaStateCompleted, compLog); err != nil {
		o.log.Error("OTC_PREMIUM_TRANSFER finalize error",
			"correlationId", correlationID, "error", err)
	}
	o.publishJSON(ctx, rabbit.RKOtcPremiumCompleted, events.OtcPremiumTransferCompleted{
		ContractID: evt.ContractID,
		TransferID: transferID,
	})
	o.log.Info("OTC_PREMIUM_TRANSFER COMPLETED", "correlationId", correlationID, "transferId", transferID)
	return nil
}

func (o *Orchestrator) otcPremiumFail(
	ctx context.Context,
	inst *store.SagaInstance,
	correlationID string,
	cause error,
) error {
	o.log.Error("OTC_PREMIUM_TRANSFER FAILED",
		"correlationId", correlationID, "error", cause)
	compLog := map[string]string{
		"failureReason": cause.Error(),
		"alertRequired": "true",
	}
	_ = o.finalize(ctx, inst, store.SagaStateFailed, compLog)
	o.publishJSON(ctx, rabbit.RKOtcPremiumFailed, events.OtcPremiumTransferFailed{
		ContractID:    corrIDAsInt64(correlationID),
		FailureReason: cause.Error(),
	})
	return cause
}
