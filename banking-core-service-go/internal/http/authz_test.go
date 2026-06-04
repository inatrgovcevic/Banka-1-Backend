package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHasAnyRoleHierarchical(t *testing.T) {
	cases := []struct {
		name    string
		held    []string
		allowed []string
		want    bool
	}{
		{"admin satisfies basic", []string{"ADMIN"}, []string{"BASIC"}, true},
		{"supervisor satisfies agent", []string{"SUPERVISOR"}, []string{"AGENT"}, true},
		{"agent satisfies basic", []string{"AGENT"}, []string{"BASIC"}, true},
		{"client_trading satisfies client_basic", []string{"CLIENT_TRADING"}, []string{"CLIENT_BASIC"}, true},
		{"basic does not satisfy client_basic", []string{"BASIC"}, []string{"CLIENT_BASIC"}, false},
		{"client_basic does not satisfy basic", []string{"CLIENT_BASIC"}, []string{"BASIC"}, false},
		{"basic does not satisfy admin", []string{"BASIC"}, []string{"ADMIN"}, false},
		{"role_ prefix normalized", []string{"ROLE_ADMIN"}, []string{"BASIC"}, true},
		{"exact match", []string{"SERVICE"}, []string{"SERVICE", "ADMIN"}, true},
		{"no overlap", []string{"CLIENT_BASIC"}, []string{"SERVICE"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasAnyRoleHierarchical(tc.held, tc.allowed); got != tc.want {
				t.Fatalf("hasAnyRoleHierarchical(%v, %v) = %v, want %v", tc.held, tc.allowed, got, tc.want)
			}
		})
	}
}

func TestRouteAuth(t *testing.T) {
	cases := []struct {
		method, path string
		mode         authMode
		roles        []string
	}{
		{"GET", "/actuator/health/liveness", authOpen, nil},
		{"GET", "/accounts/internal/default/5", authOpen, nil},
		{"GET", "/accounts/internal/by-owner/5/currency/USD", authOpen, nil},
		{"POST", "/verification/generate", authAuthenticated, nil},
		{"POST", "/transactions/payment", authRoles, rolesClientBasic},
		{"GET", "/transactions/by-client", authRoles, rolesBasic},
		{"GET", "/transactions/by-this-client", authRoles, rolesClientBasic},
		{"POST", "/api/cards/auto", authRoles, rolesServiceAdmin},
		{"POST", "/api/cards/request", authRoles, rolesClientAdmin},
		{"PUT", "/api/cards/id/7/unblock", authRoles, rolesBasic},
		{"PUT", "/api/cards/id/7/block", authRoles, rolesClientOrBasic},
		{"GET", "/api/cards/all", authRoles, rolesBasic},
		{"GET", "/api/cards/internal/account/123", authRoles, rolesService},
		{"POST", "/transactions/internal/reserve-funds", authRoles, rolesService},
		{"POST", "/internal/accounts/debit", authRoles, rolesService},
		{"POST", "/internal/accounts/system", authRoles, rolesServiceBasic},
		{"GET", "/internal/interbank/account-resolve", authRoles, rolesService},
		{"POST", "/transfers", authRoles, rolesTransfer},
		{"GET", "/payment-recipients", authRoles, rolesClientBasic},
		{"POST", "/accounts/createMarginAccount", authAuthenticated, nil},
		{"GET", "/accounts/employee/accounts", authRoles, rolesBasicService},
		{"PUT", "/accounts/employee/accounts/123/status", authRoles, rolesBasic},
		{"GET", "/accounts/api/currencies/getAll", authRoles, rolesCurrency},
		{"GET", "/accounts/client/accounts", authRoles, rolesClientAgent},
		{"GET", "/accounts/client/api/accounts/123", authRoles, rolesClientAgentService},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			mode, roles := routeAuth(tc.method, tc.path)
			if mode != tc.mode {
				t.Fatalf("mode = %d, want %d", mode, tc.mode)
			}
			if !sameRoles(roles, tc.roles) {
				t.Fatalf("roles = %v, want %v", roles, tc.roles)
			}
		})
	}
}

func TestEnforceAuthForbidsWrongRole(t *testing.T) {
	handler := &Handler{cfg: testAuthConfig()}
	// /api/cards/auto requires SERVICE or ADMIN; a CLIENT_BASIC token must be 403.
	token := signedTestJWT(t, handler.cfg.JWTSecret, map[string]any{
		"id":    7,
		"roles": "CLIENT_BASIC",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	req := httptest.NewRequest("POST", "/api/cards/auto", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	if handler.enforceAuth(rec, req) {
		t.Fatal("expected CLIENT_BASIC to be forbidden on /api/cards/auto")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestEnforceAuthAllowsHierarchicalRole(t *testing.T) {
	handler := &Handler{cfg: testAuthConfig()}
	// /transactions/by-client requires BASIC; an ADMIN token satisfies it via hierarchy.
	token := signedTestJWT(t, handler.cfg.JWTSecret, map[string]any{
		"id":    7,
		"roles": "ADMIN",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	req := httptest.NewRequest("GET", "/transactions/by-client", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	if !handler.enforceAuth(rec, req) {
		t.Fatalf("expected ADMIN to satisfy BASIC requirement, got status %d", rec.Code)
	}
}

func TestEnforceAuthAllowsServiceTokenWithoutUserID(t *testing.T) {
	// Servisni tokeni nemaju numericki id; SERVICE rola mora i dalje da prodje.
	handler := &Handler{cfg: testAuthConfig()}
	token := signedTestJWT(t, handler.cfg.JWTSecret, map[string]any{
		"sub":   "banking-core-service",
		"roles": "SERVICE",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	req := httptest.NewRequest("POST", "/internal/accounts/credit", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	if !handler.enforceAuth(rec, req) {
		t.Fatalf("expected id-less SERVICE token to pass /internal/accounts, got status %d", rec.Code)
	}
}

func TestEnforceAuthRequiresAuthForUnknownPath(t *testing.T) {
	handler := &Handler{cfg: testAuthConfig()}
	req := httptest.NewRequest("GET", "/some/unknown/path", nil)
	rec := httptest.NewRecorder()

	if handler.enforceAuth(rec, req) {
		t.Fatal("expected unauthenticated request to unknown path to be rejected")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func sameRoles(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
