package http

import (
	"net/http"
	"strconv"
	"strings"
)

// applyCORS replicira security-lib CorsConfigurationSource (registrovan za "/**").
// Postavlja CORS header-e na odgovor kada Origin odgovara dozvoljenoj listi.
// Za preflight (OPTIONS) dodaje Allow-Methods/Allow-Headers/Max-Age.
//
// Posto je allowCredentials=true, Access-Control-Allow-Origin mora da bude
// konkretan origin (echo), ne "*".
func (h *Handler) applyCORS(w http.ResponseWriter, r *http.Request) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return
	}
	header := w.Header()
	header.Add("Vary", "Origin")
	if !h.originAllowed(origin) {
		return
	}

	header.Set("Access-Control-Allow-Origin", origin)
	if h.cfg.CORSAllowCredentials {
		header.Set("Access-Control-Allow-Credentials", "true")
	}
	if len(h.cfg.CORSExposedHeaders) > 0 {
		header.Set("Access-Control-Expose-Headers", strings.Join(h.cfg.CORSExposedHeaders, ", "))
	}

	if r.Method == http.MethodOptions {
		header.Add("Vary", "Access-Control-Request-Method")
		header.Add("Vary", "Access-Control-Request-Headers")
		if len(h.cfg.CORSAllowedMethods) > 0 {
			header.Set("Access-Control-Allow-Methods", strings.Join(h.cfg.CORSAllowedMethods, ", "))
		}
		if len(h.cfg.CORSAllowedHeaders) > 0 {
			header.Set("Access-Control-Allow-Headers", strings.Join(h.cfg.CORSAllowedHeaders, ", "))
		}
		if h.cfg.CORSMaxAgeSeconds > 0 {
			header.Set("Access-Control-Max-Age", strconv.FormatInt(h.cfg.CORSMaxAgeSeconds, 10))
		}
	}
}

func (h *Handler) originAllowed(origin string) bool {
	for _, allowed := range h.cfg.CORSAllowedOrigins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "*" || strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}
