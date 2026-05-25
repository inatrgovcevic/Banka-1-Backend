package config_test

import (
	"os"
	"testing"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	// Provide only the required fields; verify defaults are applied.
	t.Setenv("SAGA_DB_URL", "postgres://test:test@localhost:5432/testdb?sslmode=disable")
	t.Setenv("SAGA_JWT_SECRET", "dev-secret")
	t.Setenv("SAGA_PROFILE", "dev")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.Server.HTTPPort != 8095 {
		t.Errorf("Server.HTTPPort: got %d, want 8095", cfg.Server.HTTPPort)
	}
	if cfg.Server.MigrationsPath != "/migrations" {
		t.Errorf("Server.MigrationsPath: got %q, want /migrations", cfg.Server.MigrationsPath)
	}
	if cfg.Saga.RabbitMQURL != "amqp://guest:guest@rabbitmq:5672/" {
		t.Errorf("Saga.RabbitMQURL: got %q, want default", cfg.Saga.RabbitMQURL)
	}
	if cfg.Saga.Services.BankingCoreURL != "http://banking-core-service:8084" {
		t.Errorf("Saga.Services.BankingCoreURL: got %q", cfg.Saga.Services.BankingCoreURL)
	}
	if cfg.Saga.Services.TradingURL != "http://trading-service:8088" {
		t.Errorf("Saga.Services.TradingURL: got %q", cfg.Saga.Services.TradingURL)
	}
	if cfg.Saga.Services.MarketURL != "http://market-service:8085" {
		t.Errorf("Saga.Services.MarketURL: got %q", cfg.Saga.Services.MarketURL)
	}
	if cfg.Saga.Cleanup.Interval.Minutes() != 15 {
		t.Errorf("Saga.Cleanup.Interval: got %v, want 15m", cfg.Saga.Cleanup.Interval)
	}
	if cfg.Saga.Cleanup.StuckCutoff.Hours() != 1 {
		t.Errorf("Saga.Cleanup.StuckCutoff: got %v, want 1h", cfg.Saga.Cleanup.StuckCutoff)
	}
	// 336h = 14 days
	if cfg.Saga.Cleanup.IdempotencyRetention.Hours() != 336 {
		t.Errorf("Saga.Cleanup.IdempotencyRetention: got %v, want 336h", cfg.Saga.Cleanup.IdempotencyRetention)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("SAGA_DB_URL", "postgres://u:p@db:5432/saga?sslmode=disable")
	t.Setenv("SAGA_JWT_SECRET", "super-secret")
	t.Setenv("SAGA_JWT_ISSUER", "mybank")
	t.Setenv("SAGA_SERVER_HTTP_PORT", "9999")
	t.Setenv("SAGA_SAGA_RABBITMQ_URL", "amqp://custom:custom@broker:5672/")
	t.Setenv("SAGA_SAGA_SERVICES_TRADING_URL", "http://trading:9000")
	t.Setenv("SAGA_SAGA_CLEANUP_INTERVAL", "5m")
	t.Setenv("SAGA_PROFILE", "dev")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.Server.HTTPPort != 9999 {
		t.Errorf("Server.HTTPPort: got %d, want 9999", cfg.Server.HTTPPort)
	}
	if cfg.JWT.Issuer != "mybank" {
		t.Errorf("JWT.Issuer: got %q, want mybank", cfg.JWT.Issuer)
	}
	if cfg.Saga.RabbitMQURL != "amqp://custom:custom@broker:5672/" {
		t.Errorf("Saga.RabbitMQURL: got %q", cfg.Saga.RabbitMQURL)
	}
	if cfg.Saga.Services.TradingURL != "http://trading:9000" {
		t.Errorf("Saga.Services.TradingURL: got %q", cfg.Saga.Services.TradingURL)
	}
	if cfg.Saga.Cleanup.Interval.Minutes() != 5 {
		t.Errorf("Saga.Cleanup.Interval: got %v, want 5m", cfg.Saga.Cleanup.Interval)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	// Unset required vars.
	os.Unsetenv("SAGA_DB_URL")
	os.Unsetenv("SAGA_JWT_SECRET")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() expected error when required fields missing, got nil")
	}
}

func TestLoad_ProdProfile_DevSecret(t *testing.T) {
	t.Setenv("SAGA_DB_URL", "postgres://u:p@db:5432/saga?sslmode=disable")
	t.Setenv("SAGA_JWT_SECRET", "dev-secret")
	t.Setenv("SAGA_PROFILE", "prod")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() expected error for dev-secret in prod profile, got nil")
	}
}

func TestLoad_ProdProfile_ValidSecret(t *testing.T) {
	t.Setenv("SAGA_DB_URL", "postgres://u:p@db:5432/saga?sslmode=disable")
	t.Setenv("SAGA_JWT_SECRET", "a-very-strong-production-secret-key-here")
	t.Setenv("SAGA_PROFILE", "prod")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() unexpected error in prod profile with valid secret: %v", err)
	}
	if cfg.JWT.Secret != "a-very-strong-production-secret-key-here" {
		t.Errorf("JWT.Secret not stored correctly")
	}
}
