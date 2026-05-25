// Package config loads saga-orchestrator-service configuration from environment
// variables using kelseyhightower/envconfig.
//
// # Env var naming convention
//
// All vars are prefixed SAGA_ (the Load prefix). Because the top-level struct
// has named sub-structs, envconfig double-prefixes nested fields:
//
//   - Config.Server.HTTPPort      → SAGA_SERVER_HTTP_PORT
//   - Config.DB.URL               → SAGA_DB_URL
//   - Config.JWT.Secret           → SAGA_JWT_SECRET
//   - Config.Saga.RabbitMQURL     → SAGA_SAGA_RABBITMQ_URL        (double prefix!)
//   - Config.Saga.Services.BankingCoreURL → SAGA_SAGA_SERVICES_BANKING_CORE_URL
//   - Config.Saga.Cleanup.Interval        → SAGA_SAGA_CLEANUP_INTERVAL
//
// The docker-compose.yml service block MUST use these exact names.
// See setup/docker-compose.yml for the canonical list.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	sharedConfig "github.com/raf-si-2025/banka-1-go/shared/config"
)

// Config is the top-level configuration struct for saga-orchestrator-service.
type Config struct {
	Server ServerConfig
	DB     DBConfig
	JWT    JWTConfig
	Saga   SagaConfig
}

// ServerConfig holds HTTP server settings.
// Env prefix: SAGA_SERVER_*.
type ServerConfig struct {
	// HTTPPort is the port the admin HTTP server listens on.
	// Env: SAGA_SERVER_HTTP_PORT (default: 8095)
	HTTPPort int `envconfig:"HTTP_PORT" default:"8095"`

	// LogLevel controls the slog log level.
	// Env: SAGA_SERVER_LOG_LEVEL (default: info)
	LogLevel slog.Level `envconfig:"LOG_LEVEL" default:"info"`

	// LogJSON switches between JSON and text log format.
	// Env: SAGA_SERVER_LOG_JSON (default: true)
	LogJSON bool `envconfig:"LOG_JSON" default:"true"`

	// ReadTimeout / WriteTimeout for the admin HTTP server.
	ReadTimeout  time.Duration `envconfig:"READ_TIMEOUT" default:"30s"`
	WriteTimeout time.Duration `envconfig:"WRITE_TIMEOUT" default:"30s"`

	// ShutdownTimeout is how long to wait for in-flight requests to drain.
	ShutdownTimeout time.Duration `envconfig:"SHUTDOWN_TIMEOUT" default:"15s"`

	// MigrationsPath is the directory containing Goose SQL files.
	// Default "/migrations" matches the Docker image layout.
	// Set SAGA_SERVER_MIGRATIONS_PATH=./migrations when running outside Docker.
	MigrationsPath string `envconfig:"MIGRATIONS_PATH" default:"/migrations"`
}

// DBConfig holds Postgres pool settings.
// Env vars: SAGA_DB_URL (required), SAGA_DB_MAX_CONNS, SAGA_DB_MIN_CONNS.
type DBConfig struct {
	URL      string `envconfig:"URL" required:"true"`
	MaxConns int32  `envconfig:"MAX_CONNS" default:"10"`
	MinConns int32  `envconfig:"MIN_CONNS" default:"2"`
}

// JWTConfig holds settings for S2S JWT issuance.
// Env vars: SAGA_JWT_SECRET (required), SAGA_JWT_ISSUER, SAGA_JWT_TTL.
type JWTConfig struct {
	Secret string        `envconfig:"SECRET" required:"true"`
	Issuer string        `envconfig:"ISSUER" default:"banka1"`
	TTL    time.Duration `envconfig:"TTL" default:"1h"`
}

// SagaConfig holds all saga-orchestrator-specific settings.
// NOTE: because this is nested under Config.Saga, envconfig uses the
// double-prefix SAGA_SAGA_* for all its fields.
type SagaConfig struct {
	// RabbitMQURL is the AMQP broker URL.
	// Env: SAGA_SAGA_RABBITMQ_URL (default: amqp://guest:guest@rabbitmq:5672/)
	RabbitMQURL string `envconfig:"RABBITMQ_URL" default:"amqp://guest:guest@rabbitmq:5672/"`

	Services ServicesConfig
	Cleanup  CleanupConfig
}

// ServicesConfig holds base URLs for downstream Java services called by saga handlers.
// Env prefix: SAGA_SAGA_SERVICES_*.
type ServicesConfig struct {
	BankingCoreURL string        `envconfig:"BANKING_CORE_URL" default:"http://banking-core-service:8084"`
	TradingURL     string        `envconfig:"TRADING_URL" default:"http://trading-service:8088"`
	MarketURL      string        `envconfig:"MARKET_URL" default:"http://market-service:8085"`
	// Timeout applies to all downstream REST calls.
	// Use 30s as default — LiquidateForFund can be slow.
	Timeout time.Duration `envconfig:"TIMEOUT" default:"30s"`
}

// CleanupConfig controls the stuck-saga sweeper and idempotency-log pruning.
// Env prefix: SAGA_SAGA_CLEANUP_*.
type CleanupConfig struct {
	// Interval is how often the cleanup goroutine wakes.
	// Env: SAGA_SAGA_CLEANUP_INTERVAL (default: 15m)
	Interval time.Duration `envconfig:"INTERVAL" default:"15m"`

	// StuckCutoff is the age beyond which a saga still in IN_PROGRESS or
	// COMPENSATING is considered stuck. A WARN log + Prometheus counter is
	// emitted for each stuck saga; no automatic recovery.
	// Env: SAGA_SAGA_CLEANUP_STUCK_CUTOFF (default: 1h)
	StuckCutoff time.Duration `envconfig:"STUCK_CUTOFF" default:"1h"`

	// IdempotencyRetention is how long to keep rows in the saga_idempotency_log
	// table (if it exists — it is an orphan table from the Java version that may
	// not be present). Rows older than this are deleted on each tick.
	// Env: SAGA_SAGA_CLEANUP_IDEMPOTENCY_RETENTION (default: 336h = 14 days)
	IdempotencyRetention time.Duration `envconfig:"IDEMPOTENCY_RETENTION" default:"336h"`
}

// Load reads all SAGA_* env vars into a Config struct.
// In production (SAGA_PROFILE != "dev" | "test") it validates that
// SAGA_JWT_SECRET is present (already enforced by `required:"true"` tag).
func Load() (*Config, error) {
	var c Config
	if err := sharedConfig.Load("SAGA", &c); err != nil {
		return nil, fmt.Errorf("config: load env: %w", err)
	}

	profile := os.Getenv("SAGA_PROFILE")
	if profile == "" {
		profile = "prod"
	}

	// In prod profile, reject obviously-dev JWT secret.
	if profile != "dev" && profile != "test" {
		if c.JWT.Secret == "dev-secret" || c.JWT.Secret == "" {
			return nil, fmt.Errorf("config: prod profile requires a non-empty, non-dev JWT secret (SAGA_JWT_SECRET)")
		}
	}

	return &c, nil
}
