package api

import (
	"context"
	"log/slog"
	"net/http"
)

// ---------------------------------------------------------------------------
// Interfaces
// ---------------------------------------------------------------------------

// PublicStockService is the seam for GET /public-stock.
type PublicStockService interface {
	// GetPublicStocks returns the list of publicly-offered stocks with available
	// quantities (already adjusted for active option contract reservations).
	GetPublicStocks(ctx context.Context) ([]PublicStockEntry, error)
}

// PublicStockEntry is one row in the public-stock response.
// Matches the wire format Tim 2 expects: {"stock":{"ticker":"AAPL"},"sellers":[...]}
type PublicStockEntry struct {
	Stock   StockRef    `json:"stock"`
	Sellers []SellerRow `json:"sellers"`
}

// StockRef carries only the ticker (matches Java StockDescription).
type StockRef struct {
	Ticker string `json:"ticker"`
}

// SellerRow is one seller entry in the public-stock list.
type SellerRow struct {
	Seller SellerID `json:"seller"`
	Amount int      `json:"amount"`
}

// SellerID mirrors ForeignBankId but uses lowercase-start JSON keys.
type SellerID struct {
	RoutingNumber int    `json:"routingNumber"`
	ID            string `json:"id"`
}

// ---------------------------------------------------------------------------
// PublicStockHandler
// ---------------------------------------------------------------------------

// PublicStockHandler handles GET /public-stock.
type PublicStockHandler struct {
	svc PublicStockService
	log *slog.Logger
}

// NewPublicStockHandler constructs the handler.
func NewPublicStockHandler(svc PublicStockService, log *slog.Logger) *PublicStockHandler {
	if log == nil {
		log = slog.Default()
	}
	return &PublicStockHandler{svc: svc, log: log}
}

// Get handles GET /public-stock (§3.1).
func (h *PublicStockHandler) Get(w http.ResponseWriter, r *http.Request) {
	entries, err := h.svc.GetPublicStocks(r.Context())
	if err != nil {
		h.log.ErrorContext(r.Context(), "public-stock: get failed", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []PublicStockEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}
