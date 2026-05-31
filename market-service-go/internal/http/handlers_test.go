package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"banka1/market-service-go/internal/auth"
	"banka1/market-service-go/internal/fx"
	"banka1/market-service-go/internal/platform"
)

func TestStockInfo(t *testing.T) {
	handlers := NewHandlers(platform.Config{Stock: platform.StockConfig{MarketDataBaseURL: "https://www.alphavantage.co"}}, &App{})
	req := httptest.NewRequest(http.MethodGet, "/stocks/info", nil)
	rec := httptest.NewRecorder()

	handlers.StockInfo(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["service"] != "stock-service" {
		t.Fatalf("unexpected service payload: %+v", payload)
	}
}

func TestCalculateValidationErrorShape(t *testing.T) {
	cfg := platform.Config{
		JWT:  platform.JWTConfig{Secret: "secret", Issuer: "banka1", IDClaim: "id", RolesClaim: "roles", PermissionsClaim: "permissions"},
		CORS: platform.CORSConfig{AllowedMethods: []string{"GET"}, AllowedOrigins: []string{"http://localhost:4200"}},
		FX:   platform.FXConfig{CommissionPercentage: "0.70"},
	}
	app := &App{Config: cfg}
	app.FXService = fx.NewService(app.Config, &fx.Repository{})
	jwtService := auth.NewJWTService(cfg.JWT)
	router := NewRouter(cfg, slog.Default(), nil, jwtService, app)

	req := httptest.NewRequest(http.MethodGet, "/internal/calculate/no-commission?fromCurrency=NOK&toCurrency=SEK&amount=0", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["code"] != "ERR_VALIDATION" {
		t.Fatalf("unexpected validation payload: %+v", payload)
	}
	if payload["title"] != "Validation error" {
		t.Fatalf("expected Java-aligned title 'Validation error', got %v", payload["title"])
	}
	if payload["message"] != "Molimo proverite unete podatke." {
		t.Fatalf("expected Java-aligned message, got %v", payload["message"])
	}
	validation, ok := payload["validationErrors"].(map[string]any)
	if !ok {
		t.Fatalf("expected validationErrors object, got %T", payload["validationErrors"])
	}
	if validation["amount"] == nil || validation["fromCurrency"] == nil || validation["toCurrency"] == nil {
		t.Fatalf("expected per-field validation errors, got %+v", validation)
	}
}

func TestActuatorInfoIsEmptyJSON(t *testing.T) {
	cfg := platform.Config{JWT: platform.JWTConfig{Secret: "secret", Issuer: "banka1", IDClaim: "id", RolesClaim: "roles", PermissionsClaim: "permissions"}, CORS: platform.CORSConfig{AllowedMethods: []string{"GET"}, AllowedOrigins: []string{"http://localhost:4200"}}}
	app := &App{Config: cfg}
	app.FXService = fx.NewService(cfg, &fx.Repository{})
	jwtService := auth.NewJWTService(cfg.JWT)
	router := NewRouter(cfg, slog.Default(), nil, jwtService, app)

	req := httptest.NewRequest(http.MethodGet, "/actuator/info", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(payload) != 0 {
		t.Fatalf("expected empty object (Java default), got %+v", payload)
	}
}

func TestCorrelationIDIsGeneratedWhenMissing(t *testing.T) {
	cfg := platform.Config{JWT: platform.JWTConfig{Secret: "secret", Issuer: "banka1", IDClaim: "id", RolesClaim: "roles", PermissionsClaim: "permissions"}, CORS: platform.CORSConfig{AllowedMethods: []string{"GET"}, AllowedOrigins: []string{"http://localhost:4200"}}}
	app := &App{Config: cfg}
	app.FXService = fx.NewService(cfg, &fx.Repository{})
	jwtService := auth.NewJWTService(cfg.JWT)
	router := NewRouter(cfg, slog.Default(), nil, jwtService, app)

	req := httptest.NewRequest(http.MethodGet, "/actuator/info", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Correlation-Id"); got == "" {
		t.Fatalf("expected generated X-Correlation-Id in response, got empty")
	}
}

func TestCorrelationIDIsPreservedFromRequest(t *testing.T) {
	cfg := platform.Config{JWT: platform.JWTConfig{Secret: "secret", Issuer: "banka1", IDClaim: "id", RolesClaim: "roles", PermissionsClaim: "permissions"}, CORS: platform.CORSConfig{AllowedMethods: []string{"GET"}, AllowedOrigins: []string{"http://localhost:4200"}}}
	app := &App{Config: cfg}
	app.FXService = fx.NewService(cfg, &fx.Repository{})
	jwtService := auth.NewJWTService(cfg.JWT)
	router := NewRouter(cfg, slog.Default(), nil, jwtService, app)

	req := httptest.NewRequest(http.MethodGet, "/actuator/info", nil)
	req.Header.Set("X-Correlation-Id", "abc-123")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Correlation-Id"); got != "abc-123" {
		t.Fatalf("expected echo of supplied correlation id, got %q", got)
	}
}

func TestActuatorHealthReturnsUp(t *testing.T) {
	cfg := platform.Config{JWT: platform.JWTConfig{Secret: "secret", Issuer: "banka1", IDClaim: "id", RolesClaim: "roles", PermissionsClaim: "permissions"}, CORS: platform.CORSConfig{AllowedMethods: []string{"GET"}, AllowedOrigins: []string{"http://localhost:4200"}}}
	app := &App{Config: cfg}
	app.FXService = fx.NewService(cfg, &fx.Repository{})
	jwtService := auth.NewJWTService(cfg.JWT)
	router := NewRouter(cfg, slog.Default(), nil, jwtService, app)

	req := httptest.NewRequest(http.MethodGet, "/actuator/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["status"] != "UP" {
		t.Fatalf("expected status UP, got %+v", payload)
	}
}
