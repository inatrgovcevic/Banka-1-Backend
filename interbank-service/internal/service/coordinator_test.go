package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"
)

// ---------------------------------------------------------------------------
// fakeOutboundClient
// ---------------------------------------------------------------------------

type fakeOutboundClient struct {
	vote     protocol.TransactionVote
	sendErr  error
	commitErr error
	committed bool
	rolledBack bool
}

func (f *fakeOutboundClient) SendNewTx(_ context.Context, _ int, _ protocol.InterbankTransactionPayload) (protocol.TransactionVote, error) {
	return f.vote, f.sendErr
}

func (f *fakeOutboundClient) SendCommitTx(_ context.Context, _ int, _ protocol.ForeignBankId) error {
	f.committed = true
	return f.commitErr
}

func (f *fakeOutboundClient) SendRollbackTx(_ context.Context, _ int, _ protocol.ForeignBankId) error {
	f.rolledBack = true
	return nil
}

// ---------------------------------------------------------------------------
// fakeContractStore
// ---------------------------------------------------------------------------

type fakeContractStore struct {
	inserted []*store.Contract
}

func (f *fakeContractStore) Insert(_ context.Context, c *store.Contract) error {
	cp := *c
	f.inserted = append(f.inserted, &cp)
	return nil
}

// ---------------------------------------------------------------------------
// fakeExecutorForCoordinator
// Acts as a minimal stand-in for *Executor without a real Postgres backend.
// The Coordinator requires *Executor (not an interface), so we use a real
// Executor wired with stub store/clients that return predictable results.
// ---------------------------------------------------------------------------

// coordExecStore is a minimal ExecutorStore for Coordinator tests (named to
// avoid conflict with executor_test.go's fakeExecStore).
type coordExecStore struct {
	prepared map[string]*store.Transaction
	fail     bool
}

func newCoordExecStore() *coordExecStore {
	return &coordExecStore{prepared: make(map[string]*store.Transaction)}
}

func (f *coordExecStore) PersistPrepared(_ context.Context, t *store.Transaction) error {
	if f.fail {
		return errors.New("fake: persist failed")
	}
	key := t.TransactionIdLocal
	cp := *t
	f.prepared[key] = &cp
	return nil
}

func (f *coordExecStore) FindTx(_ context.Context, _ int, id string) (*store.Transaction, error) {
	if t, ok := f.prepared[id]; ok {
		cp := *t
		return &cp, nil
	}
	return nil, nil
}

func (f *coordExecStore) UpdateTxStatus(_ context.Context, _ int, id, status string) error {
	if t, ok := f.prepared[id]; ok {
		t.Status = status
	}
	return nil
}

// fakeBCReserver satisfies BankingCoreReserver with no real reservations.
type fakeBCReserver struct{}

func (fakeBCReserver) ResolveAccount(_ context.Context, _ string) (*AccountInfo, error) {
	return &AccountInfo{Currency: "USD", AvailableBalance: decimal.NewFromFloat(99999)}, nil
}

func (fakeBCReserver) FindAccountByOwnerAndCurrency(_ context.Context, _ int64, _ string) (string, error) {
	return "111000001234567890", nil
}

func (fakeBCReserver) ReserveMonas(_ context.Context, _, _ string, _ decimal.Decimal, _, _ interface{}) (string, error) {
	return "res-monas-01", nil
}

func (fakeBCReserver) CommitMonas(_ context.Context, _ string) error { return nil }
func (fakeBCReserver) ReleaseMonas(_ context.Context, _ string) error { return nil }

// These ReserveMonas signatures need to match the interface.
// BankingCoreReserver has ReserveMonas(ctx, accountNum, currency string, amount decimal.Decimal, txIDRouting int, txIDLocal string)
// We need to match that exactly.

// fakeTDReserver satisfies TradingReserver with no-ops.
type fakeTDReserver struct{}

func (fakeTDReserver) ReserveStock(_ context.Context, _ int64, _ string, _ int, _ int, _ string) (string, error) {
	return "res-stock-01", nil
}
func (fakeTDReserver) CommitStock(_ context.Context, _ string) error  { return nil }
func (fakeTDReserver) ReleaseStock(_ context.Context, _ string) error { return nil }
func (fakeTDReserver) ReserveOption(_ context.Context, _ protocol.ForeignBankId, _, _ string, _ int) error {
	return nil
}
func (fakeTDReserver) ExerciseOption(_ context.Context, _ protocol.ForeignBankId) error { return nil }
func (fakeTDReserver) ReleaseOption(_ context.Context, _ protocol.ForeignBankId) error  { return nil }

// fakeBCReserverFull implements BankingCoreReserver with correct signatures.
type fakeBCReserverFull struct{}

func (f fakeBCReserverFull) ResolveAccount(ctx context.Context, num string) (*AccountInfo, error) {
	return &AccountInfo{Currency: "USD", AvailableBalance: decimal.NewFromFloat(99999)}, nil
}

func (f fakeBCReserverFull) FindAccountByOwnerAndCurrency(ctx context.Context, ownerID int64, currency string) (string, error) {
	return "111000001234567890", nil
}

func (f fakeBCReserverFull) ReserveMonas(ctx context.Context, accountNum, currency string, amount decimal.Decimal, txIDRouting int, txIDLocal string) (string, error) {
	return "res-monas-01", nil
}

func (f fakeBCReserverFull) CommitMonas(ctx context.Context, reservationID string) error { return nil }
func (f fakeBCReserverFull) ReleaseMonas(ctx context.Context, reservationID string) error { return nil }

// ---------------------------------------------------------------------------
// build helpers
// ---------------------------------------------------------------------------

func buildNeg(ongoing bool, settlementFuture bool) *store.Negotiation {
	sd := time.Now().Add(24 * time.Hour)
	if !settlementFuture {
		sd = time.Now().Add(-24 * time.Hour)
	}
	return &store.Negotiation{
		ID:                    "neg-test-001",
		BuyerRouting:          theirRouting,
		BuyerID:               "C-2",
		SellerRouting:         myRouting,
		SellerID:              "C-15",
		StockTicker:           "AAPL",
		Amount:                10,
		PriceCurrency:         "USD",
		PriceAmount:           decimal.NewFromFloat(150),
		PremiumCurrency:       "USD",
		PremiumAmount:         decimal.NewFromFloat(5),
		SettlementDate:        sd,
		LastModifiedByRouting: myRouting, // last mod by us → their turn to accept
		LastModifiedByID:      "sys",
		IsOngoing:             ongoing,
		IsAuthoritative:       true,
	}
}

func buildCoordinator(outbound OutboundClient, negStore NegotiationStoreIface, contractSt ContractStoreIface) *Coordinator {
	es := newCoordExecStore()
	// Wire NegotiationSellerLookup so the Validator can resolve option negotiations.
	negLookup := NewNegotiationSellerLookup(negStore)
	exec := NewExecutor(myRouting, es, fakeBCReserverFull{}, fakeTDReserver{}, negLookup, nil)
	return NewCoordinator(myRouting, exec, outbound, negStore, contractSt, fakeBCReserverFull{}, nil)
}

// ---------------------------------------------------------------------------
// Coordinator tests
// ---------------------------------------------------------------------------

func TestCoordinator_AcceptNegotiation_HappyPath(t *testing.T) {
	ns := newFakeNegotiationStore()
	neg := buildNeg(true, true)
	ns.mu.Lock()
	ns.rows[neg.ID] = neg
	ns.mu.Unlock()
	cs := &fakeContractStore{}
	outbound := &fakeOutboundClient{
		vote: protocol.TransactionVote{Vote: protocol.VoteYes},
	}
	coord := buildCoordinator(outbound, ns, cs)
	err := coord.AcceptNegotiation(context.Background(), neg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !outbound.committed {
		t.Error("expected SendCommitTx to be called")
	}
	if len(cs.inserted) == 0 {
		t.Error("expected contract to be inserted")
	}
}

func TestCoordinator_AcceptNegotiation_PartnerRejects(t *testing.T) {
	ns := newFakeNegotiationStore()
	neg := buildNeg(true, true)
	ns.mu.Lock()
	ns.rows[neg.ID] = neg
	ns.mu.Unlock()
	cs := &fakeContractStore{}
	outbound := &fakeOutboundClient{
		vote: protocol.TransactionVote{
			Vote:    protocol.VoteNo,
			Reasons: []protocol.NoVoteReason{{Reason: protocol.ReasonInsufficientAsset}},
		},
	}
	coord := buildCoordinator(outbound, ns, cs)
	err := coord.AcceptNegotiation(context.Background(), neg)
	if err == nil {
		t.Fatal("expected error when partner rejects")
	}
	if !errors.Is(err, ErrInterbankProtocol) {
		t.Errorf("expected ErrInterbankProtocol, got %v", err)
	}
	// Partner rollback should have been sent.
	if !outbound.rolledBack {
		t.Error("expected SendRollbackTx to be called on partner reject")
	}
}

func TestCoordinator_AcceptNegotiation_PartnerSendError(t *testing.T) {
	ns := newFakeNegotiationStore()
	neg := buildNeg(true, true)
	ns.mu.Lock()
	ns.rows[neg.ID] = neg
	ns.mu.Unlock()
	cs := &fakeContractStore{}
	outbound := &fakeOutboundClient{
		sendErr: errors.New("network timeout"),
	}
	coord := buildCoordinator(outbound, ns, cs)
	err := coord.AcceptNegotiation(context.Background(), neg)
	if err == nil {
		t.Fatal("expected error on send failure")
	}
	if !errors.Is(err, ErrInterbankProtocol) {
		t.Errorf("expected ErrInterbankProtocol, got %v", err)
	}
}

func TestCoordinator_CommitTxSendFailure_NonFatal(t *testing.T) {
	// If SendCommitTx fails, AcceptNegotiation should still return nil
	// (retry scheduler handles it). Contract must be inserted.
	ns := newFakeNegotiationStore()
	neg := buildNeg(true, true)
	ns.mu.Lock()
	ns.rows[neg.ID] = neg
	ns.mu.Unlock()
	cs := &fakeContractStore{}
	outbound := &fakeOutboundClient{
		vote:      protocol.TransactionVote{Vote: protocol.VoteYes},
		commitErr: errors.New("partner unreachable"),
	}
	coord := buildCoordinator(outbound, ns, cs)
	err := coord.AcceptNegotiation(context.Background(), neg)
	if err != nil {
		t.Fatalf("expected nil (best-effort commit); got %v", err)
	}
	if len(cs.inserted) == 0 {
		t.Error("contract must be inserted even if SendCommitTx fails")
	}
}
