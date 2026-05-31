package http

import (
	"net/http"

	"banka1/banking-core-service-go/internal/service"
)

func (h *Handler) verificationGenerate(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(w, r, true)
	if !ok {
		return
	}
	var req service.VerificationGenerateRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Verification.Generate(r.Context(), principal, req)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) verificationValidate(w http.ResponseWriter, r *http.Request) {
	var req service.VerificationValidateRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Verification.Validate(r.Context(), req)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) verificationStatus(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseIntPath(w, rawID)
	if !ok {
		return
	}
	resp, err := h.services.Verification.Status(r.Context(), id)
	respond(w, resp, http.StatusOK, err)
}
