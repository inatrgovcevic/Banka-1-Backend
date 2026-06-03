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
	ListingTypeOption  ListingType = "OPTION"
)

type PriceAlertCondition string

const (
	PriceAlertAbove           PriceAlertCondition = "ABOVE"
	PriceAlertBelow           PriceAlertCondition = "BELOW"
	PriceAlertPctDropIntraday PriceAlertCondition = "PCT_DROP_INTRADAY"
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
	ID                  int64
	ExchangeName        string
	ExchangeAcronym     string
	ExchangeMICCode     string
	Polity              string
	Currency            string
	TimeZone            string
	OpenTime            string
	CloseTime           string
	PreMarketOpenTime   *string
	PreMarketCloseTime  *string
	PostMarketOpenTime  *string
	PostMarketCloseTime *string
	IsActive            bool
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

type PriceAlert struct {
	ID               int64
	UserID           int64
	RecipientType    string
	ListingID        int64
	Condition        PriceAlertCondition
	Threshold        string
	NotificationType string
	Active           bool
	CreatedAt        time.Time
	LastTriggeredAt  *time.Time
}

type Watchlist struct {
	ID        int64
	UserID    int64
	Name      string
	CreatedAt time.Time
	ItemCount int64
}

type WatchlistItem struct {
	ID          int64
	WatchlistID int64
	ListingID   int64
	AddedAt     time.Time
	Listing     Listing
}

type DividendData struct {
	ListingID     int64
	Ticker        string
	Price         string
	Currency      string
	DividendYield string
}
