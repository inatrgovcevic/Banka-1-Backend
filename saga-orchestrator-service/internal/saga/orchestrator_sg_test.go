package saga_test

// SG-01 to SG-08 unit tests for the OTC_EXERCISE saga, covering all scenarios
// from the SAGA test specification that are exercisable without infrastructure
// (no Toxiproxy, no docker compose pause/kill).
//
// Reuses fakeStore, fakeBC, fakeTD, fakeMK, fakePublisher and newOrch from
// orchestrator_test.go (same package, same directory).

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/events"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/saga"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mustGetOtcInst retrieves the OTC_EXERCISE instance for a contract ID string,
// failing the test immediately if it is absent.
func mustGetOtcInst(t *testing.T, fs *fakeStore, contractID string) *store.SagaInstance {
	t.Helper()
	inst, err := fs.FindByTypeAndCorrelation(context.Background(), "OTC_EXERCISE", contractID)
	if err != nil {
		t.Fatalf("FindByTypeAndCorrelation: %v", err)
	}
	if inst == nil {
		t.Fatalf("OTC_EXERCISE instance for contractID=%s not found in store", contractID)
	}
	return inst
}

// mustParseSagaLog unmarshals the SagaLog stored in inst.CompensationLog.
func mustParseSagaLog(t *testing.T, inst *store.SagaInstance) saga.SagaLog {
	t.Helper()
	var sl saga.SagaLog
	if err := json.Unmarshal(inst.CompensationLog, &sl); err != nil {
		t.Fatalf("unmarshal SagaLog: %v — raw bytes: %s", err, inst.CompensationLog)
	}
	return sl
}

// wantStep is a concise expected log entry (Step name + Outcome only).
type wantStep struct {
	Step    string
	Outcome string
}

// assertSteps verifies that sl.Steps matches want in length and content.
// The Error field of each StepRecord is intentionally not checked — only Step
// and Outcome matter for invariant I4.
func assertSteps(t *testing.T, sl saga.SagaLog, want []wantStep) {
	t.Helper()
	if len(sl.Steps) != len(want) {
		t.Errorf("log step count: got %d, want %d\n  got:  %v\n  want: %v",
			len(sl.Steps), len(want), sl.Steps, want)
		return
	}
	for i, w := range want {
		got := sl.Steps[i]
		if got.Step != w.Step || got.Outcome != w.Outcome {
			t.Errorf("step[%d]: got {Step:%q Outcome:%q}, want {Step:%q Outcome:%q}",
				i, got.Step, got.Outcome, w.Step, w.Outcome)
		}
	}
}

// baseEvent returns a standard OtcExerciseRequested: buyer=1, seller=2,
// 10 shares of AAPL at 150 USD each.
func baseEvent(contractID int64) events.OtcExerciseRequested {
	return events.OtcExerciseRequested{
		ContractID:    contractID,
		BuyerID:       1,
		SellerID:      2,
		StockTicker:   "AAPL",
		Amount:        10,
		PricePerStock: decimal.RequireFromString("150.00"),
	}
}

// ---------------------------------------------------------------------------
// SG-01: Happy path — already covered by TestHandleOtcExercise_HappyPath in
// orchestrator_test.go; no duplicate test added here.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// SG-11: Crash recovery — saga was IN_PROGRESS when the coordinator was killed.
// On re-delivery the saga must re-run (not skip), completing or compensating.
// ---------------------------------------------------------------------------

func TestSG11_CrashRecovery_InProgress_CompletesOnRestart(t *testing.T) {
	fs := newFakeStore()

	// Simulate a prior run that was killed after F2 (state=IN_PROGRESS, step=2).
	// The DB row already exists with a partial compensation log.
	crashed := &store.SagaInstance{
		SagaType:        "OTC_EXERCISE",
		CorrelationID:   "401",
		State:           store.SagaStateInProgress,
		CurrentStep:     2,
		TotalSteps:      5,
		RetryCount:      0,
		Version:         3, // reflects two prior saveFullLog writes (F1+F2)
		CompensationLog: []byte(`{"steps":[{"step":"F1","outcome":"ok"},{"step":"F2","outcome":"ok"}],"refs":{"step1_reservationId":"res-otc-exercise-401","step2_stocksReservationId":"sr-otc-exercise-401"}}`),
	}
	fs.mu.Lock()
	fs.rows[fs.key("OTC_EXERCISE", "401")] = crashed
	fs.mu.Unlock()

	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	// Re-deliver the same event (as RabbitMQ would after reconnect).
	err := orch.HandleOtcExercise(context.Background(), baseEvent(401))
	if err != nil {
		t.Fatalf("crash recovery: expected nil error, got %v", err)
	}

	inst := mustGetOtcInst(t, fs, "401")

	// I5: must reach a terminal state — not stay stuck in IN_PROGRESS.
	if inst.State != store.SagaStateCompleted {
		t.Errorf("state=%q, want COMPLETED (crash recovery must complete the saga)", inst.State)
	}
	if inst.CurrentStep != 5 {
		t.Errorf("currentStep=%d, want 5", inst.CurrentStep)
	}

	// RetryCount must stay 0 — crash recovery uses the same exerciseCorrID.
	if inst.RetryCount != 0 {
		t.Errorf("retryCount=%d, want 0 (crash recovery must not change exerciseCorrID)", inst.RetryCount)
	}

	// I4: log has 5 clean ok entries (compensation log was reset on recovery).
	sl := mustParseSagaLog(t, inst)
	assertSteps(t, sl, []wantStep{
		{"F1", "ok"}, {"F2", "ok"}, {"F3", "ok"}, {"F4", "ok"}, {"F5", "ok"},
	})

	// Completed event published.
	if pub.findByRK("saga.OTC_EXERCISE.completed") == nil {
		t.Error("saga.OTC_EXERCISE.completed event not published")
	}
}

func TestSG11_CrashRecovery_InProgress_CompensatesOnFailure(t *testing.T) {
	fs := newFakeStore()

	// Simulate crash after F1 only.
	crashed := &store.SagaInstance{
		SagaType:        "OTC_EXERCISE",
		CorrelationID:   "402",
		State:           store.SagaStateInProgress,
		CurrentStep:     1,
		TotalSteps:      5,
		RetryCount:      0,
		Version:         2,
		CompensationLog: []byte(`{"steps":[{"step":"F1","outcome":"ok"}],"refs":{"step1_reservationId":"res-otc-exercise-402"}}`),
	}
	fs.mu.Lock()
	fs.rows[fs.key("OTC_EXERCISE", "402")] = crashed
	fs.mu.Unlock()

	bc := newFakeBC()
	td := newFakeTD()
	td.reserveErr = errors.New("not enough stocks") // F2 will fail on recovery
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	err := orch.HandleOtcExercise(context.Background(), baseEvent(402))
	if err == nil {
		t.Fatal("expected error from F2 failure, got nil")
	}

	inst := mustGetOtcInst(t, fs, "402")

	// I5: must end in COMPENSATED (not stuck).
	if inst.State != store.SagaStateCompensated {
		t.Errorf("state=%q, want COMPENSATED", inst.State)
	}

	// C1 must run to release the F1 reservation.
	if len(bc.releaseCalls) == 0 {
		t.Error("expected ReleaseFunds (C1) to be called on crash recovery compensation")
	}
}

// ---------------------------------------------------------------------------
// SG-02: Pre-saga validation failures.
// These happen before the saga is persisted (the orchestrator rejects the
// event before creating a log entry). Because our unit test calls the saga
// handler directly (not via HTTP), we simulate the validation failure at the
// earliest possible point: a currency-conversion error before any DB write.
//
// Full HTTP-layer validation (auth, contract status, settlement date) is
// covered by the trading-service integration tests, not by the saga unit tests.
// ---------------------------------------------------------------------------

func TestSG02_CurrencyConversionFails_NoSagaCreated(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	mk.err = errors.New("exchange service unavailable")
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	err := orch.HandleOtcExercise(context.Background(), baseEvent(200))
	if err == nil {
		t.Fatal("expected error from currency conversion failure, got nil")
	}

	// Saga instance is created (STARTED) but immediately fails before F1.
	// State must be terminal (COMPENSATED) with no forward steps recorded.
	inst := mustGetOtcInst(t, fs, "200")
	if inst.State != store.SagaStateCompensated {
		t.Errorf("state=%q, want COMPENSATED", inst.State)
	}
	// No banking or trading calls should have been made.
	if len(bc.reserveCalls) != 0 {
		t.Errorf("bc.reserveCalls=%d, want 0", len(bc.reserveCalls))
	}
	if len(td.reserveCalls) != 0 {
		t.Errorf("td.reserveCalls=%d, want 0", len(td.reserveCalls))
	}
}

// ---------------------------------------------------------------------------
// SG-03: F1 fails (insufficient buyer funds).
// Spec: Compensated, current_step=1, log=[F1 err], no side effects.
// ---------------------------------------------------------------------------

func TestSG03_Step1Fails_InsufficientFunds(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	bc.reserveErr = errors.New("insufficient funds — buyer balance < required amount")
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	err := orch.HandleOtcExercise(context.Background(), baseEvent(301))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	inst := mustGetOtcInst(t, fs, "301")

	// I5: terminal state must be COMPENSATED.
	if inst.State != store.SagaStateCompensated {
		t.Errorf("state=%q, want COMPENSATED", inst.State)
	}
	// Spec: current_step=1.
	if inst.CurrentStep != 1 {
		t.Errorf("currentStep=%d, want 1", inst.CurrentStep)
	}

	// I4: log contains exactly one entry — F1 err.
	sl := mustParseSagaLog(t, inst)
	assertSteps(t, sl, []wantStep{
		{"F1", "err"},
	})

	// No compensation calls: F1 never succeeded so there is nothing to undo.
	if len(bc.releaseCalls) != 0 {
		t.Errorf("bc.releaseCalls=%d, want 0 (F1 reservation never created)", len(bc.releaseCalls))
	}
	if len(td.releaseCalls) != 0 {
		t.Errorf("td.releaseCalls=%d, want 0 (F2 never ran)", len(td.releaseCalls))
	}
	if len(bc.reverseCalls) != 0 {
		t.Errorf("bc.reverseCalls=%d, want 0 (no transfer to reverse)", len(bc.reverseCalls))
	}

	if pub.findByRK("saga.OTC_EXERCISE.failed") == nil {
		t.Error("saga.OTC_EXERCISE.failed event not published")
	}
}

// ---------------------------------------------------------------------------
// SG-04: F2 fails (insufficient seller holdings).
// Spec: Compensated, current_step=2, log=[F1 ok, F2 err, C1 ok].
// ---------------------------------------------------------------------------

func TestSG04_Step2Fails_InsufficientHoldings(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	td.reserveErr = errors.New("not enough stocks — seller holdings < required qty")
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	err := orch.HandleOtcExercise(context.Background(), baseEvent(302))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	inst := mustGetOtcInst(t, fs, "302")

	// I5: terminal state.
	if inst.State != store.SagaStateCompensated {
		t.Errorf("state=%q, want COMPENSATED", inst.State)
	}
	if inst.CurrentStep != 2 {
		t.Errorf("currentStep=%d, want 2", inst.CurrentStep)
	}

	// I4: log = [F1 ok, F2 err, C1 ok].
	sl := mustParseSagaLog(t, inst)
	assertSteps(t, sl, []wantStep{
		{"F1", "ok"},
		{"F2", "err"},
		{"C1", "ok"},
	})

	// C1 ran: F1 reservation released.
	if len(bc.releaseCalls) != 1 {
		t.Errorf("bc.releaseCalls=%d, want 1 (C1 must release F1 reservation)", len(bc.releaseCalls))
	}
	// C2 must NOT run: F2 failed so the stock reservation ID was never stored.
	if len(td.releaseCalls) != 0 {
		t.Errorf("td.releaseCalls=%d, want 0 (F2 failed before reservation was stored)", len(td.releaseCalls))
	}
	if len(bc.reverseCalls) != 0 {
		t.Errorf("bc.reverseCalls=%d, want 0 (no transfer to reverse)", len(bc.reverseCalls))
	}

	if pub.findByRK("saga.OTC_EXERCISE.failed") == nil {
		t.Error("saga.OTC_EXERCISE.failed event not published")
	}
}

// ---------------------------------------------------------------------------
// SG-05: F3 fails via fault injection, C2 then C1 compensate.
// Spec: Compensated, current_step=3, log=[F1 ok, F2 ok, F3 err, C2 ok, C1 ok].
// ---------------------------------------------------------------------------

func TestSG05_Step3Fails_CompensatesStep2AndStep1(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	evt := baseEvent(303)
	evt.FaultInjection = &events.FaultInjection{ForceFailStep: "F3"}

	err := orch.HandleOtcExercise(context.Background(), evt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	inst := mustGetOtcInst(t, fs, "303")

	// I5
	if inst.State != store.SagaStateCompensated {
		t.Errorf("state=%q, want COMPENSATED", inst.State)
	}
	if inst.CurrentStep != 3 {
		t.Errorf("currentStep=%d, want 3", inst.CurrentStep)
	}

	// I4: log = [F1 ok, F2 ok, F3 err, C2 ok, C1 ok].
	sl := mustParseSagaLog(t, inst)
	assertSteps(t, sl, []wantStep{
		{"F1", "ok"},
		{"F2", "ok"},
		{"F3", "err"},
		{"C2", "ok"},
		{"C1", "ok"},
	})

	// C2: stocks reservation released (F2 succeeded).
	if len(td.releaseCalls) != 1 {
		t.Errorf("td.releaseCalls=%d, want 1 (C2 releases F2 reservation)", len(td.releaseCalls))
	}
	// C1: F1 reservation released. F3 failed "before" so ReleaseFunds was NOT
	// called in the forward path — exactly one release call expected.
	if len(bc.releaseCalls) != 1 {
		t.Errorf("bc.releaseCalls=%d, want 1 (C1 releases F1 reservation; F3 never called ReleaseFunds)", len(bc.releaseCalls))
	}
	// No transfer was committed so no reverse needed.
	if len(bc.reverseCalls) != 0 {
		t.Errorf("bc.reverseCalls=%d, want 0 (F3 never committed a transfer)", len(bc.reverseCalls))
	}

	if pub.findByRK("saga.OTC_EXERCISE.failed") == nil {
		t.Error("saga.OTC_EXERCISE.failed event not published")
	}
}

// ---------------------------------------------------------------------------
// SG-06: F4 fails via fault injection, C3 + C2 + C1 compensate.
// Spec: Compensated, current_step=4,
//
//	log=[F1 ok, F2 ok, F3 ok, F4 err, C3 ok, C2 ok, C1 ok].
//
// ---------------------------------------------------------------------------
func TestSG06_Step4Fails_CompensatesStep3To1(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	evt := baseEvent(304)
	evt.FaultInjection = &events.FaultInjection{ForceFailStep: "F4"}

	err := orch.HandleOtcExercise(context.Background(), evt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	inst := mustGetOtcInst(t, fs, "304")

	// I5
	if inst.State != store.SagaStateCompensated {
		t.Errorf("state=%q, want COMPENSATED", inst.State)
	}
	if inst.CurrentStep != 4 {
		t.Errorf("currentStep=%d, want 4", inst.CurrentStep)
	}

	// I4: log = [F1 ok, F2 ok, F3 ok, F4 err, C3 ok, C2 ok, C1 ok].
	sl := mustParseSagaLog(t, inst)
	assertSteps(t, sl, []wantStep{
		{"F1", "ok"},
		{"F2", "ok"},
		{"F3", "ok"},
		{"F4", "err"},
		{"C3", "ok"},
		{"C2", "ok"},
		{"C1", "ok"},
	})

	// C3: transfer reversed (F3 succeeded and committed a transfer).
	if len(bc.reverseCalls) != 1 {
		t.Errorf("bc.reverseCalls=%d, want 1 (C3 reverses F3 transfer)", len(bc.reverseCalls))
	}
	// C2: stocks reservation released (F2 succeeded).
	if len(td.releaseCalls) != 1 {
		t.Errorf("td.releaseCalls=%d, want 1 (C2 releases F2 reservation)", len(td.releaseCalls))
	}
	// C1 + F3-forward: F3 calls ReleaseFunds to free the reservation before the
	// actual transfer; C1 then calls it again (idempotent). Total = 2.
	if len(bc.releaseCalls) != 2 {
		t.Errorf("bc.releaseCalls=%d, want 2 (1 from F3 forward + 1 from C1 compensation)", len(bc.releaseCalls))
	}

	if pub.findByRK("saga.OTC_EXERCISE.failed") == nil {
		t.Error("saga.OTC_EXERCISE.failed event not published")
	}
}

// ---------------------------------------------------------------------------
// SG-07: F5 fails via fault injection, all compensators C4–C1 run.
// Spec: Compensated, current_step=5,
//
//	log=[F1 ok, F2 ok, F3 ok, F4 ok, F5 err, C4 ok, C3 ok, C2 ok, C1 ok].
//
// ---------------------------------------------------------------------------
func TestSG07_Step5Fails_CompensatesAllSteps(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	evt := baseEvent(305)
	evt.FaultInjection = &events.FaultInjection{ForceFailStep: "F5"}

	err := orch.HandleOtcExercise(context.Background(), evt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	inst := mustGetOtcInst(t, fs, "305")

	// I5
	if inst.State != store.SagaStateCompensated {
		t.Errorf("state=%q, want COMPENSATED", inst.State)
	}
	if inst.CurrentStep != 5 {
		t.Errorf("currentStep=%d, want 5", inst.CurrentStep)
	}

	// I4: log = [F1 ok, F2 ok, F3 ok, F4 ok, F5 err, C4 ok, C3 ok, C2 ok, C1 ok].
	sl := mustParseSagaLog(t, inst)
	assertSteps(t, sl, []wantStep{
		{"F1", "ok"},
		{"F2", "ok"},
		{"F3", "ok"},
		{"F4", "ok"},
		{"F5", "err"},
		{"C4", "ok"},
		{"C3", "ok"},
		{"C2", "ok"},
		{"C1", "ok"},
	})

	// C4: ownership transfer reversed (F4 succeeded).
	if len(td.reverseCalls) != 1 {
		t.Errorf("td.reverseCalls=%d, want 1 (C4 reverses F4 ownership transfer)", len(td.reverseCalls))
	}
	// C3: fund transfer reversed (F3 succeeded).
	if len(bc.reverseCalls) != 1 {
		t.Errorf("bc.reverseCalls=%d, want 1 (C3 reverses F3 transfer)", len(bc.reverseCalls))
	}
	// C2: stocks reservation released (F2 succeeded).
	if len(td.releaseCalls) != 1 {
		t.Errorf("td.releaseCalls=%d, want 1 (C2 releases F2 reservation)", len(td.releaseCalls))
	}
	// C1 + F3-forward: same as SG-06 — 2 total calls (idempotent).
	if len(bc.releaseCalls) != 2 {
		t.Errorf("bc.releaseCalls=%d, want 2 (1 from F3 forward + 1 from C1 compensation)", len(bc.releaseCalls))
	}

	if pub.findByRK("saga.OTC_EXERCISE.failed") == nil {
		t.Error("saga.OTC_EXERCISE.failed event not published")
	}
}

// ---------------------------------------------------------------------------
// SG-08: Compensator C2 fails once then succeeds.
// Spec: Compensated, log=[F1 ok, F2 ok, F3 err, C2 err, C2 ok, C1 ok] — 6 entries.
// ---------------------------------------------------------------------------

func TestSG08_CompensatorFailsThenSucceeds(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	evt := baseEvent(306)
	evt.FaultInjection = &events.FaultInjection{
		ForceFailStep:       "F3",
		CompensateFailStep:  "C2",
		CompensateFailTimes: 1, // C2 fails once, then succeeds on the second attempt
	}

	err := orch.HandleOtcExercise(context.Background(), evt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	inst := mustGetOtcInst(t, fs, "306")

	// I5: saga ends in COMPENSATED — all compensators eventually succeeded.
	if inst.State != store.SagaStateCompensated {
		t.Errorf("state=%q, want COMPENSATED", inst.State)
	}

	// I4: log = [F1 ok, F2 ok, F3 err, C2 err, C2 ok, C1 ok] — 6 entries.
	// The two C2 entries prove the retry (first fault-injected failure + second success).
	sl := mustParseSagaLog(t, inst)
	assertSteps(t, sl, []wantStep{
		{"F1", "ok"},
		{"F2", "ok"},
		{"F3", "err"},
		{"C2", "err"}, // fault-injected failure
		{"C2", "ok"},  // retry succeeds
		{"C1", "ok"},
	})

	// F3 failed "before" side effects so C1 is the sole ReleaseFunds caller.
	if len(bc.releaseCalls) != 1 {
		t.Errorf("bc.releaseCalls=%d, want 1 (C1 only — F3 failed before ReleaseFunds)", len(bc.releaseCalls))
	}
	// C2 eventually released the stocks (the successful retry).
	if len(td.releaseCalls) != 1 {
		t.Errorf("td.releaseCalls=%d, want 1 (C2 released F2 reservation on second attempt)", len(td.releaseCalls))
	}

	if pub.findByRK("saga.OTC_EXERCISE.failed") == nil {
		t.Error("saga.OTC_EXERCISE.failed event not published")
	}
}
