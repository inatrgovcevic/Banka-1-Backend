package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testSecret = "super-secret-test-value"

func TestS2SIssuer_RoundTrip(t *testing.T) {
	issuer := NewS2SIssuer("banka1", "interbank-service", []string{"SERVICE"}, testSecret, time.Hour)
	tok, err := issuer.IssueToken()
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !strings.HasPrefix(tok, "eyJ") {
		t.Errorf("not JWT-shaped: %q", tok)
	}
	claims, err := VerifyJWT(tok, testSecret)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Subject != "interbank-service" {
		t.Errorf("sub=%q want interbank-service", claims.Subject)
	}
	if claims.Issuer != "banka1" {
		t.Errorf("iss=%q want banka1", claims.Issuer)
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != "SERVICE" {
		t.Errorf("roles=%v want [SERVICE]", claims.Roles)
	}
}

func TestS2SIssuer_CachesUntilExpiry(t *testing.T) {
	issuer := NewS2SIssuer("banka1", "interbank-service", []string{"SERVICE"}, testSecret, time.Hour)
	t1, err := issuer.IssueToken()
	if err != nil {
		t.Fatalf("issue 1: %v", err)
	}
	t2, err := issuer.IssueToken()
	if err != nil {
		t.Fatalf("issue 2: %v", err)
	}
	if t1 != t2 {
		t.Errorf("expected cached token; t1 != t2")
	}
}

func TestVerifyJWT_RejectsWrongSecret(t *testing.T) {
	issuer := NewS2SIssuer("banka1", "x", []string{"SERVICE"}, testSecret, time.Hour)
	tok, _ := issuer.IssueToken()
	_, err := VerifyJWT(tok, "different-secret")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestVerifyJWT_RejectsExpired(t *testing.T) {
	issuer := NewS2SIssuer("banka1", "x", []string{"SERVICE"}, testSecret, -1*time.Second) // already expired
	tok, _ := issuer.IssueToken()
	_, err := VerifyJWT(tok, testSecret)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestRequireJWT_NoAuth_401(t *testing.T) {
	h := RequireJWT(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d want 401", rec.Code)
	}
}

func TestRequireJWT_GoodToken_200_PopulatesContext(t *testing.T) {
	issuer := NewS2SIssuer("banka1", "interbank-service", []string{"SERVICE"}, testSecret, time.Hour)
	tok, _ := issuer.IssueToken()
	var captured *Claims
	h := RequireJWT(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = GetClaims(r.Context())
		w.WriteHeader(200)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body)
	}
	if captured == nil || captured.Subject != "interbank-service" {
		t.Errorf("claims=%+v", captured)
	}
}

func TestRequireJWT_MalformedAuthHeader_401(t *testing.T) {
	h := RequireJWT(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	cases := []string{"", "notbearer", "Bearer ", "Bearer notajwt", "Basic xyz"}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/x", nil)
		if c != "" {
			req.Header.Set("Authorization", c)
		}
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("authorization=%q: code=%d want 401", c, rec.Code)
		}
	}
}

func TestRequirePermission_HasPermission_OK(t *testing.T) {
	issuer := NewS2SIssuer("banka1", "u-1", []string{"BASIC"}, testSecret, time.Hour)
	issuer.Permissions = []string{"OTC_TRADE"}
	tok, _ := issuer.IssueToken()
	h := RequireJWT(testSecret)(RequirePermission("OTC_TRADE", "ROLE_ADMIN")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestRequirePermission_HasRole_OK(t *testing.T) {
	// Role values stored in claims.Roles get auto-prefixed with ROLE_ during the check.
	issuer := NewS2SIssuer("banka1", "u-1", []string{"ADMIN"}, testSecret, time.Hour)
	tok, _ := issuer.IssueToken()
	h := RequireJWT(testSecret)(RequirePermission("OTC_TRADE", "ROLE_ADMIN")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestRequirePermission_NoMatch_403(t *testing.T) {
	issuer := NewS2SIssuer("banka1", "u-1", []string{"BASIC"}, testSecret, time.Hour)
	tok, _ := issuer.IssueToken()
	h := RequireJWT(testSecret)(RequirePermission("OTC_TRADE", "ROLE_ADMIN")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code=%d want 403", rec.Code)
	}
}
