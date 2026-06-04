package http

import (
	"time"

	authpkg "banka1/trading-service-go/internal/auth"
	"banka1/trading-service-go/internal/platform"

	gpauth "banka1/go-platform/auth"
	"github.com/golang-jwt/jwt/v5"
)

func newTestJWT() *gpauth.Service {
	return authpkg.NewJWTService(platform.JWTConfig{
		Secret:           "test-secret-key-123",
		Issuer:           "banka1",
		IDClaim:          "id",
		RolesClaim:       "roles",
		PermissionsClaim: "permissions",
	})
}

func makeTestToken(svc *gpauth.Service, role string) string {
	_ = svc
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":         "banka1",
		"id":          float64(1),
		"sub":         "user@bank.io",
		"exp":         time.Now().Add(time.Hour).Unix(),
		"roles":       role,
		"permissions": []string{},
	})
	raw, _ := tok.SignedString([]byte("test-secret-key-123"))
	return raw
}
