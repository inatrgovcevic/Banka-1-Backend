package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrometheusEndpointReturnsTextMetrics(t *testing.T) {
	handler := &Handler{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/actuator/prometheus", nil)

	handler.prometheus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "banking_core_service_up 1") {
		t.Fatalf("metrics body does not contain service up metric: %s", rec.Body.String())
	}
}
