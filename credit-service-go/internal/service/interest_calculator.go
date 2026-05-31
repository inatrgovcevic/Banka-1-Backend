package service

import (
	"Banka1Back/credit-service-go/internal/model"

	"github.com/shopspring/decimal"
)

type InterestRateResult struct {
	NominalInterestRate   decimal.Decimal
	EffectiveInterestRate decimal.Decimal
}

var amountTiers = []decimal.Decimal{
	decimal.NewFromInt(100000),
	decimal.NewFromInt(500000),
	decimal.NewFromInt(1000000),
	decimal.NewFromInt(5000000),
}

var (
	monthlyDivisor          = decimal.NewFromInt(12)
	baseAnnualRatePercent   = decimal.NewFromFloat(12.0)
	stepPerTierPercent      = decimal.NewFromFloat(1.0)
	overdueIncrementPercent = decimal.NewFromFloat(2.0)
	referenceRate           = decimal.NewFromFloat(0.5)
)

func calculateInterestRate(
	amount decimal.Decimal,
	loanType model.LoanType,
	interestType model.InterestType,
	status model.Status,
) InterestRateResult {
	tierIndex := 0

	for tierIndex < len(amountTiers)-1 && amount.GreaterThan(amountTiers[tierIndex]) {
		tierIndex++
	}

	annualPercent := baseAnnualRatePercent.
		Sub(stepPerTierPercent.Mul(decimal.NewFromInt(int64(tierIndex)))).
		Add(loanTypeMargin(loanType))

	if status == model.StatusOverdue {
		annualPercent = annualPercent.Add(overdueIncrementPercent)
	}

	nominal := annualPercent.Div(monthlyDivisor).Div(decimal.NewFromInt(100)).Round(10)
	effective := nominal

	if interestType == model.InterestVariable {
		effective = annualPercent.Add(referenceRate).Div(monthlyDivisor).Div(decimal.NewFromInt(100)).Round(10)
	}

	return InterestRateResult{
		NominalInterestRate:   nominal,
		EffectiveInterestRate: effective,
	}
}

func loanTypeMargin(loanType model.LoanType) decimal.Decimal {
	switch loanType {
	case model.LoanGotovinski:
		return decimal.NewFromFloat(1.75)
	case model.LoanStambeni:
		return decimal.NewFromFloat(1.50)
	case model.LoanAuto:
		return decimal.NewFromFloat(1.25)
	case model.LoanRefinansirajuci:
		return decimal.NewFromFloat(1.00)
	case model.LoanStudentski:
		return decimal.NewFromFloat(0.75)
	default:
		return decimal.Zero
	}
}
