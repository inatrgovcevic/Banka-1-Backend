package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"banka1/market-service-go/internal/platform"

	gpauth "banka1/go-platform/auth"
	"github.com/golang-jwt/jwt/v5"
)

func makeTestService() *JWTService {
	return NewJWTService(platform.JWTConfig{
		Secret:           "test-secret-123456",
		Issuer:           "banka1",
		IDClaim:          "id",
		RolesClaim:       "roles",
		PermissionsClaim: "permissions",
	})
}

func signTestToken(role string, permissions []string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":         "banka1",
		"id":          7,
		"sub":         "tester",
		"exp":         time.Now().Add(time.Hour).Unix(),
		"roles":       role,
		"permissions": permissions,
	})
	raw, _ := token.SignedString([]byte("test-secret-123456"))
	return raw
}

// ---------------------------------------------------------------------------
// PrincipalFromContext
// ---------------------------------------------------------------------------

func TestPrincipalFromContext_WhenSet_ReturnsPrincipal(t *testing.T) {
	t.Parallel()
	svc := makeTestService()
	raw := signTestToken("ADMIN", nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()

	var principal Principal
	var ok bool

	handler := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok = PrincipalFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rec, req)

	if !ok {
		t.Fatal("expected PrincipalFromContext to return ok=true after middleware")
	}
	if principal.ID != 7 {
		t.Fatalf("principal.ID = %d, want 7", principal.ID)
	}
}

func TestPrincipalFromContext_EmptyContext_ReturnsFalse(t *testing.T) {
	t.Parallel()
	_, ok := PrincipalFromContext(context.Background())
	if ok {
		t.Fatal("expected PrincipalFromContext to return ok=false on empty context")
	}
}

// ---------------------------------------------------------------------------
// RequirePermissions
// ---------------------------------------------------------------------------

func TestRequirePermissions_HasPermission_CallsNext(t *testing.T) {
	t.Parallel()
	svc := makeTestService()
	raw := signTestToken("ADMIN", []string{"TRADE_UNLIMITED"})

	called := false
	handler := svc.Middleware(RequirePermissions("TRADE_UNLIMITED")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestRequirePermissions_MissingPermission_Returns403(t *testing.T) {
	t.Parallel()
	svc := makeTestService()
	raw := signTestToken("AGENT", []string{"BANKING_BASIC"})

	handler := svc.Middleware(RequirePermissions("TRADE_UNLIMITED")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// PrincipalFromContext with explicit context
// ---------------------------------------------------------------------------

func TestPrincipalFromContext_WithPrincipal_ReturnsIt(t *testing.T) {
	t.Parallel()
	ctx := gpauth.WithPrincipal(context.Background(), Principal{ID: 42, Role: "BASIC"})
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if p.ID != 42 {
		t.Fatalf("ID = %d, want 42", p.ID)
	}
}
