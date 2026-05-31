package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"banka1/banking-core-service-go/internal/config"
)

func TestPrincipalFromTokenVerifiesHS256Signature(t *testing.T) {
	handler := &Handler{cfg: testAuthConfig()}
	token := signedTestJWT(t, handler.cfg.JWTSecret, map[string]any{
		"id":    42,
		"roles": "SERVICE",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})

	principal, ok := handler.principalFromToken(token)
	if !ok {
		t.Fatal("expected signed token to be accepted")
	}
	if principal.ID != 42 {
		t.Fatalf("principal id = %d, want 42", principal.ID)
	}
	if !hasServiceRole(principal.Roles) {
		t.Fatalf("principal roles = %v, want SERVICE role", principal.Roles)
	}
}

func TestPrincipalFromTokenRejectsTamperedSignature(t *testing.T) {
	handler := &Handler{cfg: testAuthConfig()}
	token := signedTestJWT(t, handler.cfg.JWTSecret, map[string]any{
		"id":    42,
		"roles": "SERVICE",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("test token has %d parts, want 3", len(parts))
	}
	tamperedPayload, err := json.Marshal(map[string]any{
		"id":    43,
		"roles": "SERVICE",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	token = parts[0] + "." + base64.RawURLEncoding.EncodeToString(tamperedPayload) + "." + parts[2]

	if _, ok := handler.principalFromToken(token); ok {
		t.Fatal("expected tampered token to be rejected")
	}
}

func TestRequireServiceRoleRejectsNonServicePrincipal(t *testing.T) {
	handler := &Handler{cfg: testAuthConfig()}
	token := signedTestJWT(t, handler.cfg.JWTSecret, map[string]any{
		"id":    42,
		"roles": "USER",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	req := httptest.NewRequest("POST", "/transactions/internal/reserve-funds", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	if handler.requireServiceRole(rec, req) {
		t.Fatal("expected non-service principal to be rejected")
	}
	if rec.Code != 403 {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestRequireServiceRoleAcceptsSpaceSeparatedRolesClaim(t *testing.T) {
	handler := &Handler{cfg: testAuthConfig()}
	token := signedTestJWT(t, handler.cfg.JWTSecret, map[string]any{
		"id":    42,
		"roles": "CLIENT_BASIC SERVICE",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	req := httptest.NewRequest("POST", "/transactions/internal/reserve-funds", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	if !handler.requireServiceRole(rec, req) {
		t.Fatal("expected space-separated SERVICE role to be accepted")
	}
}

func TestRequireServiceRoleAcceptsServiceTokenWithoutUserID(t *testing.T) {
	handler := &Handler{cfg: testAuthConfig()}
	token := signedTestJWT(t, handler.cfg.JWTSecret, map[string]any{
		"sub":   "banking-core-service",
		"roles": "SERVICE",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	req := httptest.NewRequest("POST", "/transactions/internal/reserve-funds", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	if !handler.requireServiceRole(rec, req) {
		t.Fatal("expected service token without user id to be accepted")
	}
}

func TestRequireAuthenticatedAcceptsServiceTokenWithoutUserID(t *testing.T) {
	handler := &Handler{cfg: testAuthConfig()}
	token := signedTestJWT(t, handler.cfg.JWTSecret, map[string]any{
		"sub":   "banking-core-service",
		"roles": "SERVICE",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	req := httptest.NewRequest("POST", "/accounts/createMarginAccount", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	if !handler.requireAuthenticated(rec, req) {
		t.Fatal("expected signed service token to satisfy authenticated request")
	}
}

func TestMarginEndpointRequiresAuthentication(t *testing.T) {
	handler := &Handler{cfg: testAuthConfig()}
	req := httptest.NewRequest("POST", "/accounts/createMarginAccount", nil)
	rec := httptest.NewRecorder()

	handler.createUserMarginAccount(rec, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func testAuthConfig() config.Config {
	return config.Config{
		JWTSecret:           "01234567890123456789012345678901",
		JWTRoleClaim:        "roles",
		JWTPermissionsClaim: "permissions",
		JWTIssuer:           "banka1",
		JWTExpiration:       time.Hour,
	}
}

func signedTestJWT(t *testing.T, secret string, claims map[string]any) string {
	t.Helper()
	headerJSON, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		t.Fatal(err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
