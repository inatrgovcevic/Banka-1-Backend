package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServerPort string

	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string

	RabbitHost     string
	RabbitPort     string
	RabbitUsername string
	RabbitPassword string

	JWTSecret           string
	JWTIssuer           string
	JWTRoleClaim        string
	JWTPermissionsClaim string
	JWTExpiration       time.Duration

	NotificationExchange   string
	NotificationRoutingKey string
	VerificationRoutingKey string

	AccountServiceURL string
	MarketServiceURL  string
	UserServiceURL    string
	VerificationURL   string

	BankAccountNumber     string
	ExchangeAccountNumber string
	BankClientID          int64
	ExchangeClientID      int64

	MasterCardFXFeePercent  string
	MasterCardNetworkFee    string
	CardDefaultLimit        string
	VerificationTTLMinutes  int64
	VerificationMaxAttempts int64

	MigrationsEnabled bool
	SkipVerification  bool
}

func Load() Config {
	return Config{
		ServerPort: env("SERVER_PORT", "8084"),

		DBHost:     env("BANKING_CORE_DB_HOST", "postgres"),
		DBPort:     env("BANKING_CORE_DB_PORT", "5432"),
		DBName:     env("BANKING_CORE_DB_NAME", "banking_core"),
		DBUser:     firstEnv("BANKING_CORE_DB_USER", "POSTGRES_USER", "postgres"),
		DBPassword: firstEnv("BANKING_CORE_DB_PASSWORD", "POSTGRES_PASSWORD", "postgres"),

		RabbitHost:     env("RABBITMQ_HOST", "rabbitmq"),
		RabbitPort:     env("RABBITMQ_PORT", "5672"),
		RabbitUsername: env("RABBITMQ_USERNAME", "guest"),
		RabbitPassword: env("RABBITMQ_PASSWORD", "guest"),

		JWTSecret:           env("JWT_SECRET", ""),
		JWTIssuer:           env("BANKA_SECURITY_ISSUER", "banka1"),
		JWTRoleClaim:        env("BANKA_SECURITY_ROLES_CLAIM", "roles"),
		JWTPermissionsClaim: env("BANKA_SECURITY_PERMISSIONS_CLAIM", "permissions"),
		JWTExpiration:       time.Duration(envInt64("BANKA_SECURITY_EXPIRATION_TIME", 3600000)) * time.Millisecond,

		NotificationExchange:   env("NOTIFICATION_EXCHANGE", "employee.events"),
		NotificationRoutingKey: env("NOTIFICATION_ROUTING_KEY", "employee.#"),
		VerificationRoutingKey: firstEnv("VERIFICATION_ROUTING_KEY", "NOTIFICATION_VERIFICATION_ROUTING_KEY", "client.verification"),

		AccountServiceURL: env("SERVICES_ACCOUNT_URL", "http://localhost:8084"),
		MarketServiceURL:  env("SERVICES_EXCHANGE_URL", "http://market-service:8085"),
		UserServiceURL:    env("SERVICES_USER_URL", "http://user-service:8081"),
		VerificationURL:   env("SERVICES_VERIFICATION_URL", "http://localhost:8084/verification"),

		BankAccountNumber:     env("BANK_ACCOUNT_NUMBER", "111000110000000312"),
		ExchangeAccountNumber: env("EXCHANGE_ACCOUNT_NUMBER", "111000300000002012"),
		BankClientID:          envInt64("BANK_CLIENT_ID", -1),
		ExchangeClientID:      envInt64("EXCHANGE_CLIENT_ID", -3),

		MasterCardFXFeePercent:  env("CARD_MASTERCARD_FX_FEE_PERCENT", "0.015"),
		MasterCardNetworkFee:    env("CARD_MASTERCARD_FX_NETWORK_FEE_EUR", "0.30"),
		CardDefaultLimit:        env("CARD_CREATION_AUTOMATIC_DEFAULT_LIMIT", "1000000"),
		VerificationTTLMinutes:  envInt64("VERIFICATION_OTP_TTL_MINUTES", 5),
		VerificationMaxAttempts: envInt64("VERIFICATION_OTP_MAX_ATTEMPTS", 3),

		MigrationsEnabled: envBool("BANKING_CORE_GO_MIGRATIONS_ENABLED", true),
		SkipVerification:  envBool("SKIP_VERIFICATION", false),
	}
}

func (c Config) DatabaseURL() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.DBUser, c.DBPassword),
		Host:   c.DBHost + ":" + c.DBPort,
		Path:   c.DBName,
	}
	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()
	return u.String()
}

func (c Config) RabbitURL() string {
	u := &url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(c.RabbitUsername, c.RabbitPassword),
		Host:   c.RabbitHost + ":" + c.RabbitPort,
		Path:   "/",
	}
	return u.String()
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func firstEnv(first, second, fallback string) string {
	if value, ok := os.LookupEnv(first); ok && value != "" {
		return value
	}
	if value, ok := os.LookupEnv(second); ok && value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
