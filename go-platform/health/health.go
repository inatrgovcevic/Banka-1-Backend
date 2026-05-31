// Package health serves the four Spring Boot Actuator endpoints every Go
// service exposes: /actuator/health, /liveness, /readiness, /info.
//
// Liveness is always lightweight (the process is up). Readiness runs
// caller-registered Checkers; a single failure → 503 with the failing
// component listed.
package health

import (
	"context"
	"net/http"
	"sync"
	"time"

	"banka1/go-platform/httpx"
)

// Checker reports whether one dependency is currently usable. The context
// supplied to Check is short-lived (the Handler wraps each call with a
// configurable per-check timeout).
type Checker interface {
	Name() string
	Check(context.Context) error
}

// CheckerFunc is the function adapter for Checker.
type CheckerFunc struct {
	Label string
	Fn    func(context.Context) error
}

func (c CheckerFunc) Name() string                 { return c.Label }
func (c CheckerFunc) Check(ctx context.Context) error { return c.Fn(ctx) }

// Handler exposes the four /actuator/* endpoints.
type Handler struct {
	mu       sync.RWMutex
	checks   []Checker
	timeout  time.Duration
	infoBody any
}

// NewHandler creates a Handler with the standard 2s per-check timeout and an
// empty /actuator/info body (matches Java default).
func NewHandler() *Handler {
	return &Handler{timeout: 2 * time.Second, infoBody: map[string]any{}}
}

// WithTimeout returns a copy with the supplied per-check timeout.
func (h *Handler) WithTimeout(d time.Duration) *Handler {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.timeout = d
	return h
}

// WithInfo sets the body returned by /actuator/info. Default is `{}`.
func (h *Handler) WithInfo(body any) *Handler {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.infoBody = body
	return h
}

// Register adds checkers run by /actuator/health/readiness.
func (h *Handler) Register(checks ...Checker) *Handler {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks = append(h.checks, checks...)
	return h
}

// MountStandard registers the four actuator routes on mux.
//
//	GET /actuator/health             → aggregate (200 = UP, 503 = DOWN)
//	GET /actuator/health/liveness    → always 200 "UP"
//	GET /actuator/health/readiness   → 200/503 from Checkers
//	GET /actuator/info               → infoBody, default `{}`
func (h *Handler) MountStandard(mux *http.ServeMux) {
	mux.Handle("GET /actuator/health", http.HandlerFunc(h.Aggregate))
	mux.Handle("GET /actuator/health/liveness", http.HandlerFunc(h.Liveness))
	mux.Handle("GET /actuator/health/readiness", http.HandlerFunc(h.Readiness))
	mux.Handle("GET /actuator/info", http.HandlerFunc(h.Info))
}

// Liveness writes {"status":"UP"} unconditionally.
func (h *Handler) Liveness(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "UP"})
}

// Readiness runs every registered Checker. If any fails the response is 503
// with a `details` map showing the failure per component.
func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	checks := append([]Checker(nil), h.checks...)
	timeout := h.timeout
	h.mu.RUnlock()

	details := make(map[string]map[string]string, len(checks))
	healthy := true
	for _, c := range checks {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		err := c.Check(ctx)
		cancel()
		entry := map[string]string{"status": "UP"}
		if err != nil {
			entry["status"] = "DOWN"
			entry["error"] = err.Error()
			healthy = false
		}
		details[c.Name()] = entry
	}
	status := http.StatusOK
	state := "UP"
	if !healthy {
		status = http.StatusServiceUnavailable
		state = "DOWN"
	}
	payload := map[string]any{"status": state}
	if len(details) > 0 {
		payload["details"] = details
	}
	httpx.JSON(w, status, payload)
}

// Aggregate is the plain /actuator/health endpoint. Returns the same shape as
// readiness so monitoring tools that expect the Spring Boot aggregate keep
// working.
func (h *Handler) Aggregate(w http.ResponseWriter, r *http.Request) {
	h.Readiness(w, r)
}

// Info writes the configured info body.
func (h *Handler) Info(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	body := h.infoBody
	h.mu.RUnlock()
	httpx.JSON(w, http.StatusOK, body)
}
