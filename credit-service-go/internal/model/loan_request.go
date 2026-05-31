package model

import "github.com/shopspring/decimal"

type LoanRequest struct {
	BaseEntity

	LoanType                LoanType         `json:"loanType"`
	InterestType            InterestType     `json:"interestType"`
	Amount                  decimal.Decimal  `json:"amount"`
	Currency                CurrencyCode     `json:"currency"`
	Purpose                 string           `json:"purpose"`
	MonthlySalary           decimal.Decimal  `json:"monthlySalary"`
	EmploymentStatus        EmploymentStatus `json:"employmentStatus"`
	CurrentEmploymentPeriod int              `json:"currentEmploymentPeriod"`
	RepaymentPeriod         int              `json:"repaymentPeriod"`
	ContactPhone            string           `json:"contactPhone"`
	AccountNumber           string           `json:"accountNumber"`
	ClientID                int64            `json:"clientId"`
	Status                  Status           `json:"status"`
	UserEmail               string           `json:"userEmail"`
	Username                string           `json:"username"`
}
