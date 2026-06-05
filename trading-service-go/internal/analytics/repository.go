package analytics

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// querier is satisfied by *pgxpool.Pool; extracted for unit-test injection.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Repository reads the analytics_* tables with raw pgx. NUMERIC columns are cast
// to ::text and parsed via shopspring/decimal to preserve scale (mirrors
// market-service-go).
type Repository struct {
	db querier
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// LatestCompletedRun returns the most recent COMPLETED run, or (nil, nil) when
// none exists. Mirrors findFirstByStatusOrderByCompletedAtDesc("COMPLETED").
func (r *Repository) LatestCompletedRun(ctx context.Context) (*JobRun, error) {
	var run JobRun
	err := r.db.QueryRow(ctx, `
		SELECT run_id, job_name, status, started_at, completed_at, message
		FROM analytics_job_runs
		WHERE status = 'COMPLETED'
		ORDER BY completed_at DESC
		LIMIT 1`).Scan(
		&run.RunID, &run.JobName, &run.Status, &run.StartedAt, &run.CompletedAt, &run.Message,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// SegmentsByRun returns segments for a run ordered by risk_score DESC, user_id ASC.
func (r *Repository) SegmentsByRun(ctx context.Context, runID string) ([]ClientSegment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT user_id, cluster_id, segment_label,
		       total_portfolio_value::text, total_cost_basis::text, unrealized_pnl::text,
		       holdings_count, max_holding_percent::text,
		       order_count, average_order_value::text, buy_sell_ratio::text, risk_score::text
		FROM analytics_client_segments
		WHERE run_id = $1
		ORDER BY risk_score DESC, user_id ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	segments := make([]ClientSegment, 0)
	for rows.Next() {
		var s ClientSegment
		var totalPortfolioValue, totalCostBasis, unrealizedPnl, maxHoldingPercent, averageOrderValue, buySellRatio, riskScore string
		if err := rows.Scan(
			&s.UserID, &s.ClusterID, &s.SegmentLabel,
			&totalPortfolioValue, &totalCostBasis, &unrealizedPnl,
			&s.HoldingsCount, &maxHoldingPercent,
			&s.OrderCount, &averageOrderValue, &buySellRatio, &riskScore,
		); err != nil {
			return nil, err
		}
		var decErr error
		assignDecimal(totalPortfolioValue, &s.TotalPortfolioValue, &decErr)
		assignDecimal(totalCostBasis, &s.TotalCostBasis, &decErr)
		assignDecimal(unrealizedPnl, &s.UnrealizedPnl, &decErr)
		assignDecimal(maxHoldingPercent, &s.MaxHoldingPercent, &decErr)
		assignDecimal(averageOrderValue, &s.AverageOrderValue, &decErr)
		assignDecimal(buySellRatio, &s.BuySellRatio, &decErr)
		assignDecimal(riskScore, &s.RiskScore, &decErr)
		if decErr != nil {
			return nil, decErr
		}
		segments = append(segments, s)
	}
	return segments, rows.Err()
}

// PortfolioRiskByRunAndUser returns the risk row for (run, user), or (nil, nil).
func (r *Repository) PortfolioRiskByRunAndUser(ctx context.Context, runID string, userID int64) (*PortfolioRisk, error) {
	var p PortfolioRisk
	var totalMarketValue, totalCostBasis, unrealizedPnl, maxHoldingPercent, diversificationScore, riskScore string
	err := r.db.QueryRow(ctx, `
		SELECT user_id, total_market_value::text, total_cost_basis::text, unrealized_pnl::text,
		       holdings_count, max_holding_percent::text, diversification_score::text,
		       risk_score::text, risk_level
		FROM analytics_portfolio_risk
		WHERE run_id = $1 AND user_id = $2`, runID, userID).Scan(
		&p.UserID, &totalMarketValue, &totalCostBasis, &unrealizedPnl,
		&p.HoldingsCount, &maxHoldingPercent, &diversificationScore, &riskScore, &p.RiskLevel,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var decErr error
	assignDecimal(totalMarketValue, &p.TotalMarketValue, &decErr)
	assignDecimal(totalCostBasis, &p.TotalCostBasis, &decErr)
	assignDecimal(unrealizedPnl, &p.UnrealizedPnl, &decErr)
	assignDecimal(maxHoldingPercent, &p.MaxHoldingPercent, &decErr)
	assignDecimal(diversificationScore, &p.DiversificationScore, &decErr)
	assignDecimal(riskScore, &p.RiskScore, &decErr)
	if decErr != nil {
		return nil, decErr
	}
	return &p, nil
}

// TopTickersByRun returns the run's tickers ordered by ticker_rank ASC.
func (r *Repository) TopTickersByRun(ctx context.Context, runID string) ([]TopTicker, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ticker_rank, listing_id, ticker, traded_quantity, traded_notional::text,
		       order_count, transaction_count
		FROM analytics_top_tickers
		WHERE run_id = $1
		ORDER BY ticker_rank ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tickers := make([]TopTicker, 0)
	for rows.Next() {
		var t TopTicker
		var tradedNotional string
		if err := rows.Scan(&t.Rank, &t.ListingID, &t.Ticker, &t.TradedQuantity, &tradedNotional, &t.OrderCount, &t.TransactionCount); err != nil {
			return nil, err
		}
		var decErr error
		assignDecimal(tradedNotional, &t.TradedNotional, &decErr)
		if decErr != nil {
			return nil, decErr
		}
		tickers = append(tickers, t)
	}
	return tickers, rows.Err()
}

// assignDecimal parses a ::text NUMERIC into dst, recording the first error.
func assignDecimal(text string, dst *decimal.Decimal, errp *error) {
	if *errp != nil {
		return
	}
	d, err := decimal.NewFromString(text)
	if err != nil {
		*errp = err
		return
	}
	*dst = d
}
