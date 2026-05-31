package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Installment struct {
	BaseEntity

	LoanID                int64           `json:"loanId"`
	InstallmentAmount     decimal.Decimal `json:"installmentAmount"`
	InterestRateAtPayment decimal.Decimal `json:"interestRateAtPayment"`
	Currency              CurrencyCode    `json:"currency"`
	ExpectedDueDate       time.Time       `json:"expectedDueDate"`
	ActualDueDate         *time.Time      `json:"actualDueDate,omitempty"`
	PaymentStatus         PaymentStatus   `json:"paymentStatus"`
	Retry                 int             `json:"retry"`
}
