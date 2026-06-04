package api

import "github.com/shopspring/decimal"

// DividendPayoutDto ↔ trading-service dividend DividendPayoutDto (WP-14
// Celina 3.7) — the read-only projection GET /dividends returns, newest first.
type DividendPayoutDto struct {
	ID           int64           `json:"id"`
	UserID       int64           `json:"userId"`
	StockTicker  *string         `json:"stockTicker"`
	ListingID    int64           `json:"listingId"`
	Quantity     int             `json:"quantity"`
	GrossAmount  decimal.Decimal `json:"grossAmount"`
	Currency     *string         `json:"currency"`
	TaxAmountRsd decimal.Decimal `json:"taxAmountRsd"`
	NetAmount    decimal.Decimal `json:"netAmount"`
	AccountID    *int64          `json:"accountId"`
	PaymentDate  LocalDate       `json:"paymentDate"`
	ForBank      bool            `json:"forBank"`
}
