package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"banka1/banking-core-service-go/internal/config"
)

func serviceJWT(cfg config.Config) (string, error) {
	if strings.TrimSpace(cfg.JWTSecret) == "" {
		return "", errors.New("JWT_SECRET is not configured")
	}
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	claims := map[string]any{
		"sub":                   "banking-core-service",
		"iss":                   cfg.JWTIssuer,
		cfg.JWTRoleClaim:        "SERVICE",
		cfg.JWTPermissionsClaim: []string{},
		"exp":                   time.Now().Add(cfg.JWTExpiration).Unix(),
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	mac := hmac.New(sha256.New, []byte(cfg.JWTSecret))
	mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
