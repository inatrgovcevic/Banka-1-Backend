package api

import (
	"testing"

	"Banka1Back/credit-service-go/internal/dto"
	"Banka1Back/credit-service-go/internal/model"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func validRequest() dto.LoanRequestDTO {
	return dto.LoanRequestDTO{
		LoanType:                model.LoanGotovinski,
		InterestType:            model.InterestFixed,
		Amount:                  decimal.NewFromInt(50000),
		Currency:                model.CurrencyRSD,
		Purpose:                 "renovacija",
		MonthlySalary:           decimal.NewFromInt(1000),
		EmploymentStatus:        model.EmploymentPermanent,
		CurrentEmploymentPeriod: 12,
		RepaymentPeriod:         24,
		ContactPhone:            "060123456",
		AccountNumber:           "1234567890123456789",
	}
}

func TestValidateLoanRequest_Valid_NoErrors(t *testing.T) {
	errs := validateLoanRequest(validRequest())
	assert.Empty(t, errs)
}

func TestValidateLoanRequest_MissingLoanType(t *testing.T) {
	req := validRequest()
	req.LoanType = ""
	errs := validateLoanRequest(req)
	assert.Contains(t, errs, "loanType")
}

func TestValidateLoanRequest_MissingInterestType(t *testing.T) {
	req := validRequest()
	req.InterestType = ""
	errs := validateLoanRequest(req)
	assert.Contains(t, errs, "interestType")
}

func TestValidateLoanRequest_ZeroAmount(t *testing.T) {
	req := validRequest()
	req.Amount = decimal.Zero
	errs := validateLoanRequest(req)
	assert.Contains(t, errs, "amount")
}

func TestValidateLoanRequest_NegativeAmount(t *testing.T) {
	req := validRequest()
	req.Amount = decimal.NewFromInt(-1)
	errs := validateLoanRequest(req)
	assert.Contains(t, errs, "amount")
}

func TestValidateLoanRequest_MissingCurrency(t *testing.T) {
	req := validRequest()
	req.Currency = ""
	errs := validateLoanRequest(req)
	assert.Contains(t, errs, "currency")
}

func TestValidateLoanRequest_MissingPurpose(t *testing.T) {
	req := validRequest()
	req.Purpose = ""
	errs := validateLoanRequest(req)
	assert.Contains(t, errs, "purpose")
}

func TestValidateLoanRequest_ZeroMonthlySalary(t *testing.T) {
	req := validRequest()
	req.MonthlySalary = decimal.Zero
	errs := validateLoanRequest(req)
	assert.Contains(t, errs, "monthlySalary")
}

func TestValidateLoanRequest_MissingEmploymentStatus(t *testing.T) {
	req := validRequest()
	req.EmploymentStatus = ""
	errs := validateLoanRequest(req)
	assert.Contains(t, errs, "employmentStatus")
}

func TestValidateLoanRequest_ZeroEmploymentPeriod(t *testing.T) {
	req := validRequest()
	req.CurrentEmploymentPeriod = 0
	errs := validateLoanRequest(req)
	assert.Contains(t, errs, "currentEmploymentPeriod")
}

func TestValidateLoanRequest_ZeroRepaymentPeriod(t *testing.T) {
	req := validRequest()
	req.RepaymentPeriod = 0
	errs := validateLoanRequest(req)
	assert.Contains(t, errs, "repaymentPeriod")
}

func TestValidateLoanRequest_MissingContactPhone(t *testing.T) {
	req := validRequest()
	req.ContactPhone = ""
	errs := validateLoanRequest(req)
	assert.Contains(t, errs, "contactPhone")
}

func TestValidateLoanRequest_ShortAccountNumber(t *testing.T) {
	req := validRequest()
	req.AccountNumber = "123"
	errs := validateLoanRequest(req)
	assert.Contains(t, errs, "accountNumber")
}

func TestValidateLoanRequest_AccountNumberWith20Digits(t *testing.T) {
	req := validRequest()
	req.AccountNumber = "12345678901234567890"
	errs := validateLoanRequest(req)
	assert.Contains(t, errs, "accountNumber")
}

func TestValidateLoanRequest_AccountNumberWithLetters(t *testing.T) {
	req := validRequest()
	req.AccountNumber = "123456789012345678A"
	errs := validateLoanRequest(req)
	assert.Contains(t, errs, "accountNumber")
}

func TestValidateLoanRequest_MultipleErrors(t *testing.T) {
	req := dto.LoanRequestDTO{}
	errs := validateLoanRequest(req)
	assert.Greater(t, len(errs), 5)
}

func TestDecimalZero_IsZero(t *testing.T) {
	assert.True(t, decimalZero().IsZero())
}
