package funds

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// =========================== fund_holdings ================================

const holdingColumns = `id, fund_id, stock_ticker, quantity,
	avg_unit_price::text, deleted, created_at, updated_at, version`

func scanHolding(row pgx.Row) (*FundHolding, error) {
	var (
		h       FundHolding
		avgText string
	)
	if err := row.Scan(&h.ID, &h.FundID, &h.StockTicker, &h.Quantity, &avgText,
		&h.Deleted, &h.CreatedAt, &h.UpdatedAt, &h.Version); err != nil {
		return nil, err
	}
	avg, err := decimal.NewFromString(avgText)
	if err != nil {
		return nil, err
	}
	h.AvgUnitPrice = avg
	return &h, nil
}

func scanHoldings(rows pgx.Rows) ([]FundHolding, error) {
	defer rows.Close()
	out := make([]FundHolding, 0)
	for rows.Next() {
		h, err := scanHolding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *h)
	}
	return out, rows.Err()
}

// FindHoldingsActive mirrors findByFundIdAndDeletedFalse.
func (r *Repository) FindHoldingsActive(ctx context.Context, q Querier, fundID int64) ([]FundHolding, error) {
	if q == nil {
		q = r.db
	}
	rows, err := q.Query(ctx,
		`SELECT `+holdingColumns+` FROM fund_holdings
		 WHERE fund_id = $1 AND deleted = false`, fundID)
	if err != nil {
		return nil, err
	}
	return scanHoldings(rows)
}

// FindHolding mirrors findByFundIdAndStockTickerAndDeletedFalse.
func (r *Repository) FindHolding(ctx context.Context, q Querier, fundID int64, ticker string) (*FundHolding, error) {
	if q == nil {
		q = r.db
	}
	h, err := scanHolding(q.QueryRow(ctx,
		`SELECT `+holdingColumns+` FROM fund_holdings
		 WHERE fund_id = $1 AND stock_ticker = $2 AND deleted = false`,
		fundID, ticker))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return h, err
}

// InsertHolding mirrors the persist branch of FundHoldingRepository.save.
func (r *Repository) InsertHolding(ctx context.Context, q Querier, h *FundHolding) error {
	if q == nil {
		q = r.db
	}
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now().UTC()
	}
	return q.QueryRow(ctx, `
		INSERT INTO fund_holdings
			(fund_id, stock_ticker, quantity, avg_unit_price, deleted, created_at, updated_at, version)
		VALUES ($1,$2,$3,$4,false,$5,$6,0)
		RETURNING id, version`,
		h.FundID, h.StockTicker, h.Quantity, h.AvgUnitPrice, h.CreatedAt, h.UpdatedAt,
	).Scan(&h.ID, &h.Version)
}

// UpdateHolding mirrors the merge branch. Bumps version like JPA @Version.
func (r *Repository) UpdateHolding(ctx context.Context, q Querier, h *FundHolding) error {
	if q == nil {
		q = r.db
	}
	tag, err := q.Exec(ctx, `
		UPDATE fund_holdings
		SET quantity = $1, avg_unit_price = $2, deleted = $3, updated_at = $4, version = version + 1
		WHERE id = $5`,
		h.Quantity, h.AvgUnitPrice, h.Deleted, h.UpdatedAt, h.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	h.Version++
	return nil
}

// =========================== fund_value_snapshots =========================

const snapshotColumns = `id, fund_id, snapshot_date, liquidity_value::text,
	holdings_value::text, total_value::text, created_at`

func scanSnapshot(row pgx.Row) (*FundValueSnapshot, error) {
	var (
		s        FundValueSnapshot
		liqText  string
		holdText string
		totText  string
	)
	if err := row.Scan(&s.ID, &s.FundID, &s.SnapshotDate, &liqText, &holdText, &totText, &s.CreatedAt); err != nil {
		return nil, err
	}
	liq, err := decimal.NewFromString(liqText)
	if err != nil {
		return nil, err
	}
	s.LiquidityValue = liq
	hold, err := decimal.NewFromString(holdText)
	if err != nil {
		return nil, err
	}
	s.HoldingsValue = hold
	tot, err := decimal.NewFromString(totText)
	if err != nil {
		return nil, err
	}
	s.TotalValue = tot
	return &s, nil
}

func scanSnapshots(rows pgx.Rows) ([]FundValueSnapshot, error) {
	defer rows.Close()
	out := make([]FundValueSnapshot, 0)
	for rows.Next() {
		s, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// FindSnapshots mirrors findByFundIdOrderBySnapshotDateAsc.
func (r *Repository) FindSnapshots(ctx context.Context, fundID int64) ([]FundValueSnapshot, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+snapshotColumns+` FROM fund_value_snapshots
		 WHERE fund_id = $1 ORDER BY snapshot_date ASC`, fundID)
	if err != nil {
		return nil, err
	}
	return scanSnapshots(rows)
}

// FindSnapshotByDate mirrors findByFundIdAndSnapshotDate.
func (r *Repository) FindSnapshotByDate(ctx context.Context, fundID int64, date time.Time) (*FundValueSnapshot, error) {
	s, err := scanSnapshot(r.db.QueryRow(ctx,
		`SELECT `+snapshotColumns+` FROM fund_value_snapshots
		 WHERE fund_id = $1 AND snapshot_date = $2`, fundID, date))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// UpsertSnapshot mirrors FundValueSnapshotService.recordSnapshot's save: insert
// when new for the day, otherwise update the captured values. Uses the unique
// constraint uk_fund_value_snapshot_fund_date as the conflict key.
func (r *Repository) UpsertSnapshot(ctx context.Context, q Querier, s *FundValueSnapshot) error {
	if q == nil {
		q = r.db
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	return q.QueryRow(ctx, `
		INSERT INTO fund_value_snapshots
			(fund_id, snapshot_date, liquidity_value, holdings_value, total_value, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT ON CONSTRAINT uk_fund_value_snapshot_fund_date DO UPDATE
		SET liquidity_value = EXCLUDED.liquidity_value,
		    holdings_value = EXCLUDED.holdings_value,
		    total_value = EXCLUDED.total_value
		RETURNING id`,
		s.FundID, s.SnapshotDate, s.LiquidityValue, s.HoldingsValue, s.TotalValue, s.CreatedAt,
	).Scan(&s.ID)
}

// =========================== fund_dividend_distributions ==================

const distributionColumns = `id, fund_id, stock_ticker, payment_date,
	dividend_per_share::text, source_currency, holding_quantity,
	gross_amount_source::text, gross_amount_rsd::text, strategy, status,
	reinvested_shares, reinvested_amount_rsd::text, distributed_amount_rsd::text,
	note, processed_at`

func scanDistribution(row pgx.Row) (*FundDividendDistribution, error) {
	var (
		d            FundDividendDistribution
		dpsText      string
		grossSrcText string
		grossRsdText string
		reinvAmtText *string
		distAmtText  *string
	)
	if err := row.Scan(&d.ID, &d.FundID, &d.StockTicker, &d.PaymentDate,
		&dpsText, &d.SourceCurrency, &d.HoldingQuantity, &grossSrcText,
		&grossRsdText, &d.Strategy, &d.Status, &d.ReinvestedShares,
		&reinvAmtText, &distAmtText, &d.Note, &d.ProcessedAt); err != nil {
		return nil, err
	}
	dps, err := decimal.NewFromString(dpsText)
	if err != nil {
		return nil, err
	}
	d.DividendPerShare = dps
	gs, err := decimal.NewFromString(grossSrcText)
	if err != nil {
		return nil, err
	}
	d.GrossAmountSource = gs
	gr, err := decimal.NewFromString(grossRsdText)
	if err != nil {
		return nil, err
	}
	d.GrossAmountRsd = gr
	if reinvAmtText != nil {
		v, err := decimal.NewFromString(*reinvAmtText)
		if err != nil {
			return nil, err
		}
		d.ReinvestedAmountRsd = &v
	}
	if distAmtText != nil {
		v, err := decimal.NewFromString(*distAmtText)
		if err != nil {
			return nil, err
		}
		d.DistributedAmountRsd = &v
	}
	return &d, nil
}

// FindDistribution mirrors
// FundDividendDistributionRepository.findByFundIdAndStockTickerAndPaymentDate.
func (r *Repository) FindDistribution(ctx context.Context, fundID int64, ticker string, paymentDate time.Time) (*FundDividendDistribution, error) {
	d, err := scanDistribution(r.db.QueryRow(ctx,
		`SELECT `+distributionColumns+` FROM fund_dividend_distributions
		 WHERE fund_id = $1 AND stock_ticker = $2 AND payment_date = $3`,
		fundID, ticker, paymentDate))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return d, err
}

// InsertDistribution mirrors FundDividendDistributionRepository.save (persist).
func (r *Repository) InsertDistribution(ctx context.Context, q Querier, d *FundDividendDistribution) error {
	if q == nil {
		q = r.db
	}
	if d.ProcessedAt.IsZero() {
		d.ProcessedAt = time.Now().UTC()
	}
	return q.QueryRow(ctx, `
		INSERT INTO fund_dividend_distributions
			(fund_id, stock_ticker, payment_date, dividend_per_share, source_currency,
			 holding_quantity, gross_amount_source, gross_amount_rsd, strategy, status,
			 reinvested_shares, reinvested_amount_rsd, distributed_amount_rsd, note, processed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id`,
		d.FundID, d.StockTicker, d.PaymentDate, d.DividendPerShare, d.SourceCurrency,
		d.HoldingQuantity, d.GrossAmountSource, d.GrossAmountRsd, d.Strategy, d.Status,
		d.ReinvestedShares, d.ReinvestedAmountRsd, d.DistributedAmountRsd, d.Note, d.ProcessedAt,
	).Scan(&d.ID)
}

// =========================== fund_dividend_payouts ========================

// InsertPayout mirrors FundDividendPayoutRepository.save (persist).
func (r *Repository) InsertPayout(ctx context.Context, q Querier, p *FundDividendPayout) error {
	if q == nil {
		q = r.db
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	return q.QueryRow(ctx, `
		INSERT INTO fund_dividend_payouts
			(distribution_id, client_id, client_account_number, ownership_ratio,
			 amount_rsd, status, failure_reason, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id`,
		p.DistributionID, p.ClientID, p.ClientAccountNumber, p.OwnershipRatio,
		p.AmountRsd, p.Status, p.FailureReason, p.CreatedAt,
	).Scan(&p.ID)
}
