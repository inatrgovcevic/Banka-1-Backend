// Package db owns the pgxpool initialization, readiness ping, and the
// transaction helper every Go service uses for write paths.
//
// Schema ownership stays with whoever runs Liquibase today; this package
// does not run migrations.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"banka1/go-platform/health"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config is the per-service DB config. Build with LoadConfig or assemble by
// hand; the env-var names are deliberately neutral so each service can pass
// its own prefix.
type Config struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string
}

// URL returns the postgres:// connection string this Config maps to.
func (c Config) URL() string {
	mode := c.SSLMode
	if mode == "" {
		mode = "disable"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", c.User, c.Password, c.Host, c.Port, c.Name, mode)
}

// OpenPool dials Postgres and returns a configured pgxpool.Pool.
// The caller is responsible for closing it on shutdown.
func OpenPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: open pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: initial ping: %w", err)
	}
	return pool, nil
}

// Checker returns a health.Checker that pings pool with a short timeout. Use:
//
//	healthHandler.Register(db.Checker("postgres", pool))
func Checker(name string, pool *pgxpool.Pool) health.Checker {
	return health.CheckerFunc{
		Label: name,
		Fn: func(ctx context.Context) error {
			if pool == nil {
				return errors.New("db pool is nil")
			}
			return pool.Ping(ctx)
		},
	}
}

// RunInTx executes fn inside a transaction. The tx is committed on nil error
// and rolled back on any non-nil error or panic. Use for write paths that
// touch multiple statements.
func RunInTx(ctx context.Context, pool *pgxpool.Pool, opts pgx.TxOptions, fn func(pgx.Tx) error) (err error) {
	tx, err := pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return
		}
		if cerr := tx.Commit(ctx); cerr != nil {
			err = fmt.Errorf("db: commit tx: %w", cerr)
		}
	}()
	return fn(tx)
}
