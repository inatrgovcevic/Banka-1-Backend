package service

import (
	"testing"

	"Banka1Back/credit-service-go/internal/model"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestCalculateInterestRate_GotovinskiFixed_SmallAmount(t *testing.T) {
	result := calculateInterestRate(
		decimal.NewFromInt(50000),
		model.LoanGotovinski,
		model.InterestFixed,
		model.StatusActive,
	)

	assert.False(t, result.NominalInterestRate.IsZero())
	assert.Equal(t, result.NominalInterestRate, result.EffectiveInterestRate,
		"fixed interest: nominal must equal effective")
}

func TestCalculateInterestRate_StambeniVariable_LargeAmount(t *testing.T) {
	result := calculateInterestRate(
		decimal.NewFromInt(2000000),
		model.LoanStambeni,
		model.InterestVariable,
		model.StatusActive,
	)

	assert.True(t, result.EffectiveInterestRate.GreaterThan(result.NominalInterestRate),
		"variable interest: effective must be higher than nominal due to reference rate")
}

func TestCalculateInterestRate_OverdueStatus_IncreasesRate(t *testing.T) {
	normal := calculateInterestRate(
		decimal.NewFromInt(100000),
		model.LoanGotovinski,
		model.InterestFixed,
		model.StatusActive,
	)
	overdue := calculateInterestRate(
		decimal.NewFromInt(100000),
		model.LoanGotovinski,
		model.InterestFixed,
		model.StatusOverdue,
	)

	assert.True(t, overdue.NominalInterestRate.GreaterThan(normal.NominalInterestRate),
		"overdue status must increase the interest rate")
}

func TestCalculateInterestRate_AllLoanTypes_ProduceDifferentMargins(t *testing.T) {
	amount := decimal.NewFromInt(100000)
	types := []model.LoanType{
		model.LoanGotovinski,
		model.LoanStambeni,
		model.LoanAuto,
		model.LoanRefinansirajuci,
		model.LoanStudentski,
	}
	rates := make(map[model.LoanType]decimal.Decimal)
	for _, lt := range types {
		r := calculateInterestRate(amount, lt, model.InterestFixed, model.StatusActive)
		rates[lt] = r.NominalInterestRate
	}

	assert.True(t, rates[model.LoanGotovinski].GreaterThan(rates[model.LoanStudentski]),
		"gotovinski should have higher rate than studentski")
	assert.True(t, rates[model.LoanStambeni].GreaterThan(rates[model.LoanStudentski]))
}

func TestCalculateInterestRate_TierBoundary_DecreasesByStep(t *testing.T) {
	small := calculateInterestRate(
		decimal.NewFromInt(50000),
		model.LoanAuto,
		model.InterestFixed,
		model.StatusActive,
	)
	large := calculateInterestRate(
		decimal.NewFromInt(600000),
		model.LoanAuto,
		model.InterestFixed,
		model.StatusActive,
	)
	assert.True(t, small.NominalInterestRate.GreaterThan(large.NominalInterestRate),
		"higher tier amount should produce lower rate")
}

func TestCalculateInterestRate_UnknownLoanType_ZeroMargin(t *testing.T) {
	result := calculateInterestRate(
		decimal.NewFromInt(100000),
		model.LoanType("UNKNOWN"),
		model.InterestFixed,
		model.StatusActive,
	)
	known := calculateInterestRate(
		decimal.NewFromInt(100000),
		model.LoanGotovinski,
		model.InterestFixed,
		model.StatusActive,
	)
	assert.True(t, known.NominalInterestRate.GreaterThan(result.NominalInterestRate),
		"unknown type has zero margin, should produce lower rate than known type with positive margin")
}

func TestLoanTypeMargin_AllTypes(t *testing.T) {
	cases := []struct {
		loanType model.LoanType
		wantSign int // 1 = positive, 0 = zero
	}{
		{model.LoanGotovinski, 1},
		{model.LoanStambeni, 1},
		{model.LoanAuto, 1},
		{model.LoanRefinansirajuci, 1},
		{model.LoanStudentski, 1},
		{model.LoanType("UNKNOWN"), 0},
	}

	for _, tc := range cases {
		margin := loanTypeMargin(tc.loanType)
		if tc.wantSign == 1 {
			assert.True(t, margin.IsPositive(), "expected positive margin for %s", tc.loanType)
		} else {
			assert.True(t, margin.IsZero(), "expected zero margin for %s", tc.loanType)
		}
	}
}

func TestCalculateInterestRate_HighestTier_ProducesLowestRate(t *testing.T) {
	tiers := []int64{50_000, 200_000, 700_000, 2_000_000, 6_000_000}
	prev := decimal.NewFromInt(100)
	for _, amount := range tiers {
		r := calculateInterestRate(
			decimal.NewFromInt(amount),
			model.LoanGotovinski,
			model.InterestFixed,
			model.StatusActive,
		)
		assert.True(t, r.NominalInterestRate.LessThanOrEqual(prev),
			"rate for amount %d should be <= previous tier rate", amount)
		prev = r.NominalInterestRate
	}
}
