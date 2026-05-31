package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"banka1/market-service-go/internal/platform"
	"github.com/golang-jwt/jwt/v5"
)

func TestMiddlewareAndRoles(t *testing.T) {
	service := NewJWTService(platform.JWTConfig{
		Secret:           "development_market_service_secret_123456",
		Issuer:           "banka1",
		IDClaim:          "id",
		RolesClaim:       "roles",
		PermissionsClaim: "permissions",
	})
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":         "banka1",
		"id":          7,
		"sub":         "tester",
		"exp":         time.Now().Add(time.Hour).Unix(),
		"roles":       "ADMIN",
		"permissions": []string{"READ"},
	})
	raw, err := token.SignedString([]byte("development_market_service_secret_123456"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	called := false
	handler := service.Middleware(RequireRoles("SUPERVISOR")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		t.Fatal("expected wrapped handler to be called")
	}
}

