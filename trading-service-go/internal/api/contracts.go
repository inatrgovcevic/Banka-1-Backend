// Package api holds the HTTP response DTOs. Field names and JSON shapes mirror
// the Java trading-service exactly so the existing frontend keeps working.
package api

import (
	"time"

	"github.com/shopspring/decimal"
)

func init() {
	// Render decimals as JSON numbers (not quoted strings), matching the Java
	// BigDecimal serialization the frontend expects.
	decimal.MarshalJSONWithoutQuotes = true
}

// LocalDateTime serializes the way Jackson renders a Java LocalDateTime: ISO-8601
// with no timezone and trailing-zero-trimmed fractional seconds
// (e.g. "2026-05-26T16:59:55.337"). An invalid value marshals to null.
type LocalDateTime struct {
	Time  time.Time
	Valid bool
}

func NewLocalDateTime(t time.Time) LocalDateTime {
	return LocalDateTime{Time: t, Valid: true}
}

func LocalDateTimeFromPtr(t *time.Time) LocalDateTime {
	if t == nil {
		return LocalDateTime{}
	}
	return LocalDateTime{Time: *t, Valid: true}
}

func (l LocalDateTime) MarshalJSON() ([]byte, error) {
	if !l.Valid {
		return []byte("null"), nil
	}
	return []byte(`"` + l.Time.Format("2006-01-02T15:04:05.999999999") + `"`), nil
}

// LocalDate serializes the way Jackson renders a Java LocalDate: an ISO date with
// no time or timezone (e.g. "2026-05-26"). An invalid value marshals to null. Used
// by OTC settlementDate fields (P6).
type LocalDate struct {
	Time  time.Time
	Valid bool
}

func NewLocalDate(t time.Time) LocalDate {
	return LocalDate{Time: t, Valid: true}
}

func LocalDateFromPtr(t *time.Time) LocalDate {
	if t == nil {
		return LocalDate{}
	}
	return LocalDate{Time: *t, Valid: true}
}

func (d LocalDate) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte("null"), nil
	}
	return []byte(`"` + d.Time.Format("2006-01-02") + `"`), nil
}

// UTCInstant serializes the way Jackson renders a Java Instant: ISO-8601 in UTC
// with a trailing Z (e.g. "2026-05-26T16:59:55.337Z"). The stored TIMESTAMP
// columns are naive UTC (Java reads them with toInstant(ZoneOffset.UTC); pgx
// scans them as UTC), so marshaling is a plain UTC format. An invalid value
// marshals to null. Added for the Celina 3 enriched order timestamps
// (OrderResponse.lastModification/createdAt/executedAt).
type UTCInstant struct {
	Time  time.Time
	Valid bool
}

func NewUTCInstant(t time.Time) UTCInstant {
	return UTCInstant{Time: t, Valid: true}
}

func UTCInstantFromPtr(t *time.Time) UTCInstant {
	if t == nil {
		return UTCInstant{}
	}
	return UTCInstant{Time: *t, Valid: true}
}

func (i UTCInstant) MarshalJSON() ([]byte, error) {
	if !i.Valid {
		return []byte("null"), nil
	}
	return []byte(`"` + i.Time.UTC().Format("2006-01-02T15:04:05.999999999") + `Z"`), nil
}

// AnalyticsRunResponse ↔ GET /analytics/runs/latest
type AnalyticsRunResponse struct {
	RunID       string        `json:"runId"`
	JobName     string        `json:"jobName"`
	Status      string        `json:"status"`
	StartedAt   LocalDateTime `json:"startedAt"`
	CompletedAt LocalDateTime `json:"completedAt"`
	Message     *string       `json:"message"`
}

// ClientSegmentsResponse ↔ GET /analytics/clients/segments.
// runId/computedAt are null when no completed run exists (segments is []).
type ClientSegmentsResponse struct {
	RunID      *string                     `json:"runId"`
	ComputedAt LocalDateTime               `json:"computedAt"`
	Segments   []ClientSegmentItemResponse `json:"segments"`
}

type ClientSegmentItemResponse struct {
	UserID              int64           `json:"userId"`
	ClusterID           int             `json:"clusterId"`
	SegmentLabel        string          `json:"segmentLabel"`
	TotalPortfolioValue decimal.Decimal `json:"totalPortfolioValue"`
	TotalCostBasis      decimal.Decimal `json:"totalCostBasis"`
	UnrealizedPnl       decimal.Decimal `json:"unrealizedPnl"`
	HoldingsCount       int             `json:"holdingsCount"`
	MaxHoldingPercent   decimal.Decimal `json:"maxHoldingPercent"`
	OrderCount          int             `json:"orderCount"`
	AverageOrderValue   decimal.Decimal `json:"averageOrderValue"`
	BuySellRatio        decimal.Decimal `json:"buySellRatio"`
	RiskScore           decimal.Decimal `json:"riskScore"`
}

// PortfolioRiskResponse ↔ GET /analytics/users/{userId}/portfolio-risk
type PortfolioRiskResponse struct {
	RunID                string          `json:"runId"`
	ComputedAt           LocalDateTime   `json:"computedAt"`
	UserID               int64           `json:"userId"`
	TotalMarketValue     decimal.Decimal `json:"totalMarketValue"`
	TotalCostBasis       decimal.Decimal `json:"totalCostBasis"`
	UnrealizedPnl        decimal.Decimal `json:"unrealizedPnl"`
	HoldingsCount        int             `json:"holdingsCount"`
	MaxHoldingPercent    decimal.Decimal `json:"maxHoldingPercent"`
	DiversificationScore decimal.Decimal `json:"diversificationScore"`
	RiskScore            decimal.Decimal `json:"riskScore"`
	RiskLevel            string          `json:"riskLevel"`
}

// TopTickersResponse ↔ GET /analytics/tickers/top.
// runId/computedAt are null when no completed run exists (tickers is []).
type TopTickersResponse struct {
	RunID      *string                 `json:"runId"`
	ComputedAt LocalDateTime           `json:"computedAt"`
	Tickers    []TopTickerItemResponse `json:"tickers"`
}

// TopTickerItemResponse — note the JSON key is "rank" (Java record component),
// even though the source column is ticker_rank.
type TopTickerItemResponse struct {
	Rank             int             `json:"rank"`
	ListingID        int64           `json:"listingId"`
	Ticker           string          `json:"ticker"`
	TradedQuantity   int64           `json:"tradedQuantity"`
	TradedNotional   decimal.Decimal `json:"tradedNotional"`
	OrderCount       int             `json:"orderCount"`
	TransactionCount int             `json:"transactionCount"`
}
