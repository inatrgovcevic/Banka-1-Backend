package httpx

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestCorrelationMiddlewareGeneratesIDWhenMissing(t *testing.T) {
	mw := CorrelationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if CorrelationFromContext(r.Context()) == "" {
			t.Fatal("handler did not see correlation id in context")
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if got := rec.Header().Get(HeaderCorrelationID); got == "" {
		t.Fatal("expected generated correlation id in response header")
	}
}

func TestCorrelationMiddlewarePreservesInboundID(t *testing.T) {
	const want = "fixed-id-123"
	captured := ""
	mw := CorrelationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = CorrelationFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderCorrelationID, want)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Header().Get(HeaderCorrelationID) != want {
		t.Fatalf("expected response header %q, got %q", want, rec.Header().Get(HeaderCorrelationID))
	}
	if captured != want {
		t.Fatalf("expected context id %q, got %q", want, captured)
	}
}

func TestRecoverMiddlewareReturnsSafe500WithCorrelationID(t *testing.T) {
	stack := Chain(CorrelationMiddleware, RecoverMiddleware(discardLogger()))
	final := stack(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("kaboom")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderCorrelationID, "id-1")
	rec := httptest.NewRecorder()
	final.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	var body ErrorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "ERR_INTERNAL_SERVER" || body.CorrelationID != "id-1" {
		t.Fatalf("unexpected error body: %+v", body)
	}
	if rec.Header().Get(HeaderCorrelationID) != "id-1" {
		t.Fatal("correlation id should still be on response header after panic")
	}
}

func TestRequestLogMiddlewareCapturesStatus(t *testing.T) {
	logger := discardLogger()
	stack := Chain(CorrelationMiddleware, RequestLogMiddleware(logger))
	final := stack(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("teapot"))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	final.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Fatalf("expected 418, got %d", rec.Code)
	}
	if rec.Body.String() != "teapot" {
		t.Fatalf("unexpected body %q", rec.Body.String())
	}
}

func TestCORSMiddlewareExposesCorrelationHeader(t *testing.T) {
	cfg := DefaultCORS()
	mw := CORSMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost:4200")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	expose := rec.Header().Get("Access-Control-Expose-Headers")
	if !strings.Contains(expose, HeaderCorrelationID) {
		t.Fatalf("expected expose-headers to include %s, got %q", HeaderCorrelationID, expose)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:4200" {
		t.Fatalf("expected origin to be echoed for allow-listed origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSMiddlewareShortCircuitsOptions(t *testing.T) {
	mw := CORSMiddleware(DefaultCORS())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called on OPTIONS")
	}))
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost:4200")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on preflight, got %d", rec.Code)
	}
}

func TestValidationErrorShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithCorrelation(req.Context(), "abc"))
	rec := httptest.NewRecorder()
	ValidationError(rec, req, "Molimo proverite unete podatke.", map[string]string{"amount": "amount must be greater than 0."})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var body ErrorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Code != "ERR_VALIDATION" || body.Title != "Validation error" || body.CorrelationID != "abc" {
		t.Fatalf("unexpected body: %+v", body)
	}
	if body.ValidationErrors["amount"] == "" {
		t.Fatalf("expected amount field error, got %+v", body.ValidationErrors)
	}
}

func TestChainOrdering(t *testing.T) {
	order := []string{}
	mw := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, "before-"+name)
				next.ServeHTTP(w, r)
				order = append(order, "after-"+name)
			})
		}
	}
	final := Chain(mw("a"), mw("b"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	}))
	final.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	want := []string{"before-a", "before-b", "handler", "after-b", "after-a"}
	if len(order) != len(want) {
		t.Fatalf("unexpected order: %v", order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("unexpected order: %v", order)
		}
	}
}

func TestCorrelationContextDirect(t *testing.T) {
	ctx := WithCorrelation(context.Background(), "x")
	if CorrelationFromContext(ctx) != "x" {
		t.Fatal("round-trip failed")
	}
}
