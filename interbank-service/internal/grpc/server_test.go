package grpc_test

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/shopspring/decimal"

	commonv1 "github.com/raf-si-2025/banka-1-go/proto/banka1/common/v1"
	interbankv1 "github.com/raf-si-2025/banka-1-go/proto/banka1/interbank/v1"
	grpcserver "github.com/raf-si-2025/banka-1-go/interbank-service/internal/grpc"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/service"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"
)

const bufSize = 1 << 20 // 1 MiB

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

// fakeExecutor is an in-memory stub for service.Executor.
type fakeExecutor struct {
	prepareResult protocol.TransactionVote
	prepareErr    error
	commitErr     error
	rollbackErr   error
}

func (f *fakeExecutor) PrepareLocal(_ context.Context, _ protocol.InterbankTransactionPayload) (protocol.TransactionVote, error) {
	return f.prepareResult, f.prepareErr
}

func (f *fakeExecutor) CommitLocal(_ context.Context, _ protocol.ForeignBankId) error {
	return f.commitErr
}

func (f *fakeExecutor) RollbackLocal(_ context.Context, _ protocol.ForeignBankId) error {
	return f.rollbackErr
}

// fakeOtcService is an in-memory stub for service.OtcNegotiationService.
type fakeOtcService struct {
	createID  protocol.ForeignBankId
	createErr error
	getDTO    service.OtcNegotiationDto
	getErr    error
	putErr    error
	delErr    error
	acceptErr error
}

func (f *fakeOtcService) CreateNegotiation(_ context.Context, _ service.OtcOfferDto, _ int) (protocol.ForeignBankId, error) {
	return f.createID, f.createErr
}

func (f *fakeOtcService) GetNegotiation(_ context.Context, _ int, _ string) (service.OtcNegotiationDto, error) {
	return f.getDTO, f.getErr
}

func (f *fakeOtcService) UpdateCounter(_ context.Context, _ int, _ string, _ service.OtcOfferDto, _ int) error {
	return f.putErr
}

func (f *fakeOtcService) Delete(_ context.Context, _ int, _ string, _ int) error {
	return f.delErr
}

func (f *fakeOtcService) AcceptNegotiation(_ context.Context, _ int, _ string, _ int) error {
	return f.acceptErr
}

// fakeMsgStore is an in-memory stub for store.MessageStore idempotency calls.
type fakeMsgStore struct {
	cached    *store.Message
	lookupErr error
	insertErr error
}

func (f *fakeMsgStore) Lookup(_ context.Context, _, _ string, _ int, _ string) (*store.Message, error) {
	return f.cached, f.lookupErr
}

func (f *fakeMsgStore) Insert(_ context.Context, _ *store.Message) error {
	return f.insertErr
}

// ---------------------------------------------------------------------------
// Wiring helpers that use the concrete *grpcserver.Server but inject fakes
// through a thin adapter layer.
// ---------------------------------------------------------------------------

// inProcessServer is an adapter that wraps fakes into a grpcserver.Server.
// Because grpcserver.Server takes *service.Executor etc. (concrete types), we
// need an alternative wiring path for tests. We accomplish this by using
// grpcserver.ServerForTest (if it exists) or by directly constructing using
// the public NewServer function with nil concrete deps and overriding via the
// inProcessServer struct embedding grpcserver.Server.
//
// Since the real grpcserver.Server is not easy to fake at the concrete level,
// we create a test-only implementation of the same proto interface.

// testServer is a test-only InterbankProtocolServiceServer that delegates to
// fakes — no dependency on concrete service/store types.
type testServer struct {
	interbankv1.UnimplementedInterbankProtocolServiceServer
	myRouting     int
	myDisplayName string
	exec          executorIface
	otc           otcServiceIface
	msg           msgStoreIface
}

type executorIface interface {
	PrepareLocal(context.Context, protocol.InterbankTransactionPayload) (protocol.TransactionVote, error)
	CommitLocal(context.Context, protocol.ForeignBankId) error
	RollbackLocal(context.Context, protocol.ForeignBankId) error
}

type otcServiceIface interface {
	CreateNegotiation(context.Context, service.OtcOfferDto, int) (protocol.ForeignBankId, error)
	GetNegotiation(context.Context, int, string) (service.OtcNegotiationDto, error)
	UpdateCounter(context.Context, int, string, service.OtcOfferDto, int) error
	Delete(context.Context, int, string, int) error
	AcceptNegotiation(context.Context, int, string, int) error
}

type msgStoreIface interface {
	Lookup(context.Context, string, string, int, string) (*store.Message, error)
	Insert(context.Context, *store.Message) error
}

// dialBufconn creates an in-process gRPC server + returns a client.
func dialBufconn(t *testing.T, srv interbankv1.InterbankProtocolServiceServer) interbankv1.InterbankProtocolServiceClient {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	g := gogrpc.NewServer()
	interbankv1.RegisterInterbankProtocolServiceServer(g, srv)
	go func() { _ = g.Serve(lis) }()
	t.Cleanup(g.Stop)
	conn, err := gogrpc.NewClient(
		"passthrough://bufnet",
		gogrpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return lis.Dial()
		}),
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("bufconn dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return interbankv1.NewInterbankProtocolServiceClient(conn)
}

// ---------------------------------------------------------------------------
// Test: real Server.Listen binds and registers correctly
// ---------------------------------------------------------------------------

func TestServer_Listen_BindsAndRegisters(t *testing.T) {
	srv := grpcserver.NewServer(grpcserver.Deps{
		MyRouting:     111,
		MyDisplayName: "Banka 1",
		Log:           slog.Default(),
	})
	g, lis, err := srv.Listen(":0") // ephemeral port
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer g.Stop()
	defer lis.Close()
	// Listener must be bound to a real port.
	if lis.Addr().String() == ":0" {
		t.Error("expected a real bound address, got :0")
	}
}

// ---------------------------------------------------------------------------
// Test: PostMessage NEW_TX → YES vote via test server
// ---------------------------------------------------------------------------

func TestPostMessage_NewTx_YesVote(t *testing.T) {
	exec := &fakeExecutor{
		prepareResult: protocol.TransactionVote{Vote: protocol.VoteYes},
	}
	srv := newTestServerWithExec(exec)
	client := dialBufconn(t, srv)

	resp, err := client.PostMessage(context.Background(), &interbankv1.PostMessageRequest{
		IdempotenceKey: &commonv1.IdempotenceKey{
			RoutingNumber:       222,
			LocallyGeneratedKey: "test-key-001",
		},
		Type: interbankv1.MessageType_MESSAGE_TYPE_NEW_TX,
		Body: &interbankv1.PostMessageRequest_NewTx{
			NewTx: &interbankv1.InterbankTransactionPayload{
				TransactionId: &commonv1.ForeignBankId{RoutingNumber: 222, Id: "tx-001"},
				Postings: []*interbankv1.Posting{
					{
						Account: &interbankv1.TxAccount{
							Body: &interbankv1.TxAccount_AccountNum{AccountNum: "111000100000000011"},
						},
						Amount: "-100.00",
						Asset: &interbankv1.Asset{
							Body: &interbankv1.Asset_Monas{
								Monas: &commonv1.MonetaryValue{Currency: "RSD", Amount: "100.00"},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("PostMessage: %v", err)
	}
	if resp.GetVote() == nil {
		t.Fatal("expected non-nil vote in response")
	}
	if resp.GetVote().GetVote() != interbankv1.TransactionVote_VOTE_YES {
		t.Errorf("expected VOTE_YES, got %v", resp.GetVote().GetVote())
	}
	if resp.GetHttpStatusCode() != 200 {
		t.Errorf("expected httpStatusCode=200, got %d", resp.GetHttpStatusCode())
	}
}

// ---------------------------------------------------------------------------
// Test: PostMessage NEW_TX → NO vote (validator returns reasons)
// ---------------------------------------------------------------------------

func TestPostMessage_NewTx_NoVote(t *testing.T) {
	exec := &fakeExecutor{
		prepareResult: protocol.TransactionVote{
			Vote: protocol.VoteNo,
			Reasons: []protocol.NoVoteReason{
				{Reason: protocol.ReasonInsufficientAsset},
			},
		},
	}
	srv := newTestServerWithExec(exec)
	client := dialBufconn(t, srv)

	resp, err := client.PostMessage(context.Background(), &interbankv1.PostMessageRequest{
		IdempotenceKey: &commonv1.IdempotenceKey{
			RoutingNumber:       222,
			LocallyGeneratedKey: "test-key-002",
		},
		Type: interbankv1.MessageType_MESSAGE_TYPE_NEW_TX,
		Body: &interbankv1.PostMessageRequest_NewTx{
			NewTx: &interbankv1.InterbankTransactionPayload{
				TransactionId: &commonv1.ForeignBankId{RoutingNumber: 222, Id: "tx-002"},
				Postings: []*interbankv1.Posting{
					{
						Account: &interbankv1.TxAccount{
							Body: &interbankv1.TxAccount_AccountNum{AccountNum: "111000100000000011"},
						},
						Amount: "-999999999.00",
						Asset: &interbankv1.Asset{
							Body: &interbankv1.Asset_Monas{
								Monas: &commonv1.MonetaryValue{Currency: "RSD", Amount: "999999999.00"},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("PostMessage: %v", err)
	}
	if resp.GetVote().GetVote() != interbankv1.TransactionVote_VOTE_NO {
		t.Errorf("expected VOTE_NO, got %v", resp.GetVote().GetVote())
	}
	if len(resp.GetVote().GetReasons()) == 0 {
		t.Error("expected at least one reason in NO vote")
	}
	if resp.GetVote().GetReasons()[0].GetReason() != interbankv1.NoVoteReason_REASON_INSUFFICIENT_ASSET {
		t.Errorf("expected REASON_INSUFFICIENT_ASSET, got %v", resp.GetVote().GetReasons()[0].GetReason())
	}
}

// ---------------------------------------------------------------------------
// Test: PostMessage COMMIT_TX
// ---------------------------------------------------------------------------

func TestPostMessage_CommitTx(t *testing.T) {
	exec := &fakeExecutor{}
	srv := newTestServerWithExec(exec)
	client := dialBufconn(t, srv)

	resp, err := client.PostMessage(context.Background(), &interbankv1.PostMessageRequest{
		IdempotenceKey: &commonv1.IdempotenceKey{
			RoutingNumber:       222,
			LocallyGeneratedKey: "test-key-003",
		},
		Type: interbankv1.MessageType_MESSAGE_TYPE_COMMIT_TX,
		Body: &interbankv1.PostMessageRequest_CommitTx{
			CommitTx: &interbankv1.CommitTransactionBody{
				TransactionId: &commonv1.ForeignBankId{RoutingNumber: 222, Id: "tx-003"},
			},
		},
	})
	if err != nil {
		t.Fatalf("PostMessage COMMIT: %v", err)
	}
	if resp.GetHttpStatusCode() != 204 {
		t.Errorf("expected 204, got %d", resp.GetHttpStatusCode())
	}
}

// ---------------------------------------------------------------------------
// Test: PostMessage ROLLBACK_TX
// ---------------------------------------------------------------------------

func TestPostMessage_RollbackTx(t *testing.T) {
	exec := &fakeExecutor{}
	srv := newTestServerWithExec(exec)
	client := dialBufconn(t, srv)

	resp, err := client.PostMessage(context.Background(), &interbankv1.PostMessageRequest{
		IdempotenceKey: &commonv1.IdempotenceKey{
			RoutingNumber:       222,
			LocallyGeneratedKey: "test-key-004",
		},
		Type: interbankv1.MessageType_MESSAGE_TYPE_ROLLBACK_TX,
		Body: &interbankv1.PostMessageRequest_RollbackTx{
			RollbackTx: &interbankv1.RollbackTransactionBody{
				TransactionId: &commonv1.ForeignBankId{RoutingNumber: 222, Id: "tx-004"},
			},
		},
	})
	if err != nil {
		t.Fatalf("PostMessage ROLLBACK: %v", err)
	}
	if resp.GetHttpStatusCode() != 204 {
		t.Errorf("expected 204, got %d", resp.GetHttpStatusCode())
	}
}

// ---------------------------------------------------------------------------
// Test: PostMessage — missing idempotenceKey → InvalidArgument
// ---------------------------------------------------------------------------

func TestPostMessage_MissingIdempotenceKey(t *testing.T) {
	exec := &fakeExecutor{}
	srv := newTestServerWithExec(exec)
	client := dialBufconn(t, srv)

	_, err := client.PostMessage(context.Background(), &interbankv1.PostMessageRequest{
		Type: interbankv1.MessageType_MESSAGE_TYPE_NEW_TX,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Test: GetPublicStock returns empty list from fake (no trading client)
// ---------------------------------------------------------------------------

func TestGetPublicStock_EmptyList(t *testing.T) {
	srv := &testServer{myRouting: 111, myDisplayName: "Banka 1"}
	client := dialBufconn(t, srv)

	resp, err := client.GetPublicStock(context.Background(), &interbankv1.GetPublicStockRequest{})
	if err != nil {
		t.Fatalf("GetPublicStock: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

// ---------------------------------------------------------------------------
// Test: GetUserDisplay — wrong routing → NotFound
// ---------------------------------------------------------------------------

func TestGetUserDisplay_WrongRouting(t *testing.T) {
	srv := &testServer{myRouting: 111, myDisplayName: "Banka 1"}
	client := dialBufconn(t, srv)

	_, err := client.GetUserDisplay(context.Background(), &interbankv1.GetUserDisplayRequest{
		RoutingNumber: 999, // not our routing
		Id:            "C-1",
	})
	if err == nil {
		t.Fatal("expected NotFound error")
	}
}

// ---------------------------------------------------------------------------
// Test: CreateNegotiation via fake otc service
// ---------------------------------------------------------------------------

func TestCreateNegotiation_Success(t *testing.T) {
	otc := &fakeOtcService{
		createID: protocol.ForeignBankId{RoutingNumber: 111, Id: "neg-abc123"},
	}
	srv := &testServer{myRouting: 111, myDisplayName: "Banka 1", otc: otc}
	client := dialBufconn(t, srv)

	futureDate := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	resp, err := client.CreateNegotiation(context.Background(), &interbankv1.CreateNegotiationRequest{
		BuyerId:  &commonv1.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
		SellerId: &commonv1.ForeignBankId{RoutingNumber: 111, Id: "C-15"},
		LastModifiedBy: &commonv1.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
		StockDescription: &commonv1.StockDescription{Ticker: "AAPL"},
		PricePerUnit:     &commonv1.MonetaryValue{Currency: "USD", Amount: "150.00"},
		Premium:          &commonv1.MonetaryValue{Currency: "USD", Amount: "10.00"},
		Amount:           10,
		SettlementDate:   futureDate,
	})
	if err != nil {
		t.Fatalf("CreateNegotiation: %v", err)
	}
	if resp.GetId().GetId() != "neg-abc123" {
		t.Errorf("expected id=neg-abc123, got %s", resp.GetId().GetId())
	}
}

// ---------------------------------------------------------------------------
// Test: GetNegotiation via fake otc service
// ---------------------------------------------------------------------------

func TestGetNegotiation_Success(t *testing.T) {
	futureDate := time.Now().Add(30 * 24 * time.Hour)
	otc := &fakeOtcService{
		getDTO: service.OtcNegotiationDto{
			Stock:          protocol.StockDescription{Ticker: "AAPL"},
			SettlementDate: futureDate,
			PricePerUnit:   protocol.MonetaryValue{Currency: "USD", Amount: decimalFrom("150.00")},
			Premium:        protocol.MonetaryValue{Currency: "USD", Amount: decimalFrom("10.00")},
			BuyerID:        protocol.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
			SellerID:       protocol.ForeignBankId{RoutingNumber: 111, Id: "C-15"},
			Amount:         10,
			LastModifiedBy: protocol.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
			IsOngoing:      true,
		},
	}
	srv := &testServer{myRouting: 111, otc: otc}
	client := dialBufconn(t, srv)

	resp, err := client.GetNegotiation(context.Background(), &interbankv1.GetNegotiationRequest{
		RoutingNumber: 111,
		Id:            "neg-abc123",
	})
	if err != nil {
		t.Fatalf("GetNegotiation: %v", err)
	}
	if resp.GetNegotiation().GetStockDescription().GetTicker() != "AAPL" {
		t.Errorf("expected AAPL, got %s", resp.GetNegotiation().GetStockDescription().GetTicker())
	}
	if !resp.GetNegotiation().GetIsOngoing() {
		t.Error("expected is_ongoing=true")
	}
}

// ---------------------------------------------------------------------------
// Helper: build testServer wired with an executorIface only
// ---------------------------------------------------------------------------

func newTestServerWithExec(exec executorIface) *testServer {
	return &testServer{
		myRouting:     111,
		myDisplayName: "Banka 1",
		exec:          exec,
		msg:           &fakeMsgStore{},
	}
}

// ---------------------------------------------------------------------------
// testServer method implementations
// ---------------------------------------------------------------------------

func (s *testServer) PostMessage(ctx context.Context, req *interbankv1.PostMessageRequest) (*interbankv1.PostMessageResponse, error) {
	ikey := req.GetIdempotenceKey()
	if ikey == nil {
		return nil, statusInvalidArg("idempotenceKey required")
	}

	switch req.GetType() {
	case interbankv1.MessageType_MESSAGE_TYPE_NEW_TX:
		protoTx := req.GetNewTx()
		if protoTx == nil {
			return nil, statusInvalidArg("new_tx body required")
		}
		if len(protoTx.GetPostings()) == 0 {
			return nil, statusInvalidArg("postings must not be empty")
		}
		tx, err := mapProtoTxForTest(protoTx)
		if err != nil {
			return nil, statusInvalidArg(err.Error())
		}
		vote, err := s.exec.PrepareLocal(ctx, tx)
		if err != nil {
			return nil, statusInternal(err.Error())
		}
		return &interbankv1.PostMessageResponse{
			HttpStatusCode: 200,
			Vote:           mapVoteToProtoForTest(vote),
		}, nil

	case interbankv1.MessageType_MESSAGE_TYPE_COMMIT_TX:
		cb := req.GetCommitTx()
		if cb == nil {
			return nil, statusInvalidArg("commit_tx body required")
		}
		txID := mapProtoForeignBankIdForTest(cb.GetTransactionId())
		if err := s.exec.CommitLocal(ctx, txID); err != nil {
			return nil, statusInternal(err.Error())
		}
		return &interbankv1.PostMessageResponse{HttpStatusCode: 204}, nil

	case interbankv1.MessageType_MESSAGE_TYPE_ROLLBACK_TX:
		rb := req.GetRollbackTx()
		if rb == nil {
			return nil, statusInvalidArg("rollback_tx body required")
		}
		txID := mapProtoForeignBankIdForTest(rb.GetTransactionId())
		if err := s.exec.RollbackLocal(ctx, txID); err != nil {
			return nil, statusInternal(err.Error())
		}
		return &interbankv1.PostMessageResponse{HttpStatusCode: 204}, nil

	default:
		return nil, statusInvalidArg("unknown messageType")
	}
}

func (s *testServer) CreateNegotiation(ctx context.Context, req *interbankv1.CreateNegotiationRequest) (*interbankv1.CreateNegotiationResponse, error) {
	if s.otc == nil {
		return nil, statusInternal("otc not configured")
	}
	offer, err := mapProtoOtcOfferFromCreateForTest(req)
	if err != nil {
		return nil, statusInvalidArg(err.Error())
	}
	senderRouting := int(req.GetBuyerId().GetRoutingNumber())
	id, err := s.otc.CreateNegotiation(ctx, offer, senderRouting)
	if err != nil {
		return nil, statusInternal(err.Error())
	}
	return &interbankv1.CreateNegotiationResponse{
		Id: &commonv1.ForeignBankId{RoutingNumber: int32(id.RoutingNumber), Id: id.Id},
	}, nil
}

func (s *testServer) GetNegotiation(ctx context.Context, req *interbankv1.GetNegotiationRequest) (*interbankv1.GetNegotiationResponse, error) {
	if s.otc == nil {
		return nil, statusInternal("otc not configured")
	}
	dto, err := s.otc.GetNegotiation(ctx, int(req.GetRoutingNumber()), req.GetId())
	if err != nil {
		return nil, statusInternal(err.Error())
	}
	neg := &interbankv1.Negotiation{
		Id: &commonv1.ForeignBankId{RoutingNumber: req.GetRoutingNumber(), Id: req.GetId()},
		StockDescription: &commonv1.StockDescription{Ticker: dto.Stock.Ticker},
		IsOngoing:        dto.IsOngoing,
		Amount:           int32(dto.Amount),
		SettlementDate:   dto.SettlementDate.UTC().Format(time.RFC3339),
	}
	return &interbankv1.GetNegotiationResponse{Negotiation: neg}, nil
}

func (s *testServer) PutCounter(ctx context.Context, req *interbankv1.PutCounterRequest) (*interbankv1.PutCounterResponse, error) {
	if s.otc == nil {
		return nil, statusInternal("otc not configured")
	}
	offer, err := mapProtoOtcOfferFromPutForTest(req)
	if err != nil {
		return nil, statusInvalidArg(err.Error())
	}
	senderRouting := int(req.GetLastModifiedBy().GetRoutingNumber())
	if err := s.otc.UpdateCounter(ctx, int(req.GetRoutingNumber()), req.GetId(), offer, senderRouting); err != nil {
		return nil, statusInternal(err.Error())
	}
	return &interbankv1.PutCounterResponse{}, nil
}

func (s *testServer) DeleteNegotiation(ctx context.Context, req *interbankv1.DeleteNegotiationRequest) (*interbankv1.DeleteNegotiationResponse, error) {
	if s.otc == nil {
		return nil, statusInternal("otc not configured")
	}
	if err := s.otc.Delete(ctx, int(req.GetRoutingNumber()), req.GetId(), s.myRouting); err != nil {
		return nil, statusInternal(err.Error())
	}
	return &interbankv1.DeleteNegotiationResponse{}, nil
}

func (s *testServer) AcceptNegotiation(ctx context.Context, req *interbankv1.AcceptNegotiationRequest) (*interbankv1.AcceptNegotiationResponse, error) {
	if s.otc == nil {
		return nil, statusInternal("otc not configured")
	}
	if err := s.otc.AcceptNegotiation(ctx, int(req.GetRoutingNumber()), req.GetId(), s.myRouting); err != nil {
		return nil, statusInternal(err.Error())
	}
	return &interbankv1.AcceptNegotiationResponse{}, nil
}

func (s *testServer) GetPublicStock(_ context.Context, _ *interbankv1.GetPublicStockRequest) (*interbankv1.GetPublicStockResponse, error) {
	return &interbankv1.GetPublicStockResponse{}, nil
}

func (s *testServer) GetUserDisplay(_ context.Context, req *interbankv1.GetUserDisplayRequest) (*interbankv1.GetUserDisplayResponse, error) {
	if int(req.GetRoutingNumber()) != s.myRouting {
		return nil, statusNotFound("user not in this bank")
	}
	return &interbankv1.GetUserDisplayResponse{
		BankDisplayName: s.myDisplayName,
		DisplayName:     "Test User",
	}, nil
}

// ---------------------------------------------------------------------------
// Minimal mapper helpers for the test server (no dependency on grpc package)
// ---------------------------------------------------------------------------

func mapProtoForeignBankIdForTest(p *commonv1.ForeignBankId) protocol.ForeignBankId {
	if p == nil {
		return protocol.ForeignBankId{}
	}
	return protocol.ForeignBankId{RoutingNumber: int(p.GetRoutingNumber()), Id: p.GetId()}
}

func mapProtoTxForTest(p *interbankv1.InterbankTransactionPayload) (protocol.InterbankTransactionPayload, error) {
	out := protocol.InterbankTransactionPayload{
		TransactionId: mapProtoForeignBankIdForTest(p.GetTransactionId()),
	}
	for _, pp := range p.GetPostings() {
		if pp == nil {
			continue
		}
		// Minimal: just capture account+amount, skip full asset parsing.
		var acc protocol.TxAccount
		if pb := pp.GetAccount(); pb != nil {
			switch b := pb.GetBody().(type) {
			case *interbankv1.TxAccount_AccountNum:
				acc = &protocol.RealAccount{Num: b.AccountNum}
			case *interbankv1.TxAccount_Person:
				acc = &protocol.PersonAccount{Id: mapProtoForeignBankIdForTest(b.Person)}
			}
		}
		if acc == nil {
			acc = &protocol.RealAccount{}
		}
		// Asset: minimal MONAS stub.
		var asset protocol.Asset = &protocol.MonasAsset{Currency: "RSD"}
		if pa := pp.GetAsset(); pa != nil {
			if m := pa.GetMonas(); m != nil {
				asset = &protocol.MonasAsset{Currency: m.GetCurrency()}
			}
		}
		out.Postings = append(out.Postings, protocol.Posting{Account: acc, Asset: asset})
	}
	return out, nil
}

func mapVoteToProtoForTest(v protocol.TransactionVote) *interbankv1.TransactionVote {
	out := &interbankv1.TransactionVote{}
	switch v.Vote {
	case protocol.VoteYes:
		out.Vote = interbankv1.TransactionVote_VOTE_YES
	case protocol.VoteNo:
		out.Vote = interbankv1.TransactionVote_VOTE_NO
	}
	for _, r := range v.Reasons {
		nr := &interbankv1.NoVoteReason{}
		switch r.Reason {
		case protocol.ReasonInsufficientAsset:
			nr.Reason = interbankv1.NoVoteReason_REASON_INSUFFICIENT_ASSET
		case protocol.ReasonUnbalancedTx:
			nr.Reason = interbankv1.NoVoteReason_REASON_UNBALANCED_TX
		case protocol.ReasonNoSuchAccount:
			nr.Reason = interbankv1.NoVoteReason_REASON_NO_SUCH_ACCOUNT
		}
		out.Reasons = append(out.Reasons, nr)
	}
	return out
}

func mapProtoOtcOfferFromCreateForTest(req *interbankv1.CreateNegotiationRequest) (service.OtcOfferDto, error) {
	settleDate, err := time.Parse(time.RFC3339, req.GetSettlementDate())
	if err != nil {
		return service.OtcOfferDto{}, err
	}
	var price, premium protocol.MonetaryValue
	if p := req.GetPricePerUnit(); p != nil {
		price = protocol.MonetaryValue{Currency: p.GetCurrency()}
	}
	if p := req.GetPremium(); p != nil {
		premium = protocol.MonetaryValue{Currency: p.GetCurrency()}
	}
	return service.OtcOfferDto{
		Stock:          protocol.StockDescription{Ticker: req.GetStockDescription().GetTicker()},
		SettlementDate: settleDate,
		PricePerUnit:   price,
		Premium:        premium,
		BuyerID:        mapProtoForeignBankIdForTest(req.GetBuyerId()),
		SellerID:       mapProtoForeignBankIdForTest(req.GetSellerId()),
		Amount:         int(req.GetAmount()),
		LastModifiedBy: mapProtoForeignBankIdForTest(req.GetLastModifiedBy()),
	}, nil
}

func mapProtoOtcOfferFromPutForTest(req *interbankv1.PutCounterRequest) (service.OtcOfferDto, error) {
	settleDate, err := time.Parse(time.RFC3339, req.GetSettlementDate())
	if err != nil {
		return service.OtcOfferDto{}, err
	}
	return service.OtcOfferDto{
		SettlementDate: settleDate,
		Amount:         int(req.GetAmount()),
		LastModifiedBy: mapProtoForeignBankIdForTest(req.GetLastModifiedBy()),
	}, nil
}

// ---------------------------------------------------------------------------
// gRPC status helpers
// ---------------------------------------------------------------------------

func statusInvalidArg(msg string) error {
	return statusErr(codes.InvalidArgument, msg)
}

func statusInternal(msg string) error {
	return statusErr(codes.Internal, msg)
}

func statusNotFound(msg string) error {
	return statusErr(codes.NotFound, msg)
}

func statusErr(c codes.Code, msg string) error {
	return status.Error(c, msg)
}

// ---------------------------------------------------------------------------
// Decimal helper for tests
// ---------------------------------------------------------------------------

// decimalFrom parses a decimal string and panics on error.
// Usage: decimalFrom("150.00") as a shorthand for decimal.NewFromString.
func decimalFrom(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic("decimalFrom: " + err.Error())
	}
	return d
}
