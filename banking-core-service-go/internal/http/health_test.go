package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLivenessDoesNotRequireDependencies(t *testing.T) {
	handler := &Handler{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/actuator/health/liveness", nil)

	handler.liveness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReadinessReportsDownWithoutDatabase(t *testing.T) {
	handler := &Handler{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/actuator/health/readiness", nil)

	handler.readiness(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
