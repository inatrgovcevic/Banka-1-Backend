package http

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"banka1/market-service-go/internal/api"
	"banka1/market-service-go/internal/auth"
	"banka1/market-service-go/internal/market"
	"banka1/market-service-go/internal/platform"
	"github.com/shopspring/decimal"
)

type Handlers struct {
	cfg platform.Config
	app *App
}

func NewHandlers(cfg platform.Config, app *App) *Handlers {
	return &Handlers{cfg: cfg, app: app}
}

func (h *Handlers) GetCurrentPriceFeed(w http.ResponseWriter, r *http.Request) {
	tickers := strings.Split(r.URL.Query().Get("tickers"), ",")
	platform.JSON(w, http.StatusOK, h.app.PriceFeed.GetCurrent(r.Context(), tickers))
}

func (h *Handlers) GetSinglePriceFeed(w http.ResponseWriter, r *http.Request) {
	ticker := strings.TrimPrefix(r.URL.Path, "/stocks/price-feed/single/")
	snapshot, ok := h.app.PriceFeed.GetSingle(r.Context(), ticker)
	if !ok {
		platform.StockError(w, r, http.StatusNotFound, "Ticker not found")
		return
	}
	platform.JSON(w, http.StatusOK, snapshot)
}

func (h *Handlers) StockInfo(w http.ResponseWriter, r *http.Request) {
	platform.JSON(w, http.StatusOK, map[string]any{
		"service":                 "stock-service",
		"status":                  "UP",
		"gatewayPrefix":           defaultString(r.Header.Get("X-Forwarded-Prefix"), "/stock"),
		"exchangeServiceBaseUrl":  h.cfg.Stock.ExchangeServiceBaseURL,
		"marketDataBaseUrl":       h.cfg.Stock.MarketDataBaseURL,
		"marketDataApiKeyConfigured": h.cfg.Stock.MarketDataAPIKey != "",
	})
}

func (h *Handlers) ExchangeInfo(w http.ResponseWriter, r *http.Request) {
	platform.JSON(w, http.StatusOK, map[string]any{
		"service":       "exchange-service",
		"status":        "UP",
		"gatewayPrefix": defaultString(r.Header.Get("X-Forwarded-Prefix"), "/api/exchange"),
	})
}

func (h *Handlers) ListStockExchanges(w http.ResponseWriter, r *http.Request) {
	items, err := h.app.MarketService.ListStockExchanges(r.Context())
	if err != nil {
		platform.StockError(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}
	platform.JSON(w, http.StatusOK, items)
}

func (h *Handlers) StockExchangeSubroutes(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/stock-exchanges/")
	if strings.HasSuffix(rest, "/is-open") {
		id, _ := strconv.ParseInt(strings.TrimSuffix(rest, "/is-open"), 10, 64)
		h.GetStockExchangeStatus(w, r, id, false)
		return
	}
	if strings.HasSuffix(rest, "/status") {
		id, _ := strconv.ParseInt(strings.TrimSuffix(rest, "/status"), 10, 64)
		h.GetStockExchangeStatus(w, r, id, true)
		return
	}
	if strings.HasSuffix(rest, "/toggle-active") && r.Method == http.MethodPut {
		if !hasAnyRole(r, "ADMIN", "SUPERVISOR") {
			platform.Error(w, http.StatusForbidden, "FORBIDDEN", "Insufficient role")
			return
		}
		id, _ := strconv.ParseInt(strings.TrimSuffix(rest, "/toggle-active"), 10, 64)
		h.ToggleStockExchange(w, r, id)
		return
	}
	http.NotFound(w, r)
}

func (h *Handlers) GetStockExchangeStatus(w http.ResponseWriter, r *http.Request, id int64, compact bool) {
	status, err := h.app.MarketService.GetExchangeStatus(r.Context(), id)
	if err != nil {
		platform.StockError(w, r, http.StatusNotFound, fmt.Sprintf("Stock exchange with id %d was not found.", id))
		return
	}
	if compact {
		afterHours := status.MarketPhase == "POST_MARKET"
		platform.JSON(w, http.StatusOK, map[string]any{"open": status.Open, "afterHours": afterHours, "closed": !status.Open && !afterHours})
		return
	}
	platform.JSON(w, http.StatusOK, status)
}

func (h *Handlers) ToggleStockExchange(w http.ResponseWriter, r *http.Request, id int64) {
	resp, err := h.app.MarketService.ToggleStockExchangeActive(r.Context(), id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	platform.JSON(w, http.StatusOK, resp)
}

func (h *Handlers) ListStockListings(w http.ResponseWriter, r *http.Request) {
	h.listListingsByType(w, r, market.ListingTypeStock)
}

func (h *Handlers) ListFuturesListings(w http.ResponseWriter, r *http.Request) {
	h.listListingsByType(w, r, market.ListingTypeFutures)
}

func (h *Handlers) ListForexListings(w http.ResponseWriter, r *http.Request) {
	h.listListingsByType(w, r, market.ListingTypeForex)
}

func (h *Handlers) listListingsByType(w http.ResponseWriter, r *http.Request, listingType market.ListingType) {
	page, _ := strconv.Atoi(defaultString(r.URL.Query().Get("page"), "0"))
	size, _ := strconv.Atoi(defaultString(r.URL.Query().Get("size"), "20"))
	if page < 0 {
		platform.StockError(w, r, http.StatusBadRequest, "Page must be zero or greater.")
		return
	}
	if size <= 0 {
		platform.StockError(w, r, http.StatusBadRequest, "Size must be greater than zero.")
		return
	}
	sortBy, err := normalizeSortBy(r.URL.Query().Get("sortBy"))
	if err != nil {
		platform.StockError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	sortDirection, err := normalizeSortDirection(r.URL.Query().Get("sortDirection"))
	if err != nil {
		platform.StockError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	filter := market.ListingFilter{
		Exchange: r.URL.Query().Get("exchange"),
		Search:   r.URL.Query().Get("search"),
	}
	if value := r.URL.Query().Get("minPrice"); value != "" { filter.MinPrice = &value }
	if value := r.URL.Query().Get("maxPrice"); value != "" { filter.MaxPrice = &value }
	if value := r.URL.Query().Get("minAsk"); value != "" { filter.MinAsk = &value }
	if value := r.URL.Query().Get("maxAsk"); value != "" { filter.MaxAsk = &value }
	if value := r.URL.Query().Get("minBid"); value != "" { filter.MinBid = &value }
	if value := r.URL.Query().Get("maxBid"); value != "" { filter.MaxBid = &value }
	if value := r.URL.Query().Get("minVolume"); value != "" {
		parsed, _ := strconv.ParseInt(value, 10, 64)
		filter.MinVolume = &parsed
	}
	if value := r.URL.Query().Get("maxVolume"); value != "" {
		parsed, _ := strconv.ParseInt(value, 10, 64)
		filter.MaxVolume = &parsed
	}
	if value := r.URL.Query().Get("settlementDateFrom"); value != "" {
		parsed, _ := time.Parse("2006-01-02", value)
		filter.SettlementDateFrom = &parsed
	}
	if value := r.URL.Query().Get("settlementDateTo"); value != "" {
		parsed, _ := time.Parse("2006-01-02", value)
		filter.SettlementDateTo = &parsed
	}
	resp, err := h.app.MarketService.ListListings(r.Context(), listingType, filter, page, size, sortBy, sortDirection)
	if err != nil {
		platform.StockError(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}
	platform.JSON(w, http.StatusOK, resp)
}

func (h *Handlers) ListingSubroutes(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/listings/")
	if strings.HasSuffix(rest, "/refresh") && r.Method == http.MethodPost {
		if !hasAnyRole(r, "ADMIN", "SUPERVISOR", "SERVICE") {
			platform.Error(w, http.StatusForbidden, "FORBIDDEN", "Insufficient role")
			return
		}
		id, _ := strconv.ParseInt(strings.TrimSuffix(rest, "/refresh"), 10, 64)
		resp, err := h.app.MarketService.RefreshListing(r.Context(), id)
		if err != nil {
			if strings.Contains(err.Error(), "not supported") {
				platform.Error(w, http.StatusUnprocessableEntity, "UNPROCESSABLE_ENTITY", err.Error())
				return
			}
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		platform.JSON(w, http.StatusOK, resp)
		return
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	period := strings.TrimSpace(r.URL.Query().Get("period"))
	if period == "" {
		platform.StockError(w, r, http.StatusBadRequest, "Required request parameter 'period' is not present.")
		return
	}
	listingType, err := h.app.MarketRepo.GetListingType(r.Context(), id)
	if err == nil && listingType == market.ListingTypeForex && hasAnyRole(r, "CLIENT_BASIC") && !hasAnyRole(r, "BASIC", "AGENT", "SUPERVISOR", "ADMIN", "SERVICE") {
		platform.StockError(w, r, http.StatusForbidden, "Forex listing details are not available to client users.")
		return
	}
	item, err := h.app.MarketService.GetListingDetails(r.Context(), id, period)
	if err != nil {
		platform.StockError(w, r, http.StatusNotFound, fmt.Sprintf("Listing with id %d was not found.", id))
		return
	}
	platform.JSON(w, http.StatusOK, item)
}

func (h *Handlers) InternalListingSubroutes(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/internal/listings/")
	if strings.HasSuffix(rest, "/refresh") && r.Method == http.MethodPost {
		id, _ := strconv.ParseInt(strings.TrimSuffix(rest, "/refresh"), 10, 64)
		resp, err := h.app.MarketService.RefreshListing(r.Context(), id)
		if err != nil {
			if strings.Contains(err.Error(), "not supported") {
				platform.StockError(w, r, http.StatusUnprocessableEntity, err.Error())
				return
			}
			platform.StockError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		platform.JSON(w, http.StatusOK, resp)
		return
	}
	http.NotFound(w, r)
}

func (h *Handlers) GetRates(w http.ResponseWriter, r *http.Request) {
	rates, err := h.app.FXService.GetRates(r.Context(), r.URL.Query().Get("date"))
	if err != nil {
		respondFXError(w, err)
		return
	}
	platform.JSON(w, http.StatusOK, rates)
}

func (h *Handlers) GetRateByCurrency(w http.ResponseWriter, r *http.Request) {
	currencyCode := strings.TrimPrefix(r.URL.Path, "/rates/")
	rate, err := h.app.FXService.GetRate(r.Context(), currencyCode, r.URL.Query().Get("date"))
	if err != nil {
		respondFXError(w, err)
		return
	}
	platform.JSON(w, http.StatusOK, rate)
}

func (h *Handlers) Calculate(w http.ResponseWriter, r *http.Request) {
	h.calculate(w, r, true)
}

func (h *Handlers) CalculateNoCommission(w http.ResponseWriter, r *http.Request) {
	h.calculate(w, r, false)
}

func (h *Handlers) calculate(w http.ResponseWriter, r *http.Request, includeCommission bool) {
	query, validation := parseConversionQuery(r)
	if len(validation) > 0 {
		platform.ExchangeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Validation error", "Molimo proverite unete podatke.", validation)
		return
	}
	result, err := h.app.FXService.Calculate(r.Context(), query.fromCurrency, query.toCurrency, query.amount, query.date, includeCommission)
	if err != nil {
		respondFXError(w, err)
		return
	}
	platform.JSON(w, http.StatusOK, result)
}

func (h *Handlers) FetchRates(w http.ResponseWriter, r *http.Request) {
	result, err := h.app.FXService.FetchAndStoreDailyRates(r.Context())
	if err != nil {
		respondFXError(w, err)
		return
	}
	platform.JSON(w, http.StatusOK, result)
}

func (h *Handlers) ImportStockExchanges(w http.ResponseWriter, r *http.Request) {
	resp, err := h.app.MarketService.ImportStockExchanges(r.Context(), h.cfg.Stock.ExchangeCSVLocation)
	if err != nil {
		platform.StockError(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	platform.JSON(w, http.StatusOK, resp)
}

func (h *Handlers) RefreshAllStocks(w http.ResponseWriter, r *http.Request) {
	go func() {
		_, _ = h.app.MarketService.RefreshAllStocks(context.Background())
	}()
	platform.JSON(w, http.StatusAccepted, api.StockBulkRefreshAcceptedResponse{Status: "STARTED", Message: "Bulk stock refresh started."})
}

func (h *Handlers) StockAdminSubroutes(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/refresh-market-data") {
		ticker := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/stocks/"), "/refresh-market-data")
		resp, err := h.app.MarketService.RefreshStockByTicker(r.Context(), ticker)
		if err != nil {
			platform.StockError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		platform.JSON(w, http.StatusOK, resp)
		return
	}
	http.NotFound(w, r)
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func hasAnyRole(r *http.Request, roles ...string) bool {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return false
	}
	granted := map[string]struct{}{principal.Role: {}}
	switch principal.Role {
	case "ADMIN":
		granted["SUPERVISOR"] = struct{}{}
		granted["AGENT"] = struct{}{}
		granted["BASIC"] = struct{}{}
	case "SUPERVISOR":
		granted["AGENT"] = struct{}{}
		granted["BASIC"] = struct{}{}
	case "AGENT":
		granted["BASIC"] = struct{}{}
	case "CLIENT_TRADING":
		granted["CLIENT_BASIC"] = struct{}{}
	}
	for _, role := range roles {
		if _, ok := granted[role]; ok {
			return true
		}
	}
	return false
}

func normalizeSortBy(value string) (string, error) {
	switch strings.ToLower(defaultString(value, "ticker")) {
	case "ticker":
		return "ticker", nil
	case "price":
		return "price", nil
	case "volume":
		return "volume", nil
	case "maintenancemargin", "maintenance_margin", "maintenance-margin":
		return "maintenanceMargin", nil
	default:
		return "", fmt.Errorf("Unsupported sortBy value '%s'. Supported values are ticker, price, volume, maintenanceMargin.", value)
	}
}

func normalizeSortDirection(value string) (string, error) {
	switch strings.ToLower(defaultString(value, "asc")) {
	case "asc", "desc":
		return strings.ToLower(defaultString(value, "asc")), nil
	default:
		return "", fmt.Errorf("Unsupported sortDirection value '%s'. Supported values are asc and desc.", value)
	}
}

type conversionQuery struct {
	fromCurrency string
	toCurrency   string
	amount       string
	date         string
}

func parseConversionQuery(r *http.Request) (conversionQuery, map[string]string) {
	errors := map[string]string{}
	fromCurrency := strings.TrimSpace(r.URL.Query().Get("fromCurrency"))
	toCurrency := strings.TrimSpace(r.URL.Query().Get("toCurrency"))
	amountText := strings.TrimSpace(r.URL.Query().Get("amount"))
	dateText := strings.TrimSpace(r.URL.Query().Get("date"))
	if !isSupportedCurrency(fromCurrency) {
		errors["fromCurrency"] = "Supported currencies are RSD, EUR, CHF, USD, GBP, JPY, CAD and AUD."
	}
	if !isSupportedCurrency(toCurrency) {
		errors["toCurrency"] = "Supported currencies are RSD, EUR, CHF, USD, GBP, JPY, CAD and AUD."
	}
	amount, err := decimal.NewFromString(amountText)
	if amountText == "" || err != nil || !amount.GreaterThan(decimal.Zero) {
		errors["amount"] = "amount must be greater than 0."
	}
	if dateText != "" {
		if _, err := time.Parse("2006-01-02", dateText); err != nil {
			errors["date"] = "date must use yyyy-MM-dd format."
		}
	}
	return conversionQuery{fromCurrency: fromCurrency, toCurrency: toCurrency, amount: amountText, date: dateText}, errors
}

func isSupportedCurrency(value string) bool {
	switch strings.ToUpper(value) {
	case "RSD", "EUR", "CHF", "USD", "GBP", "JPY", "CAD", "AUD":
		return true
	default:
		return false
	}
}

func respondFXError(w http.ResponseWriter, err error) {
	message := err.Error()
	switch {
	case strings.Contains(message, "empty"):
		platform.ExchangeError(w, http.StatusNotFound, "ERR_EXCHANGE_RATE_NOT_FOUND", "Exchange rate not found", message, nil)
	case strings.Contains(message, "rate not found"):
		platform.ExchangeError(w, http.StatusNotFound, "ERR_EXCHANGE_RATE_NOT_FOUND", "Exchange rate not found", message, nil)
	case strings.Contains(message, "fetch failed"), strings.Contains(message, "provider response"), strings.Contains(message, "inconsistent snapshot"):
		platform.ExchangeError(w, http.StatusBadGateway, "ERR_EXCHANGE_RATE_FETCH_FAILED", "Exchange rate fetch failed", message, nil)
	default:
		platform.ExchangeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Validation error", message, nil)
	}
}
