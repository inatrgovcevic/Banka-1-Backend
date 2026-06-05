package config

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewDatabasePool() (*pgxpool.Pool, error) {
	host := os.Getenv("CREDIT_DB_HOST")
	port := os.Getenv("CREDIT_DB_PORT")
	name := os.Getenv("CREDIT_DB_NAME")
	user := os.Getenv("CREDIT_DB_USER")
	password := os.Getenv("CREDIT_DB_PASSWORD")
	sslMode := os.Getenv("CREDIT_DB_SSLMODE")
	if sslMode == "" {
		sslMode = os.Getenv("POSTGRES_SSLMODE")
	}
	if sslMode == "" {
		sslMode = "disable"
	}

	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user,
		password,
		host,
		port,
		name,
		sslMode,
	)

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}

	err = pool.Ping(context.Background())
	if err != nil {
		return nil, err
	}

	return pool, nil
}
