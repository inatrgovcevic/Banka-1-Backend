package otc

import (
	"context"
	"time"

	"banka1/trading-service-go/internal/clients"
	"banka1/trading-service-go/internal/portfolio"

	gpdb "banka1/go-platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// otcRepo abstracts *Repository so the Service can be unit-tested with a stub
// (no Postgres). *Repository satisfies this interface.
type otcRepo interface {
	Pool() *pgxpool.Pool
	InsertOffer(ctx context.Context, q Querier, o *OtcOffer) error
	FindOfferByID(ctx context.Context, q Querier, id int64) (*OtcOffer, error)
	FindOfferByIDForUpdate(ctx context.Context, q Querier, id int64) (*OtcOffer, error)
	UpdateOffer(ctx context.Context, q Querier, o *OtcOffer) error
	FindActiveOffersForUser(ctx context.Context, userID int64) ([]OtcOffer, error)
	InsertOptionContract(ctx context.Context, q Querier, c *OptionContract) error
	FindOptionContractByID(ctx context.Context, q Querier, id int64) (*OptionContract, error)
	FindOptionContractByIDForUpdate(ctx context.Context, q Querier, id int64) (*OptionContract, error)
	UpdateOptionContractStatus(ctx context.Context, q Querier, id int64, status string) error
	SetOptionContractExercisedAt(ctx context.Context, q Querier, id int64, exercisedAt time.Time) error
	SumActiveBySellerAndTicker(ctx context.Context, q Querier, sellerID int64, ticker string) (int64, error)
	FindContractsByBuyerIDAndStatus(ctx context.Context, buyerID int64, status string) ([]OptionContract, error)
	FindContractsBySellerIDAndStatus(ctx context.Context, sellerID int64, status string) ([]OptionContract, error)
	FindContractsByStatusAndSettlementDateBefore(ctx context.Context, status string, before time.Time) ([]OptionContract, error)
	FindContractsByStatusAndSettlementDate(ctx context.Context, status string, date time.Time) ([]OptionContract, error)
	InsertExpiryReminderIfAbsent(ctx context.Context, q Querier, contractID int64, reminderDays int) (bool, error)
	InsertHistory(ctx context.Context, q Querier, h *NegotiationHistory) error
	HistoryForUser(ctx context.Context, userID int64, status *string, otherPartyID *int64, dateFrom, dateTo *time.Time) ([]NegotiationHistory, error)
}

// otcPortfolioRepo abstracts the subset of *portfolio.Repository the OTC Service
// uses. *portfolio.Repository satisfies this interface.
type otcPortfolioRepo interface {
	Pool() *pgxpool.Pool
	FindByUserID(ctx context.Context, q portfolio.Querier, userID int64) ([]portfolio.Portfolio, error)
	FindByID(ctx context.Context, q portfolio.Querier, id int64) (*portfolio.Portfolio, error)
	FindByUserIDAndListingID(ctx context.Context, q portfolio.Querier, userID, listingID int64) (*portfolio.Portfolio, error)
	UpdatePublic(ctx context.Context, q portfolio.Querier, id int64, publicQuantity int, isPublic bool) error
	UpdateReservedAndPublic(ctx context.Context, q portfolio.Querier, id int64, reserved, public int) error
	FindAllPublicStocks(ctx context.Context, q portfolio.Querier) ([]portfolio.Portfolio, error)
}

// marketLister abstracts the MarketClient ticker lookup. *clients.MarketClient
// satisfies this interface.
type marketLister interface {
	GetListing(ctx context.Context, id int64) (*clients.StockListing, error)
}

// customerLookup abstracts the CustomerClient name lookup.
type customerLookup interface {
	GetCustomer(ctx context.Context, id int64) (*clients.Customer, error)
}

// employeeLookup abstracts the EmployeeClient actuary-ids lookup.
type employeeLookup interface {
	ActuaryClientIDs(ctx context.Context) []int64
}

// txRunner runs fn inside a transaction. In production it is gpdb.RunInTx over a
// real pool; tests substitute a fake that calls fn(nil).
type txRunner func(ctx context.Context, fn func(pgx.Tx) error) error

// poolTxRunner adapts gpdb.RunInTx over a concrete pool into a txRunner.
func poolTxRunner(pool *pgxpool.Pool) txRunner {
	return func(ctx context.Context, fn func(pgx.Tx) error) error {
		return gpdb.RunInTx(ctx, pool, pgx.TxOptions{}, fn)
	}
}

// qRunner runs fn with a Querier inside a transaction. In production it wraps
// gpdb.RunInTx and hands the pgx.Tx (which satisfies Querier); tests substitute
// a fake that calls fn(fakeQuerier).
type qRunner func(ctx context.Context, fn func(reservationQuerier) error) error

// reservationQuerier is both an otc Querier and a portfolio.Querier — pgx.Tx and
// the test fake both satisfy it.
type reservationQuerier interface {
	Querier
}

// poolQRunner adapts gpdb.RunInTx into a qRunner (the pgx.Tx is the Querier).
func poolQRunner(pool *pgxpool.Pool) qRunner {
	return func(ctx context.Context, fn func(reservationQuerier) error) error {
		return gpdb.RunInTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
			return fn(tx)
		})
	}
}
