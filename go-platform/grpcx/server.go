package grpcx

import (
	"log/slog"

	"google.golang.org/grpc"
)

// NewServer returns a grpc.Server pre-wired with correlation, log, recovery
// interceptors (in that order). Append more options as needed.
func NewServer(logger *slog.Logger, extra ...grpc.ServerOption) *grpc.Server {
	chain := grpc.ChainUnaryInterceptor(
		CorrelationUnaryInterceptor(),
		RecoveryUnaryInterceptor(logger),
		LoggingUnaryInterceptor(logger),
	)
	opts := append([]grpc.ServerOption{chain}, extra...)
	return grpc.NewServer(opts...)
}
