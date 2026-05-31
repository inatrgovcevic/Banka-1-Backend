// Package auth is the Go equivalent of the Java security-lib. It owns
// JWT parsing/minting, the request Principal, role/permission checks, and
// the permit-all matcher.
//
// All defaults match security-lib so HS256 tokens issued by user-service or
// any Java service are accepted by Go services and vice versa.
package auth

import (
	"time"

	"banka1/go-platform/config"
)

// Config matches the Java banka.security.* properties. Build it with
// LoadConfig() to pull values from the environment.
type Config struct {
	Secret              string
	Issuer              string
	IDClaim             string
	RolesClaim          string
	PermissionsClaim    string
	EmailClaim          string
	AccessTokenDuration time.Duration
}

// LoadConfig reads JWT/security env vars with security-lib-compatible names
// and defaults.
func LoadConfig() Config {
	return Config{
		Secret:              config.Env("JWT_SECRET", ""),
		Issuer:              config.Env("BANKA_SECURITY_ISSUER", "banka1"),
		IDClaim:             config.Env("BANKA_SECURITY_ID_CLAIM", "id"),
		RolesClaim:          config.Env("BANKA_SECURITY_ROLES_CLAIM", "roles"),
		PermissionsClaim:    config.Env("BANKA_SECURITY_PERMISSIONS_CLAIM", "permissions"),
		EmailClaim:          config.Env("BANKA_SECURITY_EMAIL_CLAIM", "email"),
		AccessTokenDuration: time.Duration(config.EnvInt("BANKA_SECURITY_EXPIRATION_TIME", 3600000)) * time.Millisecond,
	}
}
