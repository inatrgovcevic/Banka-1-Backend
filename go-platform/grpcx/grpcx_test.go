package grpcx

import (
	"context"
	"log/slog"
	"testing"

	"banka1/go-platform/auth"
	"banka1/go-platform/httpx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func TestCorrelationUnaryInterceptorPreservesInboundID(t *testing.T) {
	interceptor := CorrelationUnaryInterceptor()
	md := metadata.Pairs(MetadataKey, "abc")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	got := ""
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/m"}, func(ctx context.Context, _ any) (any, error) {
		got = httpx.CorrelationFromContext(ctx)
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor failed: %v", err)
	}
	if got != "abc" {
		t.Fatalf("expected correlation 'abc', got %q", got)
	}
}

func TestCorrelationUnaryInterceptorGeneratesWhenMissing(t *testing.T) {
	interceptor := CorrelationUnaryInterceptor()
	got := ""
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/m"}, func(ctx context.Context, _ any) (any, error) {
		got = httpx.CorrelationFromContext(ctx)
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor failed: %v", err)
	}
	if got == "" {
		t.Fatal("expected generated correlation id")
	}
}

func TestRecoveryUnaryInterceptorConvertsPanic(t *testing.T) {
	interceptor := RecoveryUnaryInterceptor(discardLogger())
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/m"}, func(ctx context.Context, _ any) (any, error) {
		panic("kaboom")
	})
	if err == nil {
		t.Fatal("expected error from panic")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected codes.Internal, got %v", status.Code(err))
	}
}

func TestAuthUnaryInterceptorRejectsMissing(t *testing.T) {
	svc := auth.NewService(auth.Config{Secret: "abc-secret-abc-secret-abc-secret", Issuer: "banka1", IDClaim: "id", RolesClaim: "roles", PermissionsClaim: "permissions"})
	interceptor := AuthUnaryInterceptor(svc)
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/m"}, func(ctx context.Context, _ any) (any, error) {
		t.Fatal("handler should not run")
		return nil, nil
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestAuthUnaryInterceptorAcceptsValid(t *testing.T) {
	svc := auth.NewService(auth.Config{Secret: "abc-secret-abc-secret-abc-secret", Issuer: "banka1", IDClaim: "id", RolesClaim: "roles", PermissionsClaim: "permissions"})
	token, _ := svc.GenerateServiceToken("test", 60_000_000_000)
	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx := metadata.NewIncomingContext(context.Background(), md)
	interceptor := AuthUnaryInterceptor(svc)
	ran := false
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/m"}, func(ctx context.Context, _ any) (any, error) {
		ran = true
		if p, ok := auth.PrincipalFromContext(ctx); !ok || p.Role != "SERVICE" {
			t.Fatalf("principal missing or wrong: %+v", p)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !ran {
		t.Fatal("handler should run")
	}
}

func TestNewServerWiresInterceptors(t *testing.T) {
	srv := NewServer(discardLogger())
	if srv == nil {
		t.Fatal("server nil")
	}
}
