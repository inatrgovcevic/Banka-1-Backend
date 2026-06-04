// Package order serves the /orders endpoints (creation + lifecycle) and the
// asynchronous matching/execution engine over the `orders` and `transactions`
// tables, plus the shared `portfolio` and `actuary_info` tables (via those
// packages' repositories). The `trading` schema is owned by Java Liquibase; this
// service runs no migrations. NUMERIC columns are read as ::text into
// shopspring/decimal to preserve scale; last_modification is set to now() on
// every write (mirrors the JPA @PrePersist/@PreUpdate callback).
package order

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
// either standalone or inside the execution transaction.
type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Order mirrors a row of the `orders` table.
type Order struct {
	ID                    int64
	UserID                int64
	ListingID             int64
	OrderType             string
	Quantity              int
	ContractSize          int
	PricePerUnit          decimal.Decimal
	LimitValue            *decimal.Decimal
	StopValue             *decimal.Decimal
	Direction             string
	Status                string
	ApprovedBy            *int64
	IsDone                bool
	LastModification      time.Time
	RemainingPortions     int
	AfterHours            bool
	ExchangeClosed        bool
	AllOrNone             bool
	Margin                bool
	AccountID             int64
	ReservedLimitExposure decimal.Decimal
	PurchaseFor           *string
	FundID                *int64
	// CreatedAt is stamped once at insert (mirrors @PrePersist on Order.createdAt,
	// migration 004). ExecutedAt is set when the order reaches DONE.
	CreatedAt  time.Time
	ExecutedAt *time.Time
}

// Transaction mirrors a row of the `transactions` table (one executed portion).
type Transaction struct {
	ID           int64
	OrderID      int64
	Quantity     int
	PricePerUnit decimal.Decimal
	TotalPrice   decimal.Decimal
	Commission   decimal.Decimal
	Timestamp    time.Time
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Pool exposes the pool so the service can open execution transactions.
func (r *Repository) Pool() *pgxpool.Pool { return r.db }

const orderColumns = `id, user_id, listing_id, order_type, quantity, contract_size,
	price_per_unit::text, limit_value::text, stop_value::text, direction, status,
	approved_by, is_done, last_modification, remaining_portions, after_hours,
	exchange_closed, all_or_none, margin, account_id, reserved_limit_exposure::text,
	purchase_for, fund_id, created_at, executed_at`

func scanOrder(row pgx.Row) (*Order, error) {
	var (
		o            Order
		priceText    string
		limitText    *string
		stopText     *string
		reservedText string
	)
	if err := row.Scan(&o.ID, &o.UserID, &o.ListingID, &o.OrderType, &o.Quantity, &o.ContractSize,
		&priceText, &limitText, &stopText, &o.Direction, &o.Status,
		&o.ApprovedBy, &o.IsDone, &o.LastModification, &o.RemainingPortions, &o.AfterHours,
		&o.ExchangeClosed, &o.AllOrNone, &o.Margin, &o.AccountID, &reservedText,
		&o.PurchaseFor, &o.FundID, &o.CreatedAt, &o.ExecutedAt); err != nil {
		return nil, err
	}
	price, err := decimal.NewFromString(priceText)
	if err != nil {
		return nil, err
	}
	o.PricePerUnit = price
	reserved, err := decimal.NewFromString(reservedText)
	if err != nil {
		return nil, err
	}
	o.ReservedLimitExposure = reserved
	if limitText != nil {
		v, err := decimal.NewFromString(*limitText)
		if err != nil {
			return nil, err
		}
		o.LimitValue = &v
	}
	if stopText != nil {
		v, err := decimal.NewFromString(*stopText)
		if err != nil {
			return nil, err
		}
		o.StopValue = &v
	}
	return &o, nil
}

func scanOrders(rows pgx.Rows) ([]Order, error) {
	defer rows.Close()
	out := make([]Order, 0)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}

// FindByID returns the order with the given id, or (nil, nil) when absent.
func (r *Repository) FindByID(ctx context.Context, q Querier, id int64) (*Order, error) {
	o, err := scanOrder(q.QueryRow(ctx, `SELECT `+orderColumns+` FROM orders WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return o, err
}

// FindByIDForUpdate returns the order under a write lock, or (nil, nil) when
// absent. Mirrors OrderRepository.findByIdForUpdate.
func (r *Repository) FindByIDForUpdate(ctx context.Context, q Querier, id int64) (*Order, error) {
	o, err := scanOrder(q.QueryRow(ctx, `SELECT `+orderColumns+` FROM orders WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return o, err
}

// FindAll returns every order (no ORDER BY — mirrors JpaRepository.findAll row
// order). The supervisor overview filters out PENDING_CONFIRMATION in the service.
func (r *Repository) FindAll(ctx context.Context, q Querier) ([]Order, error) {
	rows, err := q.Query(ctx, `SELECT `+orderColumns+` FROM orders`)
	if err != nil {
		return nil, err
	}
	return scanOrders(rows)
}

// FindByStatus returns all orders with the given status (derived query order).
func (r *Repository) FindByStatus(ctx context.Context, q Querier, status string) ([]Order, error) {
	rows, err := q.Query(ctx, `SELECT `+orderColumns+` FROM orders WHERE status = $1`, status)
	if err != nil {
		return nil, err
	}
	return scanOrders(rows)
}

// FindByUserID returns all orders placed by a user (mirrors findByUserId).
func (r *Repository) FindByUserID(ctx context.Context, q Querier, userID int64) ([]Order, error) {
	rows, err := q.Query(ctx, `SELECT `+orderColumns+` FROM orders WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	return scanOrders(rows)
}

// Insert persists a new order, setting last_modification to now() (mirrors
// @PrePersist) and returning the generated id and timestamp. The struct is
// updated in place.
func (r *Repository) Insert(ctx context.Context, q Querier, o *Order) error {
	var limit, stop, reserved any
	if o.LimitValue != nil {
		limit = o.LimitValue.String()
	}
	if o.StopValue != nil {
		stop = o.StopValue.String()
	}
	reserved = o.ReservedLimitExposure.String()
	return q.QueryRow(ctx, `
		INSERT INTO orders (
			user_id, listing_id, order_type, quantity, contract_size, price_per_unit,
			limit_value, stop_value, direction, status, approved_by, is_done,
			last_modification, remaining_portions, after_hours, exchange_closed,
			all_or_none, margin, account_id, reserved_limit_exposure, purchase_for, fund_id,
			created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12, now(), $13,$14,$15,$16,$17,$18,$19,$20,$21, now())
		RETURNING id, last_modification, created_at`,
		o.UserID, o.ListingID, o.OrderType, o.Quantity, o.ContractSize, o.PricePerUnit.String(),
		limit, stop, o.Direction, o.Status, o.ApprovedBy, o.IsDone,
		o.RemainingPortions, o.AfterHours, o.ExchangeClosed, o.AllOrNone, o.Margin, o.AccountID,
		reserved, o.PurchaseFor, o.FundID).
		Scan(&o.ID, &o.LastModification, &o.CreatedAt)
}

// Update writes the mutable columns of an existing order, bumping
// last_modification to now() (mirrors @PreUpdate). user_id/listing_id are
// immutable. The struct's LastModification is refreshed from the DB.
func (r *Repository) Update(ctx context.Context, q Querier, o *Order) error {
	var limit, stop any
	if o.LimitValue != nil {
		limit = o.LimitValue.String()
	}
	if o.StopValue != nil {
		stop = o.StopValue.String()
	}
	return q.QueryRow(ctx, `
		UPDATE orders SET
			order_type = $2, quantity = $3, contract_size = $4, price_per_unit = $5,
			limit_value = $6, stop_value = $7, direction = $8, status = $9, approved_by = $10,
			is_done = $11, remaining_portions = $12, after_hours = $13, exchange_closed = $14,
			all_or_none = $15, margin = $16, account_id = $17, reserved_limit_exposure = $18,
			purchase_for = $19, fund_id = $20, executed_at = $21, last_modification = now()
		WHERE id = $1
		RETURNING last_modification`,
		o.ID, o.OrderType, o.Quantity, o.ContractSize, o.PricePerUnit.String(),
		limit, stop, o.Direction, o.Status, o.ApprovedBy,
		o.IsDone, o.RemainingPortions, o.AfterHours, o.ExchangeClosed,
		o.AllOrNone, o.Margin, o.AccountID, o.ReservedLimitExposure.String(),
		o.PurchaseFor, o.FundID, o.ExecutedAt).
		Scan(&o.LastModification)
}

// InsertTransaction records one executed portion (mirrors transactionRepository
// .save), setting timestamp to now().
func (r *Repository) InsertTransaction(ctx context.Context, q Querier, t *Transaction) error {
	return q.QueryRow(ctx, `
		INSERT INTO transactions (order_id, quantity, price_per_unit, total_price, commission, timestamp)
		VALUES ($1, $2, $3, $4, $5, now())
		RETURNING id, timestamp`,
		t.OrderID, t.Quantity, t.PricePerUnit.String(), t.TotalPrice.String(), t.Commission.String()).
		Scan(&t.ID, &t.Timestamp)
}

// --- Reads used by the tax domain (P4) ------------------------------------
//
// The capital-gains FIFO needs the full BUY/SELL order + transaction history for
// the in-scope (user, listing) pairs. These mirror the OrderRepository /
// TransactionRepository derived queries TaxServiceImpl autowires. None add an
// ORDER BY — the tax service sorts transactions in memory exactly as Java does.

const transactionColumns = `id, order_id, quantity, price_per_unit::text, total_price::text, commission::text, timestamp`

func scanTransaction(row pgx.Row) (*Transaction, error) {
	var (
		t              Transaction
		priceText      string
		totalText      string
		commissionText string
	)
	if err := row.Scan(&t.ID, &t.OrderID, &t.Quantity, &priceText, &totalText, &commissionText, &t.Timestamp); err != nil {
		return nil, err
	}
	price, err := decimal.NewFromString(priceText)
	if err != nil {
		return nil, err
	}
	t.PricePerUnit = price
	total, err := decimal.NewFromString(totalText)
	if err != nil {
		return nil, err
	}
	t.TotalPrice = total
	commission, err := decimal.NewFromString(commissionText)
	if err != nil {
		return nil, err
	}
	t.Commission = commission
	return &t, nil
}

func scanTransactions(rows pgx.Rows) ([]Transaction, error) {
	defer rows.Close()
	out := make([]Transaction, 0)
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// FindByDirection mirrors OrderRepository.findByDirection (BUY/SELL).
func (r *Repository) FindByDirection(ctx context.Context, q Querier, direction string) ([]Order, error) {
	rows, err := q.Query(ctx, `SELECT `+orderColumns+` FROM orders WHERE direction = $1`, direction)
	if err != nil {
		return nil, err
	}
	return scanOrders(rows)
}

// FindByUserIDAndDirection mirrors OrderRepository.findByUserIdAndDirection.
func (r *Repository) FindByUserIDAndDirection(ctx context.Context, q Querier, userID int64, direction string) ([]Order, error) {
	rows, err := q.Query(ctx, `SELECT `+orderColumns+` FROM orders WHERE user_id = $1 AND direction = $2`, userID, direction)
	if err != nil {
		return nil, err
	}
	return scanOrders(rows)
}

// FindByUserIDIn mirrors OrderRepository.findByUserIdIn(Set<Long>). An empty set
// returns no rows (matching JPA's empty-IN short-circuit, no DB round-trip).
func (r *Repository) FindByUserIDIn(ctx context.Context, q Querier, userIDs []int64) ([]Order, error) {
	if len(userIDs) == 0 {
		return []Order{}, nil
	}
	rows, err := q.Query(ctx, `SELECT `+orderColumns+` FROM orders WHERE user_id = ANY($1)`, userIDs)
	if err != nil {
		return nil, err
	}
	return scanOrders(rows)
}

// FindTransactionsByOrderIDsAndTimestampBetween mirrors
// TransactionRepository.findByOrderIdInAndTimestampBetween. Spring Data "Between"
// is INCLUSIVE on both ends (BETWEEN start AND end); the precise half-open
// [start,end) gain window is re-applied by allocateSellTaxLots. Empty ids -> none.
func (r *Repository) FindTransactionsByOrderIDsAndTimestampBetween(ctx context.Context, q Querier, orderIDs []int64, start, end time.Time) ([]Transaction, error) {
	if len(orderIDs) == 0 {
		return []Transaction{}, nil
	}
	rows, err := q.Query(ctx, `SELECT `+transactionColumns+` FROM transactions WHERE order_id = ANY($1) AND timestamp BETWEEN $2 AND $3`, orderIDs, start, end)
	if err != nil {
		return nil, err
	}
	return scanTransactions(rows)
}

// BankHeldBuyQuantity mirrors OrderRepository.bankHeldBuyQuantity (WP-14
// dividend bank-held split): the sum of EXECUTED quantity (quantity -
// remaining_portions) across the holder's PurchaseFor.BANK BUY orders for one
// listing. Untouched orders contribute 0 (remaining_portions == quantity). The
// caller clamps to [0, holder.quantity].
func (r *Repository) BankHeldBuyQuantity(ctx context.Context, q Querier, userID, listingID int64) (int64, error) {
	var sum int64
	err := q.QueryRow(ctx, `
		SELECT COALESCE(SUM(quantity - remaining_portions), 0) FROM orders
		WHERE user_id = $1 AND listing_id = $2 AND purchase_for = 'BANK' AND direction = 'BUY'`,
		userID, listingID).Scan(&sum)
	return sum, err
}

// FindTransactionsByOrderID mirrors TransactionRepository.findByOrderId — all
// recorded fills for one order (the enriched My Orders executionPrice).
func (r *Repository) FindTransactionsByOrderID(ctx context.Context, q Querier, orderID int64) ([]Transaction, error) {
	rows, err := q.Query(ctx, `SELECT `+transactionColumns+` FROM transactions WHERE order_id = $1`, orderID)
	if err != nil {
		return nil, err
	}
	return scanTransactions(rows)
}

// FindTransactionsByOrderIDsAndTimestampBefore mirrors
// TransactionRepository.findByOrderIdInAndTimestampBefore (timestamp < end).
func (r *Repository) FindTransactionsByOrderIDsAndTimestampBefore(ctx context.Context, q Querier, orderIDs []int64, end time.Time) ([]Transaction, error) {
	if len(orderIDs) == 0 {
		return []Transaction{}, nil
	}
	rows, err := q.Query(ctx, `SELECT `+transactionColumns+` FROM transactions WHERE order_id = ANY($1) AND timestamp < $2`, orderIDs, end)
	if err != nil {
		return nil, err
	}
	return scanTransactions(rows)
}
