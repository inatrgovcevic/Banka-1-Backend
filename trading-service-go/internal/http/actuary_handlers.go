package http

import (
	"context"
	"net/http"

	"banka1/go-platform/auth"
	"banka1/go-platform/httpx"
	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/clients"

	"github.com/shopspring/decimal"
)

// actuaryCtx tags the request context with the caller's bearer so the outbound
// user-service /employees calls carry the caller's (SUPERVISOR) token instead of a
// minted SERVICE token (user-service-go rejects the SERVICE role on /employees).
// See clients.WithCallerAuth. The /actuaries flow makes no SERVICE-only (/internal)
// calls, so forwarding the caller token here is safe.
func actuaryCtx(r *http.Request) context.Context {
	return clients.WithCallerAuth(r.Context(), r.Header.Get("Authorization"))
}

// ActuaryAgents ↔ GET /actuaries/agents (Page<ActuaryAgentDto>).
func (h *Handlers) ActuaryAgents(w http.ResponseWriter, r *http.Request) {
	email := optionalQueryParam(r, "email")
	ime := optionalQueryParam(r, "ime")
	prezime := optionalQueryParam(r, "prezime")
	pozicija := optionalQueryParam(r, "pozicija")
	page := queryIntDefault(r, "page", 0)
	size := queryIntDefault(r, "size", 10)

	resp, err := h.app.Actuary.GetAgents(actuaryCtx(r), email, ime, prezime, pozicija, page, size)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// ActuarySetLimit ↔ PUT /actuaries/agents/{id}/limit.
func (h *Handlers) ActuarySetLimit(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	var req api.SetLimitRequest
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	if fields := validateLimit(req.Limit); fields != nil {
		writeDomainError(w, r, api.NewOrderValidation(fields))
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	if err := h.app.Actuary.SetLimit(actuaryCtx(r), principal.ID, principal.Role, id, *req.Limit); err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, api.SimpleSuccess("Limit updated successfully"))
}

// ActuaryResetLimit ↔ PUT /actuaries/agents/{id}/reset-limit.
func (h *Handlers) ActuaryResetLimit(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	if err := h.app.Actuary.ResetLimit(actuaryCtx(r), principal.ID, principal.Role, id); err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, api.SimpleSuccess("Limit reset successfully"))
}

// ActuaryNeedApproval ↔ PUT /actuaries/agents/{id}/need-approval.
func (h *Handlers) ActuaryNeedApproval(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	var req api.SetNeedApprovalRequest
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	if req.NeedApproval == nil {
		writeDomainError(w, r, api.NewOrderValidation(map[string]string{"needApproval": "must not be null"}))
		return
	}
	if err := h.app.Actuary.SetNeedApproval(actuaryCtx(r), id, *req.NeedApproval); err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, api.SimpleSuccess("Need-approval flag updated successfully"))
}

// ActuaryProfit ↔ GET /actuaries/profit (List<ActuaryProfitDto>).
func (h *Handlers) ActuaryProfit(w http.ResponseWriter, r *http.Request) {
	from, err := parseDateTimeParam(r, "from")
	if err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Invalid value for parameter 'from'"))
		return
	}
	to, err := parseDateTimeParam(r, "to")
	if err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Invalid value for parameter 'to'"))
		return
	}
	resp, err := h.app.Actuary.ProfitByActuary(actuaryCtx(r), from, to)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// ActuaryBankSummary ↔ GET /actuaries/profit/bank-summary (BankProfitSummaryDto).
func (h *Handlers) ActuaryBankSummary(w http.ResponseWriter, r *http.Request) {
	from, err := parseDateTimeParam(r, "from")
	if err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Invalid value for parameter 'from'"))
		return
	}
	to, err := parseDateTimeParam(r, "to")
	if err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Invalid value for parameter 'to'"))
		return
	}
	resp, err := h.app.Actuary.BankProfitSummary(actuaryCtx(r), from, to)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// validateLimit reproduces SetLimitRequestDto bean validation
// (@NotNull @DecimalMin(value="0.0", inclusive=false)). Returns nil when valid.
//
// NOTE: the exact Hibernate Validator default messages must be confirmed against
// the live Java service during the parity sweep (the @DecimalMin message in
// particular is version-sensitive) and adjusted if they differ.
func validateLimit(limit *decimal.Decimal) map[string]string {
	if limit == nil {
		return map[string]string{"limit": "must not be null"}
	}
	if !limit.GreaterThan(decimal.Zero) {
		return map[string]string{"limit": "must be greater than 0.0"}
	}
	return nil
}
