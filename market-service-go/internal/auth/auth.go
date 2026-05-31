// Package auth is the in-service adapter onto banka1/go-platform/auth.
// It exists so handlers keep importing banka1/market-service-go/internal/auth
// while the real JWT/security implementation lives in the shared module.
package auth

import (
	"context"
	"net/http"

	"banka1/market-service-go/internal/platform"

	gpauth "banka1/go-platform/auth"
)

// Principal aliases the shared Principal so existing handlers keep their
// import paths.
type Principal = gpauth.Principal

// JWTService aliases the shared go-platform auth.Service.
type JWTService = gpauth.Service

// NewJWTService builds a Service from the market-service-go JWTConfig type.
func NewJWTService(cfg platform.JWTConfig) *JWTService {
	return gpauth.NewService(gpauth.Config{
		Secret:           cfg.Secret,
		Issuer:           cfg.Issuer,
		IDClaim:          cfg.IDClaim,
		RolesClaim:       cfg.RolesClaim,
		PermissionsClaim: cfg.PermissionsClaim,
		EmailClaim:       "email",
	})
}

// PrincipalFromContext delegates to gpauth.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	return gpauth.PrincipalFromContext(ctx)
}

// RequireRoles delegates to gpauth.RequireRoles.
func RequireRoles(roles ...string) func(http.Handler) http.Handler {
	return gpauth.RequireRoles(roles...)
}

// RequirePermissions delegates to gpauth.RequirePermissions.
func RequirePermissions(perms ...string) func(http.Handler) http.Handler {
	return gpauth.RequirePermissions(perms...)
}
