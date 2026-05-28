package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"banka1/banking-core-service-go/internal/decimal"
	"banka1/banking-core-service-go/internal/service"
)

func (h *Handler) newPayment(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(w, r, true)
	if !ok {
		return
	}
	var req service.NewPaymentRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Transactions.NewPayment(r.Context(), principal, req)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) transactionsByClient(w http.ResponseWriter, r *http.Request, mode string) {
	id, ok := intQueryRequired(w, r, "id")
	if !ok {
		return
	}
	page, size := pageParams(r)
	var resp any
	var err error
	switch mode {
	case "sender":
		resp, err = h.services.Transactions.FindBySenderClient(r.Context(), id, page, size)
	case "recipient":
		resp, err = h.services.Transactions.FindByRecipientClient(r.Context(), id, page, size)
	default:
		resp, err = h.services.Transactions.FindByClient(r.Context(), id, page, size)
	}
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) transactionsByThisClient(w http.ResponseWriter, r *http.Request, mode string) {
	principal, ok := h.principalFromRequest(w, r, true)
	if !ok {
		return
	}
	page, size := pageParams(r)
	var resp any
	var err error
	switch mode {
	case "sender":
		resp, err = h.services.Transactions.FindBySenderClient(r.Context(), principal.ID, page, size)
	case "recipient":
		resp, err = h.services.Transactions.FindByRecipientClient(r.Context(), principal.ID, page, size)
	default:
		resp, err = h.services.Transactions.FindByClient(r.Context(), principal.ID, page, size)
	}
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) transactionsForAccount(w http.ResponseWriter, r *http.Request, accountNumber string, employeeAccess bool) {
	principal, ok := h.principalFromRequest(w, r, !employeeAccess)
	if !ok {
		return
	}
	page, size := pageParams(r)
	resp, err := h.services.Transactions.FindForAccount(r.Context(), principal, accountNumber, page, size, employeeAccess)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) findPayments(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(w, r, false)
	if !ok {
		return
	}
	filter, ok := paymentFilterFromQuery(w, r)
	if !ok {
		return
	}
	page, size := pageParams(r)
	resp, err := h.services.Transactions.FindPayments(r.Context(), principal, filter, page, size)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) paymentRecipientsList(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(w, r, true)
	if !ok {
		return
	}
	page := intQuery(r, "page", 0)
	size := intQuery(r, "size", 20)
	resp, err := h.services.Transactions.ListRecipients(r.Context(), principal.ID, page, size)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) paymentRecipientCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(w, r, true)
	if !ok {
		return
	}
	var req service.PaymentRecipientRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Transactions.CreateRecipient(r.Context(), principal.ID, req)
	respond(w, resp, http.StatusCreated, err)
}

func (h *Handler) paymentRecipientUpdate(w http.ResponseWriter, r *http.Request, rawID string) {
	principal, ok := h.principalFromRequest(w, r, true)
	if !ok {
		return
	}
	id, parsed := parseIntPath(w, rawID)
	if !parsed {
		return
	}
	var req service.PaymentRecipientRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Transactions.UpdateRecipient(r.Context(), principal.ID, id, req)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) paymentRecipientDelete(w http.ResponseWriter, r *http.Request, rawID string) {
	principal, ok := h.principalFromRequest(w, r, true)
	if !ok {
		return
	}
	id, parsed := parseIntPath(w, rawID)
	if !parsed {
		return
	}
	respondNoContent(w, h.services.Transactions.DeleteRecipient(r.Context(), principal.ID, id))
}

func (h *Handler) executeTransfer(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(w, r, true)
	if !ok {
		return
	}
	var req service.TransferRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Transfers.Execute(r.Context(), principal, req)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) listTransfers(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(w, r, true)
	if !ok {
		return
	}
	clientID, parsed := intQueryRequired(w, r, "clientId")
	if !parsed {
		return
	}
	page := intQuery(r, "page", 0)
	size := intQuery(r, "size", 20)
	resp, err := h.services.Transfers.ListByClient(r.Context(), principal, clientID, page, size)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) transferDetails(w http.ResponseWriter, r *http.Request, orderNumber string) {
	principal, ok := h.principalFromRequest(w, r, true)
	if !ok {
		return
	}
	resp, err := h.services.Transfers.GetDetails(r.Context(), principal, orderNumber)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) transfersForAccount(w http.ResponseWriter, r *http.Request, accountNumber string) {
	principal, ok := h.principalFromRequest(w, r, true)
	if !ok {
		return
	}
	page := intQuery(r, "page", 0)
	size := intQuery(r, "size", 20)
	resp, err := h.services.Transfers.ListByAccount(r.Context(), principal, accountNumber, page, size)
	respond(w, resp, http.StatusOK, err)
}

func paymentFilterFromQuery(w http.ResponseWriter, r *http.Request) (service.PaymentFilter, bool) {
	q := r.URL.Query()
	filter := service.PaymentFilter{
		AccountNumber: strings.TrimSpace(q.Get("accountNumber")),
		Status:        strings.TrimSpace(q.Get("status")),
	}
	var ok bool
	if filter.FromDate, ok = optionalTimeQuery(w, q.Get("fromDate")); !ok {
		return service.PaymentFilter{}, false
	}
	if filter.ToDate, ok = optionalTimeQuery(w, q.Get("toDate")); !ok {
		return service.PaymentFilter{}, false
	}
	if filter.InitialAmountMin, ok = optionalDecimalQuery(w, q.Get("initialAmountMin")); !ok {
		return service.PaymentFilter{}, false
	}
	if filter.InitialAmountMax, ok = optionalDecimalQuery(w, q.Get("initialAmountMax")); !ok {
		return service.PaymentFilter{}, false
	}
	if filter.FinalAmountMin, ok = optionalDecimalQuery(w, q.Get("finalAmountMin")); !ok {
		return service.PaymentFilter{}, false
	}
	if filter.FinalAmountMax, ok = optionalDecimalQuery(w, q.Get("finalAmountMax")); !ok {
		return service.PaymentFilter{}, false
	}
	return filter, true
}

func optionalTimeQuery(w http.ResponseWriter, raw string) (*time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02"} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return &parsed, true
		}
	}
	writeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Neispravni podaci", "neispravan datum")
	return nil, false
}

func optionalDecimalQuery(w http.ResponseWriter, raw string) (*decimal.Decimal, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	parsed, err := decimal.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Neispravni podaci", err.Error())
		return nil, false
	}
	return &parsed, true
}

func intQueryRequired(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		writeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Neispravni podaci", key+" je obavezan")
		return 0, false
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Neispravni podaci", key+" nije validan")
		return 0, false
	}
	return value, true
}

func decodeRawDecimal(raw json.RawMessage) (decimal.Decimal, error) {
	return parseRawDecimal(raw)
}
