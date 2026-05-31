package http

import (
	"net/http"

	"banka1/trading-service-go/internal/api"

	gpauth "banka1/go-platform/auth"
	"banka1/go-platform/httpx"
)

// PortfolioSummary ↔ GET /portfolio.
func (h *Handlers) PortfolioSummary(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	resp, err := h.app.Portfolio.GetPortfolio(r.Context(), principal.ID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// PortfolioSetPublic ↔ PUT /portfolio/{id}/set-public. 200 with empty body.
func (h *Handlers) PortfolioSetPublic(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	var req api.SetPublicQuantityRequest
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	if err := h.app.Portfolio.SetPublicQuantity(r.Context(), principal.ID, id, req); err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// PortfolioExerciseOption ↔ POST /portfolio/{id}/exercise-option. 200 empty body.
func (h *Handlers) PortfolioExerciseOption(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	// AuthenticatedUser.isAgent(): AGENT, SUPERVISOR or ADMIN.
	isAgent := principal.HasAnyRole("AGENT", "SUPERVISOR", "ADMIN")
	if err := h.app.Portfolio.ExerciseOption(r.Context(), principal.ID, isAgent, id); err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}
