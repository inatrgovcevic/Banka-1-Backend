// Package auth provides HTTP middleware and helpers for the two authentication
// surfaces used by the inter-bank stack: X-Api-Key headers (inter-bank protocol)
// and HS256 JWTs (frontend wrapper + service-to-service).
package auth

import (
	"context"
	"crypto/subtle"
	"net/http"
)

// Partner describes a remote bank we trust for inbound inter-bank traffic.
type Partner struct {
	Routing       int
	DisplayName   string
	BaseURL       string
	InboundToken  string // token the partner sends to us
	OutboundToken string // token we send to the partner
}

// PartnerStore enumerates the partners we recognize for X-Api-Key auth.
// Implementations typically wrap a config struct loaded at boot.
type PartnerStore interface {
	Partners() []Partner
}

type partnerCtxKey struct{}

// PutPartner stores the matched Partner in ctx for downstream handlers.
func PutPartner(ctx context.Context, p Partner) context.Context {
	return context.WithValue(ctx, partnerCtxKey{}, &p)
}

// GetPartner retrieves the Partner stored in ctx, if any.
func GetPartner(ctx context.Context) (*Partner, bool) {
	p, ok := ctx.Value(partnerCtxKey{}).(*Partner)
	return p, ok
}

// RequireXApiKey returns middleware that enforces the X-Api-Key header against
// known partners. Constant-time compare via crypto/subtle defeats timing attacks
// (Java baseline uses MessageDigest.isEqual).
//
// On match: the Partner is added to request context and the chain continues.
// On miss or missing header: responds 401 immediately.
func RequireXApiKey(store PartnerStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := []byte(r.Header.Get("X-Api-Key"))
			if len(tok) == 0 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			for _, p := range store.Partners() {
				stored := []byte(p.InboundToken)
				if len(stored) == 0 {
					continue
				}
				if subtle.ConstantTimeCompare(stored, tok) == 1 {
					next.ServeHTTP(w, r.WithContext(PutPartner(r.Context(), p)))
					return
				}
			}
			w.WriteHeader(http.StatusUnauthorized)
		})
	}
}
