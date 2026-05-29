package api

import "github.com/shopspring/decimal"

// Portfolio DTOs — JSON shapes mirror order-service exactly (see
// com.banka1.order.dto.{PortfolioSummaryResponse,PortfolioResponse,
// SetPublicQuantityRequestDto}). Jackson serializes nulls (Include.ALWAYS), so
// nullable fields use pointers to render JSON null rather than being omitted.

// PortfolioSummaryResponse ↔ GET /portfolio.
type PortfolioSummaryResponse struct {
	Holdings      []PortfolioResponse `json:"holdings"`
	TotalProfit   decimal.Decimal     `json:"totalProfit"`
	YearlyTaxPaid decimal.Decimal     `json:"yearlyTaxPaid"`
	MonthlyTaxDue decimal.Decimal     `json:"monthlyTaxDue"`
}

// PortfolioResponse is a single holding row.
type PortfolioResponse struct {
	ID                   int64           `json:"id"`
	ListingID            int64           `json:"listingId"`
	ListingType          string          `json:"listingType"`
	Ticker               *string         `json:"ticker"`
	Quantity             int             `json:"quantity"`
	PublicQuantity       int             `json:"publicQuantity"`
	Exercisable          *bool           `json:"exercisable"`
	LastModified         LocalDateTime   `json:"lastModified"`
	CurrentPrice         decimal.Decimal `json:"currentPrice"`
	AveragePurchasePrice decimal.Decimal `json:"averagePurchasePrice"`
	Profit               decimal.Decimal `json:"profit"`
}

// SetPublicQuantityRequest ↔ PUT /portfolio/{id}/set-public body. publicQuantity
// has no bean validation in Java; null/negative are rejected in the service.
type SetPublicQuantityRequest struct {
	PublicQuantity *int `json:"publicQuantity"`
}
