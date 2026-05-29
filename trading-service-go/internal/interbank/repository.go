package interbank

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx so repository methods run
// either standalone or inside a gpdb.RunInTx (every interbank mutation runs in a tx).
type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Repository centralizes raw-SQL access to interbank_stock_reservations +
// interbank_option_reservations. Both tables live in the shared `trading` database;
// the schema is owned by Java Liquibase (this service runs no migrations).
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

// Pool exposes the pool so the service can open a RunInTx itself.
func (r *Repository) Pool() *pgxpool.Pool { return r.db }

// ===================== interbank_stock_reservations ======================

// InsertStockReservation mirrors the persist of a new InterbankStockReservation
// (status HELD). created_at uses the DB DEFAULT NOW(); finalized_at stays NULL.
// reservation_id is cast to ::uuid (the column is UUID; we carry it as a string).
func (r *Repository) InsertStockReservation(ctx context.Context, q Querier, reservationID string, txRouting int, txLocal string, portfolioID int64, ticker string, quantity int) error {
	_, err := q.Exec(ctx, `
		INSERT INTO interbank_stock_reservations
			(reservation_id, transaction_id_routing, transaction_id_local, portfolio_id, ticker, quantity, status)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, 'HELD')`,
		reservationID, txRouting, txLocal, portfolioID, ticker, quantity)
	return err
}

// FindStockReservationByReservationID mirrors
// InterbankStockReservationRepository.findByReservationId. Returns (nil, nil) when
// no row matches (the service maps that to the IllegalArgument "not found" → 404).
func (r *Repository) FindStockReservationByReservationID(ctx context.Context, q Querier, reservationID string) (*StockReservation, error) {
	var res StockReservation
	err := q.QueryRow(ctx,
		`SELECT reservation_id, portfolio_id, quantity, status
		   FROM interbank_stock_reservations WHERE reservation_id = $1::uuid`,
		reservationID).Scan(&res.ReservationID, &res.PortfolioID, &res.Quantity, &res.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// FinalizeStockReservation flips the status (COMMITTED or RELEASED) and stamps
// finalized_at = NOW(), mirroring the reservation.save() in commitStock/releaseStock.
func (r *Repository) FinalizeStockReservation(ctx context.Context, q Querier, reservationID, status string) error {
	_, err := q.Exec(ctx,
		`UPDATE interbank_stock_reservations SET status = $2, finalized_at = NOW()
		   WHERE reservation_id = $1::uuid`,
		reservationID, status)
	return err
}

// ===================== interbank_option_reservations =====================

// FindOptionReservationByNegotiationID mirrors
// InterbankOptionReservationRepository.findById (negotiation_id is the PK). Returns
// (nil, nil) when no row matches.
func (r *Repository) FindOptionReservationByNegotiationID(ctx context.Context, q Querier, negotiationID string) (*OptionReservation, error) {
	var res OptionReservation
	err := q.QueryRow(ctx,
		`SELECT negotiation_id, reservation_id, status
		   FROM interbank_option_reservations WHERE negotiation_id = $1`,
		negotiationID).Scan(&res.NegotiationID, &res.ReservationID, &res.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// InsertOptionReservation mirrors the persist of a new InterbankOptionReservation.
// created_at / updated_at use the DB DEFAULT CURRENT_TIMESTAMP (matches the Java
// @PrePersist setting both to now()).
func (r *Repository) InsertOptionReservation(ctx context.Context, q Querier, negotiationID, reservationID, status string, sellerUserID int64, ticker string, quantity int) error {
	_, err := q.Exec(ctx, `
		INSERT INTO interbank_option_reservations
			(negotiation_id, reservation_id, status, seller_user_id, ticker, quantity)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		negotiationID, reservationID, status, sellerUserID, ticker, quantity)
	return err
}

// UpdateOptionReservationStatus flips the status and stamps updated_at = NOW()
// (mirrors the @PreUpdate on save), used by exercise (→EXERCISED) and release
// (→RELEASED).
func (r *Repository) UpdateOptionReservationStatus(ctx context.Context, q Querier, negotiationID, status string) error {
	_, err := q.Exec(ctx,
		`UPDATE interbank_option_reservations SET status = $2, updated_at = NOW()
		   WHERE negotiation_id = $1`,
		negotiationID, status)
	return err
}
