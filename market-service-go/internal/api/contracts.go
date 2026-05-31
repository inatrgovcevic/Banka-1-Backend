package api

import "github.com/shopspring/decimal"

func init() {
	decimal.MarshalJSONWithoutQuotes = true
}

type StockExchangeResponse struct {
	ID                  int64   `json:"id"`
	ExchangeName        string  `json:"exchangeName"`
	ExchangeAcronym     string  `json:"exchangeAcronym"`
	ExchangeMICCode     string  `json:"exchangeMICCode"`
	Polity              string  `json:"polity"`
	Currency            string  `json:"currency"`
	TimeZone            string  `json:"timeZone"`
	OpenTime            string  `json:"openTime"`
	CloseTime           string  `json:"closeTime"`
	PreMarketOpenTime   *string `json:"preMarketOpenTime"`
	PreMarketCloseTime  *string `json:"preMarketCloseTime"`
	PostMarketOpenTime  *string `json:"postMarketOpenTime"`
	PostMarketCloseTime *string `json:"postMarketCloseTime"`
	IsActive            bool    `json:"isActive"`
}

type StockExchangeStatusResponse struct {
	ID                    int64   `json:"id"`
	ExchangeName          string  `json:"exchangeName"`
	ExchangeAcronym       string  `json:"exchangeAcronym"`
	ExchangeMICCode       string  `json:"exchangeMICCode"`
	Polity                string  `json:"polity"`
	TimeZone              string  `json:"timeZone"`
	LocalDate             string  `json:"localDate"`
	LocalTime             string  `json:"localTime"`
	OpenTime              string  `json:"openTime"`
	CloseTime             string  `json:"closeTime"`
	PreMarketOpenTime     *string `json:"preMarketOpenTime"`
	PreMarketCloseTime    *string `json:"preMarketCloseTime"`
	PostMarketOpenTime    *string `json:"postMarketOpenTime"`
	PostMarketCloseTime   *string `json:"postMarketCloseTime"`
	IsActive              bool    `json:"isActive"`
	WorkingDay            bool    `json:"workingDay"`
	Holiday               bool    `json:"holiday"`
	Open                  bool    `json:"open"`
	RegularMarketOpen     bool    `json:"regularMarketOpen"`
	TestModeBypassEnabled bool    `json:"testModeBypassEnabled"`
	MarketPhase           string  `json:"marketPhase"`
}

type StockExchangeToggleResponse struct {
	ID              int64  `json:"id"`
	ExchangeName    string `json:"exchangeName"`
	ExchangeMICCode string `json:"exchangeMICCode"`
	IsActive        bool   `json:"isActive"`
}

type ListingDailyPriceInfoResponse struct {
	Date          string           `json:"date"`
	Price         decimal.Decimal  `json:"price"`
	Ask           decimal.Decimal  `json:"ask"`
	Bid           decimal.Decimal  `json:"bid"`
	Change        decimal.Decimal  `json:"change"`
	ChangePercent *decimal.Decimal `json:"changePercent"`
	Volume        int64            `json:"volume"`
	DollarVolume  decimal.Decimal  `json:"dollarVolume"`
}

type ListingStockDetailsResponse struct {
	OutstandingShares int64           `json:"outstandingShares"`
	DividendYield     decimal.Decimal `json:"dividendYield"`
	ContractSize      int32           `json:"contractSize"`
}

type ListingFuturesDetailsResponse struct {
	ContractSize   int32  `json:"contractSize"`
	ContractUnit   string `json:"contractUnit"`
	SettlementDate string `json:"settlementDate"`
}

type ListingForexDetailsResponse struct {
	BaseCurrency  string          `json:"baseCurrency"`
	QuoteCurrency string          `json:"quoteCurrency"`
	ExchangeRate  decimal.Decimal `json:"exchangeRate"`
	Liquidity     string          `json:"liquidity"`
	ContractSize  int32           `json:"contractSize"`
}

type StockOptionDetailsResponse struct {
	ID                int64           `json:"id"`
	Ticker            string          `json:"ticker"`
	OptionType        string          `json:"optionType"`
	StrikePrice       decimal.Decimal `json:"strikePrice"`
	ImpliedVolatility decimal.Decimal `json:"impliedVolatility"`
	OpenInterest      int32           `json:"openInterest"`
	Last              decimal.Decimal `json:"last"`
	Bid               decimal.Decimal `json:"bid"`
	Ask               decimal.Decimal `json:"ask"`
	Volume            int64           `json:"volume"`
	InTheMoney        bool            `json:"inTheMoney"`
}

type StockOptionSettlementGroupResponse struct {
	SettlementDate string                       `json:"settlementDate"`
	Calls          []StockOptionDetailsResponse `json:"calls"`
	Puts           []StockOptionDetailsResponse `json:"puts"`
}

type ListingDetailsResponse struct {
	ListingID           int64                             `json:"listingId"`
	SecurityID          int64                             `json:"securityId"`
	ListingType         string                            `json:"listingType"`
	Ticker              string                            `json:"ticker"`
	Name                string                            `json:"name"`
	StockExchangeID     int64                             `json:"stockExchangeId"`
	ExchangeMICCode     string                            `json:"exchangeMICCode"`
	ExchangeAcronym     string                            `json:"exchangeAcronym"`
	ExchangeName        string                            `json:"exchangeName"`
	LastRefresh         string                            `json:"lastRefresh"`
	Price               decimal.Decimal                   `json:"price"`
	Ask                 decimal.Decimal                   `json:"ask"`
	Bid                 decimal.Decimal                   `json:"bid"`
	Change              decimal.Decimal                   `json:"change"`
	ChangePercent       *decimal.Decimal                  `json:"changePercent"`
	Volume              int64                             `json:"volume"`
	DollarVolume        decimal.Decimal                   `json:"dollarVolume"`
	InitialMarginCost   decimal.Decimal                   `json:"initialMarginCost"`
	RequestedPeriod     string                            `json:"requestedPeriod"`
	PriceHistory        []ListingDailyPriceInfoResponse   `json:"priceHistory"`
	StockDetails        *ListingStockDetailsResponse      `json:"stockDetails"`
	FuturesDetails      *ListingFuturesDetailsResponse    `json:"futuresDetails"`
	ForexDetails        *ListingForexDetailsResponse      `json:"forexDetails"`
	OptionGroups        []StockOptionSettlementGroupResponse `json:"optionGroups"`
	Currency            string                            `json:"currency"`
	ContractSize        *int32                            `json:"contractSize"`
	MaintenanceMargin   *decimal.Decimal                  `json:"maintenanceMargin"`
	OptionType          *string                           `json:"optionType"`
	StrikePrice         *decimal.Decimal                  `json:"strikePrice"`
	UnderlyingListingID *int64                            `json:"underlyingListingId"`
	SettlementDate      *string                           `json:"settlementDate"`
	UnderlyingPrice     *decimal.Decimal                  `json:"underlyingPrice"`
}

type ListingSummaryResponse struct {
	ListingID         int64           `json:"listingId"`
	ListingType       string          `json:"listingType"`
	Ticker            string          `json:"ticker"`
	Name              string          `json:"name"`
	ExchangeMICCode   string          `json:"exchangeMICCode"`
	Currency          string          `json:"currency"`
	Price             decimal.Decimal `json:"price"`
	Change            decimal.Decimal `json:"change"`
	Volume            int64           `json:"volume"`
	InitialMarginCost decimal.Decimal `json:"initialMarginCost"`
	SettlementDate    *string         `json:"settlementDate"`
}

type PageableSort struct {
	Sorted   bool `json:"sorted"`
	Unsorted bool `json:"unsorted"`
	Empty    bool `json:"empty"`
}

type PageableObject struct {
	Paged      bool         `json:"paged"`
	PageSize   int          `json:"pageSize"`
	Unpaged    bool         `json:"unpaged"`
	PageNumber int          `json:"pageNumber"`
	Offset     int          `json:"offset"`
	Sort       PageableSort `json:"sort"`
}

type PageListingSummaryResponse struct {
	TotalElements    int64                  `json:"totalElements"`
	TotalPages       int                    `json:"totalPages"`
	Pageable         PageableObject         `json:"pageable"`
	NumberOfElements int                    `json:"numberOfElements"`
	First            bool                   `json:"first"`
	Last             bool                   `json:"last"`
	Size             int                    `json:"size"`
	Content          []ListingSummaryResponse `json:"content"`
	Number           int                    `json:"number"`
	Sort             PageableSort           `json:"sort"`
	Empty            bool                   `json:"empty"`
}

type ListingRefreshResponse struct {
	ListingID         int64  `json:"listingId"`
	Ticker            string `json:"ticker"`
	ListingType       string `json:"listingType"`
	DailySnapshotDate string `json:"dailySnapshotDate"`
	LastRefresh       string `json:"lastRefresh"`
}

type StockMarketDataRefreshResponse struct {
	Ticker                string `json:"ticker"`
	StockID               int64  `json:"stockId"`
	ListingID             int64  `json:"listingId"`
	RefreshedDailyEntries int    `json:"refreshedDailyEntries"`
	LastRefresh           string `json:"lastRefresh"`
}

type StockBulkRefreshAcceptedResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type StockExchangeImportResponse struct {
	Source         string `json:"source"`
	ProcessedRows  int    `json:"processedRows"`
	CreatedCount   int    `json:"createdCount"`
	UpdatedCount   int    `json:"updatedCount"`
	UnchangedCount int    `json:"unchangedCount"`
}
