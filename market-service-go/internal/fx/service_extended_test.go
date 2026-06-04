package fx

import (
	"testing"
	"time"

	"banka1/market-service-go/internal/platform"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cfgWithMarginAndCommission(margin, commission string) platform.Config {
	return platform.Config{FX: platform.FXConfig{
		MarginPercentage:     margin,
		CommissionPercentage: commission,
	}}
}

// ---------------------------------------------------------------------------
// baseCurrency
// ---------------------------------------------------------------------------

func TestBaseCurrency_ReturnsRSDWithUnitRates(t *testing.T) {
	t.Parallel()
	date := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	rate := baseCurrency(date)
	assert.Equal(t, "RSD", rate.CurrencyCode)
	assert.True(t, rate.BuyingRate.Equal(decimal.NewFromInt(1)))
	assert.True(t, rate.SellingRate.Equal(decimal.NewFromInt(1)))
	assert.Equal(t, "2024-06-01", rate.Date)
}

// ---------------------------------------------------------------------------
// resolveMarginFactor / resolveCommissionFactor
// ---------------------------------------------------------------------------

func TestResolveMarginFactor_1Percent(t *testing.T) {
	t.Parallel()
	svc := &Service{cfg: cfgWithMarginAndCommission("1.00", "0.70")}
	factor := svc.resolveMarginFactor()
	expected := decimal.NewFromFloat(0.01)
	assert.True(t, factor.Equal(expected), "factor = %s, want 0.01", factor.String())
}

func TestResolveCommissionFactor_0_7Percent(t *testing.T) {
	t.Parallel()
	svc := &Service{cfg: cfgWithMarginAndCommission("1.00", "0.70")}
	factor := svc.resolveCommissionFactor()
	expected := decimal.NewFromFloat(0.007)
	assert.True(t, factor.Equal(expected), "factor = %s, want 0.007", factor.String())
}

// ---------------------------------------------------------------------------
// calculateBuyingRate / calculateSellingRate
// ---------------------------------------------------------------------------

func TestCalculateBuyingRate_AppliesDiscount(t *testing.T) {
	t.Parallel()
	svc := &Service{cfg: cfgWithMarginAndCommission("2.00", "0.70")}
	marketRate := decimal.NewFromFloat(100.0)
	buying := svc.calculateBuyingRate(marketRate)
	// buying = 100 * (1 - 0.02) = 98
	assert.True(t, buying.Equal(decimal.NewFromFloat(98)), "buying = %s, want 98", buying.String())
}

func TestCalculateSellingRate_AppliesPremium(t *testing.T) {
	t.Parallel()
	svc := &Service{cfg: cfgWithMarginAndCommission("2.00", "0.70")}
	marketRate := decimal.NewFromFloat(100.0)
	selling := svc.calculateSellingRate(marketRate)
	// selling = 100 * (1 + 0.02) = 102
	assert.True(t, selling.Equal(decimal.NewFromFloat(102)), "selling = %s, want 102", selling.String())
}

// ---------------------------------------------------------------------------
// Calculate — same-currency with date
// ---------------------------------------------------------------------------

func TestCalculate_SameCurrency_WithDate_ReturnsOne(t *testing.T) {
	t.Parallel()
	svc := NewService(platform.Config{FX: platform.FXConfig{CommissionPercentage: "0.70"}}, &Repository{})
	resp, err := svc.Calculate(nil, "EUR", "EUR", "50", "2024-01-15", false)
	require.NoError(t, err)
	assert.True(t, resp.Rate.Equal(decimal.NewFromInt(1)))
	assert.Equal(t, "50", resp.FromAmount.String())
	assert.Equal(t, "2024-01-15", resp.Date)
}

func TestCalculate_SameCurrency_UppercaseTrimmed(t *testing.T) {
	t.Parallel()
	svc := NewService(platform.Config{FX: platform.FXConfig{CommissionPercentage: "0.70"}}, &Repository{})
	resp, err := svc.Calculate(nil, " usd ", " USD ", "100", "2024-01-15", false)
	require.NoError(t, err)
	assert.Equal(t, "USD", resp.FromCurrency)
	assert.Equal(t, "USD", resp.ToCurrency)
}

func TestCalculate_InvalidAmount_ReturnsError(t *testing.T) {
	t.Parallel()
	svc := NewService(platform.Config{FX: platform.FXConfig{CommissionPercentage: "0.70"}}, &Repository{})
	_, err := svc.Calculate(nil, "USD", "EUR", "not-a-number", "2024-01-15", false)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// FetchOnStartup — disabled
// ---------------------------------------------------------------------------

func TestFetchOnStartup_Disabled_ReturnsNil(t *testing.T) {
	t.Parallel()
	svc := NewService(platform.Config{FX: platform.FXConfig{FetchOnStartup: false}}, &Repository{})
	err := svc.FetchOnStartup(nil)
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// SetClient
// ---------------------------------------------------------------------------

func TestSetClient_DoesNotPanic(t *testing.T) {
	t.Parallel()
	svc := NewService(platform.Config{}, &Repository{})
	assert.NotPanics(t, func() { svc.SetClient(nil) })
}
