package httpx

import (
	"net/http"
	"strings"
)

// CORSConfig captures the values the existing security-lib reads from
// BANKA_SECURITY_CORS_* env vars. Lists are explicit allow-lists; "*" is
// honoured for AllowedOrigins (use sparingly).
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAgeSeconds    int
}

// DefaultCORS returns a CORSConfig with the same defaults as
// security-lib's CorsProperties: localhost dev origins, the standard verb
// set, and X-Correlation-Id exposed/allowed.
func DefaultCORS() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{"http://localhost:4200", "http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Accept", "X-Requested-With", "X-Verification-Code", HeaderCorrelationID},
		ExposedHeaders:   []string{"Location", HeaderCorrelationID},
		AllowCredentials: true,
		MaxAgeSeconds:    3600,
	}
}

// CORSMiddleware applies the supplied policy. It short-circuits OPTIONS
// requests with 204.
func CORSMiddleware(cfg CORSConfig) func(http.Handler) http.Handler {
	allow := make(map[string]struct{}, len(cfg.AllowedOrigins))
	wildcard := false
	for _, o := range cfg.AllowedOrigins {
		if o == "*" {
			wildcard = true
		}
		allow[o] = struct{}{}
	}
	headersJoined := strings.Join(cfg.AllowedHeaders, ",")
	methodsJoined := strings.Join(cfg.AllowedMethods, ",")
	exposedJoined := strings.Join(cfg.ExposedHeaders, ",")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if wildcard {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if _, ok := allow[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					if cfg.AllowCredentials {
						w.Header().Set("Access-Control-Allow-Credentials", "true")
					}
				}
			}
			if headersJoined != "" {
				w.Header().Set("Access-Control-Allow-Headers", headersJoined)
			}
			if exposedJoined != "" {
				w.Header().Set("Access-Control-Expose-Headers", exposedJoined)
			}
			if methodsJoined != "" {
				w.Header().Set("Access-Control-Allow-Methods", methodsJoined)
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
