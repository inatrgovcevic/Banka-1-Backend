package platform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"

	gpauth "banka1/go-platform/auth"
)

// Principal aliases banka1/go-platform/auth.Principal. Existing handlers
// keep their import path.
type Principal = gpauth.Principal

// JWTService aliases the shared go-platform auth.Service so this service's
// existing API surface stays compatible with downstream code.
type JWTService = gpauth.Service

// NewJWTService wraps gpauth.NewService with the user-service JWTConfig type.
func NewJWTService(cfg JWTConfig) *JWTService {
	return gpauth.NewService(gpauth.Config{
		Secret:              cfg.Secret,
		Issuer:              cfg.Issuer,
		IDClaim:             cfg.IDClaim,
		RolesClaim:          cfg.RolesClaim,
		PermissionsClaim:    cfg.PermissionsClaim,
		EmailClaim:          "email",
		AccessTokenDuration: cfg.AccessTokenDuration,
	})
}

// PrincipalFromContext delegates to gpauth.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	return gpauth.PrincipalFromContext(ctx)
}

// RequireAnyRole keeps the legacy name; delegates to gpauth.RequireRoles.
func RequireAnyRole(roles ...string) func(http.Handler) http.Handler {
	return gpauth.RequireRoles(roles...)
}

// RandomURLToken returns a URL-safe 32-byte random token.
func RandomURLToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// SHA256Hex hashes value as hex SHA-256.
func SHA256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

