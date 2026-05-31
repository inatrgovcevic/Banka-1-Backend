package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"banka1/go-platform/httpx"
	"github.com/golang-jwt/jwt/v5"
)

func makeService(t *testing.T) *Service {
	t.Helper()
	return NewService(Config{
		Secret:              "test-secret-test-secret-test-secret-12345",
		Issuer:              "banka1",
		IDClaim:             "id",
		RolesClaim:          "roles",
		PermissionsClaim:    "permissions",
		EmailClaim:          "email",
		AccessTokenDuration: time.Hour,
	})
}

func TestGenerateAndParseRoundTrip(t *testing.T) {
	svc := makeService(t)
	token, err := svc.GenerateAccessToken(42, "admin", "ADMIN", []string{"READ", "WRITE"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	p, err := svc.Parse(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.ID != 42 || p.Subject != "admin" || p.Role != "ADMIN" {
		t.Fatalf("unexpected principal: %+v", p)
	}
	if len(p.Permissions) != 2 || p.Permissions[0] != "READ" || p.Permissions[1] != "WRITE" {
		t.Fatalf("permissions lost: %+v", p.Permissions)
	}
}

func TestParseBearerStripsPrefix(t *testing.T) {
	svc := makeService(t)
	token, _ := svc.GenerateAccessToken(1, "x", "BASIC", nil)
	if _, err := svc.ParseBearer("Bearer " + token); err != nil {
		t.Fatalf("bearer parse failed: %v", err)
	}
	if _, err := svc.ParseBearer("NotBearer " + token); err == nil {
		t.Fatal("expected error for bad prefix")
	}
}

func TestParseRejectsWrongIssuer(t *testing.T) {
	svc := makeService(t)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":         "other",
		"sub":         "x",
		"exp":         time.Now().Add(time.Hour).Unix(),
		"id":          1,
		"roles":       "BASIC",
		"permissions": []string{},
	})
	signed, _ := token.SignedString([]byte(svc.cfg.Secret))
	if _, err := svc.Parse(signed); err == nil {
		t.Fatal("expected wrong-issuer rejection")
	}
}

func TestParseRejectsExpired(t *testing.T) {
	svc := makeService(t)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":         "banka1",
		"sub":         "x",
		"exp":         time.Now().Add(-time.Minute).Unix(),
		"id":          1,
		"roles":       "BASIC",
		"permissions": []string{},
	})
	signed, _ := token.SignedString([]byte(svc.cfg.Secret))
	if _, err := svc.Parse(signed); err == nil {
		t.Fatal("expected expired rejection")
	}
}

func TestParseRejectsWrongSignature(t *testing.T) {
	svc := makeService(t)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":         "banka1",
		"sub":         "x",
		"exp":         time.Now().Add(time.Hour).Unix(),
		"id":          1,
		"roles":       "BASIC",
		"permissions": []string{},
	})
	signed, _ := token.SignedString([]byte("wrong-secret"))
	if _, err := svc.Parse(signed); err == nil {
		t.Fatal("expected wrong-signature rejection")
	}
}

func TestParseRejectsTokenWithoutExp(t *testing.T) {
	svc := makeService(t)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":         "banka1",
		"sub":         "x",
		"id":          1,
		"roles":       "BASIC",
		"permissions": []string{},
	})
	signed, _ := token.SignedString([]byte(svc.cfg.Secret))
	if _, err := svc.Parse(signed); err == nil {
		t.Fatal("expected missing-exp rejection")
	}
}

func TestServiceTokenCarriesServiceRole(t *testing.T) {
	svc := makeService(t)
	token, err := svc.GenerateServiceToken("market-service-go", time.Minute)
	if err != nil {
		t.Fatalf("generate service token: %v", err)
	}
	p, err := svc.Parse(token)
	if err != nil {
		t.Fatalf("parse service token: %v", err)
	}
	if p.Role != "SERVICE" || !p.HasAnyRole("SERVICE") {
		t.Fatalf("expected SERVICE role: %+v", p)
	}
}

func TestRoleHierarchy(t *testing.T) {
	admin := Principal{Role: "ADMIN"}
	if !admin.HasAnyRole("SUPERVISOR", "AGENT", "BASIC") {
		t.Fatal("admin should pass supervisor/agent/basic")
	}
	if admin.HasAnyRole("SERVICE", "CLIENT_BASIC") {
		t.Fatal("admin should not magically be SERVICE or CLIENT_BASIC")
	}
	clientTrading := Principal{Role: "CLIENT_TRADING"}
	if !clientTrading.HasAnyRole("CLIENT_BASIC") {
		t.Fatal("client-trading should subsume client-basic")
	}
}

func TestMiddlewareRejectsMissingToken(t *testing.T) {
	svc := makeService(t)
	called := false
	h := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(httpx.WithCorrelation(req.Context(), "abc"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if called {
		t.Fatal("handler should not have been called")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var body httpx.ErrorBody
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.CorrelationID != "abc" {
		t.Fatalf("unauthorized response should still echo correlation id, got %q", body.CorrelationID)
	}
}

func TestMiddlewareAcceptsValidToken(t *testing.T) {
	svc := makeService(t)
	token, _ := svc.GenerateAccessToken(7, "u", "ADMIN", []string{"READ"})
	called := false
	h := svc.Middleware(RequireRoles("SUPERVISOR")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if p, ok := PrincipalFromContext(r.Context()); !ok || p.ID != 7 {
			t.Fatalf("principal missing or wrong: %+v", p)
		}
	})))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !called {
		t.Fatal("handler should have run")
	}
}

func TestRequireRolesDeniesOnLowerRole(t *testing.T) {
	svc := makeService(t)
	token, _ := svc.GenerateAccessToken(7, "u", "BASIC", nil)
	h := svc.Middleware(RequireRoles("ADMIN")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRequirePermissions(t *testing.T) {
	svc := makeService(t)
	token, _ := svc.GenerateAccessToken(7, "u", "BASIC", []string{"orders:read"})
	allow := svc.Middleware(RequirePermissions("orders:read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	deny := svc.Middleware(RequirePermissions("admin:write")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})))

	reqAllow := httptest.NewRequest(http.MethodGet, "/", nil)
	reqAllow.Header.Set("Authorization", "Bearer "+token)
	recAllow := httptest.NewRecorder()
	allow.ServeHTTP(recAllow, reqAllow)
	if recAllow.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recAllow.Code)
	}

	reqDeny := httptest.NewRequest(http.MethodGet, "/", nil)
	reqDeny.Header.Set("Authorization", "Bearer "+token)
	recDeny := httptest.NewRecorder()
	deny.ServeHTTP(recDeny, reqDeny)
	if recDeny.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recDeny.Code)
	}
}

func TestPermitAll(t *testing.T) {
	pa := NewPermitAll([]string{
		"/stocks/public/**",
		"/internal/calculate/**",
		"/actuator/info",
		"/actuator/health/liveness",
	})
	cases := map[string]bool{
		"/actuator/info":                  true,
		"/actuator/health/liveness":       true,
		"/stocks/public/widget":           true,
		"/stocks/public/widget/anything":  true,
		"/internal/calculate/no-commission": true,
		"/api/listings/15":                false,
		"/actuator/health":                false,
		"/actuator/info/x":                false,
	}
	for path, want := range cases {
		if got := pa.Matches(path); got != want {
			t.Errorf("Matches(%q)=%v, want %v", path, got, want)
		}
	}
}

func TestOptionalMiddlewareAllowsAnonymous(t *testing.T) {
	svc := makeService(t)
	reached := false
	h := svc.OptionalMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		if _, ok := PrincipalFromContext(r.Context()); ok {
			t.Fatal("did not expect principal for anonymous request")
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !reached {
		t.Fatal("optional middleware should pass through anonymous")
	}
}

func TestParseEmptySecretFails(t *testing.T) {
	svc := NewService(Config{Issuer: "banka1"})
	if _, err := svc.GenerateAccessToken(1, "x", "BASIC", nil); err == nil {
		t.Fatal("expected error when secret missing")
	}
	if _, err := svc.Parse("anything"); err == nil {
		t.Fatal("expected parse error when secret missing")
	}
}

func TestPermitAllIgnoresBlankPatterns(t *testing.T) {
	pa := NewPermitAll([]string{"  ", "", "/x/**"})
	if !pa.Matches("/x/y") {
		t.Fatal("expected /x/** to match")
	}
	if pa.Matches("/y/x") {
		t.Fatal("/y/x should not match /x/**")
	}
}

func TestBearerSyntaxRejected(t *testing.T) {
	svc := makeService(t)
	if _, err := svc.ParseBearer(""); err == nil {
		t.Fatal("empty header should fail")
	}
	if _, err := svc.ParseBearer("Bearer "); err == nil {
		t.Fatal("empty token should fail")
	}
	if _, err := svc.ParseBearer("bearer " + strings.Repeat("a", 10)); err == nil {
		t.Fatal("lowercase Bearer should fail (matches Java behavior)")
	}
}
