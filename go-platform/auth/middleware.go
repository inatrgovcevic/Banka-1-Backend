package auth

import (
	"net/http"
	"strings"

	"banka1/go-platform/httpx"
)

// Middleware authenticates inbound HTTP requests. On success it stores the
// Principal on the request context. On failure it writes a 401 ErrorBody
// (httpx contract).
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := s.ParseBearer(r.Header.Get("Authorization"))
		if err != nil {
			httpx.Error(w, r, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Missing or invalid bearer token")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), principal)))
	})
}

// OptionalMiddleware attaches the Principal when the request carries a valid
// token, but does not reject the request when missing or invalid. Use for
// endpoints that vary on auth but do not require it.
func (s *Service) OptionalMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if header := r.Header.Get("Authorization"); header != "" {
			if principal, err := s.ParseBearer(header); err == nil {
				r = r.WithContext(WithPrincipal(r.Context(), principal))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRoles wraps a handler so it only executes when the principal has any
// of the listed roles (taking the role hierarchy into account). Use after
// Middleware.
func RequireRoles(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Missing principal")
				return
			}
			if !principal.HasAnyRole(roles...) {
				httpx.Error(w, r, http.StatusForbidden, "ERR_FORBIDDEN", "Insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermissions wraps a handler so it only executes when the principal
// has at least one of the listed permissions. Use after Middleware.
func RequirePermissions(perms ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Missing principal")
				return
			}
			if !principal.HasAnyPermission(perms...) {
				httpx.Error(w, r, http.StatusForbidden, "ERR_FORBIDDEN", "Insufficient permission")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// PermitAll matches request paths against a list of patterns. Patterns
// support Ant-style "/foo/**" suffixes (recursive) and exact paths. Use as
// a guard around Middleware when implementing a service-wide auth filter.
type PermitAll struct {
	prefixes []string
	exacts   map[string]struct{}
}

// NewPermitAll parses the security-lib style comma-separated permit-all
// list, e.g. "/stocks/public/**,/internal/calculate/**,/actuator/info".
func NewPermitAll(patterns []string) PermitAll {
	pa := PermitAll{exacts: map[string]struct{}{}}
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, "/**") {
			pa.prefixes = append(pa.prefixes, strings.TrimSuffix(p, "**"))
		} else {
			pa.exacts[p] = struct{}{}
		}
	}
	return pa
}

// Matches returns true if path should bypass authentication.
func (p PermitAll) Matches(path string) bool {
	if _, ok := p.exacts[path]; ok {
		return true
	}
	for _, prefix := range p.prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
