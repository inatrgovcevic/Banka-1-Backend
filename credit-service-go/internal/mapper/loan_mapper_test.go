package mapper_test

import (
	"testing"
	"time"

	"Banka1Back/credit-service-go/internal/mapper"
	"Banka1Back/credit-service-go/internal/model"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleLoan() model.Loan {
	return model.Loan{
		BaseEntity: model.BaseEntity{ID: 42},
		LoanType:              model.LoanGotovinski,
		AccountNumber:         "1234567890123456789",
		Amount:                decimal.NewFromInt(100000),
		RepaymentPeriod:       24,
		NominalInterestRate:   decimal.NewFromFloat(0.01),
		EffectiveInterestRate: decimal.NewFromFloat(0.011),
		InterestType:          model.InterestFixed,
		AgreementDate:         time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		MaturityDate:          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		InstallmentAmount:     decimal.NewFromFloat(4700),
		NextInstallmentDate:   time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		RemainingDebt:         decimal.NewFromInt(100000),
		Status:                model.StatusActive,
	}
}

func TestLoanToResponseDTO_MapsAllFields(t *testing.T) {
	t.Parallel()
	loan := sampleLoan()
	dto := mapper.LoanToResponseDTO(loan)

	assert.Equal(t, int64(42), dto.LoanNumber)
	assert.Equal(t, model.LoanGotovinski, dto.LoanType)
	assert.Equal(t, "1234567890123456789", dto.AccountNumber)
	assert.True(t, decimal.NewFromInt(100000).Equal(dto.Amount))
	assert.Equal(t, 24, dto.RepaymentMethod)
	assert.Equal(t, model.InterestFixed, dto.InterestType)
	assert.Equal(t, model.StatusActive, dto.Status)
}

func TestLoanRequestToResponseDTO_MapsIDAndCreatedAt(t *testing.T) {
	t.Parallel()
	now := time.Now()
	req := model.LoanRequest{
		BaseEntity: model.BaseEntity{ID: 7, CreatedAt: now},
	}

	dto := mapper.LoanRequestToResponseDTO(req)
	assert.Equal(t, int64(7), dto.ID)
	assert.Equal(t, now, dto.CreatedAt)
}

func TestInstallmentToResponseDTO_MapsAllFields(t *testing.T) {
	t.Parallel()
	due := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	installment := model.Installment{
		InstallmentAmount:     decimal.NewFromFloat(500),
		InterestRateAtPayment: decimal.NewFromFloat(0.01),
		Currency:              model.CurrencyRSD,
		ExpectedDueDate:       due,
		PaymentStatus:         model.PaymentUnpaid,
	}

	dto := mapper.InstallmentToResponseDTO(installment)
	assert.True(t, decimal.NewFromFloat(500).Equal(dto.InstallmentAmount))
	assert.Equal(t, model.CurrencyRSD, dto.Currency)
	assert.Equal(t, due, dto.ExpectedDueDate)
	assert.Equal(t, model.PaymentUnpaid, dto.PaymentStatus)
	assert.Nil(t, dto.ActualDueDate)
}

func TestInstallmentsToResponseDTOs_EmptySlice(t *testing.T) {
	t.Parallel()
	result := mapper.InstallmentsToResponseDTOs([]model.Installment{})
	assert.Empty(t, result)
}

func TestInstallmentsToResponseDTOs_MultipleInstallments(t *testing.T) {
	t.Parallel()
	installments := []model.Installment{
		{PaymentStatus: model.PaymentPaid},
		{PaymentStatus: model.PaymentUnpaid},
		{PaymentStatus: model.PaymentOverdue},
	}
	result := mapper.InstallmentsToResponseDTOs(installments)
	require.Len(t, result, 3)
	assert.Equal(t, model.PaymentPaid, result[0].PaymentStatus)
	assert.Equal(t, model.PaymentOverdue, result[2].PaymentStatus)
}

func TestLoanInfoToResponseDTO_CombinesLoanAndInstallments(t *testing.T) {
	t.Parallel()
	loan := sampleLoan()
	installments := []model.Installment{
		{PaymentStatus: model.PaymentUnpaid, Currency: model.CurrencyRSD},
	}

	dto := mapper.LoanInfoToResponseDTO(loan, installments)
	assert.Equal(t, int64(42), dto.Loan.LoanNumber)
	require.Len(t, dto.Installments, 1)
	assert.Equal(t, model.CurrencyRSD, dto.Installments[0].Currency)
}
