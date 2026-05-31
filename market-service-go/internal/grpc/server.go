package grpc

import (
	"context"
	"encoding/json"
	"log/slog"

	httpapi "banka1/market-service-go/internal/http"
	marketv1 "banka1/market-service-go/proto/market/v1"

	"banka1/go-platform/grpcx"
	"google.golang.org/grpc"
)

// NewServer returns a grpc.Server wired with the go-platform unary
// interceptors (correlation, recover, log) so every RPC carries the same
// observability fields as the HTTP side.
func NewServer(app *httpapi.App) *grpc.Server {
	return NewServerWithLogger(app, slog.Default())
}

// NewServerWithLogger lets callers supply the structured logger.
func NewServerWithLogger(app *httpapi.App, logger *slog.Logger) *grpc.Server {
	server := grpcx.NewServer(logger)
	marketv1.RegisterMarketServiceServer(server, &service{app: app})
	return server
}

type service struct {
	marketv1.UnimplementedMarketServiceServer
	app *httpapi.App
}

func (s *service) GetListing(ctx context.Context, req *marketv1.GetListingRequest) (*marketv1.GetListingResponse, error) {
	payload, err := s.app.MarketService.GetListingDetails(ctx, req.GetListingId(), "DAY")
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &marketv1.GetListingResponse{JsonPayload: string(raw)}, nil
}

func (s *service) GetQuote(ctx context.Context, req *marketv1.GetQuoteRequest) (*marketv1.GetQuoteResponse, error) {
	payload, _ := s.app.PriceFeed.GetSingle(ctx, req.GetTicker())
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &marketv1.GetQuoteResponse{JsonPayload: string(raw)}, nil
}

func (s *service) Convert(ctx context.Context, req *marketv1.ConvertRequest) (*marketv1.ConvertResponse, error) {
	payload, err := s.app.FXService.Calculate(ctx, req.GetFromCurrency(), req.GetToCurrency(), req.GetAmount(), req.GetDate(), req.GetIncludeCommission())
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &marketv1.ConvertResponse{JsonPayload: string(raw)}, nil
}
