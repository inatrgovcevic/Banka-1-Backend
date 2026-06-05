package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Contract status constants — mirror Java NegotiationContractStatus enum.
const (
	ContractStatusPendingPremium = "PENDING_PREMIUM" // created; premium 2PC not yet committed
	ContractStatusActive         = "ACTIVE"          // premium received; option is live
	ContractStatusExercised      = "EXERCISED"       // option exercised successfully
	ContractStatusExpired        = "EXPIRED"         // settlement_date passed without exercise
	ContractStatusReleased       = "RELEASED"        // cancelled before expiry
)

// Contract mirrors the interbank_contracts row.
// Column notes (verified against migration 20260524000001):
//   - Price columns are strike_currency / strike_amount (NOT price_currency/price_amount)
//     because the SQL DDL uses the options-domain terminology "strike price".
//   - There are NO premium_* columns in this table — premiums are tracked in the
//     negotiation row and via the 2PC transaction.
//   - option_pseudo_owner_routing/id record which party conceptually "owns" the option
//     (the buyer holds the right; stored so the 2PC executor knows who to credit on exercise).
//   - No updated_at; exercised_at and expired_at are set on status transitions.
type Contract struct {
	ID                     string
	NegotiationID          string
	BuyerRouting           int
	BuyerID                string
	SellerRouting          int
	SellerID               string
	StockTicker            string
	Amount                 int
	StrikeCurrency         string
	StrikeAmount           decimal.Decimal
	SettlementDate         time.Time
	Status                 string
	OptionPseudoOwnerRouting int
	OptionPseudoOwnerID    string
	Version                int64
	CreatedAt              time.Time
	ExercisedAt            *time.Time
	ExpiredAt              *time.Time
}

type ContractStore struct{ pool querier }

func NewContractStore(pool *pgxpool.Pool) *ContractStore { return &ContractStore{pool: pool} }

const contractSelectCols = `
	id, negotiation_id, buyer_routing_number, buyer_id, seller_routing_number, seller_id,
	stock_ticker, amount, strike_currency, strike_amount, settlement_date, status,
	option_pseudo_owner_routing, option_pseudo_owner_id,
	version, created_at, exercised_at, expired_at`

func scanContract(row pgx.Row) (*Contract, error) {
	var c Contract
	err := row.Scan(
		&c.ID, &c.NegotiationID, &c.BuyerRouting, &c.BuyerID, &c.SellerRouting, &c.SellerID,
		&c.StockTicker, &c.Amount, &c.StrikeCurrency, &c.StrikeAmount, &c.SettlementDate, &c.Status,
		&c.OptionPseudoOwnerRouting, &c.OptionPseudoOwnerID,
		&c.Version, &c.CreatedAt, &c.ExercisedAt, &c.ExpiredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Insert writes a new contract row, populating CreatedAt.
func (s *ContractStore) Insert(ctx context.Context, c *Contract) error {
	return s.pool.QueryRow(ctx, `
		INSERT INTO interbank_contracts
			(id, negotiation_id, buyer_routing_number, buyer_id, seller_routing_number, seller_id,
			 stock_ticker, amount, strike_currency, strike_amount, settlement_date, status,
			 option_pseudo_owner_routing, option_pseudo_owner_id, version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,0)
		RETURNING created_at`,
		c.ID, c.NegotiationID, c.BuyerRouting, c.BuyerID, c.SellerRouting, c.SellerID,
		c.StockTicker, c.Amount, c.StrikeCurrency, c.StrikeAmount, c.SettlementDate, c.Status,
		c.OptionPseudoOwnerRouting, c.OptionPseudoOwnerID,
	).Scan(&c.CreatedAt)
}

// FindByNegotiationID returns (nil, nil) if not found.
func (s *ContractStore) FindByNegotiationID(ctx context.Context, negotiationID string) (*Contract, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+contractSelectCols+` FROM interbank_contracts WHERE negotiation_id = $1`, negotiationID)
	return scanContract(row)
}

// FindByID returns (nil, nil) if not found.
func (s *ContractStore) FindByID(ctx context.Context, id string) (*Contract, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+contractSelectCols+` FROM interbank_contracts WHERE id = $1`, id)
	return scanContract(row)
}

// SumActiveBySellerAndTicker returns the total stock quantity reserved across
// ACTIVE inter-bank contracts where the seller matches (sellerRouting, sellerID)
// AND ticker matches. Used by GET /public-stock to subtract the
// committed-but-not-exercised inventory from the advertised quantity.
func (s *ContractStore) SumActiveBySellerAndTicker(ctx context.Context, sellerRouting int, sellerID, ticker string) (int, error) {
	var sum int
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM interbank_contracts
		WHERE seller_routing_number = $1
		  AND seller_id             = $2
		  AND stock_ticker          = $3
		  AND status                = 'ACTIVE'`,
		sellerRouting, sellerID, ticker).Scan(&sum)
	return sum, err
}

// UpdateStatus flips a contract's status (e.g. ACTIVE→EXERCISED or →EXPIRED).
// For EXERCISED transitions, exercised_at is set to now(); for EXPIRED, expired_at.
// Other transitions leave those columns NULL.
func (s *ContractStore) UpdateStatus(ctx context.Context, id, status string) error {
	var err error
	switch status {
	case ContractStatusExercised:
		_, err = s.pool.Exec(ctx, `
			UPDATE interbank_contracts
			SET status       = $1,
			    exercised_at = now(),
			    version      = version + 1
			WHERE id = $2`, status, id)
	case ContractStatusExpired:
		_, err = s.pool.Exec(ctx, `
			UPDATE interbank_contracts
			SET status    = $1,
			    expired_at = now(),
			    version   = version + 1
			WHERE id = $2`, status, id)
	default:
		_, err = s.pool.Exec(ctx, `
			UPDATE interbank_contracts
			SET status  = $1,
			    version = version + 1
			WHERE id = $2`, status, id)
	}
	return err
}
