// Package analytics serves the read-only /analytics endpoints over the
// analytics_* tables. Those tables are populated by an external Spark/clustering
// job; this service never writes them. analytics is NOT part of SPECIFIKACIJA.md
// — it is governed by parity with the Java trading-service only.
package analytics

import (
	"time"

	"github.com/shopspring/decimal"
)

// JobRun mirrors a row of analytics_job_runs.
type JobRun struct {
	RunID       string
	JobName     string
	Status      string
	StartedAt   time.Time
	CompletedAt *time.Time // nullable
	Message     *string    // nullable
}

// ClientSegment mirrors a row of analytics_client_segments.
type ClientSegment struct {
	UserID              int64
	ClusterID           int
	SegmentLabel        string
	TotalPortfolioValue decimal.Decimal
	TotalCostBasis      decimal.Decimal
	UnrealizedPnl       decimal.Decimal
	HoldingsCount       int
	MaxHoldingPercent   decimal.Decimal
	OrderCount          int
	AverageOrderValue   decimal.Decimal
	BuySellRatio        decimal.Decimal
	RiskScore           decimal.Decimal
}

// PortfolioRisk mirrors a row of analytics_portfolio_risk.
type PortfolioRisk struct {
	UserID               int64
	TotalMarketValue     decimal.Decimal
	TotalCostBasis       decimal.Decimal
	UnrealizedPnl        decimal.Decimal
	HoldingsCount        int
	MaxHoldingPercent    decimal.Decimal
	DiversificationScore decimal.Decimal
	RiskScore            decimal.Decimal
	RiskLevel            string
}

// TopTicker mirrors a row of analytics_top_tickers.
type TopTicker struct {
	Rank             int
	ListingID        int64
	Ticker           string
	TradedQuantity   int64
	TradedNotional   decimal.Decimal
	OrderCount       int
	TransactionCount int
}
