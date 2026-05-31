// Package rabbitmq is the shared AMQP layer for Go services. It owns
// connection management (with backoff retry), idempotent exchange/queue
// declaration, a JSON publisher that preserves the existing Java contracts
// (durable topic exchange, persistent JSON messages), and a consumer
// scaffold with manual ack, panic recovery, and idempotency hook.
package rabbitmq

import (
	"fmt"
	"time"

	"banka1/go-platform/config"
)

// Config matches the existing RABBITMQ_* and NOTIFICATION_EXCHANGE env vars
// every Java service consumes.
type Config struct {
	Host     string
	Port     string
	Username string
	Password string
	Exchange string
	// AllowNoop enables NoopPublisher fallback when the broker is unreachable.
	// Set true for services where messaging is best-effort (e.g. email
	// notifications); leave false where missing publish must fail the start.
	AllowNoop bool
	// MaxDialAttempts is the number of times Dial retries before giving up.
	MaxDialAttempts int
	// DialBackoff is the initial backoff; doubled each retry, capped at 10s.
	DialBackoff time.Duration
}

// LoadConfig returns a Config built from the standard env vars. The default
// exchange name is "employee.events" — the existing topic exchange every
// notification flow already uses.
func LoadConfig() Config {
	return Config{
		Host:            config.Env("RABBITMQ_HOST", "rabbitmq"),
		Port:            config.Env("RABBITMQ_PORT", "5672"),
		Username:        config.Env("RABBITMQ_USERNAME", "guest"),
		Password:        config.Env("RABBITMQ_PASSWORD", "guest"),
		Exchange:        config.Env("NOTIFICATION_EXCHANGE", "employee.events"),
		AllowNoop:       config.EnvBool("RABBITMQ_ALLOW_NOOP", false),
		MaxDialAttempts: config.EnvInt("RABBITMQ_MAX_DIAL_ATTEMPTS", 5),
		DialBackoff:     config.EnvDuration("RABBITMQ_DIAL_BACKOFF", time.Second),
	}
}

// URL returns the amqp:// URL this Config represents.
func (c Config) URL() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%s/", c.Username, c.Password, c.Host, c.Port)
}
