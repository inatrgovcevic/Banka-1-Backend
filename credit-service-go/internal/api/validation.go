package api

import (
	"regexp"

	"Banka1Back/credit-service-go/internal/dto"

	"github.com/shopspring/decimal"
)

var accountNumberRegex = regexp.MustCompile(`^\d{19}$`)

func validateLoanRequest(request dto.LoanRequestDTO) map[string]string {
	errors := make(map[string]string)

	if request.LoanType == "" {
		errors["loanType"] = "loanType ne sme biti null"
	}

	if request.InterestType == "" {
		errors["interestType"] = "interestType ne sme biti null"
	}

	if request.Amount.LessThanOrEqual(decimalZero()) {
		errors["amount"] = "amount mora biti >0"
	}

	if request.Currency == "" {
		errors["currency"] = "currency ne sme biti null"
	}

	if request.Purpose == "" {
		errors["purpose"] = "purpose ne sme biti prazan"
	}

	if request.MonthlySalary.LessThanOrEqual(decimalZero()) {
		errors["monthlySalary"] = "monthlySalary mora biti > 0"
	}

	if request.EmploymentStatus == "" {
		errors["employmentStatus"] = "employmentStatus ne sme biti null"
	}

	if request.CurrentEmploymentPeriod <= 0 {
		errors["currentEmploymentPeriod"] = "currentEmploymentPeriod mora biti pozitivan"
	}

	if request.RepaymentPeriod <= 0 {
		errors["repaymentPeriod"] = "repaymentPeriod mora biti pozitivan"
	}

	if request.ContactPhone == "" {
		errors["contactPhone"] = "contactPhone ne sme biti prazan"
	}

	if !accountNumberRegex.MatchString(request.AccountNumber) {
		errors["accountNumber"] = "Broj racuna mora imati 19 cifara"
	}

	return errors
}

func decimalZero() decimal.Decimal {
	return decimal.NewFromInt(0)
}
