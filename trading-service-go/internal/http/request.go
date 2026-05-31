package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

// optionalQueryParam returns a pointer to the raw query value when the parameter
// is present (even if empty), else nil — mirroring Spring @RequestParam(required=false):
// absent => null, present => value (passed to user-service via queryParamIfPresent).
func optionalQueryParam(r *http.Request, key string) *string {
	if !r.URL.Query().Has(key) {
		return nil
	}
	v := r.URL.Query().Get(key)
	return &v
}

// queryIntDefault parses an int query param, falling back to def when absent or
// unparseable. ActuaryController is not @Validated, so the @Min/@Max on page/size
// are not actually enforced — we only apply the defaultValue.
func queryIntDefault(r *http.Request, key string, def int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	if v, err := strconv.Atoi(raw); err == nil {
		return v
	}
	return def
}

// localDateTimeLayouts accepts the ISO_LOCAL_DATE_TIME variants Spring's
// @DateTimeFormat(iso = DATE_TIME) parses (with/without fractional seconds, and
// the offset form).
var localDateTimeLayouts = []string{
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	time.RFC3339Nano,
	time.RFC3339,
}

// parseDateTimeParam parses an optional ISO datetime query param. Absent => (nil,
// nil). Present but unparseable => (nil, error) so the caller can return 400.
func parseDateTimeParam(r *http.Request, key string) (*time.Time, error) {
	if !r.URL.Query().Has(key) {
		return nil, nil
	}
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}
	for _, layout := range localDateTimeLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return &t, nil
		}
	}
	return nil, errors.New("invalid datetime: " + raw)
}

// decodeJSONLenient decodes a request body, ignoring unknown fields (matching
// Jackson's default) and treating an empty body as a no-op (leaves dst zero).
func decodeJSONLenient(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
