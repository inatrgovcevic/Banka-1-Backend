package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newMux(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	h.MountStandard(mux)
	return mux
}

func TestLivenessAlwaysUp(t *testing.T) {
	h := NewHandler().Register(CheckerFunc{Label: "always-down", Fn: func(context.Context) error {
		return errors.New("nope")
	}})
	req := httptest.NewRequest(http.MethodGet, "/actuator/health/liveness", nil)
	rec := httptest.NewRecorder()
	newMux(h).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("liveness must be 200 regardless of checks, got %d", rec.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "UP" {
		t.Fatalf("expected UP, got %v", body)
	}
}

func TestReadinessUpWhenChecksPass(t *testing.T) {
	h := NewHandler().Register(
		CheckerFunc{Label: "db", Fn: func(context.Context) error { return nil }},
		CheckerFunc{Label: "redis", Fn: func(context.Context) error { return nil }},
	)
	req := httptest.NewRequest(http.MethodGet, "/actuator/health/readiness", nil)
	rec := httptest.NewRecorder()
	newMux(h).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "UP" {
		t.Fatalf("expected UP, got %v", body)
	}
	details := body["details"].(map[string]any)
	if details["db"].(map[string]any)["status"] != "UP" {
		t.Fatalf("db should be UP, got %v", details)
	}
}

func TestReadiness503OnAnyFailure(t *testing.T) {
	h := NewHandler().Register(
		CheckerFunc{Label: "db", Fn: func(context.Context) error { return nil }},
		CheckerFunc{Label: "rabbit", Fn: func(context.Context) error { return errors.New("connection refused") }},
	)
	req := httptest.NewRequest(http.MethodGet, "/actuator/health/readiness", nil)
	rec := httptest.NewRecorder()
	newMux(h).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	var body map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&body)
	details := body["details"].(map[string]any)
	if details["rabbit"].(map[string]any)["status"] != "DOWN" {
		t.Fatalf("rabbit should be DOWN: %v", details)
	}
	if details["db"].(map[string]any)["status"] != "UP" {
		t.Fatalf("db should still be UP: %v", details)
	}
}

func TestInfoBodyDefaultsToEmpty(t *testing.T) {
	h := NewHandler()
	req := httptest.NewRequest(http.MethodGet, "/actuator/info", nil)
	rec := httptest.NewRecorder()
	newMux(h).ServeHTTP(rec, req)
	if rec.Body.String() == "" {
		t.Fatal("info body should be JSON, got empty string")
	}
	if rec.Body.String() != "{}\n" && rec.Body.String() != "{}" {
		t.Fatalf("expected {}, got %q", rec.Body.String())
	}
}

func TestInfoBodyCustomizable(t *testing.T) {
	h := NewHandler().WithInfo(map[string]string{"service": "test"})
	req := httptest.NewRequest(http.MethodGet, "/actuator/info", nil)
	rec := httptest.NewRecorder()
	newMux(h).ServeHTTP(rec, req)
	var body map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["service"] != "test" {
		t.Fatalf("custom info not applied: %v", body)
	}
}

func TestAggregateMirrorsReadiness(t *testing.T) {
	h := NewHandler().Register(CheckerFunc{Label: "x", Fn: func(context.Context) error { return errors.New("dead") }})
	req := httptest.NewRequest(http.MethodGet, "/actuator/health", nil)
	rec := httptest.NewRecorder()
	newMux(h).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("aggregate should reflect readiness, got %d", rec.Code)
	}
}

func TestReadinessNoChecks(t *testing.T) {
	h := NewHandler()
	req := httptest.NewRequest(http.MethodGet, "/actuator/health/readiness", nil)
	rec := httptest.NewRecorder()
	newMux(h).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("no checks should be UP, got %d", rec.Code)
	}
}
