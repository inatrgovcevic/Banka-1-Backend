package otc

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx so repository methods run
// either standalone (reads) or inside a RunInTx (every mutation).
type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// ErrNotFound is returned by lookups when no row matches (service layer maps it
// to the OTC 404 "ne postoji" message, mirroring orElseThrow(IllegalArgument)).
var ErrNotFound = errors.New("otc: not found")

// Repository centralizes raw-SQL access to otc_offers + option_contracts (the
// negotiation-history and expiry-reminder tables live in repository_history.go;
// the stock_reservations / stock_ownership_transfers tables in reservations.go).
type Repository struct {
	db *pgxpool.Pool
	// q is the Querier used by the standalone (non-tx) read paths. It defaults to
	// db (the pool) in production; tests inject a fake Querier so the repository
	// query/scan paths are unit-testable without Postgres.
	q Querier
}

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db, q: db} }

// Pool exposes the pool so the service can open a RunInTx itself.
func (r *Repository) Pool() *pgxpool.Pool { return r.db }

// querier returns the standalone Querier (the injected fake in tests, else the
// pool). Guards against a zero-value Repository where q was never set.
func (r *Repository) querier() Querier {
	if r.q != nil {
		return r.q
	}
	return r.db
}

// =============================== otc_offers ===============================

const offerColumns = `id, stock_ticker, buyer_id, seller_id, amount,
	price_per_stock::text, premium::text, settlement_date, status, modified_by,
	last_modified, created_at, version`

func scanOffer(row pgx.Row) (*OtcOffer, error) {
	var (
		o       OtcOffer
		ppsText string
		premStr string
	)
	if err := row.Scan(&o.ID, &o.StockTicker, &o.BuyerID, &o.SellerID, &o.Amount,
		&ppsText, &premStr, &o.SettlementDate, &o.Status, &o.ModifiedBy,
		&o.LastModified, &o.CreatedAt, &o.Version); err != nil {
		return nil, err
	}
	pps, err := decimal.NewFromString(ppsText)
	if err != nil {
		return nil, err
	}
	o.PricePerStock = pps
	prem, err := decimal.NewFromString(premStr)
	if err != nil {
		return nil, err
	}
	o.Premium = prem
	return &o, nil
}

func scanOffers(rows pgx.Rows) ([]OtcOffer, error) {
	defer rows.Close()
	out := make([]OtcOffer, 0)
	for rows.Next() {
		o, err := scanOffer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}

// InsertOffer mirrors the persist branch of otcOfferRepository.save: a new
// PENDING_SELLER offer. created_at / last_modified / version use the DB defaults
// (CURRENT_TIMESTAMP / 0) and are returned to fill the struct.
func (r *Repository) InsertOffer(ctx context.Context, q Querier, o *OtcOffer) error {
	if q == nil {
		q = r.db
	}
	return q.QueryRow(ctx, `
		INSERT INTO otc_offers
			(stock_ticker, buyer_id, seller_id, amount, price_per_stock, premium,
			 settlement_date, status, modified_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, last_modified, created_at, version`,
		o.StockTicker, o.BuyerID, o.SellerID, o.Amount, o.PricePerStock, o.Premium,
		o.SettlementDate, o.Status, o.ModifiedBy,
	).Scan(&o.ID, &o.LastModified, &o.CreatedAt, &o.Version)
}

// FindOfferByID mirrors otcOfferRepository.findById.
func (r *Repository) FindOfferByID(ctx context.Context, q Querier, id int64) (*OtcOffer, error) {
	if q == nil {
		q = r.db
	}
	o, err := scanOffer(q.QueryRow(ctx, `SELECT `+offerColumns+` FROM otc_offers WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

// FindOfferByIDForUpdate mirrors @Lock(PESSIMISTIC_WRITE)
// otcOfferRepository.findByIdForUpdate. Java uses it on accept; the Go port also
// takes it on counter/reject/withdraw to serialize concurrent transitions on the
// same offer (no observable change vs Java's optimistic @Version, just locking).
func (r *Repository) FindOfferByIDForUpdate(ctx context.Context, q Querier, id int64) (*OtcOffer, error) {
	o, err := scanOffer(q.QueryRow(ctx, `SELECT `+offerColumns+` FROM otc_offers WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

// UpdateOffer mirrors the merge branch of otcOfferRepository.save for the fields a
// transition mutates (amount/price/premium/settlement/status/modified_by). It
// bumps version and refreshes last_modified = now() (mirrors @PreUpdate), writing
// both back into the struct so toDto returns the post-save timestamp. The row is
// always FOR UPDATE locked by the caller, so no version guard is needed.
func (r *Repository) UpdateOffer(ctx context.Context, q Querier, o *OtcOffer) error {
	return q.QueryRow(ctx, `
		UPDATE otc_offers
		SET amount = $2, price_per_stock = $3, premium = $4, settlement_date = $5,
			status = $6, modified_by = $7, last_modified = now(), version = version + 1
		WHERE id = $1
		RETURNING last_modified, version`,
		o.ID, o.Amount, o.PricePerStock, o.Premium, o.SettlementDate, o.Status, o.ModifiedBy,
	).Scan(&o.LastModified, &o.Version)
}

// FindActiveOffersForUser mirrors
// findByBuyerIdAndStatusInOrSellerIdAndStatusIn(userId, ACTIVE, userId, ACTIVE)
// where ACTIVE = {PENDING_BUYER, PENDING_SELLER}. No ORDER BY (matches the Java
// derived query → same physical row order against the shared Postgres).
func (r *Repository) FindActiveOffersForUser(ctx context.Context, userID int64) ([]OtcOffer, error) {
	rows, err := r.querier().Query(ctx, `SELECT `+offerColumns+` FROM otc_offers
		WHERE (buyer_id = $1 AND status IN ('PENDING_BUYER','PENDING_SELLER'))
		   OR (seller_id = $1 AND status IN ('PENDING_BUYER','PENDING_SELLER'))`, userID)
	if err != nil {
		return nil, err
	}
	return scanOffers(rows)
}

// ============================ option_contracts ============================

const contractColumns = `id, offer_id, stock_ticker, buyer_id, seller_id, amount,
	price_per_stock::text, settlement_date, status, created_at, exercised_at, version`

func scanContract(row pgx.Row) (*OptionContract, error) {
	var (
		c       OptionContract
		ppsText string
	)
	if err := row.Scan(&c.ID, &c.OfferID, &c.StockTicker, &c.BuyerID, &c.SellerID,
		&c.Amount, &ppsText, &c.SettlementDate, &c.Status, &c.CreatedAt,
		&c.ExercisedAt, &c.Version); err != nil {
		return nil, err
	}
	pps, err := decimal.NewFromString(ppsText)
	if err != nil {
		return nil, err
	}
	c.PricePerStock = pps
	return &c, nil
}

func scanContracts(rows pgx.Rows) ([]OptionContract, error) {
	defer rows.Close()
	out := make([]OptionContract, 0)
	for rows.Next() {
		c, err := scanContract(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// InsertOptionContract mirrors the persist branch of optionContractRepository.save:
// a PENDING_PREMIUM contract created on accept. created_at / version use DB
// defaults and are returned.
func (r *Repository) InsertOptionContract(ctx context.Context, q Querier, c *OptionContract) error {
	return q.QueryRow(ctx, `
		INSERT INTO option_contracts
			(offer_id, stock_ticker, buyer_id, seller_id, amount, price_per_stock,
			 settlement_date, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, created_at, version`,
		c.OfferID, c.StockTicker, c.BuyerID, c.SellerID, c.Amount, c.PricePerStock,
		c.SettlementDate, c.Status,
	).Scan(&c.ID, &c.CreatedAt, &c.Version)
}

// FindOptionContractByID mirrors optionContractRepository.findById.
func (r *Repository) FindOptionContractByID(ctx context.Context, q Querier, id int64) (*OptionContract, error) {
	if q == nil {
		q = r.db
	}
	c, err := scanContract(q.QueryRow(ctx, `SELECT `+contractColumns+` FROM option_contracts WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

// FindOptionContractByIDForUpdate locks the contract row (used by exercise + the
// saga completion listeners so concurrent transitions serialize).
func (r *Repository) FindOptionContractByIDForUpdate(ctx context.Context, q Querier, id int64) (*OptionContract, error) {
	c, err := scanContract(q.QueryRow(ctx, `SELECT `+contractColumns+` FROM option_contracts WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

// UpdateOptionContractStatus flips the status (+ version bump) — used by the saga
// listeners (PENDING_PREMIUM→ACTIVE / →CANCELED, ACTIVE→EXERCISED) and the expire
// cron (ACTIVE→EXPIRED).
func (r *Repository) UpdateOptionContractStatus(ctx context.Context, q Querier, id int64, status string) error {
	if q == nil {
		q = r.db
	}
	tag, err := q.Exec(ctx,
		`UPDATE option_contracts SET status = $2, version = version + 1 WHERE id = $1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetOptionContractExercisedAt stamps exercised_at (status stays ACTIVE — the
// saga completion listener flips it to EXERCISED). Mirrors
// OtcService.exerciseContract setting the marker before the saga publish.
func (r *Repository) SetOptionContractExercisedAt(ctx context.Context, q Querier, id int64, exercisedAt time.Time) error {
	tag, err := q.Exec(ctx,
		`UPDATE option_contracts SET exercised_at = $2, version = version + 1 WHERE id = $1`, id, exercisedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SumActiveBySellerAndTicker mirrors
// optionContractRepository.sumActiveBySellerAndTicker: the COALESCE(SUM(amount),0)
// of the seller's still-live contracts (status IN {ACTIVE, PENDING_PREMIUM}) for a
// ticker — the reserved-stock invariant input on accept.
func (r *Repository) SumActiveBySellerAndTicker(ctx context.Context, q Querier, sellerID int64, ticker string) (int64, error) {
	var sum int64
	err := q.QueryRow(ctx, `SELECT COALESCE(SUM(amount), 0) FROM option_contracts
		WHERE seller_id = $1 AND stock_ticker = $2 AND status IN ('ACTIVE','PENDING_PREMIUM')`,
		sellerID, ticker).Scan(&sum)
	return sum, err
}

// FindContractsByBuyerIDAndStatus / FindContractsBySellerIDAndStatus mirror the
// derived queries used by myContracts. No ORDER BY (matches Java).
func (r *Repository) FindContractsByBuyerIDAndStatus(ctx context.Context, buyerID int64, status string) ([]OptionContract, error) {
	rows, err := r.querier().Query(ctx, `SELECT `+contractColumns+` FROM option_contracts
		WHERE buyer_id = $1 AND status = $2`, buyerID, status)
	if err != nil {
		return nil, err
	}
	return scanContracts(rows)
}

func (r *Repository) FindContractsBySellerIDAndStatus(ctx context.Context, sellerID int64, status string) ([]OptionContract, error) {
	rows, err := r.querier().Query(ctx, `SELECT `+contractColumns+` FROM option_contracts
		WHERE seller_id = $1 AND status = $2`, sellerID, status)
	if err != nil {
		return nil, err
	}
	return scanContracts(rows)
}

// FindContractsByStatusAndSettlementDateBefore mirrors
// findByStatusAndSettlementDateBefore — the expire-overdue cron input.
func (r *Repository) FindContractsByStatusAndSettlementDateBefore(ctx context.Context, status string, before time.Time) ([]OptionContract, error) {
	rows, err := r.querier().Query(ctx, `SELECT `+contractColumns+` FROM option_contracts
		WHERE status = $1 AND settlement_date < $2::date`, status, before)
	if err != nil {
		return nil, err
	}
	return scanContracts(rows)
}

// FindContractsByStatusAndSettlementDate mirrors findByStatusAndSettlementDate —
// the expiry-reminder cron input (contracts settling exactly on the D-N target).
func (r *Repository) FindContractsByStatusAndSettlementDate(ctx context.Context, status string, date time.Time) ([]OptionContract, error) {
	rows, err := r.querier().Query(ctx, `SELECT `+contractColumns+` FROM option_contracts
		WHERE status = $1 AND settlement_date = $2::date`, status, date)
	if err != nil {
		return nil, err
	}
	return scanContracts(rows)
}
