package config

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config is the root configuration container for the notification service.
// All values are sourced from environment variables, preserving compatibility
// with the existing deployment infrastructure.
type Config struct {
	Server ServerConfig
	AMQP   AMQPConfig
	Rabbit RabbitConfig
	DB     DBConfig
	SMTP   SMTPConfig
	Retry  RetryConfig
}

type ServerConfig struct {
	Host     string
	HTTPPort int
}

type AMQPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	VHost    string
}

func (a AMQPConfig) URL() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		a.Username, a.Password, a.Host, a.Port, a.VHost)
}

type RabbitConfig struct {
	Exchange        string
	Queue           string
	BindingPatterns []string
	Prefetch        int
	Workers         int
}

type DBConfig struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
}

type SMTPConfig struct {
	Host               string
	Port               int
	Username           string
	Password           string
	StartTLS           bool
	AuthRequired       bool
	InsecureSkipVerify bool
}

type RetryConfig struct {
	MaxRetries          int
	DelaySeconds        int
	SchedulerIntervalMs int
}

// Load constructs a Config by reading environment variables.
// Returns an error only for invalid numeric values.
func Load() (*Config, error) {
	amqpPort, err := parseInt(getenv("RABBITMQ_PORT", "5672"), "RABBITMQ_PORT")
	if err != nil {
		return nil, err
	}
	dbPort, err := parseInt(getenv("POSTGRES_PORT", "5432"), "POSTGRES_PORT")
	if err != nil {
		return nil, err
	}
	smtpPort, err := parseInt(getenv("MAIL_PORT", "587"), "MAIL_PORT")
	if err != nil {
		return nil, err
	}
	httpPort, err := parseInt(getenv("NOTIFICATION_SERVICE_PORT", "8006"), "NOTIFICATION_SERVICE_PORT")
	if err != nil {
		return nil, err
	}
	prefetch, err := parseInt(getenv("RABBITMQ_PREFETCH", "10"), "RABBITMQ_PREFETCH")
	if err != nil {
		return nil, err
	}
	workers, err := parseInt(getenv("RABBITMQ_WORKERS", "4"), "RABBITMQ_WORKERS")
	if err != nil {
		return nil, err
	}
	maxRetries, err := parseInt(getenv("NOTIFICATION_RETRY_MAX_RETRIES", "4"), "NOTIFICATION_RETRY_MAX_RETRIES")
	if err != nil {
		return nil, err
	}
	delaySecs, err := parseInt(getenv("NOTIFICATION_RETRY_DELAY_SECONDS", "5"), "NOTIFICATION_RETRY_DELAY_SECONDS")
	if err != nil {
		return nil, err
	}
	schedulerMs, err := parseInt(getenv("NOTIFICATION_RETRY_SCHEDULER_DELAY_MILLIS", "1000"), "NOTIFICATION_RETRY_SCHEDULER_DELAY_MILLIS")
	if err != nil {
		return nil, err
	}

	bindingPatterns := []string{
		getenv("NOTIFICATION_ROUTING_KEY", "employee.#"),
		getenv("NOTIFICATION_CLIENT_ROUTING_KEY", "client.#"),
		getenv("NOTIFICATION_CARD_ROUTING_KEY", "card.#"),
		getenv("NOTIFICATION_CREDIT_ROUTING_KEY", "credit.#"),
		getenv("NOTIFICATION_TAX_ROUTING_KEY", "tax.#"),
		getenv("NOTIFICATION_OTC_ROUTING_KEY", "otc.#"),
		getenv("NOTIFICATION_VERIFICATION_ROUTING_KEY", "verification.#"),
		getenv("NOTIFICATION_ACCOUNT_ROUTING_KEY", "account.#"),
		getenv("NOTIFICATION_TRANSACTION_ROUTING_KEY", "transaction.#"),
		getenv("NOTIFICATION_PRICE_ROUTING_KEY", "price.#"),
		getenv("NOTIFICATION_ORDER_ROUTING_KEY", "order.#"),
	}

	return &Config{
		Server: ServerConfig{
			Host:     getenv("NOTIFICATION_SERVICE_HOST", "0.0.0.0"),
			HTTPPort: httpPort,
		},
		AMQP: AMQPConfig{
			Host:     getenv("RABBITMQ_HOST", "localhost"),
			Port:     amqpPort,
			Username: getenv("RABBITMQ_USERNAME", "guest"),
			Password: getenv("RABBITMQ_PASSWORD", "guest"),
			VHost:    getenv("RABBITMQ_VHOST", ""),
		},
		Rabbit: RabbitConfig{
			Exchange:        getenv("NOTIFICATION_EXCHANGE", "employee.events"),
			Queue:           getenv("NOTIFICATION_QUEUE", "notification-service-queue"),
			BindingPatterns: bindingPatterns,
			Prefetch:        prefetch,
			Workers:         workers,
		},
		DB: DBConfig{
			Host:     getenv("POSTGRES_HOST", "localhost"),
			Port:     dbPort,
			Name:     getenv("POSTGRES_DB", "notification_db"),
			User:     getenv("POSTGRES_USER", "postgres"),
			Password: getenv("POSTGRES_PASSWORD", "postgres"),
		},
		SMTP: SMTPConfig{
			Host:               getenv("MAIL_HOST", "smtp.gmail.com"),
			Port:               smtpPort,
			Username:           getenv("MAIL_USERNAME", ""),
			Password:           getenv("MAIL_PASSWORD", ""),
			StartTLS:           getenvBool("MAIL_SMTP_STARTTLS", true),
			AuthRequired:       getenvBool("MAIL_SMTP_AUTH", true),
			InsecureSkipVerify: getenvBool("MAIL_SMTP_INSECURE_SKIP_VERIFY", false),
		},
		Retry: RetryConfig{
			MaxRetries:          maxRetries,
			DelaySeconds:        delaySecs,
			SchedulerIntervalMs: schedulerMs,
		},
	}, nil
}

// NewDatabasePool opens a pgxpool connection using the DB config section.
func NewDatabasePool(cfg DBConfig) (*pgxpool.Pool, error) {
	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name,
	)

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func parseInt(s, varName string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value for %s=%q: %w", varName, s, err)
	}
	return n, nil
}
