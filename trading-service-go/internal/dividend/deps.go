package dividend

import (
	"context"
	"time"

	"banka1/trading-service-go/internal/clients"
	"banka1/trading-service-go/internal/portfolio"

	gpdb "banka1/go-platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// dividendRepo abstracts *Repository so Service can be tested without a real DB.
type dividendRepo interface {
	Pool() *pgxpool.Pool
	Insert(ctx context.Context, q Querier, p *Payout) error
	ExistsForDate(ctx context.Context, q Querier, userID, listingID int64, paymentDate time.Time, forBank bool) (bool, error)
	FindByUserID(ctx context.Context, q Querier, userID int64) ([]Payout, error)
	FindByUserIDAndListingID(ctx context.Context, q Querier, userID, listingID int64) ([]Payout, error)
}

// dividendPortfolios abstracts portfolio.Repository for Distribute.
type dividendPortfolios interface {
	Pool() *pgxpool.Pool
	FindStockHoldersByListingID(ctx context.Context, q portfolio.Querier, listingID int64) ([]portfolio.Portfolio, error)
}

// dividendMarket abstracts MarketClient methods used by Service.
type dividendMarket interface {
	FetchDividendData(ctx context.Context) []clients.DividendData
	ConvertNoCommission(ctx context.Context, amount decimal.Decimal, from, to string) (decimal.Decimal, bool)
}

// dividendAccount abstracts AccountClient methods used by Service.
type dividendAccount interface {
	GetBankRsdOwnerAccount(ctx context.Context) *clients.OwnerAccount
	GetStateRsdOwnerAccount(ctx context.Context) *clients.OwnerAccount
	GetAccountInCurrency(ctx context.Context, ownerID int64, currency string) *clients.OwnerAccount
	GetDefaultRsdAccountNumberForOwner(ctx context.Context, ownerID int64) string
	CreditAccount(ctx context.Context, accountNumber string, amount decimal.Decimal, ownerID int64) error
}

// bankHeldFn returns the bank-held BUY quantity for a user+listing inside a tx.
// A function field avoids the cross-package Querier type mismatch between
// order.Querier (same methods) and dividend.Querier.
type bankHeldFn func(ctx context.Context, q Querier, userID, listingID int64) (int64, error)

// txRunner runs fn inside a single transaction. Production uses gpdb.RunInTx
// over the repo pool; tests inject a fake that calls fn(nil).
type txRunner func(ctx context.Context, fn func(pgx.Tx) error) error

func poolTxRunner(pool *pgxpool.Pool) txRunner {
	return func(ctx context.Context, fn func(pgx.Tx) error) error {
		return gpdb.RunInTx(ctx, pool, pgx.TxOptions{}, fn)
	}
}
