package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"
)

// ---------------------------------------------------------------------------
// In-memory fakes
// ---------------------------------------------------------------------------

// fakeExecStore is an in-memory ExecutorStore.
type fakeExecStore struct {
	txns    map[string]*store.Transaction // key: "routing:id"
	failOn  string                        // if non-empty, PersistPrepared returns error
	nextID  int64
}

func (f *fakeExecStore) key(routing int, id string) string {
	return protocol.ForeignBankId{RoutingNumber: routing, Id: id}.Id + ":" + string(rune(routing+'0'))
}

func (f *fakeExecStore) PersistPrepared(ctx context.Context, t *store.Transaction) error {
	if f.failOn != "" {
		return errors.New(f.failOn)
	}
	if f.txns == nil {
		f.txns = make(map[string]*store.Transaction)
	}
	f.nextID++
	t.ID = f.nextID
	t.Status = store.TxStatusPrepared
	k := txKey(t.TransactionIdRouting, t.TransactionIdLocal)
	cp := *t
	f.txns[k] = &cp
	return nil
}

func (f *fakeExecStore) FindTx(ctx context.Context, routing int, id string) (*store.Transaction, error) {
	if f.txns == nil {
		return nil, nil
	}
	t, ok := f.txns[txKey(routing, id)]
	if !ok {
		return nil, nil
	}
	cp := *t
	return &cp, nil
}

func (f *fakeExecStore) UpdateTxStatus(ctx context.Context, routing int, id, status string) error {
	if f.txns == nil {
		return nil
	}
	t, ok := f.txns[txKey(routing, id)]
	if !ok {
		return nil
	}
	t.Status = status
	return nil
}

func txKey(routing int, id string) string {
	return string(rune(routing)) + ":" + id
}

// fakeBankingCoreReserver implements BankingCoreReserver.
type fakeBankingCoreReserver struct {
	// BankingCoreReader part (same as validator_test.go fakeBC)
	byNum           map[string]*AccountInfo
	byOwnerCurrency map[string]string // "ownerID:currency" → account num

	// reservation tracking
	reserved  []string // reservation IDs created
	committed []string
	released  []string

	failReserve bool
	reserveErr  error
	// failAfter: if > 0, the Nth call to ReserveMonas (1-indexed) will fail.
	failAfter int
}

func newFakeBC(accounts map[string]*AccountInfo) *fakeBankingCoreReserver {
	return &fakeBankingCoreReserver{byNum: accounts}
}

func (f *fakeBankingCoreReserver) ResolveAccount(_ context.Context, num string) (*AccountInfo, error) {
	a, ok := f.byNum[num]
	if !ok {
		return nil, ErrAccountNotFound
	}
	return a, nil
}

func (f *fakeBankingCoreReserver) FindAccountByOwnerAndCurrency(_ context.Context, ownerID int64, currency string) (string, error) {
	if f.byOwnerCurrency == nil {
		return "", ErrAccountNotFound
	}
	key := formatOwnerKey(ownerID, currency)
	num, ok := f.byOwnerCurrency[key]
	if !ok {
		return "", ErrAccountNotFound
	}
	return num, nil
}

func formatOwnerKey(ownerID int64, currency string) string {
	// simple key format matching validator_test.go
	return string(rune(ownerID)) + ":" + currency
}

// failAfter: if > 0, the Nth ReserveMonas call (1-indexed) will fail.
// When 0, behaviour is determined by failReserve.
func (f *fakeBankingCoreReserver) ReserveMonas(_ context.Context, accountNum, currency string, amount decimal.Decimal, txIDRouting int, txIDLocal string) (string, error) {
	callNum := len(f.reserved) + 1 // 1-indexed call number before appending
	if f.failAfter > 0 && callNum >= f.failAfter {
		return "", errors.New("fakeBankingCoreReserver: ReserveMonas forced failure on call " + string(rune('0'+callNum)))
	}
	if f.failReserve {
		if f.reserveErr != nil {
			return "", f.reserveErr
		}
		return "", errors.New("fakeBankingCoreReserver: ReserveMonas forced failure")
	}
	id := "monas-res-" + accountNum
	f.reserved = append(f.reserved, id)
	return id, nil
}

func (f *fakeBankingCoreReserver) CommitMonas(_ context.Context, reservationID string) error {
	f.committed = append(f.committed, reservationID)
	return nil
}

func (f *fakeBankingCoreReserver) ReleaseMonas(_ context.Context, reservationID string) error {
	f.released = append(f.released, reservationID)
	return nil
}

// fakeTradingReserver implements TradingReserver.
type fakeTradingReserver struct {
	reserved  []string
	committed []string
	released  []string

	failReserveStock bool
	optionSellers    map[string]string // negotiation id → sellerForeignID (for lookup validation)
}

func (f *fakeTradingReserver) ReserveStock(_ context.Context, sellerUserID int64, ticker string, quantity int, txIDRouting int, txIDLocal string) (string, error) {
	if f.failReserveStock {
		return "", errors.New("fakeTradingReserver: ReserveStock forced failure")
	}
	id := "stock-res-" + ticker
	f.reserved = append(f.reserved, id)
	return id, nil
}

func (f *fakeTradingReserver) CommitStock(_ context.Context, reservationID string) error {
	f.committed = append(f.committed, reservationID)
	return nil
}

func (f *fakeTradingReserver) ReleaseStock(_ context.Context, reservationID string) error {
	f.released = append(f.released, reservationID)
	return nil
}

func (f *fakeTradingReserver) ReserveOption(_ context.Context, negotiationID protocol.ForeignBankId, sellerForeignID, ticker string, quantity int) error {
	id := "option-res-" + negotiationID.Id
	f.reserved = append(f.reserved, id)
	return nil
}

func (f *fakeTradingReserver) ExerciseOption(_ context.Context, negotiationID protocol.ForeignBankId) error {
	f.committed = append(f.committed, "option-exercise-"+negotiationID.Id)
	return nil
}

func (f *fakeTradingReserver) ReleaseOption(_ context.Context, negotiationID protocol.ForeignBankId) error {
	f.released = append(f.released, "option-release-"+negotiationID.Id)
	return nil
}

// fakeNegotiationLookup implements NegotiationReader for the Executor's validator needs,
// and also serves as OptionNegotiationLookup for reservePosting.
type fakeNegotiationLookup struct {
	negs map[string]*fakeNegForExec // key: negotiation local id
}

type fakeNegForExec struct {
	SellerID   string
	IsOngoing  bool
	Amount     int
	Settlement string // RFC3339
}

func (f *fakeNegotiationLookup) FindNegotiation(ctx context.Context, id protocol.ForeignBankId) (*NegotiationLite, error) {
	n, ok := f.negs[id.Id]
	if !ok {
		return nil, errors.New("not found")
	}
	return &NegotiationLite{IsOngoing: n.IsOngoing, Amount: n.Amount}, nil
}

func (f *fakeNegotiationLookup) FindSellerID(ctx context.Context, negID string) (string, error) {
	n, ok := f.negs[negID]
	if !ok {
		return "", errors.New("negotiation not found: " + negID)
	}
	return n.SellerID, nil
}

// discardLogger returns a slog.Logger that discards all output.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestExecutor(myRouting int, s *fakeExecStore, bc *fakeBankingCoreReserver, td *fakeTradingReserver) *Executor {
	negs := &fakeNegotiationLookup{negs: map[string]*fakeNegForExec{}}
	return newExecutorWithNegs(myRouting, s, bc, td, negs)
}

func newExecutorWithNegs(myRouting int, s *fakeExecStore, bc *fakeBankingCoreReserver, td *fakeTradingReserver, negs *fakeNegotiationLookup) *Executor {
	return NewExecutor(myRouting, s, bc, td, negs, discardLogger())
}

// balancedMonasPosting creates a simple balanced MONAS transfer.
// Returns (debit from "from", credit to "to") postings.
func balancedMonasPosting(from, to string, amount int64, currency string) []protocol.Posting {
	return []protocol.Posting{
		{Account: &protocol.RealAccount{Num: from}, Amount: decimal.NewFromInt(-amount), Asset: &protocol.MonasAsset{Currency: currency}},
		{Account: &protocol.RealAccount{Num: to}, Amount: decimal.NewFromInt(amount), Asset: &protocol.MonasAsset{Currency: currency}},
	}
}

// ---------------------------------------------------------------------------
// PrepareLocal tests
// ---------------------------------------------------------------------------

// Test 1: happy path — balanced MONAS transfer, our account has sufficient funds → YES vote.
func TestPrepareLocal_HappyPath_YesVote(t *testing.T) {
	bc := newFakeBC(map[string]*AccountInfo{
		"111000000000000001": {Currency: "USD", AvailableBalance: decimal.NewFromInt(1000)},
	})
	s := &fakeExecStore{}
	e := newTestExecutor(111, s, bc, nil)

	tx := protocol.InterbankTransactionPayload{
		TransactionId: protocol.ForeignBankId{RoutingNumber: 222, Id: "tx-happy"},
		Postings:      balancedMonasPosting("222000000000000099", "111000000000000001", 100, "USD"),
	}
	vote, err := e.PrepareLocal(context.Background(), tx)
	if err != nil {
		t.Fatalf("PrepareLocal: %v", err)
	}
	if vote.Vote != protocol.VoteYes {
		t.Errorf("expected YES, got %q reasons=%v", vote.Vote, vote.Reasons)
	}
	// No outgoing (debit from us) postings — our account receives, so no reservation.
	// But the tx should be persisted.
	if len(s.txns) != 1 {
		t.Errorf("expected 1 persisted tx, got %d", len(s.txns))
	}
}

// Test 2: unbalanced postings → NO + UNBALANCED_TX, nothing persisted.
func TestPrepareLocal_Unbalanced_NoVote(t *testing.T) {
	s := &fakeExecStore{}
	e := newTestExecutor(111, s, nil, nil)

	tx := protocol.InterbankTransactionPayload{
		TransactionId: protocol.ForeignBankId{RoutingNumber: 222, Id: "tx-unbal"},
		Postings: []protocol.Posting{
			{Account: &protocol.RealAccount{Num: "222000000000000099"}, Amount: decimal.NewFromInt(-100), Asset: &protocol.MonasAsset{Currency: "USD"}},
			{Account: &protocol.RealAccount{Num: "111000000000000001"}, Amount: decimal.NewFromInt(99), Asset: &protocol.MonasAsset{Currency: "USD"}},
		},
	}
	vote, err := e.PrepareLocal(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vote.Vote != protocol.VoteNo {
		t.Errorf("expected NO, got %q", vote.Vote)
	}
	if len(vote.Reasons) == 0 {
		t.Error("expected at least one reason")
	}
	if vote.Reasons[0].Reason != protocol.ReasonUnbalancedTx {
		t.Errorf("expected UNBALANCED_TX reason, got %q", vote.Reasons[0].Reason)
	}
	if len(s.txns) != 0 {
		t.Errorf("nothing should be persisted on NO vote, got %d txns", len(s.txns))
	}
}

// Test 3: our account has insufficient funds → NO + INSUFFICIENT_ASSET.
func TestPrepareLocal_InsufficientFunds_NoVote(t *testing.T) {
	bc := newFakeBC(map[string]*AccountInfo{
		"111000000000000001": {Currency: "USD", AvailableBalance: decimal.NewFromInt(50)},
	})
	s := &fakeExecStore{}
	e := newTestExecutor(111, s, bc, nil)

	tx := protocol.InterbankTransactionPayload{
		TransactionId: protocol.ForeignBankId{RoutingNumber: 222, Id: "tx-insuf"},
		// Our account (111...) is the one being debited (negative amount).
		Postings: balancedMonasPosting("111000000000000001", "222000000000000099", 100, "USD"),
	}
	vote, err := e.PrepareLocal(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vote.Vote != protocol.VoteNo {
		t.Errorf("expected NO, got %q", vote.Vote)
	}
	found := false
	for _, r := range vote.Reasons {
		if r.Reason == protocol.ReasonInsufficientAsset {
			found = true
		}
	}
	if !found {
		t.Errorf("expected INSUFFICIENT_ASSET in reasons, got %v", vote.Reasons)
	}
	if len(s.txns) != 0 {
		t.Error("nothing should be persisted on NO vote")
	}
}

// Test 4: all postings are partner-routed → YES vote, no reservations made.
func TestPrepareLocal_PartnerSideOnly_NoReservation(t *testing.T) {
	bc := newFakeBC(map[string]*AccountInfo{})
	s := &fakeExecStore{}
	e := newTestExecutor(111, s, bc, nil)

	tx := protocol.InterbankTransactionPayload{
		TransactionId: protocol.ForeignBankId{RoutingNumber: 222, Id: "tx-partner-only"},
		// Both accounts are 222... (partner side) — nothing is ours.
		Postings: []protocol.Posting{
			{Account: &protocol.RealAccount{Num: "222000000000000001"}, Amount: decimal.NewFromInt(-200), Asset: &protocol.MonasAsset{Currency: "EUR"}},
			{Account: &protocol.RealAccount{Num: "222000000000000002"}, Amount: decimal.NewFromInt(200), Asset: &protocol.MonasAsset{Currency: "EUR"}},
		},
	}
	vote, err := e.PrepareLocal(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vote.Vote != protocol.VoteYes {
		t.Errorf("expected YES for partner-only tx, got %q reasons=%v", vote.Vote, vote.Reasons)
	}
	// No reservations since both postings are partner-side.
	if len(bc.reserved) != 0 {
		t.Errorf("expected no reservations for partner-only tx, got %v", bc.reserved)
	}
	// Still persisted.
	if len(s.txns) != 1 {
		t.Errorf("expected 1 persisted tx, got %d", len(s.txns))
	}
}

// Test 5: first MONAS reserve succeeds, second MONAS reserve fails
// → first reservation released (LIFO compensation); error returned (NOT a vote).
func TestPrepareLocal_ReserveFails_CompensatesLIFO(t *testing.T) {
	// Two of our accounts are debited; first reserve succeeds, second fails.
	bc := newFakeBC(map[string]*AccountInfo{
		"111000000000000001": {Currency: "USD", AvailableBalance: decimal.NewFromInt(1000)},
		"111000000000000002": {Currency: "USD", AvailableBalance: decimal.NewFromInt(1000)},
	})
	bc.failAfter = 2 // second ReserveMonas call fails
	s := &fakeExecStore{}
	e := newTestExecutor(111, s, bc, nil)

	// Multi-debit tx: two of our MONAS accounts send USD to partner.
	// Both accounts have sufficient balance, so validation passes.
	// First reserve succeeds; second fails → first must be released.
	tx := protocol.InterbankTransactionPayload{
		TransactionId: protocol.ForeignBankId{RoutingNumber: 222, Id: "tx-reserve-fail"},
		Postings: []protocol.Posting{
			// Debit 1: our account 1 (will succeed)
			{Account: &protocol.RealAccount{Num: "111000000000000001"}, Amount: decimal.NewFromInt(-100), Asset: &protocol.MonasAsset{Currency: "USD"}},
			// Debit 2: our account 2 (will fail on reserve)
			{Account: &protocol.RealAccount{Num: "111000000000000002"}, Amount: decimal.NewFromInt(-50), Asset: &protocol.MonasAsset{Currency: "USD"}},
			// Credits to partner
			{Account: &protocol.RealAccount{Num: "222000000000000003"}, Amount: decimal.NewFromInt(150), Asset: &protocol.MonasAsset{Currency: "USD"}},
		},
	}
	_, err := e.PrepareLocal(context.Background(), tx)
	if err == nil {
		t.Fatal("expected error due to second MONAS reserve failure")
	}
	// The first MONAS reservation must have been released (LIFO compensation).
	if len(bc.released) == 0 {
		t.Error("expected first MONAS reservation to be released after second reserve failure")
	}
	// Nothing should be persisted.
	if len(s.txns) != 0 {
		t.Error("nothing should be persisted when reservation fails")
	}
}

// Test 6: reservations all succeed but PersistPrepared returns error
// → all reservations released; error returned.
func TestPrepareLocal_PersistFails_CompensatesAllRefs(t *testing.T) {
	bc := newFakeBC(map[string]*AccountInfo{
		"111000000000000001": {Currency: "USD", AvailableBalance: decimal.NewFromInt(1000)},
	})
	s := &fakeExecStore{failOn: "simulated DB failure"}
	e := newTestExecutor(111, s, bc, nil)

	tx := protocol.InterbankTransactionPayload{
		TransactionId: protocol.ForeignBankId{RoutingNumber: 222, Id: "tx-persist-fail"},
		Postings:      balancedMonasPosting("111000000000000001", "222000000000000099", 100, "USD"),
	}
	_, err := e.PrepareLocal(context.Background(), tx)
	if err == nil {
		t.Fatal("expected error due to PersistPrepared failure")
	}
	// The MONAS reservation must have been released.
	if len(bc.released) == 0 {
		t.Error("expected reservation to be released after persist failure")
	}
}

// ---------------------------------------------------------------------------
// CommitLocal tests
// ---------------------------------------------------------------------------

// Test 7: happy path — PREPARED tx → commit all refs → COMMITTED.
func TestCommitLocal_HappyPath(t *testing.T) {
	bc := &fakeBankingCoreReserver{}
	td := &fakeTradingReserver{}
	s := &fakeExecStore{
		txns: map[string]*store.Transaction{
			txKey(222, "tx-commit"): {
				TransactionIdRouting: 222,
				TransactionIdLocal:   "tx-commit",
				Status:               store.TxStatusPrepared,
				ReservationRefs: []store.ReservationRef{
					{Kind: store.RefKindMonas, ReservationID: "res-monas-1"},
					{Kind: store.RefKindStock, ReservationID: "res-stock-1"},
				},
			},
		},
	}
	e := newTestExecutor(111, s, bc, td)

	err := e.CommitLocal(context.Background(), protocol.ForeignBankId{RoutingNumber: 222, Id: "tx-commit"})
	if err != nil {
		t.Fatalf("CommitLocal: %v", err)
	}
	// Refs should have been committed.
	if len(bc.committed) != 1 || bc.committed[0] != "res-monas-1" {
		t.Errorf("expected MONAS commit, got bc.committed=%v", bc.committed)
	}
	if len(td.committed) != 1 || td.committed[0] != "res-stock-1" {
		t.Errorf("expected STOCK commit, got td.committed=%v", td.committed)
	}
	// Status should be COMMITTED.
	tx := s.txns[txKey(222, "tx-commit")]
	if tx.Status != store.TxStatusCommitted {
		t.Errorf("expected COMMITTED status, got %q", tx.Status)
	}
}

// Test 8: idempotent — tx already COMMITTED → no-op, nil error.
func TestCommitLocal_Idempotent_AlreadyCommitted(t *testing.T) {
	bc := &fakeBankingCoreReserver{}
	s := &fakeExecStore{
		txns: map[string]*store.Transaction{
			txKey(222, "tx-already-committed"): {
				TransactionIdRouting: 222,
				TransactionIdLocal:   "tx-already-committed",
				Status:               store.TxStatusCommitted,
				ReservationRefs:      []store.ReservationRef{{Kind: store.RefKindMonas, ReservationID: "res-1"}},
			},
		},
	}
	e := newTestExecutor(111, s, bc, nil)

	err := e.CommitLocal(context.Background(), protocol.ForeignBankId{RoutingNumber: 222, Id: "tx-already-committed"})
	if err != nil {
		t.Fatalf("CommitLocal on already-committed tx should return nil, got: %v", err)
	}
	// No commit calls should have been made.
	if len(bc.committed) != 0 {
		t.Errorf("no commits expected on idempotent call, got %v", bc.committed)
	}
}

// Test 9: tx in ROLLED_BACK state → returns ErrAlreadyTerminal.
func TestCommitLocal_AlreadyTerminal(t *testing.T) {
	s := &fakeExecStore{
		txns: map[string]*store.Transaction{
			txKey(222, "tx-rolled-back"): {
				TransactionIdRouting: 222,
				TransactionIdLocal:   "tx-rolled-back",
				Status:               store.TxStatusRolledBack,
			},
		},
	}
	e := newTestExecutor(111, s, nil, nil)

	err := e.CommitLocal(context.Background(), protocol.ForeignBankId{RoutingNumber: 222, Id: "tx-rolled-back"})
	if !errors.Is(err, ErrAlreadyTerminal) {
		t.Errorf("expected ErrAlreadyTerminal, got %v", err)
	}
}

// Test 10: tx not found → nil (partner is master).
func TestCommitLocal_NotFound_NoOp(t *testing.T) {
	s := &fakeExecStore{}
	e := newTestExecutor(111, s, nil, nil)

	err := e.CommitLocal(context.Background(), protocol.ForeignBankId{RoutingNumber: 222, Id: "tx-missing"})
	if err != nil {
		t.Errorf("expected nil for missing tx (partner is master), got %v", err)
	}
}

// ---------------------------------------------------------------------------
// RollbackLocal tests
// ---------------------------------------------------------------------------

// Test 11: PREPARED tx → release all refs LIFO → ROLLED_BACK.
func TestRollbackLocal_HappyPath(t *testing.T) {
	bc := &fakeBankingCoreReserver{}
	td := &fakeTradingReserver{}
	negID := "neg-option-1"
	negRouting := 111
	s := &fakeExecStore{
		txns: map[string]*store.Transaction{
			txKey(222, "tx-rollback"): {
				TransactionIdRouting: 222,
				TransactionIdLocal:   "tx-rollback",
				Status:               store.TxStatusPrepared,
				ReservationRefs: []store.ReservationRef{
					{Kind: store.RefKindMonas, ReservationID: "res-monas-2"},
					{Kind: store.RefKindStock, ReservationID: "res-stock-2"},
					{Kind: store.RefKindOption, NegotiationRouting: &negRouting, NegotiationID: &negID},
				},
			},
		},
	}
	e := newTestExecutor(111, s, bc, td)

	err := e.RollbackLocal(context.Background(), protocol.ForeignBankId{RoutingNumber: 222, Id: "tx-rollback"})
	if err != nil {
		t.Fatalf("RollbackLocal: %v", err)
	}
	// All three refs released, in LIFO order (OPTION first, then STOCK, then MONAS).
	if len(bc.released) != 1 || bc.released[0] != "res-monas-2" {
		t.Errorf("expected MONAS release, got bc.released=%v", bc.released)
	}
	if len(td.released) != 2 {
		t.Errorf("expected STOCK + OPTION releases, got td.released=%v", td.released)
	}
	// Status should be ROLLED_BACK.
	tx := s.txns[txKey(222, "tx-rollback")]
	if tx.Status != store.TxStatusRolledBack {
		t.Errorf("expected ROLLED_BACK status, got %q", tx.Status)
	}
}

// Test 12: tx COMMITTED → no-op, no error (already finalized).
func TestRollbackLocal_AlreadyTerminal_NoOp(t *testing.T) {
	bc := &fakeBankingCoreReserver{}
	s := &fakeExecStore{
		txns: map[string]*store.Transaction{
			txKey(222, "tx-committed-rollback"): {
				TransactionIdRouting: 222,
				TransactionIdLocal:   "tx-committed-rollback",
				Status:               store.TxStatusCommitted,
				ReservationRefs:      []store.ReservationRef{{Kind: store.RefKindMonas, ReservationID: "res-x"}},
			},
		},
	}
	e := newTestExecutor(111, s, bc, nil)

	err := e.RollbackLocal(context.Background(), protocol.ForeignBankId{RoutingNumber: 222, Id: "tx-committed-rollback"})
	if err != nil {
		t.Errorf("RollbackLocal on COMMITTED tx should be no-op, got: %v", err)
	}
	if len(bc.released) != 0 {
		t.Errorf("no releases expected for COMMITTED tx, got %v", bc.released)
	}
}

// ---------------------------------------------------------------------------
// Additional tests
// ---------------------------------------------------------------------------

// Test 13: CommitLocal with OPTION ref → ExerciseOption called.
func TestCommitLocal_OptionRef_Exercises(t *testing.T) {
	td := &fakeTradingReserver{}
	negID := "neg-exercise-1"
	negRouting := 111
	s := &fakeExecStore{
		txns: map[string]*store.Transaction{
			txKey(222, "tx-option-commit"): {
				TransactionIdRouting: 222,
				TransactionIdLocal:   "tx-option-commit",
				Status:               store.TxStatusPrepared,
				ReservationRefs: []store.ReservationRef{
					{Kind: store.RefKindOption, NegotiationRouting: &negRouting, NegotiationID: &negID},
				},
			},
		},
	}
	e := newTestExecutor(111, s, &fakeBankingCoreReserver{}, td)

	err := e.CommitLocal(context.Background(), protocol.ForeignBankId{RoutingNumber: 222, Id: "tx-option-commit"})
	if err != nil {
		t.Fatalf("CommitLocal: %v", err)
	}
	if len(td.committed) != 1 || td.committed[0] != "option-exercise-"+negID {
		t.Errorf("expected ExerciseOption(%q), got td.committed=%v", negID, td.committed)
	}
	tx := s.txns[txKey(222, "tx-option-commit")]
	if tx.Status != store.TxStatusCommitted {
		t.Errorf("expected COMMITTED, got %q", tx.Status)
	}
}

// Test 14: LIFO order during compensation — verify third debit's release happens in LIFO order.
// Three MONAS debits: first two succeed, third fails → both previous released in reverse (LIFO).
func TestPrepareLocal_CompensationIsLIFO(t *testing.T) {
	bc := newFakeBC(map[string]*AccountInfo{
		"111000000000000001": {Currency: "USD", AvailableBalance: decimal.NewFromInt(1000)},
		"111000000000000002": {Currency: "USD", AvailableBalance: decimal.NewFromInt(1000)},
		"111000000000000003": {Currency: "USD", AvailableBalance: decimal.NewFromInt(1000)},
	})
	bc.failAfter = 3 // third ReserveMonas call fails
	s := &fakeExecStore{}
	e := newTestExecutor(111, s, bc, nil)

	tx := protocol.InterbankTransactionPayload{
		TransactionId: protocol.ForeignBankId{RoutingNumber: 222, Id: "tx-lifo"},
		Postings: []protocol.Posting{
			// Debit 1 (reserve call 1 — succeeds)
			{Account: &protocol.RealAccount{Num: "111000000000000001"}, Amount: decimal.NewFromInt(-30), Asset: &protocol.MonasAsset{Currency: "USD"}},
			// Debit 2 (reserve call 2 — succeeds)
			{Account: &protocol.RealAccount{Num: "111000000000000002"}, Amount: decimal.NewFromInt(-30), Asset: &protocol.MonasAsset{Currency: "USD"}},
			// Debit 3 (reserve call 3 — fails)
			{Account: &protocol.RealAccount{Num: "111000000000000003"}, Amount: decimal.NewFromInt(-30), Asset: &protocol.MonasAsset{Currency: "USD"}},
			// Credits to partner
			{Account: &protocol.RealAccount{Num: "222000000000000099"}, Amount: decimal.NewFromInt(90), Asset: &protocol.MonasAsset{Currency: "USD"}},
		},
	}
	_, err := e.PrepareLocal(context.Background(), tx)
	if err == nil {
		t.Fatal("expected error from third reserve failure")
	}
	// Two reservations were made; both must be released (LIFO: last-reserved-first-released).
	if len(bc.released) != 2 {
		t.Errorf("expected 2 MONAS releases (LIFO compensation for 2 successful reserves), got %d: %v", len(bc.released), bc.released)
	}
	// LIFO order: account 2 released before account 1.
	if len(bc.released) == 2 {
		if bc.released[0] != "monas-res-111000000000000002" {
			t.Errorf("LIFO: expected account 2 released first, got %q", bc.released[0])
		}
		if bc.released[1] != "monas-res-111000000000000001" {
			t.Errorf("LIFO: expected account 1 released second, got %q", bc.released[1])
		}
	}
}

// Test 15: RollbackLocal on missing tx → nil (partner is master, best-effort).
func TestRollbackLocal_NotFound_NoOp(t *testing.T) {
	s := &fakeExecStore{}
	e := newTestExecutor(111, s, nil, nil)

	err := e.RollbackLocal(context.Background(), protocol.ForeignBankId{RoutingNumber: 222, Id: "tx-missing-rb"})
	if err != nil {
		t.Errorf("expected nil for missing tx during rollback, got %v", err)
	}
}
