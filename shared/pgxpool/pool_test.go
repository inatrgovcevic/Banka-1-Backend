package pgxpool

import (
	"context"
	"os"
	"testing"
	"time"
)

// Tests require a running Postgres on TEST_DATABASE_URL.
// They skip cleanly when the env var is empty.
const skipMsg = "set TEST_DATABASE_URL to run pgxpool tests (e.g. postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable)"

func TestNew_Connects(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip(skipMsg)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := New(ctx, Config{URL: url, MaxConns: 4})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer pool.Close()
	if err := HealthCheck(ctx, pool); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
}

func TestNew_InvalidURL(t *testing.T) {
	ctx := context.Background()
	_, err := New(ctx, Config{URL: "this is not a url"})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}
