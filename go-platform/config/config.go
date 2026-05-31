// Package config centralizes environment-variable parsing for Go services.
//
// All helpers fall back to a default if the variable is unset/blank. Use
// Required for variables that must be set in production (the helper panics
// at startup, never at request time).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Env returns the value of key, trimming whitespace, or fallback when blank.
func Env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// EnvInt parses key as int, returning fallback on missing or invalid values.
func EnvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

// EnvBool parses key as bool. Accepts the strconv.ParseBool dialect
// (1/0, t/f, true/false, TRUE/FALSE, etc.).
func EnvBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

// EnvDuration parses key as a Go duration string ("15s", "2m"), with
// numeric-only fallback to milliseconds for compatibility with Java
// properties like *_INTERVAL_MS=900000.
func EnvDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	if ms, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Duration(ms) * time.Millisecond
	}
	return fallback
}

// SplitCSV parses key as comma-separated values with optional surrounding
// whitespace. Empty entries are dropped.
func SplitCSV(key, fallback string) []string {
	raw := Env(key, fallback)
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// Required returns the env value or panics if blank. Use at startup only.
func Required(key string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	panic(fmt.Sprintf("required environment variable %s is not set", key))
}

// IsSecret returns true if the key name looks like it carries a secret value.
// Use it to redact env dumps in logs.
func IsSecret(key string) bool {
	upper := strings.ToUpper(key)
	for _, marker := range []string{"SECRET", "PASSWORD", "TOKEN", "API_KEY", "APIKEY", "PRIVATE", "CREDENTIAL", "_KEY"} {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return strings.HasSuffix(upper, "KEY")
}

// Redact replaces every value whose key looks like a secret with "***".
// Caller-supplied map is not mutated.
func Redact(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for k, v := range values {
		if IsSecret(k) {
			out[k] = "***"
		} else {
			out[k] = v
		}
	}
	return out
}
