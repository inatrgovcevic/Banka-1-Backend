package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// helper: handler with auth config and nil services (for tests that short-circuit before service calls)
func testHandler() *Handler {
	return &Handler{cfg: testAuthConfig()}
}

// helper: authenticated token for a given role
func authHeader(t *testing.T, h *Handler, role string) string {
	t.Helper()
	token := signedTestJWT(t, h.cfg.JWTSecret, map[string]any{
		"id":    1,
		"roles": role,
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	return "Bearer " + token
}

// ── 405 Method Not Allowed ────────────────────────────────────────────────────
//
// Auth is checked before the route switch, so 405 is only reachable without a
// token on open (authOpen) endpoints. For authenticated routes, a missing token
// returns 401 first. We test open actuator paths here; authenticated-route 405
// behavior is covered by the live integration script.

func TestServeHTTPMethodNotAllowedOnOpenPaths(t *testing.T) {
	cases := []struct {
		method, path string
	}{
		{http.MethodDelete, "/actuator/health"},
		{http.MethodPost, "/actuator/health/liveness"},
		{http.MethodPut, "/actuator/health/readiness"},
		{http.MethodDelete, "/actuator/info"},
		{http.MethodPost, "/actuator/prometheus"},
	}

	h := testHandler()
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405 for %s %s\nbody: %s", rec.Code, tc.method, tc.path, rec.Body.String())
			}
		})
	}
}

// ── 401 on authenticated/role-guarded endpoints without token ────────────────

func TestServeHTTPUnauthorizedWithoutToken(t *testing.T) {
	paths := []struct{ method, path string }{
		{http.MethodPost, "/accounts/createMarginAccount"},
		{http.MethodPost, "/verification/generate"},
		{http.MethodPost, "/transactions/payment"},
		{http.MethodGet, "/accounts/employee/accounts"},
		{http.MethodPost, "/transactions/internal/reserve-funds"},
	}

	h := testHandler()
	for _, tc := range paths {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 for %s %s", rec.Code, tc.method, tc.path)
			}
		})
	}
}

// ── 400 on invalid JSON body (decode fails before service is called) ──────────

func TestCreateMarginAccountInvalidJSONReturns400(t *testing.T) {
	h := testHandler()
	token := authHeader(t, h, "SERVICE")

	req := httptest.NewRequest(http.MethodPost, "/accounts/createMarginAccount", strings.NewReader("{invalid json"))
	req.Header.Set("Authorization", token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for invalid JSON\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateCompanyMarginAccountInvalidJSONReturns400(t *testing.T) {
	h := testHandler()
	token := authHeader(t, h, "SERVICE")

	req := httptest.NewRequest(http.MethodPost, "/accounts/company/createMarginAccount", strings.NewReader("{bad}"))
	req.Header.Set("Authorization", token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestBuyOnMarginInvalidJSONReturns400(t *testing.T) {
	h := testHandler()
	token := authHeader(t, h, "SERVICE")

	req := httptest.NewRequest(http.MethodPost, "/transactions/stockBuyMarginTransaction", strings.NewReader("not json"))
	req.Header.Set("Authorization", token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

// ── 200 on open endpoints (no auth required) ────────────────────────────────

func TestActuatorEndpointsOpenWithoutAuth(t *testing.T) {
	paths := []string{
		"/actuator/health",
		"/actuator/health/liveness",
		"/actuator/health/readiness",
		"/actuator/info",
		"/actuator/prometheus",
	}

	h := testHandler()
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
				t.Fatalf("actuator path %s should not require auth, got %d", path, rec.Code)
			}
		})
	}
}

// ── 404 on completely unknown paths ─────────────────────────────────────────

func TestServeHTTPNotFoundForUnknownPaths(t *testing.T) {
	h := testHandler()
	paths := []string{"/unknown", "/foo/bar", "/api/v2/something"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			// Unknown paths return 401 (unauthenticated) or 404 depending on auth guard.
			// The key is it must NOT be 200.
			if rec.Code == http.StatusOK {
				t.Fatalf("unknown path %s should not return 200", path)
			}
		})
	}
}

// ── isKnownPath coverage ─────────────────────────────────────────────────────

func TestIsKnownPathMatchesRegisteredRoutes(t *testing.T) {
	known := []string{
		"/actuator/health",
		"/actuator/prometheus",
		"/accounts/createMarginAccount",
		"/transactions/payment",
		"/transfers",
		"/payment-recipients",
		"/internal/accounts/debit",
		"/internal/interbank/reserve-monas",
	}
	for _, p := range known {
		if !isKnownPath(p) {
			t.Errorf("isKnownPath(%q) = false, want true", p)
		}
	}
}

func TestIsKnownPathMatchesPrefixRoutes(t *testing.T) {
	known := []string{
		"/accounts/employee/accounts/123",
		"/accounts/getMarginUser/5",
		"/api/cards/id/42",
		"/transactions/addToMargin/7",
		"/transfers/ORDER-001",
		"/internal/accounts/ACC123/details",
	}
	for _, p := range known {
		if !isKnownPath(p) {
			t.Errorf("isKnownPath(%q) = false, want true (prefix match)", p)
		}
	}
}

func TestIsKnownPathReturnsFalseForUnknown(t *testing.T) {
	unknown := []string{"/unknown", "/api/v2/test", "/admin/users"}
	for _, p := range unknown {
		if isKnownPath(p) {
			t.Errorf("isKnownPath(%q) = true, want false", p)
		}
	}
}
