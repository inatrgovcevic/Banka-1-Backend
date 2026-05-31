// Package grpcx provides the gRPC server scaffold and interceptors every
// Go service uses. It mirrors httpx middleware semantics:
//   - propagate/generate X-Correlation-Id across gRPC metadata
//   - log every RPC with the same field set as http requests
//   - recover panics and convert them to status.Internal
//   - optionally enforce auth (auth.Service) via metadata "authorization"
package grpcx

import (
	"context"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"banka1/go-platform/auth"
	"banka1/go-platform/httpx"
	"banka1/go-platform/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// MetadataKey is the gRPC metadata header used for the correlation id.
const MetadataKey = "x-correlation-id"

// CorrelationUnaryInterceptor mirrors httpx.CorrelationMiddleware for gRPC.
// Reads x-correlation-id from incoming metadata or generates one; injects
// into ctx and adds an outgoing header.
func CorrelationUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		id := correlationFromMetadata(ctx)
		if id == "" {
			id = httpx.NewCorrelationID()
		}
		ctx = httpx.WithCorrelation(ctx, id)
		_ = grpc.SetHeader(ctx, metadata.Pairs(MetadataKey, id))
		return handler(ctx, req)
	}
}

// LoggingUnaryInterceptor logs every RPC similar to httpx.RequestLogMiddleware.
func LoggingUnaryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		st, _ := status.FromError(err)
		log.FromContext(ctx, logger).LogAttrs(ctx, slog.LevelInfo, "grpc request",
			slog.String("correlationId", httpx.CorrelationFromContext(ctx)),
			slog.String("method", info.FullMethod),
			slog.String("code", st.Code().String()),
			slog.Int64("durationMs", time.Since(start).Milliseconds()),
		)
		return resp, err
	}
}

// RecoveryUnaryInterceptor turns panics into status.Internal errors.
func RecoveryUnaryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.FromContext(ctx, logger).LogAttrs(ctx, slog.LevelError, "grpc panic recovered",
					slog.String("correlationId", httpx.CorrelationFromContext(ctx)),
					slog.String("method", info.FullMethod),
					slog.Any("panic", r),
					slog.String("stack", string(debug.Stack())),
				)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// AuthUnaryInterceptor parses the "authorization" metadata as a bearer token.
// On success the Principal is injected into ctx; on missing/invalid it
// returns codes.Unauthenticated.
func AuthUnaryInterceptor(svc *auth.Service) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		var raw string
		if values := md.Get("authorization"); len(values) > 0 {
			raw = values[0]
		}
		if raw == "" {
			return nil, status.Error(codes.Unauthenticated, "missing bearer token")
		}
		p, err := svc.ParseBearer(raw)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid bearer token")
		}
		return handler(auth.WithPrincipal(ctx, p), req)
	}
}

// OptionalAuthUnaryInterceptor attaches the principal when present, but does
// not reject the call when missing/invalid.
func OptionalAuthUnaryInterceptor(svc *auth.Service) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		if values := md.Get("authorization"); len(values) > 0 {
			if p, err := svc.ParseBearer(values[0]); err == nil {
				ctx = auth.WithPrincipal(ctx, p)
			}
		}
		return handler(ctx, req)
	}
}

func correlationFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for _, v := range md.Get(MetadataKey) {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
