package otel

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"banka1/go-platform/httpx"
)

func TestSetupNoopWhenDisabled(t *testing.T) {
	shutdown, err := Setup(context.Background(), Config{Disabled: true})
	if err != nil {
		t.Fatalf("expected no error when disabled, got %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("noop shutdown should not fail: %v", err)
	}
}

func TestLoadConfigDisablesWhenEndpointMissing(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	cfg := LoadConfig("svc")
	if !cfg.Disabled {
		t.Fatal("expected Disabled when endpoint blank")
	}
}

func TestLoadConfigEnablesWhenEndpointSet(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318")
	t.Setenv("OTEL_SDK_DISABLED", "")
	cfg := LoadConfig("svc")
	if cfg.Disabled {
		t.Fatal("expected enabled when endpoint set")
	}
	if cfg.ServiceName != "svc" {
		t.Fatalf("service name fallback failed: %v", cfg.ServiceName)
	}
}

func TestNormalizeOTLPEndpoint(t *testing.T) {
	cases := map[string]struct {
		host   string
		secure bool
	}{
		"http://otel-collector:4318":  {"otel-collector:4318", false},
		"https://otel.example:4318":   {"otel.example:4318", true},
		"otel-collector:4318":         {"otel-collector:4318", false},
		" http://x:9 ":                {"x:9", false},
	}
	for raw, want := range cases {
		gotHost, gotSecure := normalizeOTLPEndpoint(raw)
		if gotHost != want.host || gotSecure != want.secure {
			t.Errorf("normalizeOTLPEndpoint(%q) = (%q, %v), want (%q, %v)", raw, gotHost, gotSecure, want.host, want.secure)
		}
	}
}

func TestHTTPMiddlewareWorksWhenOTELDisabled(t *testing.T) {
	// With no Setup() call there's still a no-op TracerProvider — middleware
	// should not panic and the request should pass through.
	mw := HTTPMiddleware("svc", slog.New(slog.DiscardHandler))
	final := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(httpx.WithCorrelation(req.Context(), "x"))
	rec := httptest.NewRecorder()
	final.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestTraceIDEmptyWhenNoSpan(t *testing.T) {
	if TraceID(context.Background()) != "" {
		t.Fatal("expected empty trace id without span")
	}
	if SpanID(context.Background()) != "" {
		t.Fatal("expected empty span id without span")
	}
}
