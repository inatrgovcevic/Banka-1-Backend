package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/service"
	"github.com/raf-si-2025/banka-1-go/shared/auth"
)

// ---------------------------------------------------------------------------
// fakeOtcService
// ---------------------------------------------------------------------------

type fakeOtcService struct {
	createResult protocol.ForeignBankId
	createErr    error
	getResult    service.OtcNegotiationDto
	getErr       error
	updateErr    error
	deleteErr    error
	acceptErr    error
}

func (f *fakeOtcService) CreateNegotiation(_ context.Context, _ service.OtcOfferDto, _ int) (protocol.ForeignBankId, error) {
	return f.createResult, f.createErr
}

func (f *fakeOtcService) GetNegotiation(_ context.Context, _ int, _ string) (service.OtcNegotiationDto, error) {
	return f.getResult, f.getErr
}

func (f *fakeOtcService) UpdateCounter(_ context.Context, _ int, _ string, _ service.OtcOfferDto, _ int) error {
	return f.updateErr
}

func (f *fakeOtcService) Delete(_ context.Context, _ int, _ string, _ int) error {
	return f.deleteErr
}

func (f *fakeOtcService) AcceptNegotiation(_ context.Context, _ int, _ string, _ int) error {
	return f.acceptErr
}

// ---------------------------------------------------------------------------
// Build helpers for OTC tests
// ---------------------------------------------------------------------------

func buildOtcRouter(svc OtcService) http.Handler {
	partners := &staticPartnerStore{partners: []auth.Partner{{
		Routing:      testTheirRouting,
		InboundToken: testApiKey,
	}}}
	h := NewOtcHandler(svc, nil)
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireXApiKey(partners))
		r.Post("/negotiations", h.Create)
		r.Put("/negotiations/{rn}/{id}", h.Counter)
		r.Get("/negotiations/{rn}/{id}", h.Get)
		r.Delete("/negotiations/{rn}/{id}", h.Delete)
		r.Get("/negotiations/{rn}/{id}/accept", h.Accept)
	})
	return r
}

func otcOfferBody() []byte {
	offer := map[string]any{
		"stock":          map[string]any{"ticker": "AAPL"},
		"settlementDate": time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"pricePerUnit":   map[string]any{"currency": "USD", "amount": "150.00"},
		"premium":        map[string]any{"currency": "USD", "amount": "5.00"},
		"buyerId":        map[string]any{"routingNumber": testTheirRouting, "id": "C-2"},
		"sellerId":       map[string]any{"routingNumber": testMyRouting, "id": "C-15"},
		"amount":         10,
		"lastModifiedBy": map[string]any{"routingNumber": testTheirRouting, "id": "C-2"},
	}
	b, _ := json.Marshal(offer)
	return b
}

func doMethod(r http.Handler, method, path string, body []byte, apiKey string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// POST /negotiations
// ---------------------------------------------------------------------------

func TestOtc_Create_Happy(t *testing.T) {
	svc := &fakeOtcService{
		createResult: protocol.ForeignBankId{RoutingNumber: testMyRouting, Id: "neg-abc"},
	}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodPost, "/negotiations", otcOfferBody(), testApiKey)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body: %s", rr.Code, rr.Body.String())
	}
	var id protocol.ForeignBankId
	json.Unmarshal(rr.Body.Bytes(), &id)
	if id.Id != "neg-abc" {
		t.Errorf("expected neg-abc, got %s", id.Id)
	}
}

func TestOtc_Create_InvalidBody_400(t *testing.T) {
	svc := &fakeOtcService{createErr: service.ErrNegotiationInvalid}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodPost, "/negotiations", otcOfferBody(), testApiKey)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestOtc_Create_NoAuth_401(t *testing.T) {
	svc := &fakeOtcService{}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodPost, "/negotiations", otcOfferBody(), "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// PUT /negotiations/{rn}/{id}
// ---------------------------------------------------------------------------

func TestOtc_Counter_Happy(t *testing.T) {
	svc := &fakeOtcService{}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodPut, "/negotiations/111/neg-abc", otcOfferBody(), testApiKey)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body: %s", rr.Code, rr.Body.String())
	}
}

func TestOtc_Counter_TurnViolation_409(t *testing.T) {
	svc := &fakeOtcService{updateErr: errors.Join(service.ErrTurnViolation, errors.New("detail"))}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodPut, "/negotiations/111/neg-abc", otcOfferBody(), testApiKey)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 on turn violation, got %d", rr.Code)
	}
}

func TestOtc_Counter_Closed_409(t *testing.T) {
	svc := &fakeOtcService{updateErr: errors.Join(service.ErrNegotiationClosed, errors.New("detail"))}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodPut, "/negotiations/111/neg-closed", otcOfferBody(), testApiKey)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 on closed, got %d", rr.Code)
	}
}

func TestOtc_Counter_NotFound_404(t *testing.T) {
	svc := &fakeOtcService{updateErr: service.ErrNegotiationNotFound}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodPut, "/negotiations/111/neg-gone", otcOfferBody(), testApiKey)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// GET /negotiations/{rn}/{id}
// ---------------------------------------------------------------------------

func TestOtc_Get_Happy(t *testing.T) {
	svc := &fakeOtcService{
		getResult: service.OtcNegotiationDto{
			Stock:     protocol.StockDescription{Ticker: "AAPL"},
			IsOngoing: true,
			PricePerUnit: protocol.MonetaryValue{Currency: "USD", Amount: decimal.NewFromFloat(150)},
			Premium:      protocol.MonetaryValue{Currency: "USD", Amount: decimal.NewFromFloat(5)},
			SettlementDate: time.Now().Add(48 * time.Hour),
		},
	}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodGet, "/negotiations/111/neg-abc", nil, testApiKey)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var dto service.OtcNegotiationDto
	json.Unmarshal(rr.Body.Bytes(), &dto)
	if dto.Stock.Ticker != "AAPL" {
		t.Errorf("expected AAPL, got %s", dto.Stock.Ticker)
	}
}

func TestOtc_Get_NotFound_404(t *testing.T) {
	svc := &fakeOtcService{getErr: service.ErrNegotiationNotFound}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodGet, "/negotiations/111/neg-ghost", nil, testApiKey)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// DELETE /negotiations/{rn}/{id}
// ---------------------------------------------------------------------------

func TestOtc_Delete_Happy(t *testing.T) {
	svc := &fakeOtcService{}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodDelete, "/negotiations/111/neg-abc", nil, testApiKey)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
}

func TestOtc_Delete_Idempotent(t *testing.T) {
	// Even on "not found" the svc returns nil (idempotent); handler should return 204.
	svc := &fakeOtcService{deleteErr: nil}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodDelete, "/negotiations/111/neg-gone", nil, testApiKey)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 (idempotent), got %d", rr.Code)
	}
}

func TestOtc_Delete_NoAuth_401(t *testing.T) {
	svc := &fakeOtcService{}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodDelete, "/negotiations/111/neg-abc", nil, "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// GET /negotiations/{rn}/{id}/accept
// ---------------------------------------------------------------------------

func TestOtc_Accept_Happy(t *testing.T) {
	svc := &fakeOtcService{acceptErr: nil}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodGet, "/negotiations/111/neg-abc/accept", nil, testApiKey)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
}

func TestOtc_Accept_Closed_409(t *testing.T) {
	svc := &fakeOtcService{acceptErr: errors.Join(service.ErrNegotiationClosed, errors.New("detail"))}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodGet, "/negotiations/111/neg-closed/accept", nil, testApiKey)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 on closed, got %d", rr.Code)
	}
}

func TestOtc_Accept_NotFound_404(t *testing.T) {
	svc := &fakeOtcService{acceptErr: service.ErrNegotiationNotFound}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodGet, "/negotiations/111/neg-ghost/accept", nil, testApiKey)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestOtc_Accept_ProtocolFailure_5xx(t *testing.T) {
	svc := &fakeOtcService{acceptErr: service.ErrInterbankProtocol}
	r := buildOtcRouter(svc)
	rr := doMethod(r, http.MethodGet, "/negotiations/111/neg-abc/accept", nil, testApiKey)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on 2PC failure, got %d", rr.Code)
	}
}
