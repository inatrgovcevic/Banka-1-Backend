package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"banka1/banking-core-service-go/internal/service"
)

func (h *Handler) principalFromRequest(w http.ResponseWriter, r *http.Request, required bool) (service.Principal, bool) {
	roles := rolesFromHeader(r)
	if token := bearerToken(r); token != "" {
		if principal, ok := h.principalFromToken(token); ok {
			if len(principal.Roles) == 0 {
				principal.Roles = roles
			}
			return principal, true
		}
		if required {
			writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Pristup odbijen", "Neispravan JWT token")
			return service.Principal{}, false
		}
	}

	for _, header := range []string{"X-User-Id", "X-Client-Id", "X-Owner-Id"} {
		if value := strings.TrimSpace(r.Header.Get(header)); value != "" {
			id, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				return service.Principal{ID: id, Roles: roles}, true
			}
		}
	}
	if required {
		writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Pristup odbijen", "Nedostaje id korisnika")
		return service.Principal{}, false
	}
	return service.Principal{}, true
}

func (h *Handler) requireServiceRole(w http.ResponseWriter, r *http.Request) bool {
	if token := bearerToken(r); token != "" {
		claims, ok := h.verifiedJWTClaims(token)
		if !ok {
			writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Pristup odbijen", "Neispravan JWT token")
			return false
		}
		if !hasServiceRole(h.rolesFromClaims(claims)) {
			writeError(w, http.StatusForbidden, "ERR_FORBIDDEN", "Pristup odbijen", "Potrebna je SERVICE rola")
			return false
		}
		return true
	}
	if !hasServiceRole(rolesFromHeader(r)) {
		writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Pristup odbijen", "Potrebna je SERVICE rola")
		return false
	}
	return true
}

func (h *Handler) requireAuthenticated(w http.ResponseWriter, r *http.Request) bool {
	if token := bearerToken(r); token != "" {
		if _, ok := h.verifiedJWTClaims(token); ok {
			return true
		}
		writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Pristup odbijen", "Neispravan JWT token")
		return false
	}
	_, ok := h.principalFromRequest(w, r, true)
	return ok
}

func (h *Handler) principalFromToken(token string) (service.Principal, bool) {
	claims, ok := h.verifiedJWTClaims(token)
	if !ok {
		return service.Principal{}, false
	}
	var principal service.Principal
	principal.Roles = h.rolesFromClaims(claims)
	for _, key := range []string{"id", "userId", "clientId", "client_id", "sub"} {
		if id, ok := claimAsInt(claims[key]); ok {
			principal.ID = id
			return principal, true
		}
	}
	return service.Principal{}, false
}

func (h *Handler) rolesFromClaims(claims map[string]any) []string {
	roles := claimAsStrings(claims[h.cfg.JWTRoleClaim])
	if len(roles) == 0 && h.cfg.JWTRoleClaim != "roles" {
		roles = claimAsStrings(claims["roles"])
	}
	return roles
}

func (h *Handler) verifiedJWTClaims(token string) (map[string]any, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || strings.TrimSpace(h.cfg.JWTSecret) == "" {
		return nil, false
	}

	headerBytes, ok := decodeJWTPart(parts[0])
	if !ok {
		return nil, false
	}
	var header struct {
		Algorithm string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, false
	}
	if header.Algorithm != "HS256" {
		return nil, false
	}

	signature, ok := decodeJWTPart(parts[2])
	if !ok {
		return nil, false
	}
	mac := hmac.New(sha256.New, []byte(h.cfg.JWTSecret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, false
	}

	payload, ok := decodeJWTPart(parts[1])
	if !ok {
		return nil, false
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, false
	}
	if expired(claims["exp"], time.Now()) {
		return nil, false
	}
	return claims, true
}

func decodeJWTPart(value string) ([]byte, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(value)
	}
	return decoded, err == nil
}

func expired(value any, now time.Time) bool {
	if value == nil {
		return false
	}
	exp, ok := claimAsInt(value)
	if !ok {
		return true
	}
	return now.Unix() >= exp
}

func bearerToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(auth) < len("Bearer ") || !strings.EqualFold(auth[:len("Bearer ")], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(auth[len("Bearer "):])
}

func rolesFromHeader(r *http.Request) []string {
	for _, header := range []string{"X-User-Roles", "X-Roles"} {
		raw := strings.TrimSpace(r.Header.Get(header))
		if raw == "" {
			continue
		}
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}
	return nil
}

func claimAsInt(value any) (int64, bool) {
	switch v := value.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case json.Number:
		id, err := v.Int64()
		return id, err == nil
	case string:
		id, err := strconv.ParseInt(v, 10, 64)
		return id, err == nil
	default:
		return 0, false
	}
}

func claimAsStrings(value any) []string {
	switch v := value.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	case string:
		if v == "" {
			return nil
		}
		if strings.Contains(v, ",") || strings.ContainsFunc(v, unicode.IsSpace) {
			parts := strings.FieldsFunc(v, func(r rune) bool {
				return r == ',' || unicode.IsSpace(r)
			})
			out := make([]string, 0, len(parts))
			for _, part := range parts {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					out = append(out, trimmed)
				}
			}
			return out
		}
		return []string{v}
	default:
		return nil
	}
}

func hasServiceRole(roles []string) bool {
	for _, role := range roles {
		normalized := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(role)), "ROLE_")
		if normalized == "SERVICE" {
			return true
		}
	}
	return false
}
