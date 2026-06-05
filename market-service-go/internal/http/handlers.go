package http

import (
	"context"
	"encoding/json"
	"errors"
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

type createPriceAlertRequest struct {
	ListingID        int64                      `json:"listingId"`
	Condition        market.PriceAlertCondition `json:"condition"`
	Threshold        json.RawMessage            `json:"threshold"`
	NotificationType string                     `json:"notificationType"`
}

type priceAlertResponse struct {
	ID               int64                      `json:"id"`
	UserID           int64                      `json:"userId"`
	RecipientType    string                     `json:"recipientType"`
	ListingID        int64                      `json:"listingId"`
	Condition        market.PriceAlertCondition `json:"condition"`
	Threshold        decimal.Decimal            `json:"threshold"`
	NotificationType string                     `json:"notificationType"`
	Active           bool                       `json:"active"`
	CreatedAt        string                     `json:"createdAt"`
	LastTriggeredAt  *string                    `json:"lastTriggeredAt"`
}

type createWatchlistRequest struct {
	Name string `json:"name"`
}

type addWatchlistItemRequest struct {
	ListingID int64 `json:"listingId"`
}

type watchlistResponse struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"userId"`
	Name      string `json:"name"`
	ItemCount int64  `json:"itemCount"`
	CreatedAt string `json:"createdAt"`
}

type watchlistItemResponse struct {
	ID          int64              `json:"id"`
	WatchlistID int64              `json:"watchlistId"`
	ListingID   int64              `json:"listingId"`
	Ticker      string             `json:"ticker"`
	Name        string             `json:"name"`
	Price       decimal.Decimal    `json:"price"`
	Change      decimal.Decimal    `json:"change"`
	Volume      int64              `json:"volume"`
	ListingType market.ListingType `json:"listingType"`
	AddedAt     string             `json:"addedAt"`
}

type dividendDataResponse struct {
	ListingID     int64           `json:"listingId"`
	Ticker        string          `json:"ticker"`
	Price         decimal.Decimal `json:"price"`
	Currency      string          `json:"currency"`
	DividendYield decimal.Decimal `json:"dividendYield"`
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
		"service":                    "stock-service",
		"status":                     "UP",
		"gatewayPrefix":              defaultString(r.Header.Get("X-Forwarded-Prefix"), "/stock"),
		"exchangeServiceBaseUrl":     h.cfg.Stock.ExchangeServiceBaseURL,
		"marketDataBaseUrl":          h.cfg.Stock.MarketDataBaseURL,
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
	if value := r.URL.Query().Get("minPrice"); value != "" {
		filter.MinPrice = &value
	}
	if value := r.URL.Query().Get("maxPrice"); value != "" {
		filter.MaxPrice = &value
	}
	if value := r.URL.Query().Get("minAsk"); value != "" {
		filter.MinAsk = &value
	}
	if value := r.URL.Query().Get("maxAsk"); value != "" {
		filter.MaxAsk = &value
	}
	if value := r.URL.Query().Get("minBid"); value != "" {
		filter.MinBid = &value
	}
	if value := r.URL.Query().Get("maxBid"); value != "" {
		filter.MaxBid = &value
	}
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

func (h *Handlers) ListPriceAlerts(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		platform.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing principal")
		return
	}
	alerts, err := h.app.MarketService.ListPriceAlerts(r.Context(), principal.ID)
	if err != nil {
		platform.StockError(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}
	out := make([]priceAlertResponse, 0, len(alerts))
	for _, alert := range alerts {
		out = append(out, priceAlertDTO(alert))
	}
	platform.JSON(w, http.StatusOK, out)
}

func (h *Handlers) CreatePriceAlert(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		platform.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing principal")
		return
	}
	var req createPriceAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		platform.StockError(w, r, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	threshold, ok := rawJSONScalar(req.Threshold)
	if !ok {
		platform.StockError(w, r, http.StatusBadRequest, "Invalid request")
		return
	}
	alert, err := h.app.MarketService.CreatePriceAlert(r.Context(), principal.ID, recipientType(principal.Role), principal.Email, principal.Subject, req.ListingID, req.Condition, threshold, req.NotificationType)
	respondMarketResult(w, r, priceAlertDTOPtr(alert), err, http.StatusCreated)
}

func (h *Handlers) PriceAlertSubroutes(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/price-alerts/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		platform.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing principal")
		return
	}
	switch r.Method {
	case http.MethodPatch:
		alert, err := h.app.MarketService.TogglePriceAlert(r.Context(), principal.ID, id)
		respondMarketResult(w, r, priceAlertDTOPtr(alert), err, http.StatusOK)
	case http.MethodDelete:
		err := h.app.MarketService.DeletePriceAlert(r.Context(), principal.ID, id)
		respondMarketResult(w, r, nil, err, http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handlers) ListWatchlists(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		platform.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing principal")
		return
	}
	items, err := h.app.MarketService.ListWatchlists(r.Context(), principal.ID)
	if err != nil {
		platform.StockError(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}
	out := make([]watchlistResponse, 0, len(items))
	for _, item := range items {
		out = append(out, watchlistDTO(item))
	}
	platform.JSON(w, http.StatusOK, out)
}

func (h *Handlers) CreateWatchlist(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		platform.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing principal")
		return
	}
	var req createWatchlistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		platform.StockError(w, r, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	item, err := h.app.MarketService.CreateWatchlist(r.Context(), principal.ID, req.Name)
	respondMarketResult(w, r, watchlistDTOPtr(item), err, http.StatusCreated)
}

func (h *Handlers) WatchlistSubroutes(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/watchlists/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	watchlistID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		platform.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing principal")
		return
	}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		respondMarketResult(w, r, nil, h.app.MarketService.DeleteWatchlist(r.Context(), principal.ID, watchlistID), http.StatusNoContent)
		return
	}
	if len(parts) == 2 && parts[1] == "items" && r.Method == http.MethodGet {
		filter, ok := parseListingTypeFilter(w, r)
		if !ok {
			return
		}
		items, err := h.app.MarketService.ListWatchlistItems(r.Context(), principal.ID, watchlistID, filter)
		if err != nil {
			respondMarketResult(w, r, nil, err, http.StatusOK)
			return
		}
		out := make([]watchlistItemResponse, 0, len(items))
		for _, item := range items {
			out = append(out, watchlistItemDTO(item))
		}
		platform.JSON(w, http.StatusOK, out)
		return
	}
	if len(parts) == 2 && parts[1] == "items" && r.Method == http.MethodPost {
		var req addWatchlistItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			platform.StockError(w, r, http.StatusBadRequest, "Invalid JSON body")
			return
		}
		item, err := h.app.MarketService.AddWatchlistItem(r.Context(), principal.ID, watchlistID, req.ListingID)
		respondMarketResult(w, r, watchlistItemDTOPtr(item), err, http.StatusCreated)
		return
	}
	if len(parts) == 3 && parts[1] == "items" && r.Method == http.MethodDelete {
		itemID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		respondMarketResult(w, r, nil, h.app.MarketService.RemoveWatchlistItem(r.Context(), principal.ID, watchlistID, itemID), http.StatusNoContent)
		return
	}
	http.NotFound(w, r)
}

func (h *Handlers) DividendData(w http.ResponseWriter, r *http.Request) {
	items, err := h.app.MarketService.ListDividendData(r.Context())
	if err != nil {
		platform.StockError(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}
	out := make([]dividendDataResponse, 0, len(items))
	for _, item := range items {
		out = append(out, dividendDataDTO(item))
	}
	platform.JSON(w, http.StatusOK, out)
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

func respondMarketResult(w http.ResponseWriter, r *http.Request, value any, err error, status int) {
	if err != nil {
		switch {
		case errors.Is(err, market.ErrBadRequest):
			platform.StockError(w, r, http.StatusBadRequest, "Invalid request")
		case errors.Is(err, market.ErrNotFound):
			platform.StockError(w, r, http.StatusNotFound, "Resource was not found.")
		case errors.Is(err, market.ErrConflict):
			platform.StockError(w, r, http.StatusConflict, "Resource already exists.")
		default:
			platform.StockError(w, r, http.StatusInternalServerError, "Internal server error")
		}
		return
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	platform.JSON(w, status, value)
}

func recipientType(role string) string {
	if strings.HasPrefix(role, "CLIENT") || strings.HasPrefix(role, "ROLE_CLIENT") {
		return "CLIENT"
	}
	return "EMPLOYEE"
}

func rawJSONScalar(raw json.RawMessage) (string, bool) {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return "", false
	}
	if strings.HasPrefix(text, "\"") {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", false
		}
		return strings.TrimSpace(value), strings.TrimSpace(value) != ""
	}
	return text, true
}

func priceAlertDTOPtr(alert *market.PriceAlert) any {
	if alert == nil {
		return nil
	}
	dto := priceAlertDTO(*alert)
	return dto
}

func priceAlertDTO(alert market.PriceAlert) priceAlertResponse {
	var last *string
	if alert.LastTriggeredAt != nil {
		value := formatLocalDateTime(*alert.LastTriggeredAt)
		last = &value
	}
	return priceAlertResponse{
		ID:               alert.ID,
		UserID:           alert.UserID,
		RecipientType:    alert.RecipientType,
		ListingID:        alert.ListingID,
		Condition:        alert.Condition,
		Threshold:        dec(alert.Threshold),
		NotificationType: alert.NotificationType,
		Active:           alert.Active,
		CreatedAt:        formatLocalDateTime(alert.CreatedAt),
		LastTriggeredAt:  last,
	}
}

func watchlistDTOPtr(item *market.Watchlist) any {
	if item == nil {
		return nil
	}
	dto := watchlistDTO(*item)
	return dto
}

func watchlistDTO(item market.Watchlist) watchlistResponse {
	return watchlistResponse{
		ID:        item.ID,
		UserID:    item.UserID,
		Name:      item.Name,
		ItemCount: item.ItemCount,
		CreatedAt: formatLocalDateTime(item.CreatedAt),
	}
}

func watchlistItemDTOPtr(item *market.WatchlistItem) any {
	if item == nil {
		return nil
	}
	dto := watchlistItemDTO(*item)
	return dto
}

func watchlistItemDTO(item market.WatchlistItem) watchlistItemResponse {
	return watchlistItemResponse{
		ID:          item.ID,
		WatchlistID: item.WatchlistID,
		ListingID:   item.ListingID,
		Ticker:      item.Listing.Ticker,
		Name:        item.Listing.Name,
		Price:       dec(item.Listing.Price),
		Change:      dec(item.Listing.Change),
		Volume:      item.Listing.Volume,
		ListingType: item.Listing.ListingType,
		AddedAt:     formatLocalDateTime(item.AddedAt),
	}
}

func dividendDataDTO(item market.DividendData) dividendDataResponse {
	return dividendDataResponse{
		ListingID:     item.ListingID,
		Ticker:        item.Ticker,
		Price:         dec(item.Price),
		Currency:      item.Currency,
		DividendYield: decDefault(item.DividendYield),
	}
}

func parseListingTypeFilter(w http.ResponseWriter, r *http.Request) (*market.ListingType, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("listingType"))
	if raw == "" {
		return nil, true
	}
	value := market.ListingType(strings.ToUpper(raw))
	switch value {
	case market.ListingTypeStock, market.ListingTypeFutures, market.ListingTypeForex, market.ListingTypeOption:
		return &value, true
	default:
		platform.StockError(w, r, http.StatusBadRequest, "Unsupported listingType")
		return nil, false
	}
}

func dec(value string) decimal.Decimal {
	return decimal.RequireFromString(value)
}

func decDefault(value string) decimal.Decimal {
	if strings.TrimSpace(value) == "" {
		return decimal.Zero
	}
	return decimal.RequireFromString(value)
}

func formatLocalDateTime(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05")
}
