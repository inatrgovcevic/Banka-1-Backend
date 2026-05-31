package fx

import "github.com/shopspring/decimal"

type ExchangeRate struct {
	CurrencyCode string          `json:"currencyCode"`
	BuyingRate   decimal.Decimal `json:"buyingRate"`
	SellingRate  decimal.Decimal `json:"sellingRate"`
	Date         string          `json:"date"`
	CreatedAt    string          `json:"createdAt"`
}

type ConversionResponse struct {
	FromCurrency string          `json:"fromCurrency"`
	ToCurrency   string          `json:"toCurrency"`
	FromAmount   decimal.Decimal `json:"fromAmount"`
	ToAmount     decimal.Decimal `json:"toAmount"`
	Rate         decimal.Decimal `json:"rate"`
	Commission   decimal.Decimal `json:"commission"`
	Date         string          `json:"date"`
}
