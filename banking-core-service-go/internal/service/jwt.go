package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"banka1/banking-core-service-go/internal/config"
)

func serviceJWT(cfg config.Config) (string, error) {
	return serviceJWTAt(cfg, time.Now())
}

func serviceJWTAt(cfg config.Config, now time.Time) (string, error) {
	if strings.TrimSpace(cfg.JWTSecret) == "" {
		return "", errors.New("JWT_SECRET is not configured")
	}
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	claims := map[string]any{
		"sub":                   "banking-core-service",
		"iss":                   cfg.JWTIssuer,
		cfg.JWTRoleClaim:        "SERVICE",
		cfg.JWTPermissionsClaim: []string{},
		"exp":                   now.Add(cfg.JWTExpiration).Unix(),
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

type ServiceTokenCache struct {
	cfg       config.Config
	mu        sync.Mutex
	token     string
	refreshAt time.Time
	now       func() time.Time
}

func NewServiceTokenCache(cfg config.Config) *ServiceTokenCache {
	return &ServiceTokenCache{cfg: cfg, now: time.Now}
}

func (c *ServiceTokenCache) Token() (string, error) {
	if c == nil {
		return "", errors.New("service token cache is nil")
	}
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && now.Before(c.refreshAt) {
		return c.token, nil
	}
	token, err := serviceJWTAt(c.cfg, now)
	if err != nil {
		return "", err
	}
	validity := c.cfg.JWTExpiration
	if validity < time.Second {
		validity = time.Second
	}
	refreshBuffer := validity / 10
	if refreshBuffer < time.Second {
		refreshBuffer = time.Second
	}
	if refreshBuffer > 30*time.Second {
		refreshBuffer = 30 * time.Second
	}
	c.token = token
	c.refreshAt = now.Add(validity).Add(-refreshBuffer)
	return token, nil
}
