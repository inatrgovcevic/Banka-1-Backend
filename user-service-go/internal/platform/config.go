package platform

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServerPort string
	DB         DBConfig
	JWT        JWTConfig
	User       UserConfig
	Services   ServicesConfig
	CORS       CORSConfig
	RabbitMQ   RabbitConfig
	Email      EmailConfig
	JMBG       JMBGConfig
}

type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string
}

type JWTConfig struct {
	Secret              string
	Issuer              string
	IDClaim             string
	RolesClaim          string
	PermissionsClaim    string
	AccessTokenDuration time.Duration
}

type UserConfig struct {
	ConfirmationTokenDuration time.Duration
	RefreshTokenDuration      time.Duration
	EmployeeLockoutAttempts   int
	EmployeeLockoutDuration   time.Duration
}

type ServicesConfig struct {
	TradingURL string
}

type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
}

type RabbitConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	Exchange string
}

type EmailConfig struct {
	EmployeeResetPasswordURL string
	EmployeeActivateURL      string
	ClientResetPasswordURL   string
	ClientActivateURL        string
}

type JMBGConfig struct {
	AESKeyBase64 string
}

func LoadConfig() Config {
	return Config{
		ServerPort: env("SERVER_PORT", "8081"),
		DB: DBConfig{
			Host:     env("USER_SERVICE_DB_HOST", env("POSTGRES_HOST", "localhost")),
			Port:     env("USER_SERVICE_DB_PORT", env("POSTGRES_PORT", "5432")),
			Name:     env("USER_SERVICE_DB_NAME", env("POSTGRES_DB", "user_service")),
			User:     env("USER_SERVICE_DB_USER", env("POSTGRES_USER", "postgres")),
			Password: env("USER_SERVICE_DB_PASSWORD", env("POSTGRES_PASSWORD", "postgres")),
			SSLMode:  env("POSTGRES_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			Secret:              env("JWT_SECRET", ""),
			Issuer:              env("BANKA_SECURITY_ISSUER", "banka1"),
			IDClaim:             env("BANKA_SECURITY_ID", "id"),
			RolesClaim:          env("BANKA_SECURITY_ROLES_CLAIM", "roles"),
			PermissionsClaim:    env("BANKA_SECURITY_PERMISSIONS_CLAIM", "permissions"),
			AccessTokenDuration: time.Duration(envInt("BANKA_SECURITY_EXPIRATION_TIME", 3600000)) * time.Millisecond,
		},
		User: UserConfig{
			ConfirmationTokenDuration: time.Duration(envInt("TOKEN_CONFIRMATION_EXPIRATION_MINUTES", 15)) * time.Minute,
			RefreshTokenDuration:      time.Duration(envInt("TOKEN_REFRESH_EXPIRATION_DAYS", 7)) * 24 * time.Hour,
			EmployeeLockoutAttempts:   envInt("ACCOUNT_LOCKOUT_MAX_ATTEMPTS", 5),
			EmployeeLockoutDuration:   time.Duration(envInt("ACCOUNT_LOCKOUT_DURATION_MINUTES", 10)) * time.Minute,
		},
		Services: ServicesConfig{
			TradingURL: env("SERVICES_TRADING_URL", "http://trading-service:8088"),
		},
		CORS: CORSConfig{
			AllowedOrigins: splitEnv("BANKA_SECURITY_CORS_ALLOWED_ORIGINS", "http://localhost:4200,http://localhost:3000"),
			AllowedMethods: splitEnv("BANKA_SECURITY_CORS_ALLOWED_METHODS", "GET,POST,PUT,PATCH,DELETE,OPTIONS"),
		},
		RabbitMQ: RabbitConfig{
			Host:     env("RABBITMQ_HOST", "localhost"),
			Port:     env("RABBITMQ_PORT", "5672"),
			Username: env("RABBITMQ_USERNAME", "guest"),
			Password: env("RABBITMQ_PASSWORD", "guest"),
			Exchange: env("NOTIFICATION_EXCHANGE", env("RABBITMQ_EXCHANGE", "employee.events")),
		},
		Email: EmailConfig{
			EmployeeResetPasswordURL: env("USER_RESET_PASSWORD_URL", "http://localhost:4200/auth/reset-password?token="),
			EmployeeActivateURL:      env("USER_ACTIVATE_ACCOUNT_URL", "http://localhost:4200/auth/activate-account?token="),
			ClientResetPasswordURL:   env("CLIENT_RESET_PASSWORD_URL", "http://localhost:4200/auth/reset-password?token="),
			ClientActivateURL:        env("CLIENT_ACTIVATE_ACCOUNT_URL", "http://localhost:4200/auth/activate-client?token="),
		},
		JMBG: JMBGConfig{
			AESKeyBase64: env("BANKA_SECURITY_JMBG_AES_KEY", "VGhpc0lzQURldk9ubHkzMkJ5dGVBRVNLZXktMTIzNDU="),
		},
	}
}

func (c Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.DB.User, c.DB.Password, c.DB.Host, c.DB.Port, c.DB.Name, c.DB.SSLMode)
}

func (c Config) RabbitURL() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%s/",
		c.RabbitMQ.Username, c.RabbitMQ.Password, c.RabbitMQ.Host, c.RabbitMQ.Port)
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func splitEnv(key, fallback string) []string {
	raw := env(key, fallback)
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
}
