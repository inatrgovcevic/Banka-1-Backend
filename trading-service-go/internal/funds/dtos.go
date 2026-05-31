package funds

import (
	"time"

	"banka1/trading-service-go/internal/api"

	"github.com/shopspring/decimal"
)

// These are the service-layer DTOs returned to the HTTP handler. JSON tags are
// the public field names — they match the Java funds DTO field names exactly
// (camelCase) and the frontend depends on them being byte-compatible.

// FundDto mirrors InvestmentFundDto.
type FundDto struct {
	ID                       int64            `json:"id"`
	Naziv                    string           `json:"naziv"`
	Opis                     *string          `json:"opis"`
	MinimumContribution      decimal.Decimal  `json:"minimumContribution"`
	ManagerID                int64            `json:"managerId"`
	ManagerIme               *string          `json:"managerIme"`
	ManagerPrezime           *string          `json:"managerPrezime"`
	LikvidnaSredstva         decimal.Decimal  `json:"likvidnaSredstva"`
	AccountID                *int64           `json:"accountId"`
	AccountNumber            string           `json:"accountNumber"`
	DividendStrategy         string           `json:"dividendStrategy"`
	DatumKreiranja           api.LocalDate    `json:"datumKreiranja"`
	TotalValue               decimal.Decimal  `json:"totalValue"`
	Profit                   decimal.Decimal  `json:"profit"`
	AnnualizedReturn         *decimal.Decimal `json:"annualizedReturn"`
	RewardToVariabilityRatio *decimal.Decimal `json:"rewardToVariabilityRatio"`
	MaxDrawdown              *decimal.Decimal `json:"maxDrawdown"`
	Volatility               *decimal.Decimal `json:"volatility"`
}

// PositionDto mirrors ClientFundPositionDto.
type PositionDto struct {
	ID                   int64             `json:"id"`
	ClientID             int64             `json:"clientId"`
	FundID               int64             `json:"fundId"`
	FundNaziv            string            `json:"fundNaziv"`
	FundOpis             *string           `json:"fundOpis"`
	FundTotalValue       decimal.Decimal   `json:"fundTotalValue"`
	TotalInvested        decimal.Decimal   `json:"totalInvested"`
	PercentageOfFund     decimal.Decimal   `json:"percentageOfFund"`
	CurrentPositionValue decimal.Decimal   `json:"currentPositionValue"`
	ClientProfit         decimal.Decimal   `json:"clientProfit"`
	FirstInvestedAt      api.LocalDateTime `json:"firstInvestedAt"`
	LastModifiedAt       api.LocalDateTime `json:"lastModifiedAt"`
}

// HoldingDto mirrors FundHoldingDto.
type HoldingDto struct {
	ID                int64             `json:"id"`
	Ticker            string            `json:"ticker"`
	Quantity          int               `json:"quantity"`
	AvgUnitPrice      decimal.Decimal   `json:"avgUnitPrice"`
	InitialMarginCost decimal.Decimal   `json:"initialMarginCost"`
	Price             *decimal.Decimal  `json:"price"`
	Change            *decimal.Decimal  `json:"change"`
	Volume            int64             `json:"volume"`
	AcquisitionDate   api.LocalDateTime `json:"acquisitionDate"`
}

// FundPerformancePoint mirrors FundPerformancePointDto. transactionId/amount/
// inflow/status are nil when the point is the "fund creation" placeholder
// (no transactions yet).
type FundPerformancePoint struct {
	Timestamp     time.Time        `json:"timestamp"`
	TransactionID *int64           `json:"transactionId,omitempty"`
	Amount        *decimal.Decimal `json:"amount,omitempty"`
	Inflow        *bool            `json:"inflow,omitempty"`
	Status        *string          `json:"status,omitempty"`
	TotalValue    decimal.Decimal  `json:"totalValue"`
	Profit        decimal.Decimal  `json:"profit"`
}

// FundAnalytics mirrors FundDetailsAnalyticsDto.
type FundAnalytics struct {
	Fund                         FundDto                      `json:"fund"`
	Metrics                      Metrics                      `json:"metrics"`
	HistoricalValuePoints        []FundValueSnapshot          `json:"historicalValuePoints"`
	AverageFundPerformancePoints []PerformanceComparisonPoint `json:"averageFundPerformancePoints"`
}

// FundValueSnapshotPointDto carries only the public fields (drops id/created_at
// to match Java FundValueSnapshotPointDto). Used in transit by manual JSON
// projection in handlers. Kept here so the handler does not import the entity
// shape inadvertently.
type FundValueSnapshotPointDto struct {
	SnapshotDate   time.Time       `json:"snapshotDate"`
	LiquidityValue decimal.Decimal `json:"liquidityValue"`
	HoldingsValue  decimal.Decimal `json:"holdingsValue"`
	TotalValue     decimal.Decimal `json:"totalValue"`
}

// PerformanceComparisonPointDto is the public name for the per-day comparison
// row. Mirrors FundPerformanceComparisonPointDto.
type PerformanceComparisonPointDto struct {
	SnapshotDate            time.Time        `json:"snapshotDate"`
	FundPerformanceIndex    *decimal.Decimal `json:"fundPerformanceIndex"`
	AveragePerformanceIndex *decimal.Decimal `json:"averagePerformanceIndex"`
}

// DividendDistributionDto mirrors FundDividendDistributionDto.
type DividendDistributionDto struct {
	ID                   int64               `json:"id"`
	FundID               int64               `json:"fundId"`
	StockTicker          string              `json:"stockTicker"`
	PaymentDate          api.LocalDate       `json:"paymentDate"`
	DividendPerShare     decimal.Decimal     `json:"dividendPerShare"`
	SourceCurrency       string              `json:"sourceCurrency"`
	HoldingQuantity      int                 `json:"holdingQuantity"`
	GrossAmountSource    decimal.Decimal     `json:"grossAmountSource"`
	GrossAmountRsd       decimal.Decimal     `json:"grossAmountRsd"`
	Strategy             string              `json:"strategy"`
	Status               string              `json:"status"`
	ReinvestedShares     *int                `json:"reinvestedShares"`
	ReinvestedAmountRsd  *decimal.Decimal    `json:"reinvestedAmountRsd"`
	DistributedAmountRsd *decimal.Decimal    `json:"distributedAmountRsd"`
	Note                 *string             `json:"note"`
	ProcessedAt          api.LocalDateTime   `json:"processedAt"`
	Payouts              []DividendPayoutDto `json:"payouts"`
}

// ClientFundTransactionDto mirrors the ClientFundTransaction entity as the Java
// controllers serialize it (camelCase field names; occurredAt is a Jackson
// LocalDateTime — no zone). The handler returns this instead of the raw
// repository struct (whose Go field names would leak as PascalCase JSON keys).
type ClientFundTransactionDto struct {
	ID                  int64             `json:"id"`
	ClientID            int64             `json:"clientId"`
	FundID              int64             `json:"fundId"`
	Amount              decimal.Decimal   `json:"amount"`
	Inflow              bool              `json:"inflow"`
	Status              string            `json:"status"`
	OccurredAt          api.LocalDateTime `json:"occurredAt"`
	ClientAccountNumber string            `json:"clientAccountNumber"`
	FailureReason       *string           `json:"failureReason"`
}

// DividendPayoutDto mirrors FundDividendPayoutDto.
type DividendPayoutDto struct {
	ClientID            int64           `json:"clientId"`
	ClientAccountNumber *string         `json:"clientAccountNumber"`
	OwnershipRatio      decimal.Decimal `json:"ownershipRatio"`
	AmountRsd           decimal.Decimal `json:"amountRsd"`
	Status              string          `json:"status"`
	FailureReason       *string         `json:"failureReason"`
}
