package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"banka1/trading-service-go/internal/analytics"
	httpapi "banka1/trading-service-go/internal/http"
	tradingv1 "banka1/trading-service-go/proto/trading/v1"

	"banka1/go-platform/grpcx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// NewServer returns a grpc.Server wired with the go-platform unary interceptors
// (correlation, recover, log) and the TradingService registered. gRPC mirrors
// selected REST reads via the json_payload façade (same pattern as
// market-service-go); public traffic stays on HTTP/JSON through the gateway.
func NewServer(app *httpapi.App, logger *slog.Logger) *grpc.Server {
	server := grpcx.NewServer(logger)
	tradingv1.RegisterTradingServiceServer(server, &service{app: app})
	reflection.Register(server)
	return server
}

type service struct {
	tradingv1.UnimplementedTradingServiceServer
	app *httpapi.App
}

// Ping returns {"status":"UP"} so the channel is exercisable without data.
func (s *service) Ping(_ context.Context, _ *tradingv1.PingRequest) (*tradingv1.PingResponse, error) {
	return &tradingv1.PingResponse{JsonPayload: `{"status":"UP"}`}, nil
}

// GetLatestAnalyticsRun returns the same payload as GET /analytics/runs/latest.
func (s *service) GetLatestAnalyticsRun(ctx context.Context, _ *tradingv1.GetLatestAnalyticsRunRequest) (*tradingv1.GetLatestAnalyticsRunResponse, error) {
	resp, err := s.app.Analytics.LatestRun(ctx)
	if err != nil {
		if errors.Is(err, analytics.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "no completed analytics run exists")
		}
		return nil, status.Error(codes.Internal, "internal error")
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		return nil, status.Error(codes.Internal, "encode error")
	}
	return &tradingv1.GetLatestAnalyticsRunResponse{JsonPayload: string(raw)}, nil
}
