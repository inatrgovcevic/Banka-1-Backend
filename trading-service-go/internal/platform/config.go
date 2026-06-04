package platform

import (
	"os"
	"strconv"
	"strings"
)

// Config holds the trading-service-go runtime configuration. Defaults match the
// docker-compose.go-trading.yml overlay and docs/go-platform.md (REST 18088,
// gRPC 19088, shared `trading` database).
type Config struct {
	ServerPort string
	GRPCPort   string
	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string
	JWT        JWTConfig
	CORS       CORSConfig
	Services   ServicesConfig

	// OrderSchedulersEnabled gates the order cron jobs (daily actuary limit reset,
	// 15-min expired-order auto-decline). OFF by default: during coexistence the
	// Java trading-service still runs these against the same rows, so enabling both
	// would double-process. Flip to true at cut-over (when Java is retired).
	OrderSchedulersEnabled bool

	// RecurringOrderSchedulerEnabled gates the Celina 3.6 standing-order firing
	// cron (0 */15 * * * *, mirrors RecurringOrderScheduler). OFF by default for
	// the same coexistence reason as the other schedulers — if a Java order-service
	// also fires recurring_orders rows, both would double-place orders.
	RecurringOrderSchedulerEnabled bool

	// TaxSchedulerEnabled gates the monthly capital-gains collection cron
	// (0 0 0 1 * *, previous month). OFF by default during coexistence — Java runs
	// the same job on the same rows (idempotent via the tax_charges unique
	// constraints, but no reason to double-run). Flip to true at cut-over.
	TaxSchedulerEnabled bool

	// TaxCapitalGainsRate mirrors banka.tax.capital-gains-rate (default 0.15). Held
	// as a string and parsed to a decimal in NewApp so a regulatory rate change
	// needs only an env change, no redeploy.
	TaxCapitalGainsRate string

	// DividendSchedulerEnabled gates the WP-14 quarterly dividend payout cron
	// (daily 0 0 1 * * *, self-gated to the last business day of Mar/Jun/Sep/Dec
	// — mirrors DividendScheduler). OFF by default during coexistence; the
	// ADMIN-only POST /dividends/trigger covers manual/E2E runs either way.
	DividendSchedulerEnabled bool

	// P5 funds — saga delivery + gates.
	//
	// SagaEventsExchange is the topic exchange the funds service PUBLISHES on for
	// fund.subscribe.requested / fund.redeem.requested /
	// fund.redeem.with-liquidation.requested. saga-orchestrator-service binds.
	SagaEventsExchange string
	// SagaResultsExchange is the topic exchange the funds service CONSUMES from
	// for saga.FUND_SUBSCRIBE.STEP_1.fund.{success,failure} etc.
	SagaResultsExchange string
	// FundSagaConsumersEnabled gates the Go fund saga consumers. OFF during
	// coexistence — Java listeners own the durable trading.fund.* queues; if Go
	// also binds, the broker round-robins results → half-processed sagas.
	FundSagaConsumersEnabled bool
	// FundSnapshotSchedulerEnabled gates the daily fund-value snapshot cron
	// (0 10 0 * * *). OFF during coexistence — Java still runs the same daily job.
	FundSnapshotSchedulerEnabled bool

	// P6 OTC — schedulers + saga consumer gates (mutations stay live, matching
	// the P3 order / P5 funds precedent: those also write the shared `portfolio`
	// table and were not mutation-gated).
	//
	// OtcSchedulersEnabled gates the two OTC cron jobs (0 5 0 * * * expire overdue
	// ACTIVE contracts + release stock; 0 30 8 * * * expiry reminders). OFF during
	// coexistence — the Java trading-service still runs them on the same rows.
	OtcSchedulersEnabled bool
	// OtcSagaConsumersEnabled gates the three OTC saga-result consumers on
	// saga.events (trading.otc.premium.{completed,failed}, trading.otc.exercise.
	// completed). OFF during coexistence — the Java listeners own those durable
	// queues; binding from both sides would round-robin deliveries. Flip at cut-over.
	OtcSagaConsumersEnabled bool
	// OtcReminderDays mirrors otc.contract.expiration-notification-days (default 3):
	// how many days before settlement the reminder cron fires (D-N).
	OtcReminderDays int

	// AuditConsumerEnabled gates the WP-2 audit.# consumer (durable queue
	// audit-log-queue on employee.events — the sink for other services' audit
	// events, e.g. user-service-go's EMPLOYEE_PERMISSIONS_CHANGED). OFF by
	// default during coexistence: only one consumer (the Java trading-service
	// or this one) may own the durable queue. Trading's OWN order-decision
	// events are recorded by direct insert and do not need the consumer.
	AuditConsumerEnabled bool

	// P7 interbank — RoutingNumber mirrors interbank.my-routing-number
	// (=${BANKA1_ROUTING_NUMBER:111}): this bank's interbank routing number,
	// advertised in the /internal/interbank/public-stocks foreign-bank id
	// (C-<userId>). banka1 = 111. The interbank 2PC primitives are synchronous —
	// no saga / scheduler / consumer gate exists for P7.
	RoutingNumber int
}

type JWTConfig struct {
	Secret           string
	Issuer           string
	IDClaim          string
	RolesClaim       string
	PermissionsClaim string
	// ExpirationMillis mirrors banka.security.expiration-time (ms). Used as the
	// TTL for the SERVICE token minted for outbound service-to-service calls.
	ExpirationMillis int64
}

type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
}

// ServicesConfig holds base URLs for the sibling services trading-service-go
// calls. Populated now; consumed as domains land in later phases (P2+).
type ServicesConfig struct {
	UserURL        string
	BankingCoreURL string
	MarketURL      string
}

func LoadConfig() Config {
	return Config{
		ServerPort: getEnv("SERVER_PORT", "18088"),
		GRPCPort:   getEnv("GRPC_PORT", "19088"),
		DBHost:     getEnv("TRADING_DB_HOST", "postgres"),
		DBPort:     getEnv("TRADING_DB_PORT", "5432"),
		DBName:     getEnv("TRADING_DB_NAME", "trading"),
		DBUser:     getEnv("TRADING_DB_USER", "postgres"),
		DBPassword: getEnv("TRADING_DB_PASSWORD", "postgres"),
		JWT: JWTConfig{
			Secret:           getEnv("JWT_SECRET", "development_trading_service_secret_123456"),
			Issuer:           getEnv("BANKA_SECURITY_ISSUER", "banka1"),
			IDClaim:          getEnv("BANKA_SECURITY_ID_CLAIM", "id"),
			RolesClaim:       getEnv("BANKA_SECURITY_ROLES_CLAIM", "roles"),
			PermissionsClaim: getEnv("BANKA_SECURITY_PERMISSIONS_CLAIM", "permissions"),
			ExpirationMillis: getEnvInt64("BANKA_SECURITY_EXPIRATION_TIME", 3600000),
		},
		CORS: CORSConfig{
			AllowedOrigins: splitCSV(getEnv("BANKA_SECURITY_CORS_ALLOWED_ORIGINS", "http://localhost:4200")),
			AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		},
		Services: ServicesConfig{
			UserURL:        getEnv("SERVICES_USER_URL", "http://user-service:8081"),
			BankingCoreURL: getEnv("SERVICES_BANKING_CORE_URL", "http://banking-core-service:8084"),
			MarketURL:      getEnv("SERVICES_MARKET_URL", "http://market-service:8085"),
		},
		OrderSchedulersEnabled:         getEnvBool("ORDER_SCHEDULERS_ENABLED", false),
		RecurringOrderSchedulerEnabled: getEnvBool("RECURRING_ORDER_SCHEDULER_ENABLED", false),
		TaxSchedulerEnabled:            getEnvBool("TAX_SCHEDULER_ENABLED", false),
		DividendSchedulerEnabled:       getEnvBool("DIVIDEND_SCHEDULER_ENABLED", false),
		TaxCapitalGainsRate:            getEnv("BANKA_TAX_CAPITAL_GAINS_RATE", "0.15"),
		SagaEventsExchange:             getEnv("SAGA_EVENTS_EXCHANGE", "saga.events"),
		SagaResultsExchange:            getEnv("SAGA_RESULTS_EXCHANGE", "saga.exchange"),
		FundSagaConsumersEnabled:       getEnvBool("FUND_SAGA_CONSUMERS_ENABLED", false),
		FundSnapshotSchedulerEnabled:   getEnvBool("FUND_SNAPSHOT_SCHEDULER_ENABLED", false),
		OtcSchedulersEnabled:           getEnvBool("OTC_SCHEDULERS_ENABLED", false),
		OtcSagaConsumersEnabled:        getEnvBool("OTC_SAGA_CONSUMERS_ENABLED", false),
		OtcReminderDays:                int(getEnvInt64("OTC_CONTRACT_EXPIRATION_NOTIFICATION_DAYS", 3)),
		AuditConsumerEnabled:           getEnvBool("AUDIT_CONSUMER_ENABLED", false),
		RoutingNumber:                  int(getEnvInt64("BANKA1_ROUTING_NUMBER", 111)),
	}
}

// DatabaseURL builds the pgx connection string (same shape as market-service-go).
func (c Config) DatabaseURL() string {
	return "postgres://" + c.DBUser + ":" + c.DBPassword + "@" + c.DBHost + ":" + c.DBPort + "/" + c.DBName
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
