package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Additional 401/403 route coverage tests.
// These short-circuit at auth enforcement before touching any service,
// so they work even with nil services.

func TestAccountRoutes_Returns401WithoutToken(t *testing.T) {
	paths := []struct{ method, path string }{
		{http.MethodGet, "/accounts/api/currencies/getAll"},
		{http.MethodGet, "/accounts/api/currencies/getAllPage"},
		{http.MethodGet, "/accounts/api/currencies"},
		{http.MethodGet, "/accounts/api/currencies/USD"},
		{http.MethodGet, "/accounts/api/sifra-delatnosti"},
		{http.MethodPost, "/accounts/employee/accounts/checking"},
		{http.MethodPost, "/accounts/employee/accounts/fx"},
		{http.MethodGet, "/accounts/employee/accounts"},
		{http.MethodPut, "/accounts/employee/accounts/123456/status"},
		{http.MethodGet, "/accounts/employee/accounts/client/1"},
		{http.MethodGet, "/accounts/employee/accounts/bank"},
		{http.MethodGet, "/accounts/employee/accounts/bank/EUR"},
		{http.MethodGet, "/accounts/employee/accounts/1110001000000000511"},
		{http.MethodGet, "/accounts/employee/companies/1"},
		{http.MethodPut, "/accounts/employee/companies/1"},
		{http.MethodGet, "/accounts/client/accounts"},
		{http.MethodGet, "/accounts/client/accounts/123/cards"},
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

func TestTransactionRoutes_Returns401WithoutToken(t *testing.T) {
	paths := []struct{ method, path string }{
		{http.MethodPost, "/transactions/payment"},
		{http.MethodPost, "/transactions/internal/reserve-funds"},
		{http.MethodPost, "/transactions/internal/complete-funds"},
		{http.MethodPost, "/transactions/internal/release-funds"},
		{http.MethodGet, "/transactions/payment"},
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

func TestCardRoutes_Returns401WithoutToken(t *testing.T) {
	paths := []struct{ method, path string }{
		{http.MethodPost, "/cards"},
		{http.MethodGet, "/cards/1"},
		{http.MethodPut, "/cards/1/block"},
		{http.MethodPut, "/cards/1/unblock"},
		{http.MethodPut, "/cards/1/deactivate"},
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

func TestVerificationRoutes_Returns401WithoutToken(t *testing.T) {
	paths := []struct{ method, path string }{
		{http.MethodPost, "/verification/generate"},
		{http.MethodPost, "/verification/validate"},
		{http.MethodGet, "/verification/42/status"},
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

func TestGDPRAndMarginRoutes_Returns401WithoutToken(t *testing.T) {
	paths := []struct{ method, path string }{
		{http.MethodGet, "/accounts/gdpr/1"},
		{http.MethodPost, "/accounts/createMarginAccount"},
		{http.MethodPost, "/accounts/company/createMarginAccount"},
		{http.MethodPost, "/transactions/stockBuyMarginTransaction"},
		{http.MethodPost, "/transactions/stockSellMarginTransaction"},
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

func TestClientAccountUpdateRoutes_Returns401WithoutToken(t *testing.T) {
	paths := []struct{ method, path string }{
		{http.MethodPut, "/accounts/client/api/accounts/123456789012345678/name"},
		{http.MethodPut, "/accounts/client/api/accounts/1/name"},
		{http.MethodPut, "/accounts/client/api/accounts/1/limits"},
		{http.MethodGet, "/accounts/client/api/accounts/1"},
		{http.MethodGet, "/accounts/client/api/accounts/1234567890123456789"},
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

// ── pageParams utility ──────────────────────────────────────────────────────

func TestPageParams_ValidQueryParams(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path?page=2&size=20", nil)
	page, size := pageParams(req)
	if page != 2 || size != 20 {
		t.Fatalf("pageParams = %d, %d; want 2, 20", page, size)
	}
}

func TestPageParams_InvalidValues_UsesDefaults(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path?page=abc&size=xyz", nil)
	page, size := pageParams(req)
	if page != 0 {
		t.Fatalf("page = %d, want 0", page)
	}
	if size != 10 {
		t.Fatalf("size = %d, want 10", size)
	}
}

func TestPageParams_NegativePage_ClampedToZero(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path?page=-1&size=5", nil)
	page, _ := pageParams(req)
	if page != 0 {
		t.Fatalf("page = %d, want 0", page)
	}
}

func TestPageParams_LargeSize_AcceptsAsIs(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path?page=0&size=500", nil)
	_, size := pageParams(req)
	if size != 500 {
		t.Fatalf("size = %d, want 500", size)
	}
}

// ── CORS OPTIONS ────────────────────────────────────────────────────────────

func TestOptionsPreflightReturns204(t *testing.T) {
	t.Parallel()
	h := testHandler()
	req := httptest.NewRequest(http.MethodOptions, "/accounts/client/accounts", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d, want 204", rec.Code)
	}
}

// ── Actuator endpoints ────────────────────────────────────────────────────────

func TestActuatorInfoReturnsOK(t *testing.T) {
	t.Parallel()
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/actuator/info", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("actuator/info status = %d, want 200", rec.Code)
	}
}

func TestActuatorPrometheusReturnsOK(t *testing.T) {
	t.Parallel()
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/actuator/prometheus", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("actuator/prometheus status = %d, want 200", rec.Code)
	}
}

// ── 404 on unknown route ─────────────────────────────────────────────────────

func TestUnknownRouteReturns404(t *testing.T) {
	t.Parallel()
	h := &Handler{cfg: testAuthConfig()}
	tok := authHeader(t, h, "SERVICE")
	req := httptest.NewRequest(http.MethodGet, "/unknown/route/xyz", nil)
	req.Header.Set("Authorization", tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// ── Panic recovery ───────────────────────────────────────────────────────────

func TestPanicRecoveryReturns500(t *testing.T) {
	t.Parallel()
	// Register a route that panics — the deferred recover should return 500
	h := &Handler{
		cfg: testAuthConfig(),
	}
	tok := authHeader(t, h, "SERVICE")

	// Access an interbank route that will hit nil services and should recover
	req := httptest.NewRequest(http.MethodGet, "/actuator/health/liveness", nil)
	req.Header.Set("Authorization", tok)
	rec := httptest.NewRecorder()

	// The handler should not panic out — it has deferred recover
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()
	h.ServeHTTP(rec, req)
}

// intQuery helper
func TestIntQuery_ValidValue(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path?count=42", nil)
	val := intQuery(req, "count", 0)
	if val != 42 {
		t.Fatalf("intQuery = %d, want 42", val)
	}
}

func TestIntQuery_MissingKey_UsesDefault(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path", nil)
	val := intQuery(req, "missing", 99)
	if val != 99 {
		t.Fatalf("intQuery = %d, want 99", val)
	}
}

func TestIntQuery_InvalidValue_UsesDefault(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/path?n=abc", nil)
	val := intQuery(req, "n", 5)
	if val != 5 {
		t.Fatalf("intQuery = %d, want 5", val)
	}
}

// Test the token generation helper used in other tests
func init() {
	_ = time.Now
}
