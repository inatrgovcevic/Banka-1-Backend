package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// helper: create a request with a bad (non-numeric) path id
func reqBadID(method, path string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.SetPathValue("id", "bad")
	return r
}

func reqGoodID(method, path string, id string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.SetPathValue("id", id)
	return r
}

// ---- order_handlers.go early exits ----

func TestOrderCancel_BadID_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	w := httptest.NewRecorder()
	h.OrderCancel(w, reqBadID(http.MethodDelete, "/orders/bad"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOrderApprove_BadID_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	w := httptest.NewRecorder()
	h.OrderApprove(w, reqBadID(http.MethodPost, "/orders/bad/approve"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOrderDecline_BadID_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	w := httptest.NewRecorder()
	h.OrderDecline(w, reqBadID(http.MethodPost, "/orders/bad/decline"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOrderBuy_InvalidJSON_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	r := httptest.NewRequest(http.MethodPost, "/orders/buy", bytes.NewBufferString("{bad json}"))
	w := httptest.NewRecorder()
	h.OrderBuy(w, r)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestOrderSell_InvalidJSON_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	r := httptest.NewRequest(http.MethodPost, "/orders/sell", bytes.NewBufferString("{bad json}"))
	w := httptest.NewRecorder()
	h.OrderSell(w, r)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// ---- audit_handlers.go early exits ----

func TestAuditLog_InvalidFrom_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	r := httptest.NewRequest(http.MethodGet, "/audit?from=notadate", nil)
	w := httptest.NewRecorder()
	h.AuditLog(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuditLog_InvalidTo_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	r := httptest.NewRequest(http.MethodGet, "/audit?to=notadate", nil)
	w := httptest.NewRecorder()
	h.AuditLog(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---- interbank_handlers.go early exits ----

func TestInterbankReserveStock_InvalidJSON_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	r := httptest.NewRequest(http.MethodPost, "/internal/interbank/reserve-stock", bytes.NewBufferString("{bad}"))
	w := httptest.NewRecorder()
	h.InterbankReserveStock(w, r)
	// decode error → 400 (before calling service)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestInterbankReserveOption_InvalidJSON_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	r := httptest.NewRequest(http.MethodPost, "/internal/interbank/reserve-option", bytes.NewBufferString("{bad}"))
	w := httptest.NewRecorder()
	h.InterbankReserveOption(w, r)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// ---- tax_handlers.go early exits ----

func TestTaxUserDebt_BadUserID_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	r := httptest.NewRequest(http.MethodGet, "/tax/bad/debt", nil)
	r.SetPathValue("userId", "bad")
	w := httptest.NewRecorder()
	h.TaxUserDebt(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---- recurring order handlers ----

func TestRecurringOrderCreate_InvalidJSON_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	r := httptest.NewRequest(http.MethodPost, "/orders/recurring", bytes.NewBufferString("{bad}"))
	w := httptest.NewRecorder()
	h.RecurringOrderCreate(w, r)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestRecurringOrderPause_BadID_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	w := httptest.NewRecorder()
	h.RecurringOrderPause(w, reqBadID(http.MethodPost, "/orders/recurring/bad/pause"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRecurringOrderResume_BadID_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	w := httptest.NewRecorder()
	h.RecurringOrderResume(w, reqBadID(http.MethodPost, "/orders/recurring/bad/resume"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRecurringOrderDelete_BadID_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	w := httptest.NewRecorder()
	h.RecurringOrderDelete(w, reqBadID(http.MethodDelete, "/orders/recurring/bad"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
