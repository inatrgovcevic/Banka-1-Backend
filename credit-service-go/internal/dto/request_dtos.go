package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type ApproveDTO struct {
	Key string `json:"key"`
}

type BankPaymentDTO struct {
	FromAccountNumber string          `json:"fromAccountNumber"`
	ToAccountNumber   string          `json:"toAccountNumber"`
	Amount            decimal.Decimal `json:"amount"`
}

type ConversionQueryDTO struct {
	FromCurrency string          `json:"fromCurrency"`
	ToCurrency   string          `json:"toCurrency"`
	Amount       decimal.Decimal `json:"amount"`
	Date         *time.Time      `json:"date,omitempty"`
}

type PaymentDTO struct {
	FromAccountNumber string          `json:"fromAccountNumber"`
	ToAccountNumber   string          `json:"toAccountNumber"`
	FromAmount        decimal.Decimal `json:"fromAmount"`
	ToAmount          decimal.Decimal `json:"toAmount"`
	Commission        decimal.Decimal `json:"commission"`
	ClientID          *int64          `json:"clientId,omitempty"`
}
