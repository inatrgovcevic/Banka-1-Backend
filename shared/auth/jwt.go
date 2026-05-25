package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// StringOrSlice is a JSON type that unmarshals from either a JSON string
// ("ADMIN") or a JSON array (["ADMIN","SUPERVISOR"]). Spring's security-lib
// emit-uje `roles` kao goli string (single role), dok permissions emit-uje kao
// array. Ovo dozvoljava oba oblika.
type StringOrSlice []string

// UnmarshalJSON accepts either `"ADMIN"` (single string) or `["ADMIN","SUPER"]`.
func (s *StringOrSlice) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*s = nil
		return nil
	}
	if data[0] == '[' {
		var arr []string
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		*s = arr
		return nil
	}
	var single string
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}
	if single == "" {
		*s = nil
		return nil
	}
	*s = []string{single}
	return nil
}

// MarshalJSON emit-uje goli string ako je samo jedan element (Spring compat),
// inace JSON array.
func (s StringOrSlice) MarshalJSON() ([]byte, error) {
	if len(s) == 1 {
		return json.Marshal(s[0])
	}
	return json.Marshal([]string(s))
}

// Claims holds custom + standard JWT claims used across the banka1 stack.
// `roles` and `permissions` mirror security-lib (Java) — comma-tolerant arrays.
type Claims struct {
	Roles       StringOrSlice `json:"roles,omitempty"`
	Permissions StringOrSlice `json:"permissions,omitempty"`
	ID          any           `json:"id,omitempty"`
	jwt.RegisteredClaims
}

type claimsCtxKey struct{}

// PutClaims attaches verified claims to ctx (used by RequireJWT internally).
func PutClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsCtxKey{}, c)
}

// GetClaims retrieves claims attached by RequireJWT, if any.
func GetClaims(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsCtxKey{}).(*Claims)
	return c, ok
}

// VerifyJWT parses and validates an HS256 token. Returns the claims on success
// or an error (wrong signature, expired, malformed, wrong alg).
func VerifyJWT(tokenString, secret string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unsupported alg")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid claims")
	}
	return c, nil
}

// RequireJWT returns middleware that parses Authorization: Bearer <token>,
// verifies HS256 with secret, and attaches Claims to ctx. 401 on any failure.
func RequireJWT(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			if !strings.HasPrefix(authz, "Bearer ") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			raw := strings.TrimPrefix(authz, "Bearer ")
			if raw == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			claims, err := VerifyJWT(raw, secret)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(PutClaims(r.Context(), claims)))
		})
	}
}

// RequirePermission returns middleware that allows the request if the verified
// JWT contains at least one of the listed authorities. An authority is either a
// permission string (e.g. "OTC_TRADE") or "ROLE_<X>" — role names from claims.Roles
// are auto-prefixed with "ROLE_" for matching.
//
// Use AFTER RequireJWT in the chain. Returns 401 if no claims; 403 if claims have
// none of the required authorities.
func RequirePermission(required ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetClaims(r.Context())
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			granted := make(map[string]struct{}, len(claims.Permissions)+len(claims.Roles))
			for _, p := range claims.Permissions {
				granted[p] = struct{}{}
			}
			for _, role := range claims.Roles {
				granted["ROLE_"+role] = struct{}{}
			}
			for _, req := range required {
				if _, ok := granted[req]; ok {
					next.ServeHTTP(w, r)
					return
				}
			}
			w.WriteHeader(http.StatusForbidden)
		})
	}
}

// S2SIssuer mints HS256 JWTs for service-to-service auth. Tokens are cached and
// reused until the refresh window (min(30s, TTL/10)) before expiry.
//
// The issued claims include: roles, permissions (optional), iss, sub, iat, exp.
// Designed to be wire-compatible with security-lib's JWTServiceImplementation.
type S2SIssuer struct {
	Issuer      string
	Subject     string
	Roles       []string
	Permissions []string
	Secret      string
	TTL         time.Duration

	mu     sync.Mutex
	cached string
	exp    time.Time
}

// NewS2SIssuer constructs an issuer with sane defaults. Caller may set
// Permissions after construction if needed.
func NewS2SIssuer(issuer, subject string, roles []string, secret string, ttl time.Duration) *S2SIssuer {
	return &S2SIssuer{
		Issuer:  issuer,
		Subject: subject,
		Roles:   append([]string(nil), roles...),
		Secret:  secret,
		TTL:     ttl,
	}
}

// IssueToken returns the current cached JWT or mints a new one if the cached
// token has expired (or will soon expire within refresh-buf).
func (s *S2SIssuer) IssueToken() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	refreshBuf := s.TTL / 10
	if refreshBuf > 30*time.Second {
		refreshBuf = 30 * time.Second
	}
	if s.cached != "" && time.Now().Add(refreshBuf).Before(s.exp) {
		return s.cached, nil
	}
	now := time.Now()
	exp := now.Add(s.TTL)
	claims := Claims{
		Roles:       s.Roles,
		Permissions: s.Permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.Issuer,
			Subject:   s.Subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	str, err := t.SignedString([]byte(s.Secret))
	if err != nil {
		return "", err
	}
	s.cached, s.exp = str, exp
	return str, nil
}
