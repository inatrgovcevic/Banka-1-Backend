// Package clients holds the outbound service-to-service HTTP clients used by the
// portfolio and actuary domains: market-service (listing/price/FX), banking-core
// account-service (account details + transfers) and user-service (employees).
//
// This layer is net-new versus market-service-go (which only calls API-key'd
// external providers). The Java order-service attaches a freshly-minted SERVICE
// JWT to EVERY outbound call (it does NOT propagate the caller's bearer — see
// ServiceJwtAuthInterceptor), so the Go port mints+caches a SERVICE token the
// same way via go-platform/auth.
package clients

import (
	"sync"
	"time"

	gpauth "banka1/go-platform/auth"
)

// ServiceTokenProvider mints and caches a SERVICE-role JWT, refreshing it shortly
// before expiry. Mirrors order-service ServiceJwtAuthInterceptor (reuse a signed
// service token until it nears expiry).
type ServiceTokenProvider struct {
	svc     *gpauth.Service
	subject string
	ttl     time.Duration
	buffer  time.Duration

	mu        sync.Mutex
	token     string
	refreshAt time.Time
}

// NewServiceTokenProvider builds a provider. ttl mirrors banka.security.expiration-time
// (default 1h). The refresh buffer matches Java: min(30s, max(1s, ttl/10)).
func NewServiceTokenProvider(svc *gpauth.Service, subject string, ttl time.Duration) *ServiceTokenProvider {
	if ttl <= 0 {
		ttl = time.Hour
	}
	buffer := ttl / 10
	if buffer > 30*time.Second {
		buffer = 30 * time.Second
	}
	if buffer < time.Second {
		buffer = time.Second
	}
	return &ServiceTokenProvider{svc: svc, subject: subject, ttl: ttl, buffer: buffer}
}

// Token returns a valid SERVICE bearer token, minting a new one when the cached
// token is missing or near expiry.
func (p *ServiceTokenProvider) Token() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Now().Before(p.refreshAt) {
		return p.token, nil
	}
	tok, err := p.svc.GenerateServiceToken(p.subject, p.ttl)
	if err != nil {
		return "", err
	}
	p.token = tok
	p.refreshAt = time.Now().Add(p.ttl - p.buffer)
	return tok, nil
}
