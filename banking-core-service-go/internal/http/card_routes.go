package http

import (
	"net/http"
	"strings"

	"banka1/banking-core-service-go/internal/service"
)

func (h *Handler) autoCreateCard(w http.ResponseWriter, r *http.Request) {
	var req service.AutoCardCreationRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.CardService.CreateAutomaticCard(r.Context(), req)
	respond(w, resp, http.StatusCreated, err)
}

func (h *Handler) requestPersonalCard(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(w, r, false)
	if !ok {
		return
	}
	var req service.ClientCardRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.CardService.RequestPersonalCard(r.Context(), principal, req)
	respond(w, resp, http.StatusCreated, err)
}

func (h *Handler) requestBusinessCard(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(w, r, false)
	if !ok {
		return
	}
	var req service.BusinessCardRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.CardService.RequestBusinessCard(r.Context(), principal, req)
	respond(w, resp, http.StatusCreated, err)
}

func (h *Handler) cardsForClient(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseIntPath(w, rawID)
	if !ok {
		return
	}
	resp, err := h.services.CardService.GetCardsForClient(r.Context(), id)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) cardDetails(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseIntPath(w, rawID)
	if !ok {
		return
	}
	resp, err := h.services.CardService.GetCardByID(r.Context(), id)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) blockCard(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseIntPath(w, rawID)
	if !ok {
		return
	}
	respondEmptyOK(w, h.services.CardService.BlockCard(r.Context(), id))
}

func (h *Handler) unblockCard(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseIntPath(w, rawID)
	if !ok {
		return
	}
	respondEmptyOK(w, h.services.CardService.UnblockCard(r.Context(), id))
}

func (h *Handler) deactivateCard(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseIntPath(w, rawID)
	if !ok {
		return
	}
	respondEmptyOK(w, h.services.CardService.DeactivateCard(r.Context(), id))
}

func (h *Handler) updateCardLimit(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseIntPath(w, rawID)
	if !ok {
		return
	}
	var req service.UpdateCardLimitRequest
	if !decode(w, r, &req) {
		return
	}
	respondEmptyOK(w, h.services.CardService.UpdateCardLimit(r.Context(), id, req.CardLimit))
}

func (h *Handler) cardsByAccount(w http.ResponseWriter, r *http.Request, accountNumber string) {
	resp, err := h.services.CardService.GetCardsByAccount(r.Context(), strings.TrimSpace(accountNumber))
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) internalCardsByAccount(w http.ResponseWriter, r *http.Request, accountNumber string) {
	resp, err := h.services.CardService.GetInternalCardsByAccount(r.Context(), strings.TrimSpace(accountNumber))
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) allCards(w http.ResponseWriter, r *http.Request) {
	page, size := pageParams(r)
	resp, err := h.services.CardService.GetAllCards(
		r.Context(),
		page,
		size,
		r.URL.Query().Get("status"),
		r.URL.Query().Get("search"),
	)
	respond(w, resp, http.StatusOK, err)
}
