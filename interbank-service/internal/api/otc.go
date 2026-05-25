package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/service"
	"github.com/raf-si-2025/banka-1-go/shared/auth"
)

// ---------------------------------------------------------------------------
// OtcService interface (seam for testing)
// ---------------------------------------------------------------------------

// OtcService is the interface consumed by OtcHandler.
// Implemented by *service.OtcNegotiationService in production; fakes in tests.
type OtcService interface {
	CreateNegotiation(ctx context.Context, offer service.OtcOfferDto, senderRouting int) (protocol.ForeignBankId, error)
	GetNegotiation(ctx context.Context, rn int, id string) (service.OtcNegotiationDto, error)
	UpdateCounter(ctx context.Context, rn int, id string, offer service.OtcOfferDto, senderRouting int) error
	Delete(ctx context.Context, rn int, id string, senderRouting int) error
	AcceptNegotiation(ctx context.Context, rn int, id string, senderRouting int) error
}

// ---------------------------------------------------------------------------
// OtcHandler
// ---------------------------------------------------------------------------

// OtcHandler handles the 5 OTC negotiation routes.
type OtcHandler struct {
	svc OtcService
	log *slog.Logger
}

// NewOtcHandler constructs the handler.
func NewOtcHandler(svc OtcService, log *slog.Logger) *OtcHandler {
	if log == nil {
		log = slog.Default()
	}
	return &OtcHandler{svc: svc, log: log}
}

// Create handles POST /negotiations.
func (h *OtcHandler) Create(w http.ResponseWriter, r *http.Request) {
	senderRouting := requirePartnerRouting(w, r)
	if senderRouting == 0 {
		return
	}
	var offer service.OtcOfferDto
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	id, err := h.svc.CreateNegotiation(r.Context(), offer, senderRouting)
	if err != nil {
		h.handleOtcError(w, r.Context(), err)
		return
	}
	writeJSON(w, http.StatusOK, id)
}

// Counter handles PUT /negotiations/{rn}/{id}.
func (h *OtcHandler) Counter(w http.ResponseWriter, r *http.Request) {
	senderRouting := requirePartnerRouting(w, r)
	if senderRouting == 0 {
		return
	}
	rn, id := pathRnID(r)
	var offer service.OtcOfferDto
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if err := h.svc.UpdateCounter(r.Context(), rn, id, offer, senderRouting); err != nil {
		h.handleOtcError(w, r.Context(), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Get handles GET /negotiations/{rn}/{id}.
func (h *OtcHandler) Get(w http.ResponseWriter, r *http.Request) {
	rn, id := pathRnID(r)
	dto, err := h.svc.GetNegotiation(r.Context(), rn, id)
	if err != nil {
		h.handleOtcError(w, r.Context(), err)
		return
	}
	writeJSON(w, http.StatusOK, dto)
}

// Delete handles DELETE /negotiations/{rn}/{id}.
func (h *OtcHandler) Delete(w http.ResponseWriter, r *http.Request) {
	senderRouting := requirePartnerRouting(w, r)
	if senderRouting == 0 {
		return
	}
	rn, id := pathRnID(r)
	if err := h.svc.Delete(r.Context(), rn, id, senderRouting); err != nil {
		h.handleOtcError(w, r.Context(), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Accept handles GET /negotiations/{rn}/{id}/accept.
func (h *OtcHandler) Accept(w http.ResponseWriter, r *http.Request) {
	senderRouting := requirePartnerRouting(w, r)
	if senderRouting == 0 {
		return
	}
	rn, id := pathRnID(r)
	if err := h.svc.AcceptNegotiation(r.Context(), rn, id, senderRouting); err != nil {
		h.handleOtcError(w, r.Context(), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// handleOtcError maps domain errors to HTTP status codes per Tim 2 §6.3.
func (h *OtcHandler) handleOtcError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNegotiationNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrTurnViolation):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrNegotiationClosed):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrNegotiationInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrSenderNotParty):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrInterbankProtocol):
		h.log.WarnContext(ctx, "otc: 2PC protocol failure", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
	default:
		h.log.ErrorContext(ctx, "otc: unexpected error", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

// requirePartnerRouting extracts the partner routing from the context.
// Returns 0 and writes 401 if not present.
func requirePartnerRouting(w http.ResponseWriter, r *http.Request) int {
	p, ok := auth.GetPartner(r.Context())
	if !ok || p == nil {
		writeError(w, http.StatusUnauthorized, "missing partner context")
		return 0
	}
	return p.Routing
}

// pathRnID extracts {rn} (parsed as int) and {id} from chi path variables.
// Returns (0, id) when rn is not parseable as int — callers should validate.
func pathRnID(r *http.Request) (int, string) {
	rnStr := chi.URLParam(r, "rn")
	id := chi.URLParam(r, "id")
	var rn int
	for _, c := range rnStr {
		if c < '0' || c > '9' {
			return 0, id
		}
		rn = rn*10 + int(c-'0')
	}
	return rn, id
}

// writeJSON encodes v as JSON and writes it to w.
func writeJSON(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "marshal response: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}
