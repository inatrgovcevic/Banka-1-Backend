package platform

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsurePostgresDatabase(ctx context.Context, cfg Config) error {
	maintenanceURL := databaseURLFor(cfg, "postgres")
	pool, err := pgxpool.New(ctx, maintenanceURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return err
	}

	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, cfg.DBName).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}

	_, err = pool.Exec(ctx, `CREATE DATABASE `+quoteIdentifier(cfg.DBName)+` TEMPLATE template0`)
	return err
}

func OpenPostgres(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func databaseURLFor(cfg Config, name string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, name)
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
