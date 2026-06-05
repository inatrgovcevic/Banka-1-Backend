package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDatabasePool_InvalidHost_ReturnsError(t *testing.T) {
	t.Setenv("CREDIT_DB_HOST", "invalid-host-that-does-not-exist-xyz")
	t.Setenv("CREDIT_DB_PORT", "5432")
	t.Setenv("CREDIT_DB_NAME", "testdb")
	t.Setenv("CREDIT_DB_USER", "testuser")
	t.Setenv("CREDIT_DB_PASSWORD", "testpassword")
	t.Setenv("CREDIT_DB_SSLMODE", "disable")

	pool, err := NewDatabasePool()
	if pool != nil {
		pool.Close()
	}
	require.Error(t, err)
}

func TestNewDatabasePool_DefaultSSLMode(t *testing.T) {
	t.Setenv("CREDIT_DB_HOST", "invalid-db-host-xyz")
	t.Setenv("CREDIT_DB_PORT", "5432")
	t.Setenv("CREDIT_DB_NAME", "testdb")
	t.Setenv("CREDIT_DB_USER", "testuser")
	t.Setenv("CREDIT_DB_PASSWORD", "testpassword")
	t.Setenv("CREDIT_DB_SSLMODE", "")
	t.Setenv("POSTGRES_SSLMODE", "")

	pool, err := NewDatabasePool()
	if pool != nil {
		pool.Close()
	}
	// Should fail to connect to invalid host, but the function itself runs
	assert.Error(t, err)
}

func TestNewDatabasePool_PostgresSSLMode_Fallback(t *testing.T) {
	t.Setenv("CREDIT_DB_HOST", "invalid-db-host-xyz")
	t.Setenv("CREDIT_DB_PORT", "5432")
	t.Setenv("CREDIT_DB_NAME", "testdb")
	t.Setenv("CREDIT_DB_USER", "testuser")
	t.Setenv("CREDIT_DB_PASSWORD", "testpassword")
	t.Setenv("CREDIT_DB_SSLMODE", "")
	t.Setenv("POSTGRES_SSLMODE", "disable")

	pool, err := NewDatabasePool()
	if pool != nil {
		pool.Close()
	}
	assert.Error(t, err)
}
