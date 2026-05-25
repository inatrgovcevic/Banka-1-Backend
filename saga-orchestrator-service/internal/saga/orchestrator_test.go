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
// In-memory fakes
// ---------------------------------------------------------------------------

// fakeStore is an in-memory implementation of store.SagaInstanceStore behaviour.
type fakeStore struct {
	mu    sync.Mutex
	rows  map[string]*store.SagaInstance // key = sagaType+":"+correlationId
	calls []string
}

func newFakeStore() *fakeStore {
	return &fakeStore{rows: make(map[string]*store.SagaInstance)}
}

func (f *fakeStore) key(sagaType, corrID string) string { return sagaType + ":" + corrID }

func (f *fakeStore) FindByTypeAndCorrelation(_ context.Context, sagaType, correlationID string) (*store.SagaInstance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "FindByTypeAndCorrelation")
	inst := f.rows[f.key(sagaType, correlationID)]
	if inst == nil {
		return nil, nil
	}
	cp := *inst
	return &cp, nil
}

func (f *fakeStore) Insert(_ context.Context, inst *store.SagaInstance) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "Insert")
	if inst.ID == uuid.Nil {
		inst.ID = uuid.New()
	}
	k := f.key(inst.SagaType, inst.CorrelationID)
	if _, exists := f.rows[k]; exists {
		return store.ErrOptimisticLockConflict
	}
	cp := *inst
	f.rows[k] = &cp
	return nil
}

func (f *fakeStore) UpdateOptimistic(_ context.Context, inst *store.SagaInstance) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "UpdateOptimistic:"+inst.State)
	k := f.key(inst.SagaType, inst.CorrelationID)
	existing := f.rows[k]
	if existing == nil {
		return store.ErrNotFound
	}
	if existing.Version != inst.Version {
		return store.ErrOptimisticLockConflict
	}
	inst.Version++
	cp := *inst
	f.rows[k] = &cp
	return nil
}

// fakeBC is an in-memory banking-core fake.
type fakeBC struct {
	mu            sync.Mutex
	reserveCalls  []string // reservationIDs issued
	releaseCalls  []string
	transferCalls []string
	reverseCalls  []string

	// control injection
	reserveErr  error
	transferErr error
	reverseErr  error
	releaseErr  error
	accountByID map[int64]string
}

func newFakeBC() *fakeBC {
	return &fakeBC{
		accountByID: map[int64]string{
			1: "111000100000000001",
			2: "111000200000000002",
		},
	}
}

func (f *fakeBC) ReserveFunds(_ context.Context, ownerID int64, amount decimal.Decimal, corrID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reserveErr != nil {
		return "", f.reserveErr
	}
	id := "res-" + corrID
	f.reserveCalls = append(f.reserveCalls, id)
	return id, nil
}

func (f *fakeBC) ReleaseFunds(_ context.Context, reservationID, corrID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releaseCalls = append(f.releaseCalls, reservationID)
	return f.releaseErr
}

func (f *fakeBC) CommitReservation(_ context.Context, reservationID, corrID string) error { return nil }

func (f *fakeBC) InternalTransfer(_ context.Context, from, to string, amount decimal.Decimal, corrID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.transferErr != nil {
		return "", f.transferErr
	}
	id := "tr-" + corrID
	f.transferCalls = append(f.transferCalls, id)
	return id, nil
}

func (f *fakeBC) ReverseTransfer(_ context.Context, transferID, corrID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reverseCalls = append(f.reverseCalls, transferID)
	return f.reverseErr
}

func (f *fakeBC) ResolveDefaultAccountNumber(_ context.Context, ownerID int64) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if acct, ok := f.accountByID[ownerID]; ok {
		return acct, nil
	}
	return "", errors.New("account not found")
}

// fakeTD is an in-memory trading-service fake.
type fakeTD struct {
	mu                sync.Mutex
	reserveCalls      []string
	releaseCalls      []string
	transferCalls     []string
	reverseCalls      []string
	liquidateCalls    []int64
	reserveErr        error
	transferErr       error
	reverseErr        error
	liquidateErr      error
	liquidationResult string
}

func newFakeTD() *fakeTD {
	return &fakeTD{liquidationResult: "liq-default"}
}

func (f *fakeTD) ReserveStocks(_ context.Context, ownerID int64, ticker string, amount int, corrID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reserveErr != nil {
		return "", f.reserveErr
	}
	id := "sr-" + corrID
	f.reserveCalls = append(f.reserveCalls, id)
	return id, nil
}

func (f *fakeTD) ReleaseStocks(_ context.Context, reservationID, corrID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releaseCalls = append(f.releaseCalls, reservationID)
	return nil
}

func (f *fakeTD) TransferOwnership(_ context.Context, reservationID string, buyerID int64, corrID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.transferErr != nil {
		return "", f.transferErr
	}
	id := "ot-" + corrID
	f.transferCalls = append(f.transferCalls, id)
	return id, nil
}

func (f *fakeTD) ReverseOwnership(_ context.Context, ownershipTransferID, corrID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reverseCalls = append(f.reverseCalls, ownershipTransferID)
	return f.reverseErr
}

func (f *fakeTD) LiquidateForFund(_ context.Context, fundID int64, targetAmount decimal.Decimal, corrID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.liquidateErr != nil {
		return "", f.liquidateErr
	}
	f.liquidateCalls = append(f.liquidateCalls, fundID)
	return f.liquidationResult, nil
}

// fakeMK is an in-memory market-service fake.
type fakeMK struct {
	rate decimal.Decimal
	err  error
}

func newFakeMK() *fakeMK {
	return &fakeMK{rate: decimal.RequireFromString("117.50")}
}

func (f *fakeMK) ConvertCurrencyNoCommission(_ context.Context, from, to string, amount decimal.Decimal) (decimal.Decimal, error) {
	if f.err != nil {
		return decimal.Zero, f.err
	}
	return amount.Mul(f.rate), nil
}

// fakePublisher records published events.
type fakePublisher struct {
	mu     sync.Mutex
	events []publishedEvent
}

type publishedEvent struct {
	RoutingKey string
	Body       []byte
}

func (f *fakePublisher) Publish(_ context.Context, routingKey string, body []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, publishedEvent{RoutingKey: routingKey, Body: body})
	return nil
}

func (f *fakePublisher) findByRK(rk string) *publishedEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.events {
		if f.events[i].RoutingKey == rk {
			return &f.events[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newOrch creates an Orchestrator with all in-memory fakes via the
// test-exported NewOrchestratorForTest constructor.
func newOrch(fs *fakeStore, bc *fakeBC, td *fakeTD, mk *fakeMK, pub *fakePublisher) *saga.Orchestrator {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return saga.NewOrchestratorForTest(fs, bc, td, mk, pub, log)
}

// ---------------------------------------------------------------------------
// OtcExercise tests
// ---------------------------------------------------------------------------

func TestHandleOtcExercise_HappyPath(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}

	orch := newOrch(fs, bc, td, mk, pub)
	evt := events.OtcExerciseRequested{
		ContractID:      101,
		BuyerID:         1,
		SellerID:        2,
		StockTicker:     "AAPL",
		Amount:          10,
		PricePerStock:   decimal.RequireFromString("150.00"),
		Premium:         decimal.RequireFromString("5.00"),
		PremiumCurrency: "USD",
	}

	if err := orch.HandleOtcExercise(context.Background(), evt); err != nil {
		t.Fatalf("HandleOtcExercise error: %v", err)
	}

	// Check COMPLETED state persisted.
	inst, _ := fs.FindByTypeAndCorrelation(context.Background(), "OTC_EXERCISE", "101")
	if inst == nil {
		t.Fatal("saga instance not found")
	}
	if inst.State != store.SagaStateCompleted {
		t.Errorf("state=%q, want COMPLETED", inst.State)
	}
	if inst.CurrentStep != 5 {
		t.Errorf("currentStep=%d, want 5", inst.CurrentStep)
	}

	// Check compensation log has all 4 keys.
	var compLog map[string]string
	if err := json.Unmarshal(inst.CompensationLog, &compLog); err != nil {
		t.Fatalf("unmarshal comp log: %v", err)
	}
	for _, key := range []string{"step1_reservationId", "step2_stocksReservationId", "step3_transferId", "step4_ownershipTransferId"} {
		if _, ok := compLog[key]; !ok {
			t.Errorf("compensation log missing key %q", key)
		}
	}

	// Check completed event published.
	if pub.findByRK("saga.OTC_EXERCISE.completed") == nil {
		t.Error("saga.OTC_EXERCISE.completed event not published")
	}
}

func TestHandleOtcExercise_Step2Fails_CompensatesStep1(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	td.reserveErr = errors.New("not enough stocks")
	mk := newFakeMK()
	pub := &fakePublisher{}

	orch := newOrch(fs, bc, td, mk, pub)
	evt := events.OtcExerciseRequested{
		ContractID: 102, BuyerID: 1, SellerID: 2,
		StockTicker: "GOOG", Amount: 5,
		PricePerStock: decimal.RequireFromString("100"),
	}

	err := orch.HandleOtcExercise(context.Background(), evt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// State should be FAILED.
	inst, _ := fs.FindByTypeAndCorrelation(context.Background(), "OTC_EXERCISE", "102")
	if inst == nil {
		t.Fatal("instance not found")
	}
	if inst.State != store.SagaStateFailed {
		t.Errorf("state=%q, want FAILED", inst.State)
	}

	// Step 1 reservation should have been released (compensation).
	if len(bc.releaseCalls) == 0 {
		t.Error("expected ReleaseFunds to be called for step 1 compensation")
	}

	// Failed event should be published.
	if pub.findByRK("saga.OTC_EXERCISE.failed") == nil {
		t.Error("saga.OTC_EXERCISE.failed event not published")
	}
}

func TestHandleOtcExercise_Step4Fails_CompensatesStep1To3(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	td.transferErr = errors.New("ownership transfer failed")
	mk := newFakeMK()
	pub := &fakePublisher{}

	orch := newOrch(fs, bc, td, mk, pub)
	evt := events.OtcExerciseRequested{
		ContractID: 103, BuyerID: 1, SellerID: 2,
		StockTicker: "MSFT", Amount: 2,
		PricePerStock: decimal.RequireFromString("200"),
	}

	err := orch.HandleOtcExercise(context.Background(), evt)
	if err == nil {
		t.Fatal("expected error")
	}

	// Transfer S3 should have been reversed.
	if len(bc.reverseCalls) == 0 {
		t.Error("expected ReverseTransfer compensation")
	}
	// Stocks S2 should have been released.
	if len(td.releaseCalls) == 0 {
		t.Error("expected ReleaseStocks compensation")
	}
}

func TestHandleOtcExercise_IdempotentReplay(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}

	orch := newOrch(fs, bc, td, mk, pub)
	evt := events.OtcExerciseRequested{
		ContractID: 104, BuyerID: 1, SellerID: 2,
		StockTicker: "NVDA", Amount: 1,
		PricePerStock: decimal.RequireFromString("500"),
	}

	// First call — should succeed.
	if err := orch.HandleOtcExercise(context.Background(), evt); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	pubCountAfterFirst := len(pub.events)

	// Second call — same correlationID → should be skipped (idempotent).
	if err := orch.HandleOtcExercise(context.Background(), evt); err != nil {
		t.Fatalf("second call error: %v", err)
	}

	// No additional events should be published.
	if len(pub.events) != pubCountAfterFirst {
		t.Errorf("idempotent replay published additional events: %d after first, %d after second",
			pubCountAfterFirst, len(pub.events))
	}
}

// ---------------------------------------------------------------------------
// OtcPremiumTransfer tests
// ---------------------------------------------------------------------------

func TestHandleOtcPremiumTransfer_HappyPath(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}

	orch := newOrch(fs, bc, td, mk, pub)
	evt := events.OtcPremiumTransferRequested{
		ContractID: 201, BuyerID: 1, SellerID: 2,
		Premium:  decimal.RequireFromString("50"),
		Currency: "USD",
	}

	if err := orch.HandleOtcPremiumTransfer(context.Background(), evt); err != nil {
		t.Fatalf("HandleOtcPremiumTransfer error: %v", err)
	}

	inst, _ := fs.FindByTypeAndCorrelation(context.Background(), "OTC_PREMIUM_TRANSFER", "201")
	if inst == nil || inst.State != store.SagaStateCompleted {
		t.Errorf("expected COMPLETED, got %v", inst)
	}
	if pub.findByRK("saga.OTC_PREMIUM_TRANSFER.completed") == nil {
		t.Error("completed event not published")
	}
}

func TestHandleOtcPremiumTransfer_TransferFails_SagaFailed(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	bc.transferErr = errors.New("account frozen")
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}

	orch := newOrch(fs, bc, td, mk, pub)
	evt := events.OtcPremiumTransferRequested{
		ContractID: 202, BuyerID: 1, SellerID: 2,
		Premium: decimal.RequireFromString("10"), Currency: "RSD",
	}

	err := orch.HandleOtcPremiumTransfer(context.Background(), evt)
	if err == nil {
		t.Fatal("expected error")
	}
	inst, _ := fs.FindByTypeAndCorrelation(context.Background(), "OTC_PREMIUM_TRANSFER", "202")
	if inst == nil || inst.State != store.SagaStateFailed {
		t.Errorf("expected FAILED, got %v", inst)
	}
}

func TestHandleOtcPremiumTransfer_Idempotent(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	evt := events.OtcPremiumTransferRequested{
		ContractID: 203, BuyerID: 1, SellerID: 2,
		Premium: decimal.RequireFromString("20"), Currency: "RSD",
	}

	if err := orch.HandleOtcPremiumTransfer(context.Background(), evt); err != nil {
		t.Fatalf("first call: %v", err)
	}
	first := len(pub.events)
	if err := orch.HandleOtcPremiumTransfer(context.Background(), evt); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(pub.events) != first {
		t.Error("idempotent replay produced extra events")
	}
}

// ---------------------------------------------------------------------------
// FundSubscribe tests
// ---------------------------------------------------------------------------

func TestHandleFundSubscribe_HappyPath(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	evt := events.FundSubscribeRequested{
		TransactionID:     "txn-fund-001",
		Amount:            decimal.RequireFromString("1000"),
		FromAccountNumber: "111000100000000001",
		FundAccountNumber: "111000300000000003",
		FundID:            7,
	}

	if err := orch.HandleFundSubscribe(context.Background(), evt); err != nil {
		t.Fatalf("HandleFundSubscribe error: %v", err)
	}

	inst, _ := fs.FindByTypeAndCorrelation(context.Background(), "FUND_SUBSCRIBE", "txn-fund-001")
	if inst == nil || inst.State != store.SagaStateCompleted {
		t.Errorf("expected COMPLETED, got %v", inst)
	}
	if pub.findByRK("saga.FUND_SUBSCRIBE.STEP_1.fund.success") == nil {
		t.Error("fund.success event not published")
	}
}

func TestHandleFundSubscribe_TransferFails(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	bc.transferErr = errors.New("insufficient balance")
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	evt := events.FundSubscribeRequested{
		TransactionID:     "txn-fund-002",
		Amount:            decimal.RequireFromString("999999"),
		FromAccountNumber: "111000100000000001",
		FundAccountNumber: "111000300000000003",
		FundID:            7,
	}

	err := orch.HandleFundSubscribe(context.Background(), evt)
	if err == nil {
		t.Fatal("expected error")
	}

	inst, _ := fs.FindByTypeAndCorrelation(context.Background(), "FUND_SUBSCRIBE", "txn-fund-002")
	if inst == nil || inst.State != store.SagaStateFailed {
		t.Errorf("expected FAILED, got %v", inst)
	}
	if pub.findByRK("saga.FUND_SUBSCRIBE.STEP_1.fund.failure") == nil {
		t.Error("fund.failure event not published")
	}
}

func TestHandleFundSubscribe_Idempotent(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	evt := events.FundSubscribeRequested{
		TransactionID:     "txn-fund-003",
		Amount:            decimal.RequireFromString("500"),
		FromAccountNumber: "111000100000000001",
		FundAccountNumber: "111000300000000003",
		FundID:            8,
	}
	if err := orch.HandleFundSubscribe(context.Background(), evt); err != nil {
		t.Fatal(err)
	}
	first := len(pub.events)
	if err := orch.HandleFundSubscribe(context.Background(), evt); err != nil {
		t.Fatal(err)
	}
	if len(pub.events) != first {
		t.Error("idempotent replay produced extra events")
	}
}

// ---------------------------------------------------------------------------
// FundRedeem tests
// ---------------------------------------------------------------------------

func TestHandleFundRedeem_HappyPath(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	evt := events.FundRedeemRequested{
		TransactionID:     "txn-redeem-001",
		Amount:            decimal.RequireFromString("2000"),
		FromAccountNumber: "111000300000000003",
		ToAccountNumber:   "111000100000000001",
		FundID:            7,
	}
	if err := orch.HandleFundRedeem(context.Background(), evt); err != nil {
		t.Fatalf("HandleFundRedeem error: %v", err)
	}
	inst, _ := fs.FindByTypeAndCorrelation(context.Background(), "FUND_REDEEM", "txn-redeem-001")
	if inst == nil || inst.State != store.SagaStateCompleted {
		t.Errorf("expected COMPLETED, got %v", inst)
	}
	if pub.findByRK("saga.FUND_REDEEM.STEP_1.fund.success") == nil {
		t.Error("fund.success event not published")
	}
}

func TestHandleFundRedeem_TransferFails(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	bc.transferErr = errors.New("fund frozen")
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	evt := events.FundRedeemRequested{
		TransactionID:     "txn-redeem-002",
		Amount:            decimal.RequireFromString("100"),
		FromAccountNumber: "111000300000000003",
		ToAccountNumber:   "111000100000000001",
		FundID:            7,
	}
	err := orch.HandleFundRedeem(context.Background(), evt)
	if err == nil {
		t.Fatal("expected error")
	}
	if pub.findByRK("saga.FUND_REDEEM.STEP_1.fund.failure") == nil {
		t.Error("fund.failure event not published")
	}
}

// ---------------------------------------------------------------------------
// FundRedeemWithLiquidation tests
// ---------------------------------------------------------------------------

func TestHandleFundRedeemWithLiquidation_HappyPath(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	evt := events.FundRedeemWithLiquidationRequested{
		TransactionID:     "txn-liq-001",
		Amount:            decimal.RequireFromString("5000"),
		FundID:            9,
		FundAccountNumber: "111000300000000003",
		ToAccountNumber:   "111000100000000001",
	}
	if err := orch.HandleFundRedeemWithLiquidation(context.Background(), evt); err != nil {
		t.Fatalf("HandleFundRedeemWithLiquidation error: %v", err)
	}
	inst, _ := fs.FindByTypeAndCorrelation(context.Background(), "FUND_LIQUIDATION_FOR_REDEMPTION", "txn-liq-001")
	if inst == nil || inst.State != store.SagaStateCompleted {
		t.Errorf("expected COMPLETED, got %v", inst)
	}

	var log map[string]string
	json.Unmarshal(inst.CompensationLog, &log)
	if _, ok := log["step1_liquidationId"]; !ok {
		t.Error("step1_liquidationId missing from comp log")
	}
	if _, ok := log["step2_transferId"]; !ok {
		t.Error("step2_transferId missing from comp log")
	}

	if pub.findByRK("saga.FUND_LIQUIDATION_FOR_REDEMPTION.STEP_2.fund.success") == nil {
		t.Error("success event not published")
	}
}

func TestHandleFundRedeemWithLiquidation_Step1Fails(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	td.liquidateErr = errors.New("no holdings")
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	evt := events.FundRedeemWithLiquidationRequested{
		TransactionID: "txn-liq-002", Amount: decimal.RequireFromString("100"),
		FundID: 9, FundAccountNumber: "111000300000000003", ToAccountNumber: "111000100000000001",
	}
	err := orch.HandleFundRedeemWithLiquidation(context.Background(), evt)
	if err == nil {
		t.Fatal("expected error")
	}
	inst, _ := fs.FindByTypeAndCorrelation(context.Background(), "FUND_LIQUIDATION_FOR_REDEMPTION", "txn-liq-002")
	if inst == nil || inst.State != store.SagaStateFailed {
		t.Errorf("expected FAILED, got %v", inst)
	}
	if pub.findByRK("saga.FUND_LIQUIDATION_FOR_REDEMPTION.STEP_X.fund.failure") == nil {
		t.Error("failure event not published")
	}
}

func TestHandleFundRedeemWithLiquidation_Step2Fails_AlertRequired(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	bc.transferErr = errors.New("banking-core down")
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	evt := events.FundRedeemWithLiquidationRequested{
		TransactionID: "txn-liq-003", Amount: decimal.RequireFromString("200"),
		FundID: 10, FundAccountNumber: "111000300000000003", ToAccountNumber: "111000100000000001",
	}
	err := orch.HandleFundRedeemWithLiquidation(context.Background(), evt)
	if err == nil {
		t.Fatal("expected error")
	}
	inst, _ := fs.FindByTypeAndCorrelation(context.Background(), "FUND_LIQUIDATION_FOR_REDEMPTION", "txn-liq-003")
	if inst == nil || inst.State != store.SagaStateFailed {
		t.Errorf("expected FAILED, got %v", inst)
	}

	var log map[string]string
	json.Unmarshal(inst.CompensationLog, &log)
	if _, ok := log["alertRequired"]; !ok {
		t.Error("alertRequired key missing after step 2 failure")
	}
	if _, ok := log["step1_liquidationId"]; !ok {
		t.Error("step1_liquidationId should be present even though step 2 failed")
	}
}

func TestHandleFundRedeemWithLiquidation_Idempotent(t *testing.T) {
	fs := newFakeStore()
	bc := newFakeBC()
	td := newFakeTD()
	mk := newFakeMK()
	pub := &fakePublisher{}
	orch := newOrch(fs, bc, td, mk, pub)

	evt := events.FundRedeemWithLiquidationRequested{
		TransactionID: "txn-liq-004", Amount: decimal.RequireFromString("300"),
		FundID: 11, FundAccountNumber: "111000300000000003", ToAccountNumber: "111000100000000001",
	}
	if err := orch.HandleFundRedeemWithLiquidation(context.Background(), evt); err != nil {
		t.Fatal(err)
	}
	first := len(pub.events)
	if err := orch.HandleFundRedeemWithLiquidation(context.Background(), evt); err != nil {
		t.Fatal(err)
	}
	if len(pub.events) != first {
		t.Error("idempotent replay produced extra events")
	}
}
