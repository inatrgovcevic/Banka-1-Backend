package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"banka1/trading-service-go/internal/platform"

	gpauth "banka1/go-platform/auth"
	"github.com/golang-jwt/jwt/v5"
)

func makeTestService() *JWTService {
	return NewJWTService(platform.JWTConfig{
		Secret:           "trading-test-secret-123456",
		Issuer:           "banka1",
		IDClaim:          "id",
		RolesClaim:       "roles",
		PermissionsClaim: "permissions",
	})
}

func signToken(role string, permissions []string) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":         "banka1",
		"id":          float64(7),
		"sub":         "user@bank.io",
		"exp":         time.Now().Add(time.Hour).Unix(),
		"roles":       role,
		"permissions": permissions,
	})
	raw, _ := tok.SignedString([]byte("trading-test-secret-123456"))
	return raw
}

func TestNewJWTService_NotNil(t *testing.T) {
	t.Parallel()
	svc := makeTestService()
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestPrincipalFromContext_EmptyContext_ReturnsFalse(t *testing.T) {
	t.Parallel()
	_, ok := PrincipalFromContext(context.Background())
	if ok {
		t.Fatal("empty context should return ok=false")
	}
}

func TestPrincipalFromContext_WithPrincipal_ReturnsIt(t *testing.T) {
	t.Parallel()
	ctx := gpauth.WithPrincipal(context.Background(), Principal{ID: 99, Role: "AGENT"})
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if p.ID != 99 {
		t.Fatalf("ID = %d, want 99", p.ID)
	}
}

func TestRequireRoles_MatchingRole_CallsNext(t *testing.T) {
	t.Parallel()
	svc := makeTestService()
	raw := signToken("SUPERVISOR", nil)

	called := false
	handler := svc.Middleware(RequireRoles("SUPERVISOR")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !called {
		t.Fatal("expected handler to be called")
	}
}

func TestRequirePermissions_HasPermission_CallsNext(t *testing.T) {
	t.Parallel()
	svc := makeTestService()
	raw := signToken("AGENT", []string{"SECURITIES_TRADE_UNLIMITED"})

	called := false
	handler := svc.Middleware(RequirePermissions("SECURITIES_TRADE_UNLIMITED")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !called {
		t.Fatal("expected handler to be called")
	}
}
