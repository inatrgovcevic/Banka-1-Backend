package service

import (
	"testing"
	"time"

	"banka1/banking-core-service-go/internal/config"
)

func TestServiceTokenCacheReusesTokenBeforeRefresh(t *testing.T) {
	now := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	cache := NewServiceTokenCache(config.Config{
		JWTSecret:           "01234567890123456789012345678901",
		JWTIssuer:           "banka1",
		JWTRoleClaim:        "roles",
		JWTPermissionsClaim: "permissions",
		JWTExpiration:       time.Hour,
	})
	cache.now = func() time.Time { return now }

	first, err := cache.Token()
	if err != nil {
		t.Fatalf("Token() first error: %v", err)
	}
	second, err := cache.Token()
	if err != nil {
		t.Fatalf("Token() second error: %v", err)
	}
	if first != second {
		t.Fatal("Token() regenerated before refreshAt")
	}
}

func TestServiceTokenCacheRefreshesNearExpiry(t *testing.T) {
	now := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	cache := NewServiceTokenCache(config.Config{
		JWTSecret:           "01234567890123456789012345678901",
		JWTIssuer:           "banka1",
		JWTRoleClaim:        "roles",
		JWTPermissionsClaim: "permissions",
		JWTExpiration:       time.Hour,
	})
	cache.now = func() time.Time { return now }

	first, err := cache.Token()
	if err != nil {
		t.Fatalf("Token() first error: %v", err)
	}
	now = now.Add(59*time.Minute + 31*time.Second)
	second, err := cache.Token()
	if err != nil {
		t.Fatalf("Token() second error: %v", err)
	}
	if first == second {
		t.Fatal("Token() did not refresh after refreshAt")
	}
}
