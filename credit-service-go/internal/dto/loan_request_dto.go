package dto

import (
	"Banka1Back/credit-service-go/internal/model"

	"github.com/shopspring/decimal"
)

type LoanRequestDTO struct {
	LoanType                model.LoanType         `json:"loanType"`
	InterestType            model.InterestType     `json:"interestType"`
	Amount                  decimal.Decimal        `json:"amount"`
	Currency                model.CurrencyCode     `json:"currency"`
	Purpose                 string                 `json:"purpose"`
	MonthlySalary           decimal.Decimal        `json:"monthlySalary"`
	EmploymentStatus        model.EmploymentStatus `json:"employmentStatus"`
	CurrentEmploymentPeriod int                    `json:"currentEmploymentPeriod"`
	RepaymentPeriod         int                    `json:"repaymentPeriod"`
	ContactPhone            string                 `json:"contactPhone"`
	AccountNumber           string                 `json:"accountNumber"`
}
