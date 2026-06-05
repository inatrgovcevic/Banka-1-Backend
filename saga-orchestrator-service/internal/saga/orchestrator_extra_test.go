package saga_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/events"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/saga"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

// ---------------------------------------------------------------------------
// flakyStore: an in-memory store that fails UpdateOptimistic on a chosen call
// number. Used to exercise the emergency one-shot compensation paths
// (tryReleaseFundsOnce, tryReleaseStocksOnce, tryReverseTransferOnce,
// tryReverseOwnershipOnce) which only run when a saveFullLog fails mid-saga.
// ---------------------------------------------------------------------------

type flakyStore struct {
	mu          sync.Mutex
	rows        map[string]*store.SagaInstance
	updateCalls int
	failOnCall  int // 1-based call number on which UpdateOptimistic returns an error; 0 = never
}

func newFlakyStore(failOnCall int) *flakyStore {
	return &flakyStore{rows: make(map[string]*store.SagaInstance), failOnCall: failOnCall}
}

func (f *flakyStore) key(t, c string) string { return t + ":" + c }

func (f *flakyStore) FindByTypeAndCorrelation(_ context.Context, t, c string) (*store.SagaInstance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	inst := f.rows[f.key(t, c)]
	if inst == nil {
		return nil, nil
	}
	cp := *inst
	return &cp, nil
}

func (f *flakyStore) Insert(_ context.Context, inst *store.SagaInstance) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if inst.ID == uuid.Nil {
		inst.ID = uuid.New()
	}
	cp := *inst
	f.rows[f.key(inst.SagaType, inst.CorrelationID)] = &cp
	return nil
}

func (f *flakyStore) UpdateOptimistic(_ context.Context, inst *store.SagaInstance) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateCalls++
	if f.failOnCall != 0 && f.updateCalls == f.failOnCall {
		return errors.New("simulated DB write failure")
	}
	inst.Version++
	cp := *inst
	f.rows[f.key(inst.SagaType, inst.CorrelationID)] = &cp
	return nil
}

func newOrchFlaky(fs *flakyStore, bc *fakeBC, td *fakeTD, mk *fakeMK, pub *fakePublisher) *saga.Orchestrator {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return saga.NewOrchestratorForTest(fs, bc, td, mk, pub, log)
}

// otcEvt is a standard OTC exercise event for these tests.
func otcEvt(contractID int64) events.OtcExerciseRequested {
	return events.OtcExerciseRequested{
		ContractID:    contractID,
		BuyerID:       1,
		SellerID:      2,
		StockTicker:   "AAPL",
		Amount:        10,
		PricePerStock: decimal.RequireFromString("150.00"),
	}
}

// saveFullLog is called once per forward step (after advanceState).
// Update call sequence in HandleOtcExercise (happy path):
//   1 = advanceState (IN_PROGRESS)
//   2 = saveFullLog after F1   → fail here triggers tryReleaseFundsOnce
//   3 = saveFullLog after F2   → fail here triggers tryReleaseStocksOnce + tryReleaseFundsOnce
//   4 = saveFullLog after F3   → fail here triggers tryReverseTransferOnce + tryReleaseStocksOnce
//   5 = saveFullLog after F4   → fail here triggers tryReverseOwnershipOnce + tryReverseTransferOnce + tryReleaseStocksOnce

func TestOtc_SaveLogFailsAfterF1_EmergencyReleaseFunds(t *testing.T) {
	fs := newFlakyStore(2)
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrchFlaky(fs, bc, td, mk, pub)

	err := orch.HandleOtcExercise(context.Background(), otcEvt(501))
	if err == nil {
		t.Fatal("expected error from saveLog failure after F1")
	}
	if len(bc.releaseCalls) == 0 {
		t.Error("expected emergency ReleaseFunds after F1 saveLog failure")
	}
}

func TestOtc_SaveLogFailsAfterF2_EmergencyReleaseStocksAndFunds(t *testing.T) {
	fs := newFlakyStore(3)
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrchFlaky(fs, bc, td, mk, pub)

	err := orch.HandleOtcExercise(context.Background(), otcEvt(502))
	if err == nil {
		t.Fatal("expected error from saveLog failure after F2")
	}
	if len(td.releaseCalls) == 0 {
		t.Error("expected emergency ReleaseStocks after F2 saveLog failure")
	}
	if len(bc.releaseCalls) == 0 {
		t.Error("expected emergency ReleaseFunds after F2 saveLog failure")
	}
}

func TestOtc_SaveLogFailsAfterF3_EmergencyReverseTransfer(t *testing.T) {
	fs := newFlakyStore(4)
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrchFlaky(fs, bc, td, mk, pub)

	err := orch.HandleOtcExercise(context.Background(), otcEvt(503))
	if err == nil {
		t.Fatal("expected error from saveLog failure after F3")
	}
	if len(bc.reverseCalls) == 0 {
		t.Error("expected emergency ReverseTransfer after F3 saveLog failure")
	}
	if len(td.releaseCalls) == 0 {
		t.Error("expected emergency ReleaseStocks after F3 saveLog failure")
	}
}

func TestOtc_SaveLogFailsAfterF4_EmergencyReverseOwnership(t *testing.T) {
	fs := newFlakyStore(5)
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrchFlaky(fs, bc, td, mk, pub)

	err := orch.HandleOtcExercise(context.Background(), otcEvt(504))
	if err == nil {
		t.Fatal("expected error from saveLog failure after F4")
	}
	if len(td.reverseCalls) == 0 {
		t.Error("expected emergency ReverseOwnership after F4 saveLog failure")
	}
	if len(bc.reverseCalls) == 0 {
		t.Error("expected emergency ReverseTransfer after F4 saveLog failure")
	}
	if len(td.releaseCalls) == 0 {
		t.Error("expected emergency ReleaseStocks after F4 saveLog failure")
	}
}

// TestOtc_SaveLogFailsAfterF3_EmergencyCompensatorsAlsoFail exercises the
// error-logging branches of tryReverseTransferOnce and tryReleaseStocksOnce:
// the emergency one-shot compensation calls themselves fail. These run on the
// saveLog-after-F3 failure path, which returns immediately (no retry loop).
func TestOtc_SaveLogFailsAfterF3_EmergencyCompensatorsAlsoFail(t *testing.T) {
	fs := newFlakyStore(4)
	bc := newFakeBC()
	bc.reverseErr = errors.New("reverse transfer rejected")
	td := newFakeTD()
	td.reverseErr = errors.New("reverse ownership rejected")
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrchFlaky(fs, bc, td, mk, pub)

	if err := orch.HandleOtcExercise(context.Background(), otcEvt(513)); err == nil {
		t.Fatal("expected error from saveLog failure after F3")
	}
	// The emergency compensators were attempted despite returning errors.
	if len(bc.reverseCalls) == 0 {
		t.Error("expected ReverseTransfer attempt despite failure")
	}
	if len(td.releaseCalls) == 0 {
		t.Error("expected ReleaseStocks attempt despite failure")
	}
}

// TestOtc_SaveLogFailsAfterF1_EmergencyReleaseFundsFails exercises the
// error branch of tryReleaseFundsOnce.
func TestOtc_SaveLogFailsAfterF1_EmergencyReleaseFundsFails(t *testing.T) {
	fs := newFlakyStore(2)
	bc := newFakeBC()
	bc.releaseErr = errors.New("release funds rejected")
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrchFlaky(fs, bc, td, mk, pub)

	if err := orch.HandleOtcExercise(context.Background(), otcEvt(515)); err == nil {
		t.Fatal("expected error from saveLog failure after F1")
	}
	if len(bc.releaseCalls) == 0 {
		t.Error("expected ReleaseFunds attempt despite failure")
	}
}

// TestOtc_InjectDelay verifies the injectDelay fault path runs (a small delay
// on F2) and the saga still completes.
func TestOtc_InjectDelay(t *testing.T) {
	fs := newFakeStore()
	orch := newOrch(fs, newFakeBC(), newFakeTD(), newFakeMK(), &fakePublisher{})

	evt := otcEvt(520)
	evt.FaultInjection = &events.FaultInjection{
		InjectDelayStep: "F2",
		InjectDelayMs:   10,
	}
	if err := orch.HandleOtcExercise(context.Background(), evt); err != nil {
		t.Fatalf("expected saga to complete with injected delay, got %v", err)
	}
	got, _ := fs.FindByTypeAndCorrelation(context.Background(), "OTC_EXERCISE", "520")
	if got == nil || got.State != store.SagaStateCompleted {
		t.Errorf("expected COMPLETED, got %v", got)
	}
}

// TestOtc_ForceFailAfter_F1 covers injectForceFail with kind "after": the
// side effect is committed before compensation runs.
func TestOtc_ForceFailAfter_F1(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	orch := newOrch(fs, bc, newFakeTD(), newFakeMK(), &fakePublisher{})

	evt := otcEvt(521)
	evt.FaultInjection = &events.FaultInjection{
		ForceFailStep: "F1",
		ForceFailKind: "after",
	}
	// Forced failure at F1 (after) commits the reservation, then compensates and
	// surfaces the cause as an error from HandleOtcExercise.
	if err := orch.HandleOtcExercise(context.Background(), evt); err == nil {
		t.Fatal("expected error from forced F1-after failure")
	}
	got, _ := fs.FindByTypeAndCorrelation(context.Background(), "OTC_EXERCISE", "521")
	if got == nil || got.State == store.SagaStateCompleted {
		t.Errorf("expected non-COMPLETED terminal state after forced F1-after failure, got %v", got)
	}
	// The reservation side effect was committed (F1 after means the call ran)
	// and then released during compensation.
	if len(bc.releaseCalls) == 0 {
		t.Error("expected ReleaseFunds during compensation of F1-after failure")
	}
}

// TestOtc_SaveLogFailsAfterF4_EmergencyReverseOwnershipFails covers the error
// branch of tryReverseOwnershipOnce. The saveLog-after-F4 path returns
// immediately (no compensation retry loop), so this stays fast.
func TestOtc_SaveLogFailsAfterF4_EmergencyReverseOwnershipFails(t *testing.T) {
	fs := newFlakyStore(5)
	bc := newFakeBC()
	td := newFakeTD()
	td.reverseErr = errors.New("reverse ownership rejected")
	mk := newFakeMK()
	orch := newOrchFlaky(fs, bc, td, mk, &fakePublisher{})

	if err := orch.HandleOtcExercise(context.Background(), otcEvt(516)); err == nil {
		t.Fatal("expected error from saveLog failure after F4")
	}
	if len(td.reverseCalls) == 0 {
		t.Error("expected ReverseOwnership attempt despite failure")
	}
}

func TestOtc_AdvanceStateFails(t *testing.T) {
	fs := newFlakyStore(1) // fail on advanceState
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrchFlaky(fs, bc, td, mk, pub)

	if err := orch.HandleOtcExercise(context.Background(), otcEvt(505)); err == nil {
		t.Fatal("expected error from advanceState failure")
	}
}

// ---------------------------------------------------------------------------
// Dispatch — covers every saga type plus error branches.
// ---------------------------------------------------------------------------

func dispatchOrch() (*saga.Orchestrator, *fakeStore) {
	fs := newFakeStore()
	orch := newOrch(fs, newFakeBC(), newFakeTD(), newFakeMK(), &fakePublisher{})
	return orch, fs
}

func TestDispatch_NoPayload(t *testing.T) {
	orch, _ := dispatchOrch()
	inst := &store.SagaInstance{ID: uuid.New(), SagaType: "OTC_EXERCISE"}
	if err := orch.Dispatch(context.Background(), inst); err == nil {
		t.Fatal("expected error for nil payload")
	}
}

func TestDispatch_UnknownType(t *testing.T) {
	orch, _ := dispatchOrch()
	inst := &store.SagaInstance{ID: uuid.New(), SagaType: "WAT", Payload: []byte(`{}`)}
	if err := orch.Dispatch(context.Background(), inst); err == nil {
		t.Fatal("expected error for unknown saga type")
	}
}

func TestDispatch_OtcExercise(t *testing.T) {
	orch, fs := dispatchOrch()
	payload, _ := json.Marshal(otcEvt(601))
	inst := &store.SagaInstance{ID: uuid.New(), SagaType: "OTC_EXERCISE", Payload: payload}
	if err := orch.Dispatch(context.Background(), inst); err != nil {
		t.Fatalf("Dispatch OTC_EXERCISE: %v", err)
	}
	got, _ := fs.FindByTypeAndCorrelation(context.Background(), "OTC_EXERCISE", "601")
	if got == nil || got.State != store.SagaStateCompleted {
		t.Errorf("expected COMPLETED, got %v", got)
	}
}

func TestDispatch_OtcExercise_BadJSON(t *testing.T) {
	orch, _ := dispatchOrch()
	inst := &store.SagaInstance{ID: uuid.New(), SagaType: "OTC_EXERCISE", Payload: []byte(`{bad`)}
	if err := orch.Dispatch(context.Background(), inst); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestDispatch_OtcPremium(t *testing.T) {
	orch, _ := dispatchOrch()
	evt := events.OtcPremiumTransferRequested{ContractID: 602, BuyerID: 1, SellerID: 2, Premium: decimal.RequireFromString("10"), Currency: "RSD"}
	payload, _ := json.Marshal(evt)
	inst := &store.SagaInstance{ID: uuid.New(), SagaType: "OTC_PREMIUM_TRANSFER", Payload: payload}
	if err := orch.Dispatch(context.Background(), inst); err != nil {
		t.Fatalf("Dispatch OTC_PREMIUM_TRANSFER: %v", err)
	}
}

func TestDispatch_OtcPremium_BadJSON(t *testing.T) {
	orch, _ := dispatchOrch()
	inst := &store.SagaInstance{ID: uuid.New(), SagaType: "OTC_PREMIUM_TRANSFER", Payload: []byte(`x`)}
	if err := orch.Dispatch(context.Background(), inst); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestDispatch_FundSubscribe(t *testing.T) {
	orch, _ := dispatchOrch()
	evt := events.FundSubscribeRequested{
		TransactionID: "d-sub-1", Amount: decimal.RequireFromString("100"),
		FromAccountNumber: "111000100000000001", FundAccountNumber: "111000300000000003", FundID: 7,
	}
	payload, _ := json.Marshal(evt)
	inst := &store.SagaInstance{ID: uuid.New(), SagaType: "FUND_SUBSCRIBE", Payload: payload}
	if err := orch.Dispatch(context.Background(), inst); err != nil {
		t.Fatalf("Dispatch FUND_SUBSCRIBE: %v", err)
	}
}

func TestDispatch_FundSubscribe_BadJSON(t *testing.T) {
	orch, _ := dispatchOrch()
	inst := &store.SagaInstance{ID: uuid.New(), SagaType: "FUND_SUBSCRIBE", Payload: []byte(`x`)}
	if err := orch.Dispatch(context.Background(), inst); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestDispatch_FundRedeem(t *testing.T) {
	orch, _ := dispatchOrch()
	evt := events.FundRedeemRequested{
		TransactionID: "d-red-1", Amount: decimal.RequireFromString("100"),
		FundAccountNumber: "111000300000000003", ToAccountNumber: "111000100000000001", FundID: 7,
	}
	payload, _ := json.Marshal(evt)
	inst := &store.SagaInstance{ID: uuid.New(), SagaType: "FUND_REDEEM", Payload: payload}
	if err := orch.Dispatch(context.Background(), inst); err != nil {
		t.Fatalf("Dispatch FUND_REDEEM: %v", err)
	}
}

func TestDispatch_FundRedeem_BadJSON(t *testing.T) {
	orch, _ := dispatchOrch()
	inst := &store.SagaInstance{ID: uuid.New(), SagaType: "FUND_REDEEM", Payload: []byte(`x`)}
	if err := orch.Dispatch(context.Background(), inst); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestDispatch_FundLiquidation(t *testing.T) {
	orch, _ := dispatchOrch()
	evt := events.FundRedeemWithLiquidationRequested{
		TransactionID: "d-liq-1", Amount: decimal.RequireFromString("100"),
		FundID: 9, FundAccountNumber: "111000300000000003", ToAccountNumber: "111000100000000001",
	}
	payload, _ := json.Marshal(evt)
	inst := &store.SagaInstance{ID: uuid.New(), SagaType: "FUND_LIQUIDATION_FOR_REDEMPTION", Payload: payload}
	if err := orch.Dispatch(context.Background(), inst); err != nil {
		t.Fatalf("Dispatch FUND_LIQUIDATION_FOR_REDEMPTION: %v", err)
	}
}

func TestDispatch_FundLiquidation_BadJSON(t *testing.T) {
	orch, _ := dispatchOrch()
	inst := &store.SagaInstance{ID: uuid.New(), SagaType: "FUND_LIQUIDATION_FOR_REDEMPTION", Payload: []byte(`x`)}
	if err := orch.Dispatch(context.Background(), inst); err == nil {
		t.Fatal("expected unmarshal error")
	}
}
