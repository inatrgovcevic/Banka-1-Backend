package platform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const principalContextKey contextKey = "principal"

type Principal struct {
	ID          int64
	Subject     string
	Role        string
	Permissions []string
	Token       string
}

type JWTService struct {
	cfg JWTConfig
}

func NewJWTService(cfg JWTConfig) *JWTService {
	return &JWTService{cfg: cfg}
}

func (s *JWTService) GenerateAccessToken(id int64, subject, role string, permissions []string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":                  s.cfg.Issuer,
		"sub":                  subject,
		"exp":                  now.Add(s.cfg.AccessTokenDuration).Unix(),
		s.cfg.IDClaim:          id,
		s.cfg.RolesClaim:       role,
		s.cfg.PermissionsClaim: permissions,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.Secret))
}

func (s *JWTService) ParseBearer(header string) (Principal, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return Principal{}, errors.New("missing bearer token")
	}
	raw := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	token, err := jwt.Parse(raw, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.cfg.Secret), nil
	}, jwt.WithIssuer(s.cfg.Issuer))
	if err != nil || !token.Valid {
		return Principal{}, errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return Principal{}, errors.New("invalid claims")
	}
	return Principal{
		ID:          claimInt64(claims[s.cfg.IDClaim]),
		Subject:     claimString(claims["sub"]),
		Role:        claimString(claims[s.cfg.RolesClaim]),
		Permissions: claimStringSlice(claims[s.cfg.PermissionsClaim]),
		Token:       raw,
	}, nil
}

func (s *JWTService) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := s.ParseBearer(r.Header.Get("Authorization"))
		if err != nil {
			Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid bearer token")
			return
		}
		ctx := context.WithValue(r.Context(), principalContextKey, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey).(Principal)
	return principal, ok
}

func RequireAnyRole(roles ...string) func(http.Handler) http.Handler {
	allowed := map[string]struct{}{}
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing principal")
				return
			}
			if _, ok := allowed[principal.Role]; !ok {
				Error(w, http.StatusForbidden, "FORBIDDEN", "Insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RandomURLToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func SHA256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func claimString(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func claimInt64(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}

func claimStringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
