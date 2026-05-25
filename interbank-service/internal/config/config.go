// Package config loads interbank-service configuration from environment variables.
// All variables are prefixed with INTERBANK_ (handled by shared/config.Load).
package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	sharedConfig "github.com/raf-si-2025/banka-1-go/shared/config"
)

// Config is the top-level configuration struct for interbank-service.
//
// kelseyhightower/envconfig builds env var names from the load prefix plus
// the field path: ServerConfig.HTTPPort → INTERBANK_SERVER_HTTP_PORT, and
// nested struct InterbankConfig.MyRoutingNumber → INTERBANK_INTERBANK_MY_ROUTING_NUMBER
// (yes, double-prefixed). The docker-compose.yml and dev/test env files MUST
// use the double-prefix names for fields under Interbank — see setup/docker-compose.yml
// for the canonical list.
type Config struct {
	Server    ServerConfig
	DB        DBConfig
	JWT       JWTConfig
	Interbank InterbankConfig
}

// ServerConfig holds HTTP server settings.
// Env prefix: INTERBANK_SERVER_*.
type ServerConfig struct {
	HTTPPort        int           `envconfig:"HTTP_PORT" default:"8091"`
	GRPCPort        int           `envconfig:"GRPC_PORT" default:"9091"`
	LogLevel        slog.Level    `envconfig:"LOG_LEVEL" default:"info"`
	LogJSON         bool          `envconfig:"LOG_JSON" default:"true"`
	ReadTimeout     time.Duration `envconfig:"READ_TIMEOUT" default:"30s"`
	WriteTimeout    time.Duration `envconfig:"WRITE_TIMEOUT" default:"65s"`
	ShutdownTimeout time.Duration `envconfig:"SHUTDOWN_TIMEOUT" default:"15s"`
	// MigrationsPath is the directory that contains the Goose SQL migration files.
	// In Docker the binary copies migrations to /migrations and uses the absolute
	// path as default.  Set INTERBANK_SERVER_MIGRATIONS_PATH=./migrations when
	// running outside a container (e.g. `go run ./cmd/interbank-service`).
	MigrationsPath string `envconfig:"MIGRATIONS_PATH" default:"/migrations"`
}

// DBConfig holds database settings.
// Env vars: INTERBANK_DB_URL (required), INTERBANK_DB_MAX_CONNS, INTERBANK_DB_MIN_CONNS.
type DBConfig struct {
	URL      string `envconfig:"URL" required:"true"`
	MaxConns int32  `envconfig:"MAX_CONNS" default:"10"`
	MinConns int32  `envconfig:"MIN_CONNS" default:"2"`
}

// JWTConfig holds settings for JWT issuance (S2S tokens) and verification (FE requests).
// Env vars: INTERBANK_JWT_SECRET (required), INTERBANK_JWT_ISSUER, INTERBANK_JWT_TTL.
type JWTConfig struct {
	Secret string        `envconfig:"SECRET" required:"true"`
	Issuer string        `envconfig:"ISSUER" default:"banka1"`
	TTL    time.Duration `envconfig:"TTL" default:"1h"`
}

// InterbankConfig holds all interbank-protocol–related settings.
type InterbankConfig struct {
	MyRoutingNumber int       `envconfig:"MY_ROUTING_NUMBER" default:"111"`
	MyDisplayName   string    `envconfig:"MY_BANK_DISPLAY_NAME" default:"Banka 1"`
	Partners        []Partner `envconfig:"PARTNERS"`
	MockPartner     MockConf  `envconfig:"MOCK_PARTNER"`
	Services        ServicesConfig
	Outbound        OutboundConfig
	Retry           RetryConfig
}

// Partner describes a remote bank used for outbound calls and inbound auth.
// This is the config-layer partner type; production wiring maps it to auth.Partner.
type Partner struct {
	Routing       int    `envconfig:"ROUTING_NUMBER"`
	DisplayName   string `envconfig:"DISPLAY_NAME"`
	BaseURL       string `envconfig:"BASE_URL"`
	InboundToken  string `envconfig:"INBOUND_TOKEN"`
	OutboundToken string `envconfig:"OUTBOUND_TOKEN"`
}

// MockConf controls the in-process mock Banka 2 controller.
type MockConf struct {
	Enabled bool `envconfig:"ENABLED" default:"false"`
}

// ServicesConfig holds base URLs for downstream Java services.
// Env prefix: INTERBANK_SERVICES_*.
type ServicesConfig struct {
	BankingCoreURL string        `envconfig:"BANKING_CORE_URL" default:"http://banking-core-service:8084"`
	TradingURL     string        `envconfig:"TRADING_URL" default:"http://trading-service:8088"`
	UserURL        string        `envconfig:"USER_URL" default:"http://user-service:8081"`
	Timeout        time.Duration `envconfig:"TIMEOUT" default:"5s"`
}

// OutboundConfig controls the outbound HTTP client used for inter-bank calls.
// Env prefix: INTERBANK_OUTBOUND_*.
type OutboundConfig struct {
	// Timeout covers the synchronous /accept call that can block up to 60 s.
	Timeout time.Duration `envconfig:"TIMEOUT" default:"60s"`
}

// RetryConfig controls the retry scheduler.
// Env prefix: INTERBANK_RETRY_*.
type RetryConfig struct {
	Interval   time.Duration `envconfig:"INTERVAL" default:"2m"`
	MaxRetries int           `envconfig:"MAX_RETRIES" default:"5"`
}

// Load reads all INTERBANK_* env vars into a Config struct.
// In production (INTERBANK_PROFILE != "dev" | "test"), it fail-fasts when any
// configured partner token is empty or starts with "dev-".
//
// PARTNER LOADING:
// kelseyhightower/envconfig v1.4.0 does not support indexed env vars for
// slice-of-struct (INTERBANK_INTERBANK_PARTNERS_0_*), so the Partners slice
// from envconfig is typically empty. As a fallback, this loader also reads
// INTERBANK_PARTNERS_JSON — a JSON-encoded []Partner — and merges/overrides
// the envconfig Partners list with it.
//
// Example:
//
//	export INTERBANK_PARTNERS_JSON='[{"Routing":222,"DisplayName":"Banka 2","BaseURL":"http://localhost:8081/","InboundToken":"xxx","OutboundToken":"yyy"}]'
func Load() (*Config, error) {
	var c Config
	if err := sharedConfig.Load("INTERBANK", &c); err != nil {
		return nil, fmt.Errorf("config: load env: %w", err)
	}

	// JSON fallback for partners — used in Docker/dev where indexed env vars
	// don't work with envconfig.
	if pj := os.Getenv("INTERBANK_PARTNERS_JSON"); pj != "" {
		var partners []Partner
		if err := json.Unmarshal([]byte(pj), &partners); err != nil {
			return nil, fmt.Errorf("config: parse INTERBANK_PARTNERS_JSON: %w", err)
		}
		c.Interbank.Partners = partners
	}

	profile := os.Getenv("INTERBANK_PROFILE")
	if profile == "" {
		profile = "prod"
	}

	if profile != "dev" && profile != "test" {
		for _, p := range c.Interbank.Partners {
			if p.InboundToken == "" || strings.HasPrefix(p.InboundToken, "dev-") {
				return nil, fmt.Errorf("config: prod profile: partner %d has dev/empty InboundToken", p.Routing)
			}
			if p.OutboundToken == "" || strings.HasPrefix(p.OutboundToken, "dev-") {
				return nil, fmt.Errorf("config: prod profile: partner %d has dev/empty OutboundToken", p.Routing)
			}
		}
	}

	return &c, nil
}
