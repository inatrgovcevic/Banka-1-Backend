package model

import "github.com/shopspring/decimal"

type InterestRateStore struct {
	NominalInterestRate   decimal.Decimal `json:"nominalInterestRate"`
	EffectiveInterestRate decimal.Decimal `json:"effectiveInterestRate"`
}
