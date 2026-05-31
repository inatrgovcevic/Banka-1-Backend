package funds

import (
	"context"

	"github.com/shopspring/decimal"
)

// OrderCallback is the in-process call surface order execution uses when an
// INVESTMENT_FUND BUY portion settles. Mirrors the Java order-service
// TradingServiceClient (which HTTP POSTs to /funds/internal/{id}/holdings/add
// and /funds/internal/{id}/liquidity/debit). Since trading-service-go is the
// SAME consolidated process that owns both order and funds, we expose this as
// a Go interface and bind the funds Service to it in NewApp — no HTTP self-
// call, no token mint, no network hop.
//
// The interface lives in the funds package so order depends ONLY on this
// narrow surface (the dependency direction is order → funds, mirroring how
// TaxReporter lives in portfolio for the tax→portfolio one-way coupling).
type OrderCallback interface {
	// AddHolding mirrors POST /funds/internal/{fundId}/holdings/add. Called
	// from execution.go inside the order tx AFTER an INVESTMENT_FUND BUY
	// portion settles. quantity is the just-bought share count; unitPrice is
	// the execution price (USD, matches the holding column's currency basis).
	AddHolding(ctx context.Context, fundID int64, ticker string, quantity int, unitPrice decimal.Decimal) error

	// DebitLiquidity mirrors POST /funds/internal/{fundId}/liquidity/debit.
	// Called when the fund's RSD account is the funding source for a buy.
	// reason is a free-text audit string (matches the Java request body).
	DebitLiquidity(ctx context.Context, fundID int64, amount decimal.Decimal, reason string) error
}

// ServiceCallback adapts *Service to OrderCallback. AddHolding routes to
// HoldingService.AddOrUpdate; DebitLiquidity routes to Service.DebitLiquidity.
type ServiceCallback struct {
	svc     *Service
	holding *HoldingService
}

// NewOrderCallback wires the OrderCallback over an existing funds Service +
// HoldingService. The two are colocated in App so the wiring is trivial.
func NewOrderCallback(svc *Service, holding *HoldingService) *ServiceCallback {
	return &ServiceCallback{svc: svc, holding: holding}
}

// AddHolding implements OrderCallback. Records a snapshot after the holding
// update so the daily history captures the post-buy state — mirrors the Java
// FundLiquidationController.addHolding's snapshot.recordSnapshot call.
func (c *ServiceCallback) AddHolding(ctx context.Context, fundID int64, ticker string, quantity int, unitPrice decimal.Decimal) error {
	if _, err := c.holding.AddOrUpdate(ctx, nil, fundID, ticker, quantity, unitPrice); err != nil {
		return err
	}
	c.svc.snapshots.RecordSilently(ctx, fundID)
	return nil
}

// DebitLiquidity implements OrderCallback.
func (c *ServiceCallback) DebitLiquidity(ctx context.Context, fundID int64, amount decimal.Decimal, reason string) error {
	return c.svc.DebitLiquidity(ctx, fundID, amount, reason)
}
