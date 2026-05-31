package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Loan struct {
	BaseEntity

	LoanType              LoanType        `json:"loanType"`
	AccountNumber         string          `json:"accountNumber"`
	Amount                decimal.Decimal `json:"amount"`
	RepaymentPeriod       int             `json:"repaymentPeriod"`
	NominalInterestRate   decimal.Decimal `json:"nominalInterestRate"`
	EffectiveInterestRate decimal.Decimal `json:"effectiveInterestRate"`
	InterestType          InterestType    `json:"interestType"`
	AgreementDate         time.Time       `json:"agreementDate"`
	MaturityDate          time.Time       `json:"maturityDate"`
	InstallmentAmount     decimal.Decimal `json:"installmentAmount"`
	NextInstallmentDate   time.Time       `json:"nextInstallmentDate"`
	RemainingDebt         decimal.Decimal `json:"remainingDebt"`
	Currency              CurrencyCode    `json:"currency"`
	Status                Status          `json:"status"`
	UserEmail             string          `json:"userEmail"`
	Username              string          `json:"username"`
	ClientID              int64           `json:"clientId"`
	InstallmentCount      int             `json:"installmentCount"`
}
