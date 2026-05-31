package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Service parses and mints HS256 tokens using the supplied Config. It is the
// Go equivalent of security-lib's JwtTokenService.
type Service struct {
	cfg Config
}

// NewService creates a Service. Caller is responsible for ensuring
// cfg.Secret is non-empty in production.
func NewService(cfg Config) *Service {
	return &Service{cfg: cfg}
}

// Config returns the underlying configuration.
func (s *Service) Config() Config {
	return s.cfg
}

// GenerateAccessToken mints an HS256 JWT for the supplied principal-fields.
// Use this from user-service-go for normal user logins.
func (s *Service) GenerateAccessToken(id int64, subject, role string, permissions []string) (string, error) {
	return s.Generate(id, subject, "", role, permissions, s.cfg.AccessTokenDuration)
}

// GenerateServiceToken mints a JWT carrying the SERVICE role. Used for
// service-to-service calls (matches Java security-lib's hasRole('SERVICE')
// flows).
func (s *Service) GenerateServiceToken(subject string, ttl time.Duration) (string, error) {
	return s.Generate(0, subject, "", "SERVICE", []string{"SERVICE"}, ttl)
}

// Generate is the underlying mint primitive. Callers prefer GenerateAccessToken
// or GenerateServiceToken; expose this for advanced cases.
func (s *Service) Generate(id int64, subject, email, role string, permissions []string, ttl time.Duration) (string, error) {
	if strings.TrimSpace(s.cfg.Secret) == "" {
		return "", errors.New("auth: JWT secret is not configured")
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":                  s.cfg.Issuer,
		"sub":                  subject,
		"iat":                  now.Unix(),
		"exp":                  now.Add(ttl).Unix(),
		s.cfg.IDClaim:          id,
		s.cfg.RolesClaim:       role,
		s.cfg.PermissionsClaim: permissions,
	}
	if email != "" && s.cfg.EmailClaim != "" {
		claims[s.cfg.EmailClaim] = email
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.Secret))
}

// ParseBearer decodes the Authorization header value (must start with
// "Bearer ") and returns the Principal. Errors are intentionally generic so
// the middleware can return a single "invalid token" response.
func (s *Service) ParseBearer(header string) (Principal, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return Principal{}, errors.New("missing bearer token")
	}
	return s.Parse(strings.TrimSpace(strings.TrimPrefix(header, prefix)))
}

// Parse decodes a raw JWT (no Bearer prefix). Returns Principal on success.
func (s *Service) Parse(raw string) (Principal, error) {
	if strings.TrimSpace(s.cfg.Secret) == "" {
		return Principal{}, errors.New("auth: JWT secret is not configured")
	}
	token, err := jwt.Parse(raw, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.cfg.Secret), nil
	}, jwt.WithIssuer(s.cfg.Issuer), jwt.WithExpirationRequired())
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
		Email:       claimString(claims[s.cfg.EmailClaim]),
		Role:        claimString(claims[s.cfg.RolesClaim]),
		Permissions: claimStringSlice(claims[s.cfg.PermissionsClaim]),
		Token:       raw,
	}, nil
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
	case jwt.NumericDate:
		return v.Unix()
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
