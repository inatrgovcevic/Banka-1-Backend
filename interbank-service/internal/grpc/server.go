// Package grpc wires the gRPC server for the interbank-service. It exposes
// the same InterbankProtocolService interface as the HTTP layer but over gRPC,
// enabling Go-to-Go synchronous RPC from other cohort services that import
// the proto module.
package grpc

import (
	"context"
	"log/slog"
	"net"

	gogrpc "google.golang.org/grpc"

	interbankv1 "github.com/raf-si-2025/banka-1-go/proto/banka1/interbank/v1"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/client"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/service"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"
)

// GrpcMessageStore is the persistence seam for the idempotency cache used by
// PostMessage. It is satisfied by *store.MessageStore in production and by
// in-memory fakes in tests, mirroring the HTTP layer's InboundMessageStore.
type GrpcMessageStore interface {
	Lookup(ctx context.Context, direction string, senderRouting int, key string) (*store.Message, error)
	Insert(ctx context.Context, m *store.Message) error
}

// Deps holds every dependency the gRPC server handlers need.
// It mirrors what main.go already wires up for the HTTP layer, so no new
// construction logic is needed — callers just fill this struct and call
// NewServer.
type Deps struct {
	MyRouting     int
	MyDisplayName string

	// Service layer.
	Executor    *service.Executor
	OtcService  *service.OtcNegotiationService
	Coordinator *service.Coordinator

	// Stores (needed for idempotency cache + direct lookups).
	MessageStore  GrpcMessageStore
	NegStore      *store.NegotiationStore
	ContractStore *store.ContractStore

	// Downstream clients (for GetPublicStock + GetUserDisplay).
	Trading *client.TradingClient
	User    *client.UserClient

	Log *slog.Logger
}

// Server implements interbankv1.InterbankProtocolServiceServer.
// It embeds the unimplemented stub so future proto additions compile cleanly.
type Server struct {
	interbankv1.UnimplementedInterbankProtocolServiceServer
	deps Deps
}

// NewServer constructs a gRPC Server with the provided dependency set.
func NewServer(deps Deps) *Server {
	if deps.Log == nil {
		deps.Log = slog.Default()
	}
	return &Server{deps: deps}
}

// Listen binds a TCP listener on addr (e.g. ":9091") and returns:
//   - a *grpc.Server already registered with the InterbankProtocolService,
//   - the net.Listener that was bound, and
//   - any bind error.
//
// Caller runs srv.Serve(lis) in a goroutine and calls srv.GracefulStop() on
// shutdown.
func (s *Server) Listen(addr string) (*gogrpc.Server, net.Listener, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	g := gogrpc.NewServer()
	interbankv1.RegisterInterbankProtocolServiceServer(g, s)
	return g, lis, nil
}
