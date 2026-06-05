package grpc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonv1 "github.com/raf-si-2025/banka-1-go/proto/banka1/common/v1"
	interbankv1 "github.com/raf-si-2025/banka-1-go/proto/banka1/interbank/v1"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/client"
	grpcserver "github.com/raf-si-2025/banka-1-go/interbank-service/internal/grpc"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/service"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"
	"github.com/raf-si-2025/banka-1-go/shared/auth"
)

// ---------------------------------------------------------------------------
// Fakes for the *real* grpcserver.Server: NegotiationStoreIface + CoordinatorIface
// ---------------------------------------------------------------------------

type realNegStore struct {
	mu   sync.Mutex
	rows map[string]*store.Negotiation
}

func newRealNegStore() *realNegStore { return &realNegStore{rows: make(map[string]*store.Negotiation)} }

func (f *realNegStore) Insert(_ context.Context, n *store.Negotiation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *n
	f.rows[n.ID] = &cp
	return nil
}

func (f *realNegStore) FindByAuthoritativeRef(_ context.Context, _ int, id string) (*store.Negotiation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if n, ok := f.rows[id]; ok {
		cp := *n
		return &cp, nil
	}
	return nil, nil
}

func (f *realNegStore) UpdateCounter(_ context.Context, n *store.Negotiation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *n
	f.rows[n.ID] = &cp
	return nil
}

func (f *realNegStore) MarkClosed(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if n, ok := f.rows[id]; ok {
		n.IsOngoing = false
	}
	return nil
}

type noopCoordinator struct{ err error }

func (c *noopCoordinator) AcceptNegotiation(_ context.Context, _ *store.Negotiation) error {
	return c.err
}

// ---------------------------------------------------------------------------
// Fake ExecutorStore (only the methods Executor needs) — no DB.
// ---------------------------------------------------------------------------

type fakeExecStoreReal struct {
	mu          sync.Mutex
	txs         map[string]*store.Transaction
	persistErr  error
	findErr     error
}

func newFakeExecStoreReal() *fakeExecStoreReal {
	return &fakeExecStoreReal{txs: make(map[string]*store.Transaction)}
}

func (f *fakeExecStoreReal) PersistPrepared(_ context.Context, t *store.Transaction) error {
	if f.persistErr != nil {
		return f.persistErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *t
	f.txs[t.TransactionIdLocal] = &cp
	return nil
}

func (f *fakeExecStoreReal) FindTx(_ context.Context, _ int, id string) (*store.Transaction, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.txs[id]; ok {
		cp := *t
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeExecStoreReal) UpdateTxStatus(_ context.Context, _ int, id, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.txs[id]; ok {
		t.Status = status
	}
	return nil
}

// ---------------------------------------------------------------------------
// Fake GrpcMessageStore (idempotency cache) — no DB.
// ---------------------------------------------------------------------------

type fakeGrpcMsgStore struct {
	mu        sync.Mutex
	rows      map[string]*store.Message
	lookupErr error
	insertErr error
}

func newFakeGrpcMsgStore() *fakeGrpcMsgStore {
	return &fakeGrpcMsgStore{rows: make(map[string]*store.Message)}
}

func (f *fakeGrpcMsgStore) Lookup(_ context.Context, _ string, _ int, key string) (*store.Message, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if m, ok := f.rows[key]; ok {
		cp := *m
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeGrpcMsgStore) Insert(_ context.Context, m *store.Message) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *m
	f.rows[m.LocallyGeneratedKey] = &cp
	return nil
}

// ---------------------------------------------------------------------------
// Build a real *grpcserver.Server wired through fakes / httptest clients.
// ---------------------------------------------------------------------------

func newRealServer(t *testing.T, ns *realNegStore, coord service.CoordinatorIface, tradingURL, userURL string) interbankv1.InterbankProtocolServiceClient {
	t.Helper()
	issuer := auth.NewS2SIssuer("interbank", "svc", []string{"ROLE_INTERNAL"}, "test-secret", time.Minute)

	otcSvc := service.NewOtcNegotiationService(111, ns, coord, nil)

	deps := grpcserver.Deps{
		MyRouting:     111,
		MyDisplayName: "Banka 1",
		OtcService:    otcSvc,
	}
	if tradingURL != "" {
		deps.Trading = client.NewTradingClient(tradingURL, issuer, time.Second)
	}
	if userURL != "" {
		deps.User = client.NewUserClient(userURL, issuer, time.Second)
	}
	srv := grpcserver.NewServer(deps)
	return dialBufconn(t, srv)
}

// newRealServerForPostMessage wires a real Server with a real Executor (fake
// store) + a fake idempotency cache so the PostMessage 2PC dispatch can be
// exercised end-to-end without a database.
func newRealServerForPostMessage(t *testing.T, exStore *fakeExecStoreReal, msg grpcserver.GrpcMessageStore) interbankv1.InterbankProtocolServiceClient {
	t.Helper()
	exec := service.NewExecutor(111, exStore, nil, nil, nil, nil)
	deps := grpcserver.Deps{
		MyRouting:     111,
		MyDisplayName: "Banka 1",
		Executor:      exec,
		MessageStore:  msg,
	}
	return dialBufconn(t, grpcserver.NewServer(deps))
}

// foreignPosting builds a posting on the partner side (routing 222) so that
// PrepareLocal treats it as "not ours" and skips reservation/validation.
func foreignPosting(amount string) *interbankv1.Posting {
	return &interbankv1.Posting{
		Account: &interbankv1.TxAccount{
			Body: &interbankv1.TxAccount_Person{Person: &commonv1.ForeignBankId{RoutingNumber: 222, Id: "C-2"}},
		},
		Amount: amount,
		Asset: &interbankv1.Asset{
			Body: &interbankv1.Asset_Monas{Monas: &commonv1.MonetaryValue{Currency: "USD", Amount: "0"}},
		},
	}
}

func TestReal_PostMessage_NewTx_YesVote(t *testing.T) {
	cl := newRealServerForPostMessage(t, newFakeExecStoreReal(), newFakeGrpcMsgStore())
	resp, err := cl.PostMessage(context.Background(), &interbankv1.PostMessageRequest{
		IdempotenceKey: &commonv1.IdempotenceKey{RoutingNumber: 222, LocallyGeneratedKey: "k1"},
		Type:           interbankv1.MessageType_MESSAGE_TYPE_NEW_TX,
		Body: &interbankv1.PostMessageRequest_NewTx{NewTx: &interbankv1.InterbankTransactionPayload{
			TransactionId: &commonv1.ForeignBankId{RoutingNumber: 222, Id: "tx-1"},
			Postings:      []*interbankv1.Posting{foreignPosting("-100.00"), foreignPosting("100.00")},
		}},
	})
	if err != nil {
		t.Fatalf("PostMessage NEW_TX: %v", err)
	}
	if resp.GetHttpStatusCode() != 200 || resp.GetVote().GetVote() != interbankv1.TransactionVote_VOTE_YES {
		t.Errorf("resp: %+v", resp)
	}
}

func TestReal_PostMessage_NewTx_MissingBody(t *testing.T) {
	cl := newRealServerForPostMessage(t, newFakeExecStoreReal(), newFakeGrpcMsgStore())
	_, err := cl.PostMessage(context.Background(), &interbankv1.PostMessageRequest{
		IdempotenceKey: &commonv1.IdempotenceKey{RoutingNumber: 222, LocallyGeneratedKey: "k-nobody"},
		Type:           interbankv1.MessageType_MESSAGE_TYPE_NEW_TX,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for missing new_tx body, got %v", err)
	}
}

func TestReal_PostMessage_NewTx_EmptyPostings(t *testing.T) {
	cl := newRealServerForPostMessage(t, newFakeExecStoreReal(), newFakeGrpcMsgStore())
	_, err := cl.PostMessage(context.Background(), &interbankv1.PostMessageRequest{
		IdempotenceKey: &commonv1.IdempotenceKey{RoutingNumber: 222, LocallyGeneratedKey: "k-empty"},
		Type:           interbankv1.MessageType_MESSAGE_TYPE_NEW_TX,
		Body:           &interbankv1.PostMessageRequest_NewTx{NewTx: &interbankv1.InterbankTransactionPayload{}},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for empty postings, got %v", err)
	}
}

func TestReal_PostMessage_NewTx_IdempotentReplay(t *testing.T) {
	msg := newFakeGrpcMsgStore()
	cl := newRealServerForPostMessage(t, newFakeExecStoreReal(), msg)
	req := &interbankv1.PostMessageRequest{
		IdempotenceKey: &commonv1.IdempotenceKey{RoutingNumber: 222, LocallyGeneratedKey: "k-replay"},
		Type:           interbankv1.MessageType_MESSAGE_TYPE_NEW_TX,
		Body: &interbankv1.PostMessageRequest_NewTx{NewTx: &interbankv1.InterbankTransactionPayload{
			TransactionId: &commonv1.ForeignBankId{RoutingNumber: 222, Id: "tx-replay"},
			Postings:      []*interbankv1.Posting{foreignPosting("-100.00"), foreignPosting("100.00")},
		}},
	}
	if _, err := cl.PostMessage(context.Background(), req); err != nil {
		t.Fatalf("first call: %v", err)
	}
	resp, err := cl.PostMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if resp.GetHttpStatusCode() != 200 || resp.GetVote().GetVote() != interbankv1.TransactionVote_VOTE_YES {
		t.Errorf("replay resp: %+v", resp)
	}
}

func TestReal_PostMessage_CommitTx(t *testing.T) {
	cl := newRealServerForPostMessage(t, newFakeExecStoreReal(), newFakeGrpcMsgStore())
	resp, err := cl.PostMessage(context.Background(), &interbankv1.PostMessageRequest{
		IdempotenceKey: &commonv1.IdempotenceKey{RoutingNumber: 222, LocallyGeneratedKey: "k-commit"},
		Type:           interbankv1.MessageType_MESSAGE_TYPE_COMMIT_TX,
		Body: &interbankv1.PostMessageRequest_CommitTx{CommitTx: &interbankv1.CommitTransactionBody{
			TransactionId: &commonv1.ForeignBankId{RoutingNumber: 222, Id: "tx-commit"},
		}},
	})
	if err != nil {
		t.Fatalf("COMMIT_TX: %v", err)
	}
	if resp.GetHttpStatusCode() != 204 {
		t.Errorf("expected 204, got %d", resp.GetHttpStatusCode())
	}
}

func TestReal_PostMessage_CommitTx_MissingBody(t *testing.T) {
	cl := newRealServerForPostMessage(t, newFakeExecStoreReal(), newFakeGrpcMsgStore())
	_, err := cl.PostMessage(context.Background(), &interbankv1.PostMessageRequest{
		IdempotenceKey: &commonv1.IdempotenceKey{RoutingNumber: 222, LocallyGeneratedKey: "k-commit-nobody"},
		Type:           interbankv1.MessageType_MESSAGE_TYPE_COMMIT_TX,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestReal_PostMessage_RollbackTx(t *testing.T) {
	cl := newRealServerForPostMessage(t, newFakeExecStoreReal(), newFakeGrpcMsgStore())
	resp, err := cl.PostMessage(context.Background(), &interbankv1.PostMessageRequest{
		IdempotenceKey: &commonv1.IdempotenceKey{RoutingNumber: 222, LocallyGeneratedKey: "k-rb"},
		Type:           interbankv1.MessageType_MESSAGE_TYPE_ROLLBACK_TX,
		Body: &interbankv1.PostMessageRequest_RollbackTx{RollbackTx: &interbankv1.RollbackTransactionBody{
			TransactionId: &commonv1.ForeignBankId{RoutingNumber: 222, Id: "tx-rb"},
		}},
	})
	if err != nil {
		t.Fatalf("ROLLBACK_TX: %v", err)
	}
	if resp.GetHttpStatusCode() != 204 {
		t.Errorf("expected 204, got %d", resp.GetHttpStatusCode())
	}
}

func TestReal_PostMessage_RollbackTx_MissingBody(t *testing.T) {
	cl := newRealServerForPostMessage(t, newFakeExecStoreReal(), newFakeGrpcMsgStore())
	_, err := cl.PostMessage(context.Background(), &interbankv1.PostMessageRequest{
		IdempotenceKey: &commonv1.IdempotenceKey{RoutingNumber: 222, LocallyGeneratedKey: "k-rb-nobody"},
		Type:           interbankv1.MessageType_MESSAGE_TYPE_ROLLBACK_TX,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestReal_PostMessage_LookupError(t *testing.T) {
	msg := newFakeGrpcMsgStore()
	msg.lookupErr = errInjectedReal
	cl := newRealServerForPostMessage(t, newFakeExecStoreReal(), msg)
	_, err := cl.PostMessage(context.Background(), &interbankv1.PostMessageRequest{
		IdempotenceKey: &commonv1.IdempotenceKey{RoutingNumber: 222, LocallyGeneratedKey: "k-lookuperr"},
		Type:           interbankv1.MessageType_MESSAGE_TYPE_NEW_TX,
		Body: &interbankv1.PostMessageRequest_NewTx{NewTx: &interbankv1.InterbankTransactionPayload{
			Postings: []*interbankv1.Posting{foreignPosting("-1.00"), foreignPosting("1.00")},
		}},
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal on lookup error, got %v", err)
	}
}

func TestReal_PostMessage_UnknownType(t *testing.T) {
	cl := newRealServerForPostMessage(t, newFakeExecStoreReal(), newFakeGrpcMsgStore())
	_, err := cl.PostMessage(context.Background(), &interbankv1.PostMessageRequest{
		IdempotenceKey: &commonv1.IdempotenceKey{RoutingNumber: 222, LocallyGeneratedKey: "k-unknown"},
		Type:           interbankv1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for unspecified type, got %v", err)
	}
}

var errInjectedReal = realErr("boom")

type realErr string

func (e realErr) Error() string { return string(e) }

func futureRFC3339() string {
	return time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// CreateNegotiation (real handler)
// ---------------------------------------------------------------------------

func TestReal_CreateNegotiation_Success(t *testing.T) {
	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")

	resp, err := cl.CreateNegotiation(context.Background(), &interbankv1.CreateNegotiationRequest{
		BuyerId:          &commonv1.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
		SellerId:         &commonv1.ForeignBankId{RoutingNumber: 111, Id: "C-15"},
		LastModifiedBy:   &commonv1.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
		StockDescription: &commonv1.StockDescription{Ticker: "AAPL"},
		PricePerUnit:     &commonv1.MonetaryValue{Currency: "USD", Amount: "150.00"},
		Premium:          &commonv1.MonetaryValue{Currency: "USD", Amount: "10.00"},
		Amount:           10,
		SettlementDate:   futureRFC3339(),
	})
	if err != nil {
		t.Fatalf("CreateNegotiation: %v", err)
	}
	if resp.GetId().GetId() == "" {
		t.Error("expected non-empty created id")
	}
}

func TestReal_CreateNegotiation_BadDate_InvalidArg(t *testing.T) {
	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	_, err := cl.CreateNegotiation(context.Background(), &interbankv1.CreateNegotiationRequest{
		SettlementDate: "not-a-date",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestReal_CreateNegotiation_ServiceError_Mapped(t *testing.T) {
	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	// sellerId routing != myRouting → ErrNegotiationInvalid → InvalidArgument.
	_, err := cl.CreateNegotiation(context.Background(), &interbankv1.CreateNegotiationRequest{
		BuyerId:          &commonv1.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
		SellerId:         &commonv1.ForeignBankId{RoutingNumber: 999, Id: "C-15"},
		LastModifiedBy:   &commonv1.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
		StockDescription: &commonv1.StockDescription{Ticker: "AAPL"},
		PricePerUnit:     &commonv1.MonetaryValue{Currency: "USD", Amount: "150.00"},
		Premium:          &commonv1.MonetaryValue{Currency: "USD", Amount: "10.00"},
		Amount:           10,
		SettlementDate:   futureRFC3339(),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for invalid negotiation, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetNegotiation (real handler)
// ---------------------------------------------------------------------------

func TestReal_GetNegotiation_Success(t *testing.T) {
	ns := newRealNegStore()
	ns.rows["neg-1"] = &store.Negotiation{
		ID:            "neg-1",
		BuyerRouting:  222,
		BuyerID:       "C-2",
		SellerRouting: 111,
		SellerID:      "C-15",
		StockTicker:   "AAPL",
		PriceCurrency: "USD",
		PriceAmount:   decimal.NewFromFloat(150),
		PremiumCurrency: "USD",
		PremiumAmount: decimal.NewFromFloat(10),
		Amount:        10,
		IsOngoing:     true,
		SettlementDate: time.Now().Add(48 * time.Hour),
		LastModifiedByRouting: 222,
		LastModifiedByID: "C-2",
	}
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	resp, err := cl.GetNegotiation(context.Background(), &interbankv1.GetNegotiationRequest{
		RoutingNumber: 111, Id: "neg-1",
	})
	if err != nil {
		t.Fatalf("GetNegotiation: %v", err)
	}
	if resp.GetNegotiation().GetStockDescription().GetTicker() != "AAPL" {
		t.Errorf("ticker: %s", resp.GetNegotiation().GetStockDescription().GetTicker())
	}
}

func TestReal_GetNegotiation_NotFound(t *testing.T) {
	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	_, err := cl.GetNegotiation(context.Background(), &interbankv1.GetNegotiationRequest{
		RoutingNumber: 111, Id: "ghost",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// PutCounter (real handler)
// ---------------------------------------------------------------------------

func TestReal_PutCounter_Success(t *testing.T) {
	ns := newRealNegStore()
	ns.rows["neg-1"] = &store.Negotiation{
		ID:            "neg-1",
		BuyerRouting:  222,
		BuyerID:       "C-2",
		SellerRouting: 111,
		SellerID:      "C-15",
		IsOngoing:     true,
		SettlementDate: time.Now().Add(48 * time.Hour),
		LastModifiedByRouting: 111, // we modified last → partner (222) may counter
		LastModifiedByID: "C-15",
	}
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	_, err := cl.PutCounter(context.Background(), &interbankv1.PutCounterRequest{
		RoutingNumber:    111,
		Id:               "neg-1",
		BuyerId:          &commonv1.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
		SellerId:         &commonv1.ForeignBankId{RoutingNumber: 111, Id: "C-15"},
		LastModifiedBy:   &commonv1.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
		StockDescription: &commonv1.StockDescription{Ticker: "AAPL"},
		PricePerUnit:     &commonv1.MonetaryValue{Currency: "USD", Amount: "155.00"},
		Premium:          &commonv1.MonetaryValue{Currency: "USD", Amount: "11.00"},
		Amount:           10,
		SettlementDate:   futureRFC3339(),
	})
	if err != nil {
		t.Fatalf("PutCounter: %v", err)
	}
}

func TestReal_PutCounter_BadDate_InvalidArg(t *testing.T) {
	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	_, err := cl.PutCounter(context.Background(), &interbankv1.PutCounterRequest{
		RoutingNumber:  111,
		Id:             "neg-1",
		SettlementDate: "bad",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestReal_PutCounter_NotFound(t *testing.T) {
	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	_, err := cl.PutCounter(context.Background(), &interbankv1.PutCounterRequest{
		RoutingNumber:  111,
		Id:             "ghost",
		LastModifiedBy: &commonv1.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
		PricePerUnit:   &commonv1.MonetaryValue{Currency: "USD", Amount: "1.00"},
		Premium:        &commonv1.MonetaryValue{Currency: "USD", Amount: "0"},
		Amount:         1,
		SettlementDate: futureRFC3339(),
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// DeleteNegotiation (real handler)
// ---------------------------------------------------------------------------

func TestReal_DeleteNegotiation_Success(t *testing.T) {
	ns := newRealNegStore()
	ns.rows["neg-1"] = &store.Negotiation{
		ID:            "neg-1",
		BuyerRouting:  222,
		BuyerID:       "C-2",
		SellerRouting: 111,
		SellerID:      "C-15",
		IsOngoing:     true,
	}
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	_, err := cl.DeleteNegotiation(context.Background(), &interbankv1.DeleteNegotiationRequest{
		RoutingNumber: 111, Id: "neg-1",
	})
	if err != nil {
		t.Fatalf("DeleteNegotiation: %v", err)
	}
}

func TestReal_DeleteNegotiation_IdempotentMissing(t *testing.T) {
	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	_, err := cl.DeleteNegotiation(context.Background(), &interbankv1.DeleteNegotiationRequest{
		RoutingNumber: 111, Id: "ghost",
	})
	if err != nil {
		t.Fatalf("expected idempotent success for missing, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// AcceptNegotiation (real handler)
// ---------------------------------------------------------------------------

func TestReal_AcceptNegotiation_Success(t *testing.T) {
	ns := newRealNegStore()
	ns.rows["neg-1"] = &store.Negotiation{
		ID:            "neg-1",
		BuyerRouting:  222,
		BuyerID:       "C-2",
		SellerRouting: 111,
		SellerID:      "C-15",
		IsOngoing:     true,
		SettlementDate: time.Now().Add(48 * time.Hour),
		LastModifiedByRouting: 222, // partner modified last → we (111) can accept
		LastModifiedByID: "C-2",
	}
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	_, err := cl.AcceptNegotiation(context.Background(), &interbankv1.AcceptNegotiationRequest{
		RoutingNumber: 111, Id: "neg-1",
	})
	if err != nil {
		t.Fatalf("AcceptNegotiation: %v", err)
	}
}

func TestReal_AcceptNegotiation_NotFound(t *testing.T) {
	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	_, err := cl.AcceptNegotiation(context.Background(), &interbankv1.AcceptNegotiationRequest{
		RoutingNumber: 111, Id: "ghost",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetPublicStock (real handler, httptest-backed trading client)
// ---------------------------------------------------------------------------

func TestReal_GetPublicStock_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]client.PublicStockEntry{
			{Ticker: "AAPL", Quantity: 50, Sellers: []client.SellerRef{{RoutingNumber: 111, ID: "C-15"}}},
		})
	}))
	defer ts.Close()

	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, ts.URL, "")
	resp, err := cl.GetPublicStock(context.Background(), &interbankv1.GetPublicStockRequest{})
	if err != nil {
		t.Fatalf("GetPublicStock: %v", err)
	}
	if len(resp.GetEntries()) != 1 || resp.GetEntries()[0].GetTicker() != "AAPL" {
		t.Errorf("entries: %+v", resp.GetEntries())
	}
}

func TestReal_GetPublicStock_UpstreamError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, ts.URL, "")
	_, err := cl.GetPublicStock(context.Background(), &interbankv1.GetPublicStockRequest{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal on upstream failure, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetUserDisplay (real handler)
// ---------------------------------------------------------------------------

func TestReal_GetUserDisplay_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(client.UserDisplayDto{DisplayName: "Ana Anic"})
	}))
	defer ts.Close()

	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", ts.URL)
	resp, err := cl.GetUserDisplay(context.Background(), &interbankv1.GetUserDisplayRequest{
		RoutingNumber: 111, Id: "C-15",
	})
	if err != nil {
		t.Fatalf("GetUserDisplay: %v", err)
	}
	if resp.GetDisplayName() != "Ana Anic" {
		t.Errorf("displayName: %s", resp.GetDisplayName())
	}
	if resp.GetBankDisplayName() != "Banka 1" {
		t.Errorf("bankDisplayName: %s", resp.GetBankDisplayName())
	}
}

func TestReal_GetUserDisplay_EmptyDisplayName_FallsBackToFirstLast(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(client.UserDisplayDto{FirstName: "Marko", LastName: "Markovic"})
	}))
	defer ts.Close()

	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", ts.URL)
	resp, err := cl.GetUserDisplay(context.Background(), &interbankv1.GetUserDisplayRequest{
		RoutingNumber: 111, Id: "E-5",
	})
	if err != nil {
		t.Fatalf("GetUserDisplay: %v", err)
	}
	if resp.GetDisplayName() != "Marko Markovic" {
		t.Errorf("expected fallback first+last, got %s", resp.GetDisplayName())
	}
}

func TestReal_GetUserDisplay_WrongRouting_NotFound(t *testing.T) {
	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	_, err := cl.GetUserDisplay(context.Background(), &interbankv1.GetUserDisplayRequest{
		RoutingNumber: 999, Id: "C-1",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestReal_GetUserDisplay_ShortID_InvalidArg(t *testing.T) {
	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	_, err := cl.GetUserDisplay(context.Background(), &interbankv1.GetUserDisplayRequest{
		RoutingNumber: 111, Id: "C",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestReal_GetUserDisplay_BadPrefix_InvalidArg(t *testing.T) {
	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	_, err := cl.GetUserDisplay(context.Background(), &interbankv1.GetUserDisplayRequest{
		RoutingNumber: 111, Id: "X-1",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestReal_GetUserDisplay_NonNumericSuffix_InvalidArg(t *testing.T) {
	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	_, err := cl.GetUserDisplay(context.Background(), &interbankv1.GetUserDisplayRequest{
		RoutingNumber: 111, Id: "C-xx",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestReal_GetUserDisplay_UserNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", ts.URL)
	_, err := cl.GetUserDisplay(context.Background(), &interbankv1.GetUserDisplayRequest{
		RoutingNumber: 111, Id: "C-99",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// PostMessage early-return (nil idempotenceKey) — does not touch MessageStore.
// ---------------------------------------------------------------------------

func TestReal_PostMessage_NilIdempotenceKey(t *testing.T) {
	ns := newRealNegStore()
	cl := newRealServer(t, ns, &noopCoordinator{}, "", "")
	_, err := cl.PostMessage(context.Background(), &interbankv1.PostMessageRequest{
		Type: interbankv1.MessageType_MESSAGE_TYPE_NEW_TX,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for nil idempotenceKey, got %v", err)
	}
}

var _ = protocol.VoteYes // keep protocol import used across edits
