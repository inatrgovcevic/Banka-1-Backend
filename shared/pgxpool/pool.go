// Package pgxpool wraps github.com/jackc/pgx/v5/pgxpool with a small config struct
// and a health-check helper. Callers pass a Postgres URL plus optional pool tuning
// (max/min conns, lifetime, idle time). The returned *pgxpool.Pool must be closed
// by the caller at shutdown.
package pgxpool

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config controls pool sizing. Zero values mean "use pgx defaults".
type Config struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// New parses URL, applies the optional tuning, and returns a connected pool.
// The caller is responsible for closing the returned pool with .Close().
func New(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, err
	}
	if cfg.MaxConns > 0 {
		pcfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		pcfg.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		pcfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		pcfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	return pgxpool.NewWithConfig(ctx, pcfg)
}

// HealthCheck pings the pool. Used by /actuator/health-style endpoints.
func HealthCheck(ctx context.Context, pool *pgxpool.Pool) error {
	return pool.Ping(ctx)
}
