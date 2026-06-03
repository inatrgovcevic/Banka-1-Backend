// Package dividend implements WP-14 (Celina 3.7): the quarterly per-shareholder
// dividend payout. Port of trading-service dividend/* (DividendScheduler →
// DividendDistributionService → DividendPayoutExecutor over the
// dividend_payouts table, migration 006 / Java changeset 014).
//
// NOT to be confused with the fund-dividend feature in internal/funds/dividend.go
// (a fund manager recording a dividend on a fund holding) — the two are
// unrelated; this package pays every holder of every STOCK each quarter.
package dividend

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx so repository methods
// run either standalone or inside the per-holder payout transaction.
type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Payout mirrors a row of dividend_payouts (one dividend payment to one holder
// for one security; personal and bank-held parts are separate rows).
type Payout struct {
	ID           int64
	UserID       int64
	StockTicker  *string
	ListingID    int64
	Quantity     int
	GrossAmount  decimal.Decimal
	Currency     *string
	TaxAmountRsd decimal.Decimal
	NetAmount    decimal.Decimal
	AccountID    *int64
	PaymentDate  time.Time
	ForBank      bool
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Pool exposes the pool so the service can open per-holder transactions.
func (r *Repository) Pool() *pgxpool.Pool { return r.db }

const payoutColumns = `id, user_id, stock_ticker, listing_id, quantity, gross_amount::text,
	currency, tax_amount_rsd::text, net_amount::text, account_id, payment_date, for_bank`

func scanPayout(row pgx.Row) (*Payout, error) {
	var (
		p         Payout
		grossText string
		taxText   string
		netText   string
	)
	if err := row.Scan(&p.ID, &p.UserID, &p.StockTicker, &p.ListingID, &p.Quantity, &grossText,
		&p.Currency, &taxText, &netText, &p.AccountID, &p.PaymentDate, &p.ForBank); err != nil {
		return nil, err
	}
	gross, err := decimal.NewFromString(grossText)
	if err != nil {
		return nil, err
	}
	p.GrossAmount = gross
	tax, err := decimal.NewFromString(taxText)
	if err != nil {
		return nil, err
	}
	p.TaxAmountRsd = tax
	net, err := decimal.NewFromString(netText)
	if err != nil {
		return nil, err
	}
	p.NetAmount = net
	return &p, nil
}

func scanPayouts(rows pgx.Rows) ([]Payout, error) {
	defer rows.Close()
	out := make([]Payout, 0)
	for rows.Next() {
		p, err := scanPayout(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// Insert persists one payout row (mirrors payoutRepository.save).
func (r *Repository) Insert(ctx context.Context, q Querier, p *Payout) error {
	return q.QueryRow(ctx, `
		INSERT INTO dividend_payouts (user_id, stock_ticker, listing_id, quantity, gross_amount,
			currency, tax_amount_rsd, net_amount, account_id, payment_date, for_bank)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`,
		p.UserID, p.StockTicker, p.ListingID, p.Quantity, p.GrossAmount.String(),
		p.Currency, p.TaxAmountRsd.String(), p.NetAmount.String(), p.AccountID, p.PaymentDate, p.ForBank).
		Scan(&p.ID)
}

// ExistsForDate mirrors existsByUserIdAndListingIdAndPaymentDateAndForBank —
// the idempotency guard alongside the table's unique constraint.
func (r *Repository) ExistsForDate(ctx context.Context, q Querier, userID, listingID int64, paymentDate time.Time, forBank bool) (bool, error) {
	var exists bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM dividend_payouts
		WHERE user_id = $1 AND listing_id = $2 AND payment_date = $3 AND for_bank = $4)`,
		userID, listingID, paymentDate, forBank).Scan(&exists)
	return exists, err
}

// FindByUserID mirrors findByUserIdOrderByPaymentDateDesc.
func (r *Repository) FindByUserID(ctx context.Context, q Querier, userID int64) ([]Payout, error) {
	rows, err := q.Query(ctx, `SELECT `+payoutColumns+` FROM dividend_payouts WHERE user_id = $1 ORDER BY payment_date DESC`, userID)
	if err != nil {
		return nil, err
	}
	return scanPayouts(rows)
}

// FindByUserIDAndListingID mirrors findByUserIdAndListingIdOrderByPaymentDateDesc.
func (r *Repository) FindByUserIDAndListingID(ctx context.Context, q Querier, userID, listingID int64) ([]Payout, error) {
	rows, err := q.Query(ctx, `SELECT `+payoutColumns+` FROM dividend_payouts WHERE user_id = $1 AND listing_id = $2 ORDER BY payment_date DESC`, userID, listingID)
	if err != nil {
		return nil, err
	}
	return scanPayouts(rows)
}
