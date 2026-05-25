package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/service"
	"github.com/raf-si-2025/banka-1-go/shared/auth"
)

// ---------------------------------------------------------------------------
// OtcOutboundHandler
// ---------------------------------------------------------------------------

// OtcOutboundHandler handles the FE-facing /api/interbank/otc/* routes.
// These routes use JWT auth (not X-Api-Key) so Angular clients can call them
// directly without exposing the inter-bank token.
//
// Corresponds to Java InterbankOtcOutboundController (PR_33 Phase A).
type OtcOutboundHandler struct {
	svc *service.OtcOutboundService
	log *slog.Logger
}

// NewOtcOutboundHandler constructs the handler.
func NewOtcOutboundHandler(svc *service.OtcOutboundService, log *slog.Logger) *OtcOutboundHandler {
	if log == nil {
		log = slog.Default()
	}
	return &OtcOutboundHandler{svc: svc, log: log}
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// Create handles POST /api/interbank/otc/negotiations.
func (h *OtcOutboundHandler) Create(w http.ResponseWriter, r *http.Request) {
	principalID := extractPrincipalID(r)

	var req service.OutboundCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	// Allow BuyerLocalUserID in body to override the JWT principal.
	if req.BuyerLocalUserID == 0 {
		req.BuyerLocalUserID = principalID
	}

	remoteID, err := h.svc.CreateOutbound(r.Context(), principalID, req)
	if err != nil {
		h.handleOutboundError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, remoteID)
}

// List handles GET /api/interbank/otc/negotiations.
func (h *OtcOutboundHandler) List(w http.ResponseWriter, r *http.Request) {
	principalID := extractPrincipalID(r)
	isAdmin := hasAdminOrSupervisor(r)

	views, err := h.svc.ListForUser(r.Context(), principalID, isAdmin)
	if err != nil {
		h.handleOutboundError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, views)
}

// Get handles GET /api/interbank/otc/negotiations/{id}.
func (h *OtcOutboundHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	view, err := h.svc.GetOne(r.Context(), id)
	if err != nil {
		h.handleOutboundError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// Counter handles PUT /api/interbank/otc/negotiations/{id}/counter.
func (h *OtcOutboundHandler) Counter(w http.ResponseWriter, r *http.Request) {
	principalID := extractPrincipalID(r)
	id := chi.URLParam(r, "id")

	var req service.OutboundCounterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	result, err := h.svc.CounterOutbound(r.Context(), principalID, id, req)
	if err != nil {
		h.handleOutboundError(w, r, err)
		return
	}

	// Propagate partner's status code back to FE.
	if result.View != nil {
		writeJSON(w, result.StatusCode, result.View)
	} else {
		w.WriteHeader(result.StatusCode)
	}
}

// Accept handles POST /api/interbank/otc/negotiations/{id}/accept.
func (h *OtcOutboundHandler) Accept(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	statusCode, err := h.svc.AcceptOutbound(r.Context(), id)
	if err != nil {
		h.handleOutboundError(w, r, err)
		return
	}
	w.WriteHeader(statusCode)
}

// Delete handles DELETE /api/interbank/otc/negotiations/{id}.
func (h *OtcOutboundHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.svc.DeleteOutbound(r.Context(), id); err != nil {
		h.handleOutboundError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PartnerPublicStock handles GET /api/interbank/otc/public-stock?bankCode=222.
func (h *OtcOutboundHandler) PartnerPublicStock(w http.ResponseWriter, r *http.Request) {
	bankCode := 222 // default: Banka 2
	if bc := r.URL.Query().Get("bankCode"); bc != "" {
		n, err := strconv.Atoi(bc)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bankCode must be an integer")
			return
		}
		bankCode = n
	}

	entries, err := h.svc.FetchPartnerPublicStock(r.Context(), bankCode)
	if err != nil {
		h.log.WarnContext(r.Context(), "partner public-stock fetch failed (graceful empty)", "bankCode", bankCode, "err", err)
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	if entries == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

// ---------------------------------------------------------------------------
// Error mapping
// ---------------------------------------------------------------------------

func (h *OtcOutboundHandler) handleOutboundError(w http.ResponseWriter, r *http.Request, err error) {
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
	default:
		h.log.ErrorContext(r.Context(), "outbound: unexpected error", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

// ---------------------------------------------------------------------------
// JWT claim helpers
// ---------------------------------------------------------------------------

// extractPrincipalID reads the "id" claim from the verified JWT.
// Returns 0 if claims are absent or the id claim is missing/unparseable.
func extractPrincipalID(r *http.Request) int64 {
	claims, ok := auth.GetClaims(r.Context())
	if !ok || claims == nil {
		return 0
	}
	return claimToInt64(claims.ID)
}

// claimToInt64 converts the JWT id claim (string or number) to int64.
func claimToInt64(v any) int64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case json.Number:
		n, _ := strconv.ParseInt(x.String(), 10, 64)
		return n
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	}
	// Last resort: fmt.Sprint then parse.
	n, _ := strconv.ParseInt(fmt.Sprint(v), 10, 64)
	return n
}

// hasAdminOrSupervisor returns true when the JWT roles contain ADMIN or SUPERVISOR.
func hasAdminOrSupervisor(r *http.Request) bool {
	claims, ok := auth.GetClaims(r.Context())
	if !ok || claims == nil {
		return false
	}
	for _, role := range claims.Roles {
		upper := strings.ToUpper(role)
		if upper == "ADMIN" || upper == "SUPERVISOR" {
			return true
		}
	}
	return false
}
