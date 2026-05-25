package saga

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/events"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/rabbit"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

const sagaTypeOtcExercise = "OTC_EXERCISE"

// HandleOtcExercise runs the 5-step OtcExercise saga:
//
//	S1: ReserveFunds(buyer, totalCostRSD)
//	S2: ReserveStocks(seller, ticker, amount)
//	S3: ReleaseFunds(step1) + InternalTransfer(buyer→seller)
//	S4: TransferOwnership(step2, buyer)
//	S5: Final consistency check (key presence in compensation_log)
//
// On any step failure, compensates completed steps in LIFO order and
// publishes saga.OTC_EXERCISE.failed.
//
// On success, publishes saga.OTC_EXERCISE.completed.
func (o *Orchestrator) HandleOtcExercise(ctx context.Context, evt events.OtcExerciseRequested) error {
	correlationID := strconv.FormatInt(evt.ContractID, 10)

	payloadBytes, _ := json.Marshal(evt)
	inst, isNew, err := o.findOrInitialize(ctx, sagaTypeOtcExercise, correlationID, 5, payloadBytes)
	if err != nil {
		return fmt.Errorf("OTC_EXERCISE findOrInitialize: %w", err)
	}

	if !isNew {
		if inst.IsTerminal() {
			o.log.Info("OTC_EXERCISE already terminal — skipping",
				"correlationId", correlationID, "state", inst.State)
			return nil
		}
		if inst.State == store.SagaStateInProgress {
			o.log.Warn("OTC_EXERCISE still IN_PROGRESS — possible duplicate delivery; skipping",
				"correlationId", correlationID)
			return nil
		}
		// FAILED → retry: increment retry count
		inst.RetryCount++
		o.log.Info("OTC_EXERCISE retrying failed saga",
			"correlationId", correlationID, "retryCount", inst.RetryCount)
	}

	correlationSuffix := correlationID
	if inst.RetryCount > 0 {
		correlationSuffix = fmt.Sprintf("%s-retry-%d", correlationID, inst.RetryCount)
	}
	exerciseCorrID := "otc-exercise-" + correlationSuffix

	if err := o.advanceState(ctx, inst); err != nil {
		return fmt.Errorf("OTC_EXERCISE advanceState: %w", err)
	}

	// pricePerStock is in USD; default accounts are RSD — convert once.
	totalCostUSD := evt.PricePerStock.Mul(decimal.NewFromInt(int64(evt.Amount)))
	totalCostRSD, err := o.mk.ConvertCurrencyNoCommission(ctx, "USD", "RSD", totalCostUSD)
	if err != nil {
		return o.otcExerciseFail(ctx, inst, map[string]string{}, 0,
			fmt.Errorf("OTC_EXERCISE currency conversion: %w", err), correlationID)
	}

	compLog := unmarshalLog(inst.CompensationLog)

	// -----------------------------------------------------------------------
	// Step 1: Reserve funds on buyer's account.
	// -----------------------------------------------------------------------
	inst.CurrentStep = 1
	reservationID, err := o.bc.ReserveFunds(ctx, evt.BuyerID, totalCostRSD, exerciseCorrID)
	if err != nil {
		return o.otcExerciseFail(ctx, inst, compLog, 1,
			fmt.Errorf("step 1 ReserveFunds: %w", err), correlationID)
	}
	compLog["step1_reservationId"] = reservationID
	if err := o.saveLog(ctx, inst, compLog); err != nil {
		// Compensate step 1 and abort — we cannot safely proceed without persisted state.
		o.tryReleaseFunds(ctx, reservationID, exerciseCorrID)
		return fmt.Errorf("OTC_EXERCISE saveLog after step 1: %w", err)
	}
	o.log.Info("OTC_EXERCISE step 1 OK", "correlationId", correlationID, "reservationId", reservationID)

	// -----------------------------------------------------------------------
	// Step 2: Reserve seller's stocks.
	// -----------------------------------------------------------------------
	inst.CurrentStep = 2
	stocksReservationID, err := o.td.ReserveStocks(ctx, evt.SellerID, evt.StockTicker, evt.Amount, exerciseCorrID)
	if err != nil {
		return o.otcExerciseFail(ctx, inst, compLog, 2,
			fmt.Errorf("step 2 ReserveStocks: %w", err), correlationID)
	}
	compLog["step2_stocksReservationId"] = stocksReservationID
	if err := o.saveLog(ctx, inst, compLog); err != nil {
		o.tryReleaseStocks(ctx, stocksReservationID, exerciseCorrID)
		o.tryReleaseFunds(ctx, reservationID, exerciseCorrID)
		return fmt.Errorf("OTC_EXERCISE saveLog after step 2: %w", err)
	}
	o.log.Info("OTC_EXERCISE step 2 OK", "correlationId", correlationID, "stocksReservationId", stocksReservationID)

	// -----------------------------------------------------------------------
	// Step 3: Release reservation (buyer already debited in step 1) then
	//         perform the actual transfer.
	// -----------------------------------------------------------------------
	inst.CurrentStep = 3
	// Release reservation first (idempotent if already released).
	if err := o.bc.ReleaseFunds(ctx, reservationID, exerciseCorrID); err != nil {
		// ReleaseFunds failure before transfer: compensate step 2 (stocks) and step 1 (already released? try anyway).
		return o.otcExerciseFail(ctx, inst, compLog, 3,
			fmt.Errorf("step 3 ReleaseFunds: %w", err), correlationID)
	}
	buyerAccount, err := o.bc.ResolveDefaultAccountNumber(ctx, evt.BuyerID)
	if err != nil {
		return o.otcExerciseFail(ctx, inst, compLog, 3,
			fmt.Errorf("step 3 ResolveDefaultAccountNumber(buyer): %w", err), correlationID)
	}
	sellerAccount, err := o.bc.ResolveDefaultAccountNumber(ctx, evt.SellerID)
	if err != nil {
		return o.otcExerciseFail(ctx, inst, compLog, 3,
			fmt.Errorf("step 3 ResolveDefaultAccountNumber(seller): %w", err), correlationID)
	}
	transferID, err := o.bc.InternalTransfer(ctx, buyerAccount, sellerAccount, totalCostRSD, exerciseCorrID)
	if err != nil {
		return o.otcExerciseFail(ctx, inst, compLog, 3,
			fmt.Errorf("step 3 InternalTransfer: %w", err), correlationID)
	}
	compLog["step3_transferId"] = transferID
	if err := o.saveLog(ctx, inst, compLog); err != nil {
		o.tryReverseTransfer(ctx, transferID, exerciseCorrID)
		o.tryReleaseStocks(ctx, stocksReservationID, exerciseCorrID)
		return fmt.Errorf("OTC_EXERCISE saveLog after step 3: %w", err)
	}
	o.log.Info("OTC_EXERCISE step 3 OK", "correlationId", correlationID, "transferId", transferID)

	// -----------------------------------------------------------------------
	// Step 4: Transfer stock ownership to buyer.
	// -----------------------------------------------------------------------
	inst.CurrentStep = 4
	ownershipTransferID, err := o.td.TransferOwnership(ctx, stocksReservationID, evt.BuyerID, exerciseCorrID)
	if err != nil {
		return o.otcExerciseFail(ctx, inst, compLog, 4,
			fmt.Errorf("step 4 TransferOwnership: %w", err), correlationID)
	}
	compLog["step4_ownershipTransferId"] = ownershipTransferID
	if err := o.saveLog(ctx, inst, compLog); err != nil {
		o.tryReverseOwnership(ctx, ownershipTransferID, exerciseCorrID)
		o.tryReverseTransfer(ctx, transferID, exerciseCorrID)
		o.tryReleaseStocks(ctx, stocksReservationID, exerciseCorrID)
		return fmt.Errorf("OTC_EXERCISE saveLog after step 4: %w", err)
	}
	o.log.Info("OTC_EXERCISE step 4 OK", "correlationId", correlationID, "ownershipTransferId", ownershipTransferID)

	// -----------------------------------------------------------------------
	// Step 5: Final consistency check — verify all expected log keys are present.
	// -----------------------------------------------------------------------
	inst.CurrentStep = 5
	for _, key := range []string{"step1_reservationId", "step2_stocksReservationId", "step3_transferId", "step4_ownershipTransferId"} {
		if _, ok := compLog[key]; !ok {
			return o.otcExerciseFail(ctx, inst, compLog, 5,
				fmt.Errorf("step 5 consistency check failed: missing key %q", key), correlationID)
		}
	}
	o.log.Info("OTC_EXERCISE step 5 OK (consistency check)", "correlationId", correlationID)

	// -----------------------------------------------------------------------
	// All steps complete.
	// -----------------------------------------------------------------------
	if err := o.finalize(ctx, inst, store.SagaStateCompleted, compLog); err != nil {
		o.log.Error("OTC_EXERCISE finalize error (saga already logically complete)",
			"correlationId", correlationID, "error", err)
	}
	o.publishJSON(ctx, rabbit.RKOtcExerciseCompleted, events.OtcExerciseCompleted{ContractID: evt.ContractID})
	o.log.Info("OTC_EXERCISE COMPLETED", "correlationId", correlationID)
	return nil
}

// otcExerciseFail transitions the saga to COMPENSATING, runs LIFO compensation
// for completed steps, then transitions to FAILED. Returns the original cause error
// so the caller can return it to the listener (which nack's the message).
func (o *Orchestrator) otcExerciseFail(
	ctx context.Context,
	inst *store.SagaInstance,
	compLog map[string]string,
	failedAtStep int,
	cause error,
	correlationID string,
) error {
	o.log.Error("OTC_EXERCISE step failed — compensating",
		"correlationId", correlationID,
		"failedAtStep", failedAtStep,
		"error", cause,
	)

	// Persist compensating state.
	inst.State = store.SagaStateCompensating
	compLog["failedAtStep"] = strconv.Itoa(failedAtStep)
	compLog["failureReason"] = cause.Error()
	inst.CompensationLog = marshalLog(compLog)
	_ = o.store.UpdateOptimistic(ctx, inst)

	// LIFO compensation. Each compensation step is attempted independently;
	// a failure is logged but does not stop the remaining compensations.
	exerciseCorrID := "otc-exercise-" + correlationID
	if inst.RetryCount > 0 {
		exerciseCorrID = fmt.Sprintf("otc-exercise-%s-retry-%d", correlationID, inst.RetryCount)
	}

	if id, ok := compLog["step4_ownershipTransferId"]; ok {
		o.tryReverseOwnership(ctx, id, exerciseCorrID)
	}
	if id, ok := compLog["step3_transferId"]; ok {
		o.tryReverseTransfer(ctx, id, exerciseCorrID)
	}
	if id, ok := compLog["step2_stocksReservationId"]; ok {
		o.tryReleaseStocks(ctx, id, exerciseCorrID)
	}
	// Step 1 reservation: idempotent — release even if step 3 already released it.
	if id, ok := compLog["step1_reservationId"]; ok {
		o.tryReleaseFunds(ctx, id, exerciseCorrID)
	}

	// Persist FAILED state.
	inst.State = store.SagaStateFailed
	inst.CompensationLog = marshalLog(compLog)
	_ = o.store.UpdateOptimistic(ctx, inst)

	o.publishJSON(ctx, rabbit.RKOtcExerciseFailed, events.OtcExerciseFailed{
		ContractID:    corrIDAsInt64(inst.CorrelationID),
		FailureReason: cause.Error(),
		FailedAtStep:  failedAtStep,
	})
	return cause
}

// corrIDAsInt64 parses a correlationID string as int64 for result event payloads.
// Returns 0 on parse failure.
func corrIDAsInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// ---------------------------------------------------------------------------
// idempotent "best effort" compensation helpers — log on error, never panic.
// ---------------------------------------------------------------------------

func (o *Orchestrator) tryReleaseFunds(ctx context.Context, reservationID, corrID string) {
	if err := o.bc.ReleaseFunds(ctx, reservationID, corrID); err != nil {
		o.log.Error("compensation ReleaseFunds failed",
			"reservationId", reservationID, "error", err)
	}
}

func (o *Orchestrator) tryReleaseStocks(ctx context.Context, reservationID, corrID string) {
	if err := o.td.ReleaseStocks(ctx, reservationID, corrID); err != nil {
		o.log.Error("compensation ReleaseStocks failed",
			"reservationId", reservationID, "error", err)
	}
}

func (o *Orchestrator) tryReverseTransfer(ctx context.Context, transferID, corrID string) {
	if err := o.bc.ReverseTransfer(ctx, transferID, corrID); err != nil {
		o.log.Error("compensation ReverseTransfer failed",
			"transferId", transferID, "error", err)
	}
}

func (o *Orchestrator) tryReverseOwnership(ctx context.Context, ownershipTransferID, corrID string) {
	if err := o.td.ReverseOwnership(ctx, ownershipTransferID, corrID); err != nil {
		o.log.Error("compensation ReverseOwnership failed",
			"ownershipTransferId", ownershipTransferID, "error", err)
	}
}
