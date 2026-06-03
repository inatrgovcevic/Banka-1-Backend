// Package funds serves the /funds (+ /funds/internal) endpoints, the
// FUND_SUBSCRIBE / FUND_REDEEM / FUND_LIQUIDATION_FOR_REDEMPTION saga publisher
// and consumer, the daily fund-value snapshot scheduler, and the dividend
// distribution flow. It mirrors trading-service com.banka1.tradingservice.funds.*
// over the trading-service-owned tables `investment_funds`,
// `client_fund_positions`, `client_fund_transactions`, `fund_holdings`,
// `fund_value_snapshots`, `fund_dividend_distributions`, `fund_dividend_payouts`.
// The `trading` schema is owned by Java Liquibase; this service runs no
// migrations. NUMERIC is read as ::text into shopspring/decimal to preserve
// scale (matching P3 order and P4 tax). All money sits at scale 2 RSD; prices
// at scale 4; dividend per-share / ownership ratios at scale 8.
package funds

import (
	"bytes"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

// BankInvestorID is the reserved client id that represents the bank as
// investor in a fund (mirrors InvestmentFundService.BANK_INVESTOR_ID = -1L).
const BankInvestorID int64 = -1

// Base currencies. The fund books in RSD; holding prices come back from market
// in USD. Mirrors trading-service FundHoldingService constants.
const (
	FundBaseCurrency     = "RSD"
	HoldingPriceCurrency = "USD"
)

// ClientFundTransactionStatus values mirror the Java enum.
const (
	TxStatusPending   = "PENDING"
	TxStatusCompleted = "COMPLETED"
	TxStatusFailed    = "FAILED"
)

// FundDividendStrategy values mirror the Java enum.
const (
	DividendReinvest      = "REINVEST"
	DividendPayoutClients = "PAYOUT_CLIENTS"
)

// FundDividendDistributionStatus values mirror the Java enum.
const (
	DistStatusCompleted             = "COMPLETED"
	DistStatusCompletedWithWarnings = "COMPLETED_WITH_WARNINGS"
)

// FundDividendPayoutStatus values mirror the Java enum.
const (
	PayoutStatusCompleted = "COMPLETED"
	PayoutStatusFailed    = "FAILED"
	PayoutStatusSkipped   = "SKIPPED"
)

// SortField mirrors funds dto FundSortField. Sent by the discovery query
// parameter; defaults to NAME ASC when omitted.
const (
	SortByName                   = "NAME"
	SortByTotalValue             = "TOTAL_VALUE"
	SortByProfit                 = "PROFIT"
	SortByAnnualizedReturn       = "ANNUALIZED_RETURN"
	SortByRewardToVariabilityRat = "REWARD_TO_VARIABILITY_RATIO"
	SortByMaxDrawdown            = "MAX_DRAWDOWN"
	SortByVolatility             = "VOLATILITY"
)

// SortDirection values mirror Spring Sort.Direction.
const (
	SortAsc  = "ASC"
	SortDesc = "DESC"
)

// InvestmentFund mirrors a row of `investment_funds`.
type InvestmentFund struct {
	ID                  int64
	Naziv               string
	Opis                *string
	MinimumContribution decimal.Decimal
	ManagerID           int64
	LikvidnaSredstva    decimal.Decimal
	AccountNumber       string
	DividendStrategy    string
	DatumKreiranja      time.Time
	Deleted             bool
	CreatedAt           time.Time
	Version             int64
}

// ClientFundPosition mirrors a row of `client_fund_positions`. Exactly one row
// per (client_id, fund_id) pair (UNIQUE constraint
// uk_client_fund_position_client_fund).
type ClientFundPosition struct {
	ID              int64
	ClientID        int64
	FundID          int64
	TotalInvested   decimal.Decimal
	FirstInvestedAt time.Time
	LastModifiedAt  *time.Time
	Version         int64
}

// ClientFundTransaction mirrors a row of `client_fund_transactions`. inflow=
// true is an investment, false is a redemption. Status: PENDING → COMPLETED
// (saga ok) or FAILED (saga rolled back).
type ClientFundTransaction struct {
	ID                  int64
	ClientID            int64
	FundID              int64
	Amount              decimal.Decimal
	Inflow              bool
	Status              string
	OccurredAt          time.Time
	ClientAccountNumber string
	FailureReason       *string
}

// FundHolding mirrors a row of `fund_holdings`. avgUnitPrice is the weighted-
// average historical purchase price (USD) — the holding lives at scale 4 per
// the column definition. On full liquidation the row stays with quantity=0 +
// deleted=true (soft delete).
type FundHolding struct {
	ID           int64
	FundID       int64
	StockTicker  string
	Quantity     int
	AvgUnitPrice decimal.Decimal
	Deleted      bool
	CreatedAt    time.Time
	UpdatedAt    *time.Time
	Version      int64
}

// FundValueSnapshot mirrors a row of `fund_value_snapshots`. Daily capture of
// (liquidity, holdings, total) per fund (UNIQUE constraint
// uk_fund_value_snapshot_fund_date).
type FundValueSnapshot struct {
	ID             int64
	FundID         int64
	SnapshotDate   time.Time
	LiquidityValue decimal.Decimal
	HoldingsValue  decimal.Decimal
	TotalValue     decimal.Decimal
	CreatedAt      time.Time
}

// FundDividendDistribution mirrors a row of `fund_dividend_distributions`. The
// gross dividend amount is captured at both source currency (scale 8) and RSD
// (scale 2) for parity. Idempotency key: (fund_id, stock_ticker, payment_date).
type FundDividendDistribution struct {
	ID                   int64
	FundID               int64
	StockTicker          string
	PaymentDate          time.Time
	DividendPerShare     decimal.Decimal
	SourceCurrency       string
	HoldingQuantity      int
	GrossAmountSource    decimal.Decimal
	GrossAmountRsd       decimal.Decimal
	Strategy             string
	Status               string
	ReinvestedShares     *int
	ReinvestedAmountRsd  *decimal.Decimal
	DistributedAmountRsd *decimal.Decimal
	Note                 *string
	ProcessedAt          time.Time
}

// FundDividendPayout mirrors a row of `fund_dividend_payouts`. Per-client split
// row produced when distribution.strategy = PAYOUT_CLIENTS. Idempotency key:
// (distribution_id, client_id).
type FundDividendPayout struct {
	ID                  int64
	DistributionID      int64
	ClientID            int64
	ClientAccountNumber *string
	OwnershipRatio      decimal.Decimal
	AmountRsd           decimal.Decimal
	Status              string
	FailureReason       *string
	CreatedAt           time.Time
}

// --- Saga events (publish, on saga.events) ---------------------------------

// FundSubscribeRequestedEvent mirrors
// InvestmentFundService.FundSubscribeRequestedEvent. Published on
// saga.events / fund.subscribe.requested AFTER the local transaction commits.
type FundSubscribeRequestedEvent struct {
	TransactionID     string          `json:"transactionId"`
	ClientID          int64           `json:"clientId"`
	FundID            int64           `json:"fundId"`
	Amount            decimal.Decimal `json:"amount"`
	FromAccountNumber string          `json:"fromAccountNumber"`
	FundAccountNumber string          `json:"fundAccountNumber"`
}

// FundRedeemRequestedEvent mirrors
// InvestmentFundService.FundRedeemRequestedEvent. Published on saga.events /
// fund.redeem.requested when the fund has enough liquidity, or /
// fund.redeem.with-liquidation.requested when holdings must be liquidated
// first. The `liquidEnough` flag echoes the routing decision so saga-
// orchestrator-service does not have to recompute it.
type FundRedeemRequestedEvent struct {
	TransactionID     string          `json:"transactionId"`
	ClientID          int64           `json:"clientId"`
	FundID            int64           `json:"fundId"`
	Amount            decimal.Decimal `json:"amount"`
	ToAccountNumber   string          `json:"toAccountNumber"`
	FundAccountNumber string          `json:"fundAccountNumber"`
	LiquidEnough      bool            `json:"liquidEnough"`
}

// SagaResultEvent is the loose-shape payload the consumers receive on
// saga.exchange. Java listeners deserialize as Map<String,Object>, so the Go
// port treats every value as optional. Numeric ids may arrive as int64 or
// float64 depending on the publisher; the parser handles both.
type SagaResultEvent struct {
	TransactionID *FlexibleInt64   `json:"transactionId,omitempty"`
	ClientID      *int64           `json:"clientId,omitempty"`
	FundID        *int64           `json:"fundId,omitempty"`
	Amount        *decimal.Decimal `json:"amount,omitempty"`
	FailureReason *string          `json:"failureReason,omitempty"`
}

// FlexibleInt64 accepts ids encoded either as JSON numbers or strings.
type FlexibleInt64 int64

func (v *FlexibleInt64) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		return nil
	}
	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		unquoted, err := strconv.Unquote(string(data))
		if err != nil {
			return err
		}
		parsed, err := strconv.ParseInt(unquoted, 10, 64)
		if err != nil {
			return err
		}
		*v = FlexibleInt64(parsed)
		return nil
	}
	parsed, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*v = FlexibleInt64(parsed)
	return nil
}

func (v FlexibleInt64) Int64() int64 {
	return int64(v)
}
