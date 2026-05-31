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

	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user,
		password,
		host,
		port,
		name,
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
