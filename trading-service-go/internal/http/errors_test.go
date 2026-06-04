package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"banka1/trading-service-go/internal/api"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// writeDomainError
// ---------------------------------------------------------------------------

func TestWriteDomainError_OrderShape_Returns400(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := &http.Request{Method: http.MethodGet, URL: &url.URL{Path: "/orders"}}
	err := api.NewOrderError(http.StatusBadRequest, "invalid order")
	writeDomainError(w, r, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid order")
}

func TestWriteDomainError_OtcShape_Returns404(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := &http.Request{Method: http.MethodGet, URL: &url.URL{Path: "/otc/1"}}
	err := api.NewOtcError(http.StatusNotFound, "OTC not found")
	writeDomainError(w, r, err)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "OTC not found")
}

func TestWriteDomainError_GenericError_Returns500(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := &http.Request{Method: http.MethodGet, URL: &url.URL{Path: "/orders"}}
	writeDomainError(w, r, errors.New("unexpected failure"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Unexpected server error")
}

// ---------------------------------------------------------------------------
// parsePathID
// ---------------------------------------------------------------------------

func TestParsePathID_ValidID_Returns42(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/orders/42", nil)
	// Simulate path value set by Go 1.22 mux
	req.SetPathValue("id", "42")
	id, err := parsePathID(req)
	assert.NoError(t, err)
	assert.Equal(t, int64(42), id)
}

func TestParsePathID_InvalidID_ReturnsError(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/orders/abc", nil)
	req.SetPathValue("id", "abc")
	_, err := parsePathID(req)
	assert.Error(t, err)
	var de *api.DomainError
	assert.True(t, errors.As(err, &de))
	assert.Equal(t, http.StatusBadRequest, de.Status)
}

// ---------------------------------------------------------------------------
// orderSecured - basic auth check
// ---------------------------------------------------------------------------

func TestOrderSecured_NoToken_Returns401(t *testing.T) {
	t.Parallel()
	jwt := newTestJWT()
	handler := orderSecured(jwt, []string{"AGENT"}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestOrderSecured_ValidToken_WrongRole_Returns403(t *testing.T) {
	t.Parallel()
	jwt := newTestJWT()
	handler := orderSecured(jwt, []string{"AGENT"}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	req.Header.Set("Authorization", "Bearer "+makeTestToken(jwt, "CLIENT_BASIC"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestOrderSecured_ValidToken_CorrectRole_Returns200(t *testing.T) {
	t.Parallel()
	jwt := newTestJWT()
	handler := orderSecured(jwt, []string{"AGENT"}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	req.Header.Set("Authorization", "Bearer "+makeTestToken(jwt, "AGENT"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
