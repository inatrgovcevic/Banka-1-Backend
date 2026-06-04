// Package portfolio serves the /portfolio endpoints over the shared `portfolio`
// table (owned by Java Liquibase; this service runs no migrations). Mirrors
// order-service PortfolioServiceImpl. NUMERIC is read as ::text into
// shopspring/decimal to preserve scale; last_modified is NOT NULL and set to
// now() on every write (mirrors the @PrePersist/@PreUpdate callback).
package portfolio

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
// either standalone or inside a transaction (exercise-option uses a tx).
type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Portfolio mirrors a row of the `portfolio` table.
type Portfolio struct {
	ID                   int64
	UserID               int64
	ListingID            int64
	ListingType          string
	Quantity             int
	ReservedQuantity     int
	AveragePurchasePrice decimal.Decimal
	IsPublic             bool
	PublicQuantity       int
	LastModified         time.Time
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Pool exposes the pool so the service can open transactions (exercise-option).
func (r *Repository) Pool() *pgxpool.Pool { return r.db }

const selectColumns = `id, user_id, listing_id, listing_type, quantity, reserved_quantity,
		average_purchase_price::text, is_public, public_quantity, last_modified`

func scanPortfolio(row pgx.Row) (*Portfolio, error) {
	var p Portfolio
	var avgText string
	if err := row.Scan(&p.ID, &p.UserID, &p.ListingID, &p.ListingType, &p.Quantity, &p.ReservedQuantity,
		&avgText, &p.IsPublic, &p.PublicQuantity, &p.LastModified); err != nil {
		return nil, err
	}
	avg, err := decimal.NewFromString(avgText)
	if err != nil {
		return nil, err
	}
	p.AveragePurchasePrice = avg
	return &p, nil
}

// FindByUserID returns every position for a user. No ORDER BY — mirrors the Java
// derived query findByUserId so the row order matches the live service.
func (r *Repository) FindByUserID(ctx context.Context, q Querier, userID int64) ([]Portfolio, error) {
	rows, err := q.Query(ctx, `SELECT `+selectColumns+` FROM portfolio WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Portfolio, 0)
	for rows.Next() {
		p, err := scanPortfolio(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// FindStockHoldersByListingID mirrors PortfolioRepository.findByListingIdStockHolders
// (WP-14 dividend): every STOCK position with quantity > 0 for one listing.
func (r *Repository) FindStockHoldersByListingID(ctx context.Context, q Querier, listingID int64) ([]Portfolio, error) {
	rows, err := q.Query(ctx, `SELECT `+selectColumns+` FROM portfolio WHERE listing_id = $1 AND listing_type = 'STOCK' AND quantity > 0`, listingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Portfolio, 0)
	for rows.Next() {
		p, err := scanPortfolio(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// FindByID returns the position with the given id, or (nil, nil) when absent.
func (r *Repository) FindByID(ctx context.Context, q Querier, id int64) (*Portfolio, error) {
	p, err := scanPortfolio(q.QueryRow(ctx, `SELECT `+selectColumns+` FROM portfolio WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// FindByIDForUpdate is FindByID under a write lock. Used by the interbank 2PC
// commit/release (P7), which resolves the locked portfolio row by its PK (stored
// on the interbank_stock_reservations row) — a Go-port hardening; Java's
// InterbankStockReservationService commit/release fetch the row by id with no
// lock. Each interbank op locks exactly one portfolio row (no ordering hazard).
func (r *Repository) FindByIDForUpdate(ctx context.Context, q Querier, id int64) (*Portfolio, error) {
	p, err := scanPortfolio(q.QueryRow(ctx, `SELECT `+selectColumns+` FROM portfolio WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// FindByUserIDAndListingID returns the (user, listing) position, or (nil, nil).
func (r *Repository) FindByUserIDAndListingID(ctx context.Context, q Querier, userID, listingID int64) (*Portfolio, error) {
	p, err := scanPortfolio(q.QueryRow(ctx,
		`SELECT `+selectColumns+` FROM portfolio WHERE user_id = $1 AND listing_id = $2`, userID, listingID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// FindByUserIDAndListingIDForUpdate is FindByUserIDAndListingID under a write
// lock (mirrors PortfolioRepository.findByUserIdAndListingIdForUpdate). Used by
// order confirm (reserve), execution (settle), and cancel/decline (release).
func (r *Repository) FindByUserIDAndListingIDForUpdate(ctx context.Context, q Querier, userID, listingID int64) (*Portfolio, error) {
	p, err := scanPortfolio(q.QueryRow(ctx,
		`SELECT `+selectColumns+` FROM portfolio WHERE user_id = $1 AND listing_id = $2 FOR UPDATE`, userID, listingID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// UpdateReservedQuantity sets reserved_quantity (sell-order reserve on confirm,
// release on cancel/decline) and bumps last_modified.
func (r *Repository) UpdateReservedQuantity(ctx context.Context, q Querier, id int64, reserved int) error {
	_, err := q.Exec(ctx,
		`UPDATE portfolio SET reserved_quantity = $2, last_modified = now() WHERE id = $1`, id, reserved)
	return err
}

// UpdateSellPosition applies a sell fill: sets quantity, reserved_quantity and
// public_quantity together (mirrors PortfolioServiceImpl/OrderExecution sell
// branch) and bumps last_modified. The caller deletes the row when both quantity
// and reserved reach 0.
func (r *Repository) UpdateSellPosition(ctx context.Context, q Querier, id int64, quantity, reserved, public int) error {
	_, err := q.Exec(ctx,
		`UPDATE portfolio SET quantity = $2, reserved_quantity = $3, public_quantity = $4, last_modified = now() WHERE id = $1`,
		id, quantity, reserved, public)
	return err
}

// UpdatePublic sets public_quantity + is_public and bumps last_modified.
func (r *Repository) UpdatePublic(ctx context.Context, q Querier, id int64, publicQuantity int, isPublic bool) error {
	_, err := q.Exec(ctx,
		`UPDATE portfolio SET public_quantity = $2, is_public = $3, last_modified = now() WHERE id = $1`,
		id, publicQuantity, isPublic)
	return err
}

// Insert creates a new position (reserved_quantity/is_public/public_quantity use
// DB defaults; last_modified set to now()). Used by CALL exercise.
func (r *Repository) Insert(ctx context.Context, q Querier, userID, listingID int64, listingType string, quantity int, avg decimal.Decimal) error {
	_, err := q.Exec(ctx,
		`INSERT INTO portfolio (user_id, listing_id, listing_type, quantity, average_purchase_price, last_modified)
		 VALUES ($1, $2, $3, $4, $5, now())`,
		userID, listingID, listingType, quantity, avg.String())
	return err
}

// UpdateQuantityAndAvg sets quantity + average_purchase_price (CALL merge).
func (r *Repository) UpdateQuantityAndAvg(ctx context.Context, q Querier, id int64, quantity int, avg decimal.Decimal) error {
	_, err := q.Exec(ctx,
		`UPDATE portfolio SET quantity = $2, average_purchase_price = $3, last_modified = now() WHERE id = $1`,
		id, quantity, avg.String())
	return err
}

// UpdateQuantity sets quantity only (PUT decrement).
func (r *Repository) UpdateQuantity(ctx context.Context, q Querier, id int64, quantity int) error {
	_, err := q.Exec(ctx, `UPDATE portfolio SET quantity = $2, last_modified = now() WHERE id = $1`, id, quantity)
	return err
}

// Delete removes a position (option position after exercise; underlying on PUT to zero).
func (r *Repository) Delete(ctx context.Context, q Querier, id int64) error {
	_, err := q.Exec(ctx, `DELETE FROM portfolio WHERE id = $1`, id)
	return err
}

// FindAllPublicStocks mirrors PortfolioRepository.findAllPublicStocks: every STOCK
// position currently advertised for OTC (is_public = true AND public_quantity > 0).
// No ORDER BY — matches the Java derived query so, against the same Postgres, the
// row order (and thus the OTC public-stocks ticker/seller order) matches the live
// service. Used by OTC getPublicStocks (P6).
func (r *Repository) FindAllPublicStocks(ctx context.Context, q Querier) ([]Portfolio, error) {
	if q == nil {
		q = r.db
	}
	rows, err := q.Query(ctx, `SELECT `+selectColumns+` FROM portfolio
		WHERE listing_type = 'STOCK' AND is_public = true AND public_quantity > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Portfolio, 0)
	for rows.Next() {
		p, err := scanPortfolio(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// UpdateReservedAndPublic sets reserved_quantity + public_quantity together and
// bumps last_modified. Mirrors the single save() in OtcPortfolioService
// reserveForContract (reserved += amount, public = max(0, public-amount)) and
// releaseForContract (reserved = max(0, reserved-amount), public = min(qty,
// public+amount)) — the caller computes the new values (P6).
func (r *Repository) UpdateReservedAndPublic(ctx context.Context, q Querier, id int64, reserved, public int) error {
	_, err := q.Exec(ctx,
		`UPDATE portfolio SET reserved_quantity = $2, public_quantity = $3, last_modified = now() WHERE id = $1`,
		id, reserved, public)
	return err
}

// UpdateQuantityAndReserved sets quantity + reserved_quantity together and bumps
// last_modified. Mirrors the seller-side save() in StockReservationService
// transferOwnership (quantity -= amount, reserved = max(0, reserved-amount)) and
// the seller restore in reverseOwnership (quantity += amount, reserved += amount)
// (P6).
func (r *Repository) UpdateQuantityAndReserved(ctx context.Context, q Querier, id int64, quantity, reserved int) error {
	_, err := q.Exec(ctx,
		`UPDATE portfolio SET quantity = $2, reserved_quantity = $3, last_modified = now() WHERE id = $1`,
		id, quantity, reserved)
	return err
}
