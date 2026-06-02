package dto

import (
	"encoding/json"
	"fmt"
	"strconv"

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

func (d *LoanRequestDTO) UnmarshalJSON(data []byte) error {
	type Alias LoanRequestDTO
	aux := &struct {
		RepaymentPeriod json.RawMessage `json:"repaymentPeriod"`
		*Alias
	}{
		Alias: (*Alias)(d),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.RepaymentPeriod != nil {
		var n int
		if err := json.Unmarshal(aux.RepaymentPeriod, &n); err == nil {
			d.RepaymentPeriod = n
			return nil
		}
		var s string
		if err := json.Unmarshal(aux.RepaymentPeriod, &s); err != nil {
			return fmt.Errorf("repaymentPeriod: %w", err)
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("repaymentPeriod must be a number, got %q", s)
		}
		d.RepaymentPeriod = n
	}
	return nil
}
