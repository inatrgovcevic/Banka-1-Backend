package http

import (
	"net/http/httptest"
	"testing"

	"banka1/banking-core-service-go/internal/config"
)

func testCORSConfig() config.Config {
	cfg := testAuthConfig()
	cfg.CORSAllowedOrigins = []string{"http://localhost:4200"}
	cfg.CORSAllowedMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	cfg.CORSAllowedHeaders = []string{"Authorization", "Content-Type", "Accept", "X-Requested-With", "X-Verification-Code", "X-Correlation-Id"}
	cfg.CORSExposedHeaders = []string{"Location", "X-Correlation-Id"}
	cfg.CORSAllowCredentials = true
	cfg.CORSMaxAgeSeconds = 3600
	return cfg
}

func TestCORSPreflightFromAllowedOrigin(t *testing.T) {
	h := NewHandler(testCORSConfig(), nil)
	req := httptest.NewRequest("OPTIONS", "/accounts/client/accounts", nil)
	req.Header.Set("Origin", "http://localhost:4200")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != 204 {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:4200" {
		t.Fatalf("Allow-Origin = %q, want http://localhost:4200", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Allow-Credentials = %q, want true", got)
	}
	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatal("expected Access-Control-Allow-Methods on preflight")
	}
	if rec.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Fatal("expected Access-Control-Allow-Headers on preflight")
	}
	if rec.Header().Get("Access-Control-Max-Age") != "3600" {
		t.Fatalf("Max-Age = %q, want 3600", rec.Header().Get("Access-Control-Max-Age"))
	}
}

func TestCORSHeadersPresentOnUnauthorized(t *testing.T) {
	// Spring CorsFilter runs before security, so even a 401 carries CORS headers.
	h := NewHandler(testCORSConfig(), nil)
	req := httptest.NewRequest("GET", "/accounts/client/accounts", nil)
	req.Header.Set("Origin", "http://localhost:4200")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:4200" {
		t.Fatalf("Allow-Origin = %q, want http://localhost:4200", got)
	}
}

func TestCORSDisallowedOrigin(t *testing.T) {
	h := NewHandler(testCORSConfig(), nil)
	req := httptest.NewRequest("OPTIONS", "/accounts/client/accounts", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Allow-Origin = %q, want empty for disallowed origin", got)
	}
}
