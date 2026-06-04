package auth_test

import (
	"os"
	"testing"
	"time"

	"Banka1Back/credit-service-go/internal/auth"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secret = "test-secret"

func makeToken(claims jwt.MapClaims) string {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tok, _ := t.SignedString([]byte(secret))
	return "Bearer " + tok
}

func withSecret(t *testing.T) {
	t.Helper()
	os.Setenv("JWT_SECRET", secret)
	t.Cleanup(func() { os.Unsetenv("JWT_SECRET") })
}

func TestParseBearerToken_ValidToken_ReturnsUser(t *testing.T) {
	withSecret(t)
	tok := makeToken(jwt.MapClaims{
		"id":       float64(42),
		"sub":      "user@bank.io",
		"username": "jovan",
		"roles":    "CLIENT_BASIC",
		"exp":      time.Now().Add(time.Hour).Unix(),
	})

	user, err := auth.ParseBearerToken(tok)
	require.NoError(t, err)
	assert.Equal(t, int64(42), user.ID)
	assert.Equal(t, "user@bank.io", user.Email)
	assert.Equal(t, "jovan", user.Username)
	assert.Equal(t, "CLIENT_BASIC", user.Role)
}

func TestParseBearerToken_RolesAsArray_UsesFirst(t *testing.T) {
	withSecret(t)
	tok := makeToken(jwt.MapClaims{
		"id":    float64(1),
		"sub":   "u@bank.io",
		"roles": []interface{}{"ADMIN", "SUPERVISOR"},
		"exp":   time.Now().Add(time.Hour).Unix(),
	})

	user, err := auth.ParseBearerToken(tok)
	require.NoError(t, err)
	assert.Equal(t, "ADMIN", user.Role)
}

func TestParseBearerToken_EmptyHeader_ReturnsError(t *testing.T) {
	withSecret(t)
	_, err := auth.ParseBearerToken("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing authorization header")
}

func TestParseBearerToken_MissingBearerPrefix_ReturnsError(t *testing.T) {
	withSecret(t)
	_, err := auth.ParseBearerToken("Token abc123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid authorization header")
}

func TestParseBearerToken_JustBearer_ReturnsError(t *testing.T) {
	withSecret(t)
	_, err := auth.ParseBearerToken("Bearer")
	require.Error(t, err)
}

func TestParseBearerToken_MissingSecret_ReturnsError(t *testing.T) {
	os.Unsetenv("JWT_SECRET")
	_, err := auth.ParseBearerToken("Bearer some.token.here")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing JWT_SECRET")
}

func TestParseBearerToken_InvalidSignature_ReturnsError(t *testing.T) {
	withSecret(t)
	_, err := auth.ParseBearerToken("Bearer invalid.token.here")
	require.Error(t, err)
}

func TestParseBearerToken_ExpiredToken_ReturnsError(t *testing.T) {
	withSecret(t)
	tok := makeToken(jwt.MapClaims{
		"id":  float64(1),
		"sub": "u@bank.io",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})
	_, err := auth.ParseBearerToken(tok)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// User.HasRole / HasAnyRole
// ---------------------------------------------------------------------------

func TestUser_HasRole_Match(t *testing.T) {
	t.Parallel()
	u := auth.User{Role: "ADMIN"}
	assert.True(t, u.HasRole("ADMIN"))
}

func TestUser_HasRole_NoMatch(t *testing.T) {
	t.Parallel()
	u := auth.User{Role: "ADMIN"}
	assert.False(t, u.HasRole("BASIC"))
}

func TestUser_HasAnyRole_MatchFirst(t *testing.T) {
	t.Parallel()
	u := auth.User{Role: "ADMIN"}
	assert.True(t, u.HasAnyRole("ADMIN", "BASIC"))
}

func TestUser_HasAnyRole_MatchLast(t *testing.T) {
	t.Parallel()
	u := auth.User{Role: "CLIENT_BASIC"}
	assert.True(t, u.HasAnyRole("ADMIN", "CLIENT_BASIC"))
}

func TestUser_HasAnyRole_NoMatch(t *testing.T) {
	t.Parallel()
	u := auth.User{Role: "CLIENT_BASIC"}
	assert.False(t, u.HasAnyRole("ADMIN", "SUPERVISOR"))
}

func TestUser_HasAnyRole_EmptyList(t *testing.T) {
	t.Parallel()
	u := auth.User{Role: "ADMIN"}
	assert.False(t, u.HasAnyRole())
}
