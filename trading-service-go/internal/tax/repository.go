// Package tax serves the /tax (+ /internal/tax) capital-gains endpoints and the
// monthly tax-collection scheduler. It mirrors order-service TaxServiceImpl: a
// FIFO cost-basis engine over the `orders`/`transactions`/`portfolio` tables, an
// OTC capital-gains pass that reads the trading-service-owned `option_contracts`
// + `stock_ownership_transfers` tables (Java still writes them during
// coexistence), and a `tax_charges` ledger this service writes. The `trading`
// schema is owned by Java Liquibase; this service runs no migrations. NUMERIC is
// read as ::text into shopspring/decimal to preserve scale; idempotency is driven
// by the unique constraints on tax_charges (insert-then-skip-on-conflict).
package tax

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// TaxChargeStatus values. order-service entity/enums/TaxChargeStatus: trust the
// enum (RESERVED/FAILED/CHARGED), not the stale entity javadoc (PENDING/.../PAID).
const (
	StatusReserved = "RESERVED"
	StatusFailed   = "FAILED"
	StatusCharged  = "CHARGED"
)

// ErrDuplicate is returned by Insert when the row collides with a unique
// constraint (uk_tax_charges_sell_buy or uk_tax_charges_otc_contract). Callers
// skip the entry — the same effect as Java catching DataIntegrityViolationException.
var ErrDuplicate = errors.New("tax: duplicate tax charge")

// TaxCharge mirrors a row of `tax_charges`. TaxAmount is the security's original
// currency; TaxAmountRsd is the converted/settled RSD amount (null until charged
// for stock rows; pre-set for OTC rows). OtcContractID is set instead of real
// transaction ids for exercised OTC contracts.
type TaxCharge struct {
	ID                int64
	SellTransactionID int64
	BuyTransactionID  int64
	UserID            int64
	ListingID         int64
	SourceAccountID   int64
	TaxPeriodStart    time.Time
	TaxPeriodEnd      time.Time
	TaxAmount         decimal.Decimal
	TaxAmountRsd      *decimal.Decimal
	Status            string
	CreatedAt         time.Time
	ChargedAt         *time.Time
	OtcContractID     *int64
}

// OtcTaxEntry is one exercised OTC contract eligible for capital-gains tax,
// mirroring TaxServiceImpl.OtcTaxEntry. SellPricePerStock/AveragePurchasePrice are
// in the contract currency (USD in this stack).
type OtcTaxEntry struct {
	ContractID           int64
	SellerID             int64
	ListingID            int64
	Ticker               string
	Amount               int
	SellPricePerStock    decimal.Decimal
	AveragePurchasePrice decimal.Decimal
	ExercisedAt          time.Time
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const taxChargeColumns = `id, sell_transaction_id, buy_transaction_id, user_id, listing_id,
	source_account_id, tax_period_start, tax_period_end, tax_amount::text,
	tax_amount_rsd::text, status, created_at, charged_at, otc_contract_id`

func scanTaxCharge(row pgx.Row) (*TaxCharge, error) {
	var (
		c       TaxCharge
		amtText string
		rsdText *string
	)
	if err := row.Scan(&c.ID, &c.SellTransactionID, &c.BuyTransactionID, &c.UserID, &c.ListingID,
		&c.SourceAccountID, &c.TaxPeriodStart, &c.TaxPeriodEnd, &amtText, &rsdText, &c.Status,
		&c.CreatedAt, &c.ChargedAt, &c.OtcContractID); err != nil {
		return nil, err
	}
	amt, err := decimal.NewFromString(amtText)
	if err != nil {
		return nil, err
	}
	c.TaxAmount = amt
	if rsdText != nil {
		rsd, err := decimal.NewFromString(*rsdText)
		if err != nil {
			return nil, err
		}
		c.TaxAmountRsd = &rsd
	}
	return &c, nil
}

func scanTaxCharges(rows pgx.Rows) ([]TaxCharge, error) {
	defer rows.Close()
	out := make([]TaxCharge, 0)
	for rows.Next() {
		c, err := scanTaxCharge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// ExistsBySellAndBuy mirrors existsBySellTransactionIdAndBuyTransactionId.
func (r *Repository) ExistsBySellAndBuy(ctx context.Context, sellTxID, buyTxID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM tax_charges WHERE sell_transaction_id = $1 AND buy_transaction_id = $2)`,
		sellTxID, buyTxID).Scan(&exists)
	return exists, err
}

// ExistsByOtcContractID mirrors existsByOtcContractId.
func (r *Repository) ExistsByOtcContractID(ctx context.Context, contractID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM tax_charges WHERE otc_contract_id = $1)`, contractID).Scan(&exists)
	return exists, err
}

// FindByUserIDAndStatus mirrors findByUserIdAndStatus.
func (r *Repository) FindByUserIDAndStatus(ctx context.Context, userID int64, status string) ([]TaxCharge, error) {
	rows, err := r.db.Query(ctx, `SELECT `+taxChargeColumns+` FROM tax_charges WHERE user_id = $1 AND status = $2`, userID, status)
	if err != nil {
		return nil, err
	}
	return scanTaxCharges(rows)
}

// FindAll mirrors JpaRepository.findAll (no ORDER BY).
func (r *Repository) FindAll(ctx context.Context) ([]TaxCharge, error) {
	rows, err := r.db.Query(ctx, `SELECT `+taxChargeColumns+` FROM tax_charges`)
	if err != nil {
		return nil, err
	}
	return scanTaxCharges(rows)
}

// Insert persists a reservation (mirrors saveAndFlush), setting created_at to
// now() (the @PrePersist callback) and returning the generated id + created_at.
// A unique-constraint collision returns ErrDuplicate so the caller skips it
// (insert-then-skip-on-conflict, matching the Java DataIntegrityViolation catch).
func (r *Repository) Insert(ctx context.Context, c *TaxCharge) error {
	var rsd any
	if c.TaxAmountRsd != nil {
		rsd = c.TaxAmountRsd.String()
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO tax_charges (
			sell_transaction_id, buy_transaction_id, user_id, listing_id, source_account_id,
			tax_period_start, tax_period_end, tax_amount, tax_amount_rsd, status, created_at,
			charged_at, otc_contract_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10, now(), $11,$12)
		RETURNING id, created_at`,
		c.SellTransactionID, c.BuyTransactionID, c.UserID, c.ListingID, c.SourceAccountID,
		c.TaxPeriodStart, c.TaxPeriodEnd, c.TaxAmount.String(), rsd, c.Status,
		c.ChargedAt, c.OtcContractID).
		Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

// UpdateCharged settles a stock reservation: status=CHARGED, tax_amount_rsd, and
// charged_at (the success branch of collectTaxForPeriod).
func (r *Repository) UpdateCharged(ctx context.Context, id int64, taxAmountRsd decimal.Decimal, chargedAt time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE tax_charges SET status = $2, tax_amount_rsd = $3, charged_at = $4 WHERE id = $1`,
		id, StatusCharged, taxAmountRsd.String(), chargedAt)
	return err
}

// MarkCharged sets status=CHARGED + charged_at without touching tax_amount_rsd
// (OTC success branch — RSD was set at insert — and handleFailedChargeAttempt
// when the debit had already succeeded).
func (r *Repository) MarkCharged(ctx context.Context, id int64, chargedAt time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE tax_charges SET status = $2, charged_at = $3 WHERE id = $1`,
		id, StatusCharged, chargedAt)
	return err
}

// Delete removes a reservation (handleFailedChargeAttempt when the debit failed —
// the row is deleted, NOT marked FAILED, so a later run retries it).
func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM tax_charges WHERE id = $1`, id)
	return err
}

// LoadExercisedOtcTaxEntries mirrors TaxServiceImpl.loadExercisedOtcTaxEntries:
// the raw JOIN over option_contracts + stock_ownership_transfers + portfolio that
// picks, per contract, the COMPLETED transfer closest in time to the exercise.
// The caller swallows any error to an empty slice (Java logs warn + returns
// List.of()), since these trading-service-owned tables may be absent/empty during
// early coexistence.
func (r *Repository) LoadExercisedOtcTaxEntries(ctx context.Context, endExclusive time.Time) ([]OtcTaxEntry, error) {
	const sql = `
		SELECT DISTINCT ON (c.id)
		       c.id                            AS contract_id,
		       c.seller_id                     AS seller_id,
		       t.listing_id                    AS listing_id,
		       c.stock_ticker                  AS stock_ticker,
		       c.amount                        AS amount,
		       c.price_per_stock::text         AS price_per_stock,
		       p.average_purchase_price::text  AS average_purchase_price,
		       c.exercised_at                  AS exercised_at
		  FROM option_contracts c
		  JOIN stock_ownership_transfers t
		    ON t.seller_id = c.seller_id
		   AND t.buyer_id = c.buyer_id
		   AND upper(t.stock_ticker) = upper(c.stock_ticker)
		   AND t.amount = c.amount
		   AND t.status = 'COMPLETED'
		  JOIN portfolio p
		    ON p.user_id = c.seller_id
		   AND p.listing_id = t.listing_id
		 WHERE c.status = 'EXERCISED'
		   AND c.exercised_at IS NOT NULL
		   AND c.exercised_at < $1
		 ORDER BY c.id,
		          abs(extract(epoch from (t.created_at - c.exercised_at))) asc`
	rows, err := r.db.Query(ctx, sql, endExclusive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]OtcTaxEntry, 0)
	for rows.Next() {
		var (
			e       OtcTaxEntry
			ppuText string
			avgText string
		)
		if err := rows.Scan(&e.ContractID, &e.SellerID, &e.ListingID, &e.Ticker, &e.Amount,
			&ppuText, &avgText, &e.ExercisedAt); err != nil {
			return nil, err
		}
		ppu, err := decimal.NewFromString(ppuText)
		if err != nil {
			return nil, err
		}
		e.SellPricePerStock = ppu
		avg, err := decimal.NewFromString(avgText)
		if err != nil {
			return nil, err
		}
		e.AveragePurchasePrice = avg
		out = append(out, e)
	}
	return out, rows.Err()
}
