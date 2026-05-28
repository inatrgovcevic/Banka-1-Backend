package http

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"banka1/banking-core-service-go/internal/decimal"
	"banka1/banking-core-service-go/internal/service"
)

func (h *Handler) listCurrencies(w http.ResponseWriter, r *http.Request) {
	resp, err := h.services.Accounts.ListCurrencies(r.Context())
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) listCurrenciesPage(w http.ResponseWriter, r *http.Request) {
	page, size := pageParams(r)
	resp, err := h.services.Accounts.ListCurrenciesPage(r.Context(), page, size)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) getCurrencyByQuery(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if strings.TrimSpace(code) == "" {
		writeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Neispravni podaci", "code je obavezan")
		return
	}
	resp, err := h.services.Accounts.GetCurrency(r.Context(), code)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) getCurrencyByPath(w http.ResponseWriter, r *http.Request, code string) {
	resp, err := h.services.Accounts.GetCurrency(r.Context(), code)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) createCheckingAccount(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromRequest(w, r, false)
	if !ok {
		return
	}
	var req service.CheckingAccountRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Accounts.CreateCheckingAccount(r.Context(), principal, req)
	respond(w, resp, http.StatusCreated, err)
}

func (h *Handler) createFXAccount(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromRequest(w, r, false)
	if !ok {
		return
	}
	var req service.FXAccountRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Accounts.CreateFXAccount(r.Context(), principal, req)
	respond(w, resp, http.StatusCreated, err)
}

func (h *Handler) searchEmployeeAccounts(w http.ResponseWriter, r *http.Request) {
	page, size := pageParams(r)
	resp, err := h.services.Accounts.SearchAccounts(
		r.Context(),
		r.URL.Query().Get("ime"),
		r.URL.Query().Get("prezime"),
		r.URL.Query().Get("accountNumber"),
		page,
		size,
	)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) editEmployeeAccountStatus(w http.ResponseWriter, r *http.Request, accountNumber string) {
	var req service.EditStatusRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Accounts.EditStatus(r.Context(), accountNumber, req)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) employeeAccountDetails(w http.ResponseWriter, r *http.Request, accountNumber string) {
	resp, err := h.services.Accounts.GetAccountDetailsByNumber(r.Context(), accountNumber, nil)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) employeeClientAccounts(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseIntPath(w, rawID)
	if !ok {
		return
	}
	page, size := pageParams(r)
	resp, err := h.services.Accounts.GetClientAccountsPage(r.Context(), id, page, size, true)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) employeeBankAccounts(w http.ResponseWriter, r *http.Request) {
	resp, err := h.services.Accounts.GetBankAccounts(r.Context())
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) employeeBankAccountByCurrency(w http.ResponseWriter, r *http.Request, currency string) {
	resp, err := h.services.Accounts.GetBankAccountDetailsByCurrency(r.Context(), currency)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) employeeCompany(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseIntPath(w, rawID)
	if !ok {
		return
	}
	resp, err := h.services.Accounts.GetCompany(r.Context(), id)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) updateEmployeeCompany(w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseIntPath(w, rawID)
	if !ok {
		return
	}
	var req service.UpdateCompanyRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Accounts.UpdateCompany(r.Context(), id, req)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) clientAccountsPage(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromRequest(w, r, true)
	if !ok {
		return
	}
	page, size := pageParams(r)
	resp, err := h.services.Accounts.GetClientAccountsPage(r.Context(), principal.ID, page, size, false)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) clientEditNameByNumber(w http.ResponseWriter, r *http.Request, accountNumber string) {
	principal, ok := principalFromRequest(w, r, true)
	if !ok {
		return
	}
	var req service.EditAccountNameRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Accounts.EditAccountNameByNumber(r.Context(), principal, accountNumber, req)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) clientEditNameByID(w http.ResponseWriter, r *http.Request, rawID string) {
	principal, ok := principalFromRequest(w, r, true)
	if !ok {
		return
	}
	id, ok := parseIntPath(w, rawID)
	if !ok {
		return
	}
	var req service.EditAccountNameRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Accounts.EditAccountNameByID(r.Context(), principal, id, req)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) clientEditLimitsByID(w http.ResponseWriter, r *http.Request, rawID string) {
	principal, ok := principalFromRequest(w, r, true)
	if !ok {
		return
	}
	id, ok := parseIntPath(w, rawID)
	if !ok {
		return
	}
	var req service.EditAccountLimitRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Accounts.EditAccountLimitByID(r.Context(), principal, id, req)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) clientEditLimitsByNumber(w http.ResponseWriter, r *http.Request, accountNumber string) {
	principal, ok := principalFromRequest(w, r, true)
	if !ok {
		return
	}
	var req service.EditAccountLimitRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Accounts.EditAccountLimitByNumber(r.Context(), principal, accountNumber, req)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) clientAccountDetailsByID(w http.ResponseWriter, r *http.Request, rawID string) {
	principal, ok := principalFromRequest(w, r, true)
	if !ok {
		return
	}
	id, ok := parseIntPath(w, rawID)
	if !ok {
		return
	}
	resp, err := h.services.Accounts.GetAccountDetailsByID(r.Context(), id, &principal.ID)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) clientAccountDetailsByNumber(w http.ResponseWriter, r *http.Request, accountNumber string) {
	principal, ok := principalFromRequest(w, r, true)
	if !ok {
		return
	}
	resp, err := h.services.Accounts.GetAccountDetailsByNumber(r.Context(), accountNumber, &principal.ID)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) clientAccountCards(w http.ResponseWriter, r *http.Request, rawID string) {
	if _, ok := principalFromRequest(w, r, true); !ok {
		return
	}
	if _, ok := parseIntPath(w, rawID); !ok {
		return
	}
	page, size := pageParams(r)
	respond(w, service.NewPage([]any{}, page, size, 0), http.StatusOK, nil)
}

func (h *Handler) internalTransaction(w http.ResponseWriter, r *http.Request, sameOwnerRequired bool) {
	var req service.PaymentRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Accounts.Transaction(r.Context(), req, sameOwnerRequired)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) exchangeBuy(w http.ResponseWriter, r *http.Request) {
	var req service.OneSidedTransactionRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Accounts.ExchangeBuy(r.Context(), req)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) exchangeSell(w http.ResponseWriter, r *http.Request) {
	var req service.OneSidedTransactionRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Accounts.ExchangeSell(r.Context(), req)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) transactionFromBank(w http.ResponseWriter, r *http.Request) {
	var req service.BankPaymentRequest
	if !decode(w, r, &req) {
		return
	}
	respondEmptyOK(w, h.services.Accounts.TransactionFromBank(r.Context(), req))
}

func (h *Handler) creditBank(w http.ResponseWriter, r *http.Request) {
	currency, amount, ok := decodeCreditDebitBank(w, r)
	if !ok {
		return
	}
	respondEmptyOK(w, h.services.Accounts.CreditBank(r.Context(), currency, amount))
}

func (h *Handler) debitBank(w http.ResponseWriter, r *http.Request) {
	currency, amount, ok := decodeCreditDebitBank(w, r)
	if !ok {
		return
	}
	respondEmptyOK(w, h.services.Accounts.DebitBank(r.Context(), currency, amount))
}

func (h *Handler) getStateAccount(w http.ResponseWriter, r *http.Request, currency string) {
	if !strings.EqualFold(currency, "RSD") {
		writeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Neispravni podaci", "Drzavni racun postoji samo za RSD valutu, trazena valuta: "+currency)
		return
	}
	resp, err := h.services.Accounts.GetStateAccount(r.Context(), currency)
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) internalInfo(w http.ResponseWriter, r *http.Request) {
	resp, err := h.services.Accounts.Info(r.Context(), r.URL.Query().Get("fromBankNumber"), r.URL.Query().Get("toBankNumber"))
	respond(w, resp, http.StatusOK, err)
}

func (h *Handler) createSystemAccount(w http.ResponseWriter, r *http.Request) {
	var req service.CreateSystemAccountRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.services.Accounts.CreateSystemAccount(r.Context(), req)
	respond(w, resp, http.StatusCreated, err)
}

func decodeCreditDebitBank(w http.ResponseWriter, r *http.Request) (string, decimal.Decimal, bool) {
	var decoded struct {
		CurrencyCode string          `json:"currencyCode"`
		Amount       json.RawMessage `json:"amount"`
	}
	if !decode(w, r, &decoded) {
		return "", decimal.Decimal{}, false
	}
	amount, err := parseRawDecimal(decoded.Amount)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Neispravni podaci", err.Error())
		return "", decimal.Decimal{}, false
	}
	return decoded.CurrencyCode, amount, true
}

func respondEmptyOK(w http.ResponseWriter, err error) {
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func pageParams(r *http.Request) (int, int) {
	page := intQuery(r, "page", 0)
	size := intQuery(r, "size", 10)
	if page < 0 {
		page = 0
	}
	if size <= 0 {
		size = 10
	}
	return page, size
}

func intQuery(r *http.Request, key string, fallback int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func principalFromRequest(w http.ResponseWriter, r *http.Request, required bool) (service.Principal, bool) {
	roles := rolesFromHeader(r)
	for _, header := range []string{"X-User-Id", "X-Client-Id", "X-Owner-Id"} {
		if value := strings.TrimSpace(r.Header.Get(header)); value != "" {
			id, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				return service.Principal{ID: id, Roles: roles}, true
			}
		}
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	if token != "" && token != auth {
		if principal, ok := principalFromToken(token); ok {
			if len(principal.Roles) == 0 {
				principal.Roles = roles
			}
			return principal, true
		}
	}
	if required {
		writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Pristup odbijen", "Nedostaje id korisnika")
		return service.Principal{}, false
	}
	return service.Principal{}, true
}

func principalIDFromToken(token string) (int64, bool) {
	principal, ok := principalFromToken(token)
	return principal.ID, ok
}

func principalFromToken(token string) (service.Principal, bool) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return service.Principal{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return service.Principal{}, false
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return service.Principal{}, false
	}
	var principal service.Principal
	principal.Roles = claimAsStrings(claims["roles"])
	for _, key := range []string{"id", "userId", "clientId", "client_id", "sub"} {
		if id, ok := claimAsInt(claims[key]); ok {
			principal.ID = id
			return principal, true
		}
	}
	return service.Principal{}, false
}

func claimAsInt(value any) (int64, bool) {
	switch v := value.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case json.Number:
		id, err := v.Int64()
		return id, err == nil
	case string:
		id, err := strconv.ParseInt(v, 10, 64)
		return id, err == nil
	default:
		return 0, false
	}
}

func rolesFromHeader(r *http.Request) []string {
	for _, header := range []string{"X-User-Roles", "X-Roles"} {
		raw := strings.TrimSpace(r.Header.Get(header))
		if raw == "" {
			continue
		}
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}
	return nil
}

func claimAsStrings(value any) []string {
	switch v := value.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	case string:
		if v == "" {
			return nil
		}
		if strings.Contains(v, ",") {
			return strings.Split(v, ",")
		}
		return []string{v}
	default:
		return nil
	}
}
