package grpc

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonv1 "github.com/raf-si-2025/banka-1-go/proto/banka1/common/v1"
	interbankv1 "github.com/raf-si-2025/banka-1-go/proto/banka1/interbank/v1"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/service"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"
)

// ---------------------------------------------------------------------------
// PostMessage — POST /interbank equivalent
// ---------------------------------------------------------------------------

// PostMessage implements the 2PC NEW_TX / COMMIT_TX / ROLLBACK_TX dispatch.
// It mirrors the idempotency cache behaviour of the HTTP InboundHandler.
func (s *Server) PostMessage(ctx context.Context, req *interbankv1.PostMessageRequest) (*interbankv1.PostMessageResponse, error) {
	ikey := req.GetIdempotenceKey()
	if ikey == nil {
		return nil, status.Error(codes.InvalidArgument, "idempotenceKey required")
	}
	senderRouting := int(ikey.GetRoutingNumber())
	localKey := ikey.GetLocallyGeneratedKey()

	// Idempotency cache lookup — same table as HTTP path.
	cached, err := s.deps.MessageStore.Lookup(ctx, store.DirectionInbound, senderRouting, localKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "idempotency lookup: %v", err)
	}
	if cached != nil {
		// Replay cached result.
		switch req.GetType() {
		case interbankv1.MessageType_MESSAGE_TYPE_NEW_TX:
			if cached.ResponseBody != nil && cached.HttpStatus != nil && *cached.HttpStatus == 200 {
				var vote protocol.TransactionVote
				// Best-effort: if we can't decode, return a safe YES vote (cached = accepted).
				_ = unmarshalVoteFromJSON(*cached.ResponseBody, &vote)
				return &interbankv1.PostMessageResponse{
					HttpStatusCode: 200,
					Vote:           mapVoteToProto(vote),
				}, nil
			}
			return &interbankv1.PostMessageResponse{HttpStatusCode: 200}, nil
		default:
			return &interbankv1.PostMessageResponse{HttpStatusCode: 204}, nil
		}
	}

	switch req.GetType() {
	case interbankv1.MessageType_MESSAGE_TYPE_NEW_TX:
		return s.handleNewTx(ctx, req, senderRouting, localKey)

	case interbankv1.MessageType_MESSAGE_TYPE_COMMIT_TX:
		return s.handleCommitTx(ctx, req, senderRouting, localKey)

	case interbankv1.MessageType_MESSAGE_TYPE_ROLLBACK_TX:
		return s.handleRollbackTx(ctx, req, senderRouting, localKey)

	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown messageType: %v", req.GetType())
	}
}

func (s *Server) handleNewTx(ctx context.Context, req *interbankv1.PostMessageRequest, senderRouting int, localKey string) (*interbankv1.PostMessageResponse, error) {
	protoTx := req.GetNewTx()
	if protoTx == nil {
		return nil, status.Error(codes.InvalidArgument, "new_tx body required for MESSAGE_TYPE_NEW_TX")
	}
	if len(protoTx.GetPostings()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "postings must not be empty")
	}

	tx, err := mapProtoTx(protoTx)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid transaction payload: %v", err)
	}

	vote, err := s.deps.Executor.PrepareLocal(ctx, tx)
	if err != nil {
		s.deps.Log.ErrorContext(ctx, "grpc: PrepareLocal error", "err", err)
		// Store as error in cache before returning.
		_ = s.cacheMessage(ctx, store.DirectionInbound, senderRouting, localKey,
			"NEW_TX", `{}`, nil, intPtr(500))
		return nil, status.Errorf(codes.Internal, "prepare local: %v", err)
	}

	// Persist to idempotency cache.
	voteJSON := marshalVoteToJSON(vote)
	httpStatus := 200
	_ = s.cacheMessage(ctx, store.DirectionInbound, senderRouting, localKey,
		"NEW_TX", `{}`, &voteJSON, &httpStatus)

	return &interbankv1.PostMessageResponse{
		HttpStatusCode: 200,
		Vote:           mapVoteToProto(vote),
	}, nil
}

func (s *Server) handleCommitTx(ctx context.Context, req *interbankv1.PostMessageRequest, senderRouting int, localKey string) (*interbankv1.PostMessageResponse, error) {
	cb := req.GetCommitTx()
	if cb == nil {
		return nil, status.Error(codes.InvalidArgument, "commit_tx body required for MESSAGE_TYPE_COMMIT_TX")
	}
	txID := mapProtoForeignBankId(cb.GetTransactionId())

	if err := s.deps.Executor.CommitLocal(ctx, txID); err != nil {
		if errors.Is(err, service.ErrAlreadyTerminal) {
			_ = s.cacheMessage(ctx, store.DirectionInbound, senderRouting, localKey, "COMMIT_TX", `{}`, nil, intPtr(204))
			return &interbankv1.PostMessageResponse{HttpStatusCode: 204}, nil
		}
		return nil, status.Errorf(codes.Internal, "commit local: %v", err)
	}

	_ = s.cacheMessage(ctx, store.DirectionInbound, senderRouting, localKey, "COMMIT_TX", `{}`, nil, intPtr(204))
	return &interbankv1.PostMessageResponse{HttpStatusCode: 204}, nil
}

func (s *Server) handleRollbackTx(ctx context.Context, req *interbankv1.PostMessageRequest, senderRouting int, localKey string) (*interbankv1.PostMessageResponse, error) {
	rb := req.GetRollbackTx()
	if rb == nil {
		return nil, status.Error(codes.InvalidArgument, "rollback_tx body required for MESSAGE_TYPE_ROLLBACK_TX")
	}
	txID := mapProtoForeignBankId(rb.GetTransactionId())

	if err := s.deps.Executor.RollbackLocal(ctx, txID); err != nil {
		return nil, status.Errorf(codes.Internal, "rollback local: %v", err)
	}

	_ = s.cacheMessage(ctx, store.DirectionInbound, senderRouting, localKey, "ROLLBACK_TX", `{}`, nil, intPtr(204))
	return &interbankv1.PostMessageResponse{HttpStatusCode: 204}, nil
}

// ---------------------------------------------------------------------------
// CreateNegotiation — POST /negotiations equivalent
// ---------------------------------------------------------------------------

func (s *Server) CreateNegotiation(ctx context.Context, req *interbankv1.CreateNegotiationRequest) (*interbankv1.CreateNegotiationResponse, error) {
	offer, err := mapProtoOtcOfferFromCreate(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid offer: %v", err)
	}

	// For gRPC we use the buyer routing number as the "senderRouting" (auth is
	// caller responsibility at the transport layer or interceptor level).
	senderRouting := int(req.GetBuyerId().GetRoutingNumber())

	createdID, err := s.deps.OtcService.CreateNegotiation(ctx, offer, senderRouting)
	if err != nil {
		return nil, mapServiceError(err, fmt.Sprintf("create negotiation: %v", err))
	}

	return &interbankv1.CreateNegotiationResponse{
		Id: mapForeignBankIdToProto(createdID),
	}, nil
}

// ---------------------------------------------------------------------------
// PutCounter — PUT /negotiations/{rn}/{id} equivalent
// ---------------------------------------------------------------------------

func (s *Server) PutCounter(ctx context.Context, req *interbankv1.PutCounterRequest) (*interbankv1.PutCounterResponse, error) {
	rn := int(req.GetRoutingNumber())
	id := req.GetId()

	offer, err := mapProtoOtcOfferFromPut(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid offer: %v", err)
	}

	senderRouting := int(req.GetLastModifiedBy().GetRoutingNumber())

	if err := s.deps.OtcService.UpdateCounter(ctx, rn, id, offer, senderRouting); err != nil {
		return nil, mapServiceError(err, fmt.Sprintf("update counter: %v", err))
	}

	return &interbankv1.PutCounterResponse{}, nil
}

// ---------------------------------------------------------------------------
// GetNegotiation — GET /negotiations/{rn}/{id} equivalent
// ---------------------------------------------------------------------------

func (s *Server) GetNegotiation(ctx context.Context, req *interbankv1.GetNegotiationRequest) (*interbankv1.GetNegotiationResponse, error) {
	rn := int(req.GetRoutingNumber())
	id := req.GetId()

	dto, err := s.deps.OtcService.GetNegotiation(ctx, rn, id)
	if err != nil {
		return nil, mapServiceError(err, fmt.Sprintf("get negotiation: %v", err))
	}

	negotiationID := protocol.ForeignBankId{RoutingNumber: rn, Id: id}
	return &interbankv1.GetNegotiationResponse{
		Negotiation: mapNegotiationDtoToProto(negotiationID, dto),
	}, nil
}

// ---------------------------------------------------------------------------
// DeleteNegotiation — DELETE /negotiations/{rn}/{id} equivalent
// ---------------------------------------------------------------------------

func (s *Server) DeleteNegotiation(ctx context.Context, req *interbankv1.DeleteNegotiationRequest) (*interbankv1.DeleteNegotiationResponse, error) {
	rn := int(req.GetRoutingNumber())
	id := req.GetId()

	// Use our own routing as the senderRouting for gRPC calls from internal services.
	senderRouting := s.deps.MyRouting

	if err := s.deps.OtcService.Delete(ctx, rn, id, senderRouting); err != nil {
		return nil, mapServiceError(err, fmt.Sprintf("delete negotiation: %v", err))
	}

	return &interbankv1.DeleteNegotiationResponse{}, nil
}

// ---------------------------------------------------------------------------
// AcceptNegotiation — GET /negotiations/{rn}/{id}/accept equivalent
// ---------------------------------------------------------------------------

func (s *Server) AcceptNegotiation(ctx context.Context, req *interbankv1.AcceptNegotiationRequest) (*interbankv1.AcceptNegotiationResponse, error) {
	rn := int(req.GetRoutingNumber())
	id := req.GetId()

	senderRouting := s.deps.MyRouting

	if err := s.deps.OtcService.AcceptNegotiation(ctx, rn, id, senderRouting); err != nil {
		return nil, mapServiceError(err, fmt.Sprintf("accept negotiation: %v", err))
	}

	return &interbankv1.AcceptNegotiationResponse{}, nil
}

// ---------------------------------------------------------------------------
// GetPublicStock — GET /public-stock equivalent
// ---------------------------------------------------------------------------

func (s *Server) GetPublicStock(ctx context.Context, _ *interbankv1.GetPublicStockRequest) (*interbankv1.GetPublicStockResponse, error) {
	entries, err := s.deps.Trading.GetPublicStocks(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get public stocks: %v", err)
	}

	resp := &interbankv1.GetPublicStockResponse{}
	for _, e := range entries {
		entry := &interbankv1.PublicStockEntry{
			Ticker:   e.Ticker,
			Quantity: int32(e.Quantity),
		}
		for _, sel := range e.Sellers {
			entry.Sellers = append(entry.Sellers, &commonv1.ForeignBankId{
				RoutingNumber: int32(sel.RoutingNumber),
				Id:            sel.ID,
			})
		}
		resp.Entries = append(resp.Entries, entry)
	}
	return resp, nil
}

// ---------------------------------------------------------------------------
// GetUserDisplay — GET /interbank/user/{rn}/{id} equivalent
// ---------------------------------------------------------------------------

func (s *Server) GetUserDisplay(ctx context.Context, req *interbankv1.GetUserDisplayRequest) (*interbankv1.GetUserDisplayResponse, error) {
	rn := int(req.GetRoutingNumber())
	id := req.GetId()

	if rn != s.deps.MyRouting {
		return nil, status.Errorf(codes.NotFound, "user %d/%s not in this bank", rn, id)
	}

	if len(id) < 3 {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user id format: %s", id)
	}

	var userType string
	var numericPart string
	switch {
	case strings.HasPrefix(id, "C-"):
		userType = "CLIENT"
		numericPart = id[2:]
	case strings.HasPrefix(id, "E-"):
		userType = "EMPLOYEE"
		numericPart = id[2:]
	default:
		return nil, status.Errorf(codes.InvalidArgument, "user id must start with C- or E-: %s", id)
	}

	numericID, err := strconv.ParseInt(numericPart, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "user id numeric part invalid: %s", numericPart)
	}

	dto, err := s.deps.User.ResolveUser(ctx, userType, numericID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user %s not found: %v", id, err)
	}
	if dto == nil {
		return nil, status.Errorf(codes.NotFound, "user %s not found", id)
	}

	displayName := dto.DisplayName
	if displayName == "" {
		displayName = dto.FirstName + " " + dto.LastName
	}

	return &interbankv1.GetUserDisplayResponse{
		BankDisplayName: s.deps.MyDisplayName,
		DisplayName:     displayName,
	}, nil
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

// mapServiceError translates domain sentinel errors to gRPC status codes.
func mapServiceError(err error, msg string) error {
	switch {
	case errors.Is(err, service.ErrNegotiationNotFound):
		return status.Errorf(codes.NotFound, "%s", msg)
	case errors.Is(err, service.ErrNegotiationInvalid):
		return status.Errorf(codes.InvalidArgument, "%s", msg)
	case errors.Is(err, service.ErrNegotiationClosed):
		return status.Errorf(codes.FailedPrecondition, "%s", msg)
	case errors.Is(err, service.ErrTurnViolation):
		return status.Errorf(codes.Aborted, "%s", msg)
	case errors.Is(err, service.ErrSenderNotParty):
		return status.Errorf(codes.PermissionDenied, "%s", msg)
	case errors.Is(err, service.ErrInterbankProtocol):
		return status.Errorf(codes.Internal, "%s", msg)
	default:
		return status.Errorf(codes.Internal, "%s", msg)
	}
}

// cacheMessage writes a minimal idempotency cache entry for the gRPC path.
// Errors are best-effort logged; the message can still be re-processed on
// retry without harm (executor + service are idempotent).
func (s *Server) cacheMessage(ctx context.Context, direction string, senderRouting int, localKey, msgType, requestBody string, responseBody *string, httpStatus *int) error {
	m := &store.Message{
		Direction:           direction,
		SenderRoutingNumber: senderRouting,
		LocallyGeneratedKey: localKey,
		MessageType:         msgType,
		Status:              store.MessageStatusProcessed,
		RequestBody:         requestBody,
		ResponseBody:        responseBody,
		HttpStatus:          httpStatus,
	}
	if err := s.deps.MessageStore.Insert(ctx, m); err != nil {
		if !store.IsUniqueViolation(err) {
			s.deps.Log.WarnContext(ctx, "grpc: idempotency cache insert failed", "err", err)
		}
		return err
	}
	return nil
}

// intPtr returns a pointer to n — helper for inline literals.
func intPtr(n int) *int { return &n }

// marshalVoteToJSON produces a minimal JSON representation of the vote for
// storage in the idempotency cache response_body column.
func marshalVoteToJSON(v protocol.TransactionVote) string {
	if v.Vote == protocol.VoteYes {
		return `{"vote":"YES"}`
	}
	return `{"vote":"NO"}`
}

// unmarshalVoteFromJSON parses the cached vote JSON. Best-effort — leaves v
// unchanged on error.
func unmarshalVoteFromJSON(raw string, v *protocol.TransactionVote) error {
	if strings.Contains(raw, `"YES"`) {
		v.Vote = protocol.VoteYes
		return nil
	}
	if strings.Contains(raw, `"NO"`) {
		v.Vote = protocol.VoteNo
		return nil
	}
	return fmt.Errorf("unrecognised vote body: %s", raw)
}
