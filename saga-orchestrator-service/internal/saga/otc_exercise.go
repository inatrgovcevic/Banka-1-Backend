package saga

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/events"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/rabbit"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

const sagaTypeOtcExercise = "OTC_EXERCISE"

// HandleOtcExercise runs the 5-step OtcExercise saga:
//
//	F1: ReserveFunds(buyer, totalCostRSD)
//	F2: ReserveStocks(seller, ticker, amount)
//	F3: ReleaseFunds(step1) + InternalTransfer(buyer→seller)
//	F4: TransferOwnership(step2, buyer)
//	F5: Final consistency check
//
// On any step failure, compensates completed steps in LIFO order.
// Terminal states: COMPLETED (success) or COMPENSATED (compensators all ran).
//
// Fault injection is honoured when non-nil (SAGA_TEST_MODE traffic only).
func (o *Orchestrator) HandleOtcExercise(ctx context.Context, evt events.OtcExerciseRequested) error {
	correlationID := strconv.FormatInt(evt.ContractID, 10)
	fi := evt.FaultInjection

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
			// Crash recovery: process was killed while the saga was running.
			// Re-run with the same exerciseCorrID — all downstream calls are
			// idempotent per correlationID. advanceState's optimistic lock will
			// reject any true concurrent duplicate with ErrOptimisticLockConflict.
			o.log.Warn("OTC_EXERCISE crash recovery: re-running IN_PROGRESS saga",
				"correlationId", correlationID)
			inst.CompensationLog = nil // reset so re-run appends a clean step log
			// Fall through without incrementing RetryCount.
		} else {
			inst.RetryCount++
			o.log.Info("OTC_EXERCISE retrying",
				"correlationId", correlationID, "retryCount", inst.RetryCount)
		}
	}

	correlationSuffix := correlationID
	if inst.RetryCount > 0 {
		correlationSuffix = fmt.Sprintf("%s-retry-%d", correlationID, inst.RetryCount)
	}
	exerciseCorrID := "otc-exercise-" + correlationSuffix

	if err := o.advanceState(ctx, inst); err != nil {
		return fmt.Errorf("OTC_EXERCISE advanceState: %w", err)
	}

	totalCostUSD := evt.PricePerStock.Mul(decimal.NewFromInt(int64(evt.Amount)))
	totalCostRSD, err := o.mk.ConvertCurrencyNoCommission(ctx, "USD", "RSD", totalCostUSD)
	if err != nil {
		sl := newSagaLog()
		return o.otcExerciseFail(ctx, inst, sl, 0,
			fmt.Errorf("OTC_EXERCISE currency conversion: %w", err), correlationID, fi)
	}

	sl := unmarshalSagaLog(inst.CompensationLog)

	// -----------------------------------------------------------------------
	// F1: Reserve funds on buyer's account.
	// -----------------------------------------------------------------------
	inst.CurrentStep = 1
	if err := o.injectDelay(ctx, "F1", fi); err != nil {
		return err
	}
	if err := o.injectForceFail("F1", "before", fi); err != nil {
		sl.appendStep("F1", "err", err.Error())
		return o.otcExerciseFail(ctx, inst, sl, 1, err, correlationID, fi)
	}
	reservationID, err := o.bc.ReserveFunds(ctx, evt.BuyerID, totalCostRSD, exerciseCorrID)
	if afterErr := o.injectForceFail("F1", "after", fi); afterErr != nil && err == nil {
		// side effect committed; store ref before compensating
		sl.Refs["step1_reservationId"] = reservationID
		sl.appendStep("F1", "err", afterErr.Error())
		return o.otcExerciseFail(ctx, inst, sl, 1, afterErr, correlationID, fi)
	}
	if err != nil {
		sl.appendStep("F1", "err", err.Error())
		return o.otcExerciseFail(ctx, inst, sl, 1, err, correlationID, fi)
	}
	sl.Refs["step1_reservationId"] = reservationID
	sl.appendStep("F1", "ok", "")
	if err := o.saveFullLog(ctx, inst, sl); err != nil {
		o.tryReleaseFundsOnce(ctx, reservationID, exerciseCorrID)
		return fmt.Errorf("OTC_EXERCISE saveLog after F1: %w", err)
	}
	o.log.Info("OTC_EXERCISE F1 OK", "correlationId", correlationID)

	// -----------------------------------------------------------------------
	// F2: Reserve seller's stocks.
	// -----------------------------------------------------------------------
	inst.CurrentStep = 2
	if err := o.injectDelay(ctx, "F2", fi); err != nil {
		return err
	}
	if err := o.injectForceFail("F2", "before", fi); err != nil {
		sl.appendStep("F2", "err", err.Error())
		return o.otcExerciseFail(ctx, inst, sl, 2, err, correlationID, fi)
	}
	stocksResID, err := o.td.ReserveStocks(ctx, evt.SellerID, evt.StockTicker, evt.Amount, exerciseCorrID)
	if afterErr := o.injectForceFail("F2", "after", fi); afterErr != nil && err == nil {
		sl.Refs["step2_stocksReservationId"] = stocksResID
		sl.appendStep("F2", "err", afterErr.Error())
		return o.otcExerciseFail(ctx, inst, sl, 2, afterErr, correlationID, fi)
	}
	if err != nil {
		sl.appendStep("F2", "err", err.Error())
		return o.otcExerciseFail(ctx, inst, sl, 2, err, correlationID, fi)
	}
	sl.Refs["step2_stocksReservationId"] = stocksResID
	sl.appendStep("F2", "ok", "")
	if err := o.saveFullLog(ctx, inst, sl); err != nil {
		o.tryReleaseStocksOnce(ctx, stocksResID, exerciseCorrID)
		o.tryReleaseFundsOnce(ctx, reservationID, exerciseCorrID)
		return fmt.Errorf("OTC_EXERCISE saveLog after F2: %w", err)
	}
	o.log.Info("OTC_EXERCISE F2 OK", "correlationId", correlationID)

	// -----------------------------------------------------------------------
	// F3: Release reservation + internal fund transfer buyer→seller.
	// -----------------------------------------------------------------------
	inst.CurrentStep = 3
	if err := o.injectDelay(ctx, "F3", fi); err != nil {
		return err
	}
	if err := o.injectForceFail("F3", "before", fi); err != nil {
		sl.appendStep("F3", "err", err.Error())
		return o.otcExerciseFail(ctx, inst, sl, 3, err, correlationID, fi)
	}
	if err := o.bc.ReleaseFunds(ctx, reservationID, exerciseCorrID); err != nil {
		sl.appendStep("F3", "err", err.Error())
		return o.otcExerciseFail(ctx, inst, sl, 3,
			fmt.Errorf("F3 ReleaseFunds: %w", err), correlationID, fi)
	}
	buyerAccount, err := o.bc.ResolveDefaultAccountNumber(ctx, evt.BuyerID)
	if err != nil {
		sl.appendStep("F3", "err", err.Error())
		return o.otcExerciseFail(ctx, inst, sl, 3,
			fmt.Errorf("F3 ResolveAccount(buyer): %w", err), correlationID, fi)
	}
	sellerAccount, err := o.bc.ResolveDefaultAccountNumber(ctx, evt.SellerID)
	if err != nil {
		sl.appendStep("F3", "err", err.Error())
		return o.otcExerciseFail(ctx, inst, sl, 3,
			fmt.Errorf("F3 ResolveAccount(seller): %w", err), correlationID, fi)
	}
	transferID, err := o.bc.InternalTransfer(ctx, buyerAccount, sellerAccount, totalCostRSD, exerciseCorrID)
	if afterErr := o.injectForceFail("F3", "after", fi); afterErr != nil && err == nil {
		sl.Refs["step3_transferId"] = transferID
		sl.appendStep("F3", "err", afterErr.Error())
		return o.otcExerciseFail(ctx, inst, sl, 3, afterErr, correlationID, fi)
	}
	if err != nil {
		sl.appendStep("F3", "err", err.Error())
		return o.otcExerciseFail(ctx, inst, sl, 3,
			fmt.Errorf("F3 InternalTransfer: %w", err), correlationID, fi)
	}
	sl.Refs["step3_transferId"] = transferID
	sl.appendStep("F3", "ok", "")
	if err := o.saveFullLog(ctx, inst, sl); err != nil {
		o.tryReverseTransferOnce(ctx, transferID, exerciseCorrID)
		o.tryReleaseStocksOnce(ctx, stocksResID, exerciseCorrID)
		return fmt.Errorf("OTC_EXERCISE saveLog after F3: %w", err)
	}
	o.log.Info("OTC_EXERCISE F3 OK", "correlationId", correlationID)

	// -----------------------------------------------------------------------
	// F4: Transfer stock ownership to buyer.
	// -----------------------------------------------------------------------
	inst.CurrentStep = 4
	if err := o.injectDelay(ctx, "F4", fi); err != nil {
		return err
	}
	if err := o.injectForceFail("F4", "before", fi); err != nil {
		sl.appendStep("F4", "err", err.Error())
		return o.otcExerciseFail(ctx, inst, sl, 4, err, correlationID, fi)
	}
	ownershipID, err := o.td.TransferOwnership(ctx, stocksResID, evt.BuyerID, exerciseCorrID)
	if afterErr := o.injectForceFail("F4", "after", fi); afterErr != nil && err == nil {
		sl.Refs["step4_ownershipTransferId"] = ownershipID
		sl.appendStep("F4", "err", afterErr.Error())
		return o.otcExerciseFail(ctx, inst, sl, 4, afterErr, correlationID, fi)
	}
	if err != nil {
		sl.appendStep("F4", "err", err.Error())
		return o.otcExerciseFail(ctx, inst, sl, 4,
			fmt.Errorf("F4 TransferOwnership: %w", err), correlationID, fi)
	}
	sl.Refs["step4_ownershipTransferId"] = ownershipID
	sl.appendStep("F4", "ok", "")
	if err := o.saveFullLog(ctx, inst, sl); err != nil {
		o.tryReverseOwnershipOnce(ctx, ownershipID, exerciseCorrID)
		o.tryReverseTransferOnce(ctx, transferID, exerciseCorrID)
		o.tryReleaseStocksOnce(ctx, stocksResID, exerciseCorrID)
		return fmt.Errorf("OTC_EXERCISE saveLog after F4: %w", err)
	}
	o.log.Info("OTC_EXERCISE F4 OK", "correlationId", correlationID)

	// -----------------------------------------------------------------------
	// F5: Final consistency check — all expected refs present.
	// -----------------------------------------------------------------------
	inst.CurrentStep = 5
	if err := o.injectDelay(ctx, "F5", fi); err != nil {
		return err
	}
	if err := o.injectForceFail("F5", "before", fi); err != nil {
		sl.appendStep("F5", "err", err.Error())
		return o.otcExerciseFail(ctx, inst, sl, 5, err, correlationID, fi)
	}
	for _, key := range []string{"step1_reservationId", "step2_stocksReservationId", "step3_transferId", "step4_ownershipTransferId"} {
		if _, ok := sl.Refs[key]; !ok {
			err := fmt.Errorf("F5 consistency: missing %q", key)
			sl.appendStep("F5", "err", err.Error())
			return o.otcExerciseFail(ctx, inst, sl, 5, err, correlationID, fi)
		}
	}
	if afterErr := o.injectForceFail("F5", "after", fi); afterErr != nil {
		sl.appendStep("F5", "err", afterErr.Error())
		return o.otcExerciseFail(ctx, inst, sl, 5, afterErr, correlationID, fi)
	}
	sl.appendStep("F5", "ok", "")
	o.log.Info("OTC_EXERCISE F5 OK", "correlationId", correlationID)

	// -----------------------------------------------------------------------
	// All steps complete → COMPLETED.
	// -----------------------------------------------------------------------
	inst.State = store.SagaStateCompleted
	inst.CompensationLog = sl.marshalBytes()
	if err := o.store.UpdateOptimistic(ctx, inst); err != nil {
		o.log.Error("OTC_EXERCISE finalize COMPLETED error", "correlationId", correlationID, "error", err)
	}
	o.publishJSON(ctx, rabbit.RKOtcExerciseCompleted, events.OtcExerciseCompleted{ContractID: evt.ContractID})
	o.log.Info("OTC_EXERCISE COMPLETED", "correlationId", correlationID)
	return nil
}

// otcExerciseFail persists COMPENSATING, runs LIFO compensation with retry,
// then finalises to COMPENSATED (all compensators succeeded) or FAILED (stuck).
func (o *Orchestrator) otcExerciseFail(
	ctx context.Context,
	inst *store.SagaInstance,
	sl *SagaLog,
	failedAtStep int,
	cause error,
	correlationID string,
	fi *events.FaultInjection,
) error {
	o.log.Error("OTC_EXERCISE step failed — compensating",
		"correlationId", correlationID,
		"failedAtStep", failedAtStep,
		"error", cause,
	)

	inst.State = store.SagaStateCompensating
	inst.CompensationLog = sl.marshalBytes()
	_ = o.store.UpdateOptimistic(ctx, inst)

	exerciseCorrID := "otc-exercise-" + correlationID
	if inst.RetryCount > 0 {
		exerciseCorrID = fmt.Sprintf("otc-exercise-%s-retry-%d", correlationID, inst.RetryCount)
	}

	allOK := true

	// LIFO compensation. Each step is retried until success or context done.
	if id, ok := sl.Refs["step4_ownershipTransferId"]; ok {
		if !o.compensateWithRetry(ctx, "C4", fi, sl, inst, func() error {
			return o.td.ReverseOwnership(ctx, id, exerciseCorrID)
		}) {
			allOK = false
		}
	}
	if id, ok := sl.Refs["step3_transferId"]; ok {
		if !o.compensateWithRetry(ctx, "C3", fi, sl, inst, func() error {
			return o.bc.ReverseTransfer(ctx, id, exerciseCorrID)
		}) {
			allOK = false
		}
	}
	if id, ok := sl.Refs["step2_stocksReservationId"]; ok {
		if !o.compensateWithRetry(ctx, "C2", fi, sl, inst, func() error {
			return o.td.ReleaseStocks(ctx, id, exerciseCorrID)
		}) {
			allOK = false
		}
	}
	if id, ok := sl.Refs["step1_reservationId"]; ok {
		if !o.compensateWithRetry(ctx, "C1", fi, sl, inst, func() error {
			return o.bc.ReleaseFunds(ctx, id, exerciseCorrID)
		}) {
			allOK = false
		}
	}

	if allOK {
		inst.State = store.SagaStateCompensated
	} else {
		inst.State = store.SagaStateFailed
	}
	inst.CompensationLog = sl.marshalBytes()
	_ = o.store.UpdateOptimistic(ctx, inst)

	o.publishJSON(ctx, rabbit.RKOtcExerciseFailed, events.OtcExerciseFailed{
		ContractID:    corrIDAsInt64(inst.CorrelationID),
		FailureReason: cause.Error(),
		FailedAtStep:  failedAtStep,
	})
	return cause
}

// compensateWithRetry runs fn in a loop until success, honoring fault injection.
// Appends step records for each attempt. Returns true if fn eventually succeeded.
func (o *Orchestrator) compensateWithRetry(
	ctx context.Context,
	stepName string,
	fi *events.FaultInjection,
	sl *SagaLog,
	inst *store.SagaInstance,
	fn func() error,
) bool {
	const maxAttempts = 120 // 120 × 500ms = 60s — enough for a service restart
	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			sl.appendStep(stepName, "err", "context cancelled")
			_ = o.saveFullLog(ctx, inst, sl)
			return false
		default:
		}

		// Fault injection: fail this compensator N times before succeeding.
		if fi != nil && fi.CompensateFailStep == stepName {
			count := sl.CompCounts[stepName]
			if count < fi.CompensateFailTimes {
				sl.CompCounts[stepName]++
				sl.appendStep(stepName, "err", "fault injection")
				_ = o.saveFullLog(ctx, inst, sl)
				time.Sleep(150 * time.Millisecond)
				continue
			}
		}

		if err := fn(); err != nil {
			o.log.Warn("compensator failed, retrying",
				"step", stepName, "attempt", attempt+1, "error", err)
			sl.appendStep(stepName, "err", err.Error())
			_ = o.saveFullLog(ctx, inst, sl)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		sl.appendStep(stepName, "ok", "")
		_ = o.saveFullLog(ctx, inst, sl)
		return true
	}
	o.log.Error("compensator exhausted retries — saga stuck", "step", stepName)
	return false
}

// ---------------------------------------------------------------------------
// Fault injection helpers
// ---------------------------------------------------------------------------

// injectDelay sleeps for the configured duration if this step matches.
func (o *Orchestrator) injectDelay(_ context.Context, step string, fi *events.FaultInjection) error {
	if fi == nil || fi.InjectDelayStep != step || fi.InjectDelayMs <= 0 {
		return nil
	}
	o.log.Info("fault injection: delay", "step", step, "ms", fi.InjectDelayMs)
	time.Sleep(time.Duration(fi.InjectDelayMs) * time.Millisecond)
	return nil
}

// injectForceFail returns a non-nil error if fault injection requests this step fail.
// kind is "before" or "after" — only matches when ForceFailKind equals kind (or is empty).
func (o *Orchestrator) injectForceFail(step, kind string, fi *events.FaultInjection) error {
	if fi == nil || fi.ForceFailStep != step {
		return nil
	}
	if fi.ForceFailKind != "" && fi.ForceFailKind != kind {
		return nil
	}
	// Default kind is "before" when not specified.
	if fi.ForceFailKind == "" && kind != "before" {
		return nil
	}
	return errors.New("fault injection: forced failure at " + step + " (" + kind + ")")
}

// ---------------------------------------------------------------------------
// One-shot (non-retrying) compensate helpers — used only during saveLog
// failures where we must undo the just-made call before returning an error.
// ---------------------------------------------------------------------------

func (o *Orchestrator) tryReleaseFundsOnce(ctx context.Context, reservationID, corrID string) {
	if err := o.bc.ReleaseFunds(ctx, reservationID, corrID); err != nil {
		o.log.Error("emergency ReleaseFunds failed", "reservationId", reservationID, "error", err)
	}
}

func (o *Orchestrator) tryReleaseStocksOnce(ctx context.Context, reservationID, corrID string) {
	if err := o.td.ReleaseStocks(ctx, reservationID, corrID); err != nil {
		o.log.Error("emergency ReleaseStocks failed", "reservationId", reservationID, "error", err)
	}
}

func (o *Orchestrator) tryReverseTransferOnce(ctx context.Context, transferID, corrID string) {
	if err := o.bc.ReverseTransfer(ctx, transferID, corrID); err != nil {
		o.log.Error("emergency ReverseTransfer failed", "transferId", transferID, "error", err)
	}
}

func (o *Orchestrator) tryReverseOwnershipOnce(ctx context.Context, ownershipID, corrID string) {
	if err := o.td.ReverseOwnership(ctx, ownershipID, corrID); err != nil {
		o.log.Error("emergency ReverseOwnership failed", "ownershipTransferId", ownershipID, "error", err)
	}
}

// corrIDAsInt64 parses a correlationID string as int64 for result event payloads.
func corrIDAsInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
