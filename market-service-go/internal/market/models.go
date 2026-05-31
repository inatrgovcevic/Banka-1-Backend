package market

import (
	"time"

	"github.com/shopspring/decimal"
)

type ListingType string

const (
	ListingTypeStock   ListingType = "STOCK"
	ListingTypeFutures ListingType = "FUTURES"
	ListingTypeForex   ListingType = "FOREX"
)

type MarketPhase string

const (
	MarketPhaseClosed        MarketPhase = "CLOSED"
	MarketPhasePreMarket     MarketPhase = "PRE_MARKET"
	MarketPhaseRegularMarket MarketPhase = "REGULAR_MARKET"
	MarketPhasePostMarket    MarketPhase = "POST_MARKET"
)

type OptionType string

const (
	OptionTypeCall OptionType = "CALL"
	OptionTypePut  OptionType = "PUT"
)

type StockExchange struct {
	ID                 int64
	ExchangeName       string
	ExchangeAcronym    string
	ExchangeMICCode    string
	Polity             string
	Currency           string
	TimeZone           string
	OpenTime           string
	CloseTime          string
	PreMarketOpenTime  *string
	PreMarketCloseTime *string
	PostMarketOpenTime *string
	PostMarketCloseTime *string
	IsActive           bool
}

type Listing struct {
	ID              int64
	SecurityID      int64
	ListingType     ListingType
	StockExchangeID int64
	Ticker          string
	Name            string
	ExchangeMICCode string
	LastRefresh     time.Time
	Price           string
	Ask             string
	Bid             string
	Change          string
	Volume          int64
	Currency        string
	SettlementDate  *time.Time
}

type DailyPriceInfo struct {
	Date          time.Time
	Price         string
	Ask           string
	Bid           string
	Change        string
	ChangePercent string
	Volume        int64
	DollarVolume  string
}

type StockDetails struct {
	OutstandingShares int64
	DividendYield     string
	ContractSize      int32
}

type FuturesDetails struct {
	ContractSize   int32
	ContractUnit   string
	SettlementDate *time.Time
}

type ForexDetails struct {
	BaseCurrency  string
	QuoteCurrency string
	ExchangeRate  string
	Liquidity     string
	ContractSize  int32
}

type OptionDetails struct {
	ID                int64
	Ticker            string
	OptionType        OptionType
	StrikePrice       string
	ImpliedVolatility string
	OpenInterest      int32
	InTheMoney        bool
}

type OptionSettlementGroup struct {
	SettlementDate time.Time
	Calls          []OptionDetails
	Puts           []OptionDetails
}

type StockPriceSnapshot struct {
	Ticker        string          `json:"ticker"`
	CurrentPrice  decimal.Decimal `json:"currentPrice"`
	OpenPrice     decimal.Decimal `json:"openPrice"`
	PreviousClose decimal.Decimal `json:"previousClose"`
	ChangePercent decimal.Decimal `json:"changePercent"`
	Volume        int64           `json:"volume"`
	Currency      string          `json:"currency"`
	Timestamp     time.Time       `json:"timestamp"`
}
