package platform_test

import (
	"testing"

	"banka1/user-service-go/internal/platform"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndVerifyPassword_RoundTrip(t *testing.T) {
	t.Parallel()
	hash, err := platform.HashPassword("my-secret-password")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.True(t, platform.VerifyPassword("my-secret-password", hash))
}

func TestVerifyPassword_WrongPassword_ReturnsFalse(t *testing.T) {
	t.Parallel()
	hash, _ := platform.HashPassword("correct")
	assert.False(t, platform.VerifyPassword("wrong", hash))
}

func TestVerifyPassword_InvalidHash_ReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, platform.VerifyPassword("pass", "not-a-valid-hash"))
}

func TestVerifyPassword_TamperedHash_ReturnsFalse(t *testing.T) {
	t.Parallel()
	hash, _ := platform.HashPassword("password")
	tampered := hash[:len(hash)-3] + "XXX"
	assert.False(t, platform.VerifyPassword("password", tampered))
}

func TestHashPassword_ProducesDifferentHashesForSamePassword(t *testing.T) {
	t.Parallel()
	h1, _ := platform.HashPassword("same")
	h2, _ := platform.HashPassword("same")
	assert.NotEqual(t, h1, h2, "each call must produce a unique salt")
}
