package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/funds"

	gpauth "banka1/go-platform/auth"
	"banka1/go-platform/httpx"
	"github.com/shopspring/decimal"
)

// ---------------------- discovery / details (authenticated) ----------------

// FundsDiscovery ↔ GET /funds.
func (h *Handlers) FundsDiscovery(w http.ResponseWriter, r *http.Request) {
	sortField := r.URL.Query().Get("sortField")
	sortDir := r.URL.Query().Get("sortDirection")
	resp, err := h.app.Funds.Discovery(r.Context(), sortField, sortDir)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// FundsDetails ↔ GET /funds/{id}.
func (h *Handlers) FundsDetails(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	resp, err := h.app.Funds.Details(r.Context(), id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// FundsAnalytics ↔ GET /funds/{id}/analytics.
func (h *Handlers) FundsAnalytics(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	resp, err := h.app.Funds.Analytics(r.Context(), id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// FundsSecurities ↔ GET /funds/{id}/securities.
func (h *Handlers) FundsSecurities(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	views, err := h.app.Funds.GetEnrichedHoldings(r.Context(), id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	// Project to the DTO shape (FundHoldingDto JSON tags).
	out := make([]funds.HoldingDto, 0, len(views))
	for _, v := range views {
		out = append(out, funds.HoldingDto{
			ID:                v.ID,
			Ticker:            v.Ticker,
			Quantity:          v.Quantity,
			AvgUnitPrice:      v.AvgUnitPrice,
			InitialMarginCost: v.InitialMarginCost,
			Price:             v.Price,
			Change:            v.Change,
			Volume:            v.Volume,
			AcquisitionDate:   api.NewLocalDateTime(v.AcquisitionDate),
		})
	}
	httpx.JSON(w, http.StatusOK, out)
}

// FundsPerformance ↔ GET /funds/{id}/performance.
func (h *Handlers) FundsPerformance(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	resp, err := h.app.Funds.FundPerformance(r.Context(), id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// ---------------------- supervisor: create + management ----------------

// FundsCreate ↔ POST /funds (201).
func (h *Handlers) FundsCreate(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	var req struct {
		Naziv               string          `json:"naziv"`
		Opis                *string         `json:"opis"`
		MinimumContribution decimal.Decimal `json:"minimumContribution"`
		DividendStrategy    string          `json:"dividendStrategy"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	resp, err := h.app.Funds.CreateFund(r.Context(), strings.TrimSpace(req.Naziv), req.Opis, req.MinimumContribution, req.DividendStrategy, principal.ID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

// FundsSupervised ↔ GET /funds/supervised.
func (h *Handlers) FundsSupervised(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	resp, err := h.app.Funds.SupervisedBy(r.Context(), principal.ID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// ---------------------- client / bank invest + redeem (202) -------------

// FundsInvest ↔ POST /funds/{id}/invest (202).
func (h *Handlers) FundsInvest(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	var req struct {
		Amount            decimal.Decimal `json:"amount"`
		FromAccountNumber string          `json:"fromAccountNumber"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	resp, err := h.app.Funds.Invest(r.Context(), id, principal.ID, req.Amount, strings.TrimSpace(req.FromAccountNumber))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, resp)
}

// FundsRedeem ↔ POST /funds/{id}/redeem (202).
func (h *Handlers) FundsRedeem(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	var req struct {
		Amount          decimal.Decimal `json:"amount"`
		ToAccountNumber string          `json:"toAccountNumber"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	resp, err := h.app.Funds.Redeem(r.Context(), id, principal.ID, req.Amount, strings.TrimSpace(req.ToAccountNumber))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, resp)
}

// FundsBankInvest ↔ POST /funds/{id}/bank-invest (202).
func (h *Handlers) FundsBankInvest(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	var req struct {
		Amount            decimal.Decimal `json:"amount"`
		FromAccountNumber string          `json:"fromAccountNumber"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	resp, err := h.app.Funds.BankInvest(r.Context(), id, req.Amount, strings.TrimSpace(req.FromAccountNumber))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, resp)
}

// FundsBankRedeem ↔ POST /funds/{id}/bank-redeem (202).
func (h *Handlers) FundsBankRedeem(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	var req struct {
		Amount          decimal.Decimal `json:"amount"`
		ToAccountNumber string          `json:"toAccountNumber"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	resp, err := h.app.Funds.BankRedeem(r.Context(), id, req.Amount, strings.TrimSpace(req.ToAccountNumber))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, resp)
}

// ---------------------- positions / transactions ----------------------

// FundsMyPositions ↔ GET /funds/my-positions.
func (h *Handlers) FundsMyPositions(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	resp, err := h.app.Funds.MyPositions(r.Context(), principal.ID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// FundsBankPositions ↔ GET /funds/bank-positions.
func (h *Handlers) FundsBankPositions(w http.ResponseWriter, r *http.Request) {
	resp, err := h.app.Funds.BankPositions(r.Context())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// FundsPositions ↔ GET /funds/{id}/positions.
func (h *Handlers) FundsPositions(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	resp, err := h.app.Funds.FundPositions(r.Context(), id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// FundsMyTransactions ↔ GET /funds/my-transactions.
func (h *Handlers) FundsMyTransactions(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	resp, err := h.app.Funds.MyTransactions(r.Context(), principal.ID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// FundsTransactions ↔ GET /funds/{id}/transactions.
func (h *Handlers) FundsTransactions(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	resp, err := h.app.Funds.FundTransactions(r.Context(), id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// ---------------------- supervisor sell / dividend / admin ----------------

// FundsSellSecurity ↔ POST /funds/{id}/securities/{ticker}/sell.
func (h *Handlers) FundsSellSecurity(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	ticker := strings.TrimSpace(r.PathValue("ticker"))
	if ticker == "" {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Missing ticker path variable."))
		return
	}
	var req struct {
		Quantity int `json:"quantity"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	if req.Quantity <= 0 {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Quantity must be > 0."))
		return
	}
	resp, err := h.app.Liquidation.SellHolding(r.Context(), id, ticker, req.Quantity)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// FundsRecordDividend ↔ POST /funds/{id}/dividends (201).
func (h *Handlers) FundsRecordDividend(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	var raw struct {
		StockTicker      string          `json:"stockTicker"`
		DividendPerShare decimal.Decimal `json:"dividendPerShare"`
		Currency         string          `json:"currency"`
		PaymentDate      string          `json:"paymentDate"`
		Strategy         string          `json:"strategy"`
	}
	if err := decodeJSONLenient(r, &raw); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	paymentDate, err := time.Parse("2006-01-02", strings.TrimSpace(raw.PaymentDate))
	if err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Invalid paymentDate (expected ISO yyyy-MM-dd)."))
		return
	}
	req := funds.DividendRequest{
		StockTicker:      strings.TrimSpace(raw.StockTicker),
		DividendPerShare: raw.DividendPerShare,
		Currency:         strings.TrimSpace(raw.Currency),
		PaymentDate:      paymentDate,
		Strategy:         strings.TrimSpace(raw.Strategy),
	}
	resp, err := h.app.Dividend.RecordDividend(r.Context(), id, req)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

// FundsReassignManager ↔ PATCH /funds/admin/reassign-manager (204).
func (h *Handlers) FundsReassignManager(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldManagerID int64 `json:"oldManagerId"`
		NewManagerID int64 `json:"newManagerId"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	if req.OldManagerID == 0 || req.NewManagerID == 0 {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "oldManagerId and newManagerId are required."))
		return
	}
	if err := h.app.Funds.ReassignManager(r.Context(), req.OldManagerID, req.NewManagerID); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------- /funds/internal (SERVICE) -------------------------

// FundsInternalLiquidate ↔ POST /funds/internal/{fundId}/liquidate.
func (h *Handlers) FundsInternalLiquidate(w http.ResponseWriter, r *http.Request) {
	fundID, err := parsePathFundID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	var req struct {
		TargetAmount decimal.Decimal `json:"targetAmount"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	correlationID := strings.TrimSpace(r.Header.Get("X-Correlation-Id"))
	if correlationID == "" {
		correlationID = "no-correlation"
	}
	resp, err := h.app.Liquidation.LiquidateForFund(r.Context(), fundID, req.TargetAmount, correlationID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// FundsInternalAddHolding ↔ POST /funds/internal/{fundId}/holdings/add.
func (h *Handlers) FundsInternalAddHolding(w http.ResponseWriter, r *http.Request) {
	fundID, err := parsePathFundID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	var req struct {
		Ticker    string          `json:"ticker"`
		Quantity  int             `json:"quantity"`
		UnitPrice decimal.Decimal `json:"unitPrice"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	if strings.TrimSpace(req.Ticker) == "" || req.Quantity <= 0 || req.UnitPrice.Sign() <= 0 {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "ticker, quantity > 0 and unitPrice > 0 are required."))
		return
	}
	holding, err := h.app.Holding.AddOrUpdate(r.Context(), nil, fundID, strings.TrimSpace(req.Ticker), req.Quantity, req.UnitPrice)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	h.app.Snapshot.RecordSilently(r.Context(), fundID)
	// Project to a stable JSON shape matching Java FundHolding entity.
	httpx.JSON(w, http.StatusOK, struct {
		ID           int64           `json:"id"`
		FundID       int64           `json:"fundId"`
		StockTicker  string          `json:"stockTicker"`
		Quantity     int             `json:"quantity"`
		AvgUnitPrice decimal.Decimal `json:"avgUnitPrice"`
		Deleted      bool            `json:"deleted"`
		CreatedAt    time.Time       `json:"createdAt"`
		UpdatedAt    *time.Time      `json:"updatedAt"`
		Version      int64           `json:"version"`
	}{
		ID: holding.ID, FundID: holding.FundID, StockTicker: holding.StockTicker,
		Quantity: holding.Quantity, AvgUnitPrice: holding.AvgUnitPrice,
		Deleted: holding.Deleted, CreatedAt: holding.CreatedAt,
		UpdatedAt: holding.UpdatedAt, Version: holding.Version,
	})
}

// FundsInternalDebitLiquidity ↔ POST /funds/internal/{fundId}/liquidity/debit (204).
func (h *Handlers) FundsInternalDebitLiquidity(w http.ResponseWriter, r *http.Request) {
	fundID, err := parsePathFundID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	var req struct {
		Amount decimal.Decimal `json:"amount"`
		Reason string          `json:"reason"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	if req.Amount.Sign() <= 0 {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "amount must be > 0."))
		return
	}
	if err := h.app.Funds.DebitLiquidity(r.Context(), fundID, req.Amount, strings.TrimSpace(req.Reason)); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parsePathFundID reads {fundId} as int64 (the /funds/internal/{fundId}/...
// routes use that variable name, not {id}). Same 400 mapping as parsePathID.
func parsePathFundID(r *http.Request) (int64, error) {
	raw := r.PathValue("fundId")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, api.NewOrderError(http.StatusBadRequest,
			"Invalid value '"+raw+"' for parameter 'fundId', expected type: Long.")
	}
	return id, nil
}

// jsonNumber is a temporary helper retained for callers that need to decode a
// loose number from a request body (saga events sometimes flow as numeric
// IDs). Currently unused inside this file but kept package-private for
// symmetry with the saga package's decodeSagaEvent. The build will drop the
// symbol if no caller references it (Go allows unreferenced unexported
// helpers).
var _ = json.Number("0")
