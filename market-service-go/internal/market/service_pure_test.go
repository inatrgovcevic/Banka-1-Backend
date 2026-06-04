package market

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// normalizeClock
// ---------------------------------------------------------------------------

func TestNormalizeClock_HHmm_AppendsSeconds(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "09:30:00", normalizeClock("09:30"))
}

func TestNormalizeClock_HHmmss_Unchanged(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "09:30:00", normalizeClock("09:30:00"))
}

func TestNormalizeClock_Other_Unchanged(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "930", normalizeClock("930"))
}

// ---------------------------------------------------------------------------
// isWithinWindow
// ---------------------------------------------------------------------------

func TestIsWithinWindow_NilFrom_ReturnsFalse(t *testing.T) {
	t.Parallel()
	to := "17:00:00"
	assert.False(t, isWithinWindow(time.Now(), nil, &to))
}

func TestIsWithinWindow_NilTo_ReturnsFalse(t *testing.T) {
	t.Parallel()
	from := "09:00:00"
	assert.False(t, isWithinWindow(time.Now(), &from, nil))
}

func TestIsWithinWindow_FromEqualsTo_ReturnsFalse(t *testing.T) {
	t.Parallel()
	s := "09:30:00"
	now := time.Date(2024, 1, 1, 9, 30, 0, 0, time.UTC)
	assert.False(t, isWithinWindow(now, &s, &s))
}

func TestIsWithinWindow_WithinRange_ReturnsTrue(t *testing.T) {
	t.Parallel()
	from := "09:00:00"
	to := "17:00:00"
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	assert.True(t, isWithinWindow(now, &from, &to))
}

func TestIsWithinWindow_BeforeRange_ReturnsFalse(t *testing.T) {
	t.Parallel()
	from := "09:00:00"
	to := "17:00:00"
	now := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	assert.False(t, isWithinWindow(now, &from, &to))
}

func TestIsWithinWindow_AfterRange_ReturnsFalse(t *testing.T) {
	t.Parallel()
	from := "09:00:00"
	to := "17:00:00"
	now := time.Date(2024, 1, 1, 18, 0, 0, 0, time.UTC)
	assert.False(t, isWithinWindow(now, &from, &to))
}

func TestIsWithinWindow_OvernightWindow_Midnight(t *testing.T) {
	t.Parallel()
	from := "22:00:00"
	to := "06:00:00"
	now := time.Date(2024, 1, 1, 23, 30, 0, 0, time.UTC)
	assert.True(t, isWithinWindow(now, &from, &to))
}

func TestIsWithinWindow_InvalidTime_ReturnsFalse(t *testing.T) {
	t.Parallel()
	invalid := "not-a-time"
	to := "17:00:00"
	now := time.Now()
	assert.False(t, isWithinWindow(now, &invalid, &to))
}

// ---------------------------------------------------------------------------
// resolveMarketPhase
// ---------------------------------------------------------------------------

func TestResolveMarketPhase_NonWorkingDay_ReturnsClosed(t *testing.T) {
	t.Parallel()
	exchange := &StockExchange{OpenTime: "09:00:00", CloseTime: "17:00:00"}
	phase := resolveMarketPhase(time.Now(), exchange, false)
	assert.Equal(t, MarketPhaseClosed, phase)
}

func TestResolveMarketPhase_WithinRegularHours_ReturnsRegular(t *testing.T) {
	t.Parallel()
	exchange := &StockExchange{OpenTime: "09:00:00", CloseTime: "17:00:00"}
	now := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC) // Tuesday
	phase := resolveMarketPhase(now, exchange, true)
	assert.Equal(t, MarketPhaseRegularMarket, phase)
}

func TestResolveMarketPhase_PreMarket(t *testing.T) {
	t.Parallel()
	preOpen := "08:00:00"
	preClose := "09:00:00"
	exchange := &StockExchange{
		OpenTime:          "09:00:00",
		CloseTime:         "17:00:00",
		PreMarketOpenTime:  &preOpen,
		PreMarketCloseTime: &preClose,
	}
	now := time.Date(2024, 1, 2, 8, 30, 0, 0, time.UTC)
	phase := resolveMarketPhase(now, exchange, true)
	assert.Equal(t, MarketPhasePreMarket, phase)
}

func TestResolveMarketPhase_AfterMarket_ReturnsClosed(t *testing.T) {
	t.Parallel()
	exchange := &StockExchange{OpenTime: "09:00:00", CloseTime: "17:00:00"}
	now := time.Date(2024, 1, 2, 20, 0, 0, 0, time.UTC)
	phase := resolveMarketPhase(now, exchange, true)
	assert.Equal(t, MarketPhaseClosed, phase)
}

// ---------------------------------------------------------------------------
// evaluatePhase
// ---------------------------------------------------------------------------

func TestEvaluatePhase_InactiveExchange_AlwaysOpen(t *testing.T) {
	t.Parallel()
	exchange := &StockExchange{
		OpenTime:  "09:00:00",
		CloseTime: "17:00:00",
		IsActive:  false,
		TimeZone:  "UTC",
	}
	now := time.Date(2024, 1, 6, 12, 0, 0, 0, time.UTC) // Saturday
	open, afterHours, phase := evaluatePhase(now, exchange)
	assert.True(t, open)
	assert.False(t, afterHours)
	assert.Equal(t, MarketPhaseRegularMarket, phase)
}

func TestEvaluatePhase_ActiveExchange_WeekdayOpen(t *testing.T) {
	t.Parallel()
	exchange := &StockExchange{
		OpenTime:  "09:00:00",
		CloseTime: "17:00:00",
		IsActive:  true,
		TimeZone:  "UTC",
	}
	now := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC) // Tuesday
	open, afterHours, _ := evaluatePhase(now, exchange)
	assert.True(t, open)
	assert.False(t, afterHours)
}

func TestEvaluatePhase_InvalidTimezone_FallsBackToUTC(t *testing.T) {
	t.Parallel()
	exchange := &StockExchange{
		OpenTime:  "09:00:00",
		CloseTime: "17:00:00",
		IsActive:  true,
		TimeZone:  "Invalid/Zone",
	}
	now := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)
	open, _, _ := evaluatePhase(now, exchange)
	assert.True(t, open)
}

// ---------------------------------------------------------------------------
// calculateChangePercent
// ---------------------------------------------------------------------------

func TestCalculateChangePercent_PositiveChange(t *testing.T) {
	t.Parallel()
	result, ok := calculateChangePercent("110", "10")
	require.True(t, ok)
	// previous = 100, change% = 10/100 * 100 = 10%
	assert.True(t, result.Equal(decimal.NewFromInt(10)), "result = %s", result.String())
}

func TestCalculateChangePercent_ZeroPreviousPrice_ReturnsFalse(t *testing.T) {
	t.Parallel()
	_, ok := calculateChangePercent("5", "5")
	assert.False(t, ok, "previous=0 should return false")
}

func TestCalculateChangePercentPtr_ReturnsPointer(t *testing.T) {
	t.Parallel()
	result := calculateChangePercentPtr("110", "10")
	require.NotNil(t, result)
	assert.True(t, result.Equal(decimal.NewFromInt(10)))
}

func TestCalculateChangePercentPtr_ZeroPrevious_ReturnsNil(t *testing.T) {
	t.Parallel()
	result := calculateChangePercentPtr("5", "5")
	assert.Nil(t, result)
}

// ---------------------------------------------------------------------------
// calculateDollarVolume
// ---------------------------------------------------------------------------

func TestCalculateDollarVolume_Basic(t *testing.T) {
	t.Parallel()
	result := calculateDollarVolume("50.00", 200)
	assert.True(t, result.Equal(decimal.NewFromInt(10000)), "result = %s", result.String())
}

// ---------------------------------------------------------------------------
// calculateInitialMargin
// ---------------------------------------------------------------------------

func TestCalculateInitialMargin_Stock(t *testing.T) {
	t.Parallel()
	result := calculateInitialMargin(ListingTypeStock, "100", nil)
	expected := decimal.NewFromFloat(100).Mul(decimal.RequireFromString("0.50")).Mul(decimal.RequireFromString("1.10"))
	assert.True(t, result.Equal(expected), "result = %s", result.String())
}

func TestCalculateInitialMargin_Futures_WithContractSize(t *testing.T) {
	t.Parallel()
	size := int32(5)
	result := calculateInitialMargin(ListingTypeFutures, "100", &size)
	expected := decimal.NewFromInt32(5).Mul(decimal.NewFromFloat(100)).Mul(decimal.RequireFromString("0.10")).Mul(decimal.RequireFromString("1.10"))
	assert.True(t, result.Equal(expected))
}

func TestCalculateInitialMargin_Futures_NilContractSize_UsesOne(t *testing.T) {
	t.Parallel()
	result := calculateInitialMargin(ListingTypeFutures, "100", nil)
	expected := decimal.NewFromInt32(1).Mul(decimal.NewFromFloat(100)).Mul(decimal.RequireFromString("0.10")).Mul(decimal.RequireFromString("1.10"))
	assert.True(t, result.Equal(expected))
}

func TestCalculateInitialMargin_Forex(t *testing.T) {
	t.Parallel()
	result := calculateInitialMargin(ListingTypeForex, "1.2", nil)
	expected := decimal.NewFromInt(1000).Mul(decimal.RequireFromString("1.2")).Mul(decimal.RequireFromString("0.10")).Mul(decimal.RequireFromString("1.10"))
	assert.True(t, result.Equal(expected))
}

// ---------------------------------------------------------------------------
// dec / decDefault / formatLocalDateTime
// ---------------------------------------------------------------------------

func TestDec_ParsesDecimal(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "42.5", dec("42.5").String())
}

func TestDecDefault_Empty_ReturnsZero(t *testing.T) {
	t.Parallel()
	assert.True(t, decDefault("").IsZero())
}

func TestDecDefault_NonEmpty_Parses(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "3.14", decDefault("3.14").String())
}

func TestFormatLocalDateTime_FormatsCorrectly(t *testing.T) {
	t.Parallel()
	tm := time.Date(2024, 6, 15, 10, 30, 45, 0, time.UTC)
	assert.Equal(t, "2024-06-15T10:30:45", formatLocalDateTime(tm))
}

// ---------------------------------------------------------------------------
// alertSatisfied
// ---------------------------------------------------------------------------

func TestAlertSatisfied_AboveCondition_Met(t *testing.T) {
	t.Parallel()
	alert := PriceAlert{Condition: PriceAlertAbove, Threshold: "100.0000"}
	listing := Listing{Price: "105.0", Change: "5.0"}
	assert.True(t, alertSatisfied(alert, listing))
}

func TestAlertSatisfied_AboveCondition_NotMet(t *testing.T) {
	t.Parallel()
	alert := PriceAlert{Condition: PriceAlertAbove, Threshold: "100.0000"}
	listing := Listing{Price: "95.0", Change: "0.0"}
	assert.False(t, alertSatisfied(alert, listing))
}

func TestAlertSatisfied_BelowCondition_Met(t *testing.T) {
	t.Parallel()
	alert := PriceAlert{Condition: PriceAlertBelow, Threshold: "50.0000"}
	listing := Listing{Price: "45.0", Change: "-5.0"}
	assert.True(t, alertSatisfied(alert, listing))
}

func TestAlertSatisfied_PctDropIntraday_Met(t *testing.T) {
	t.Parallel()
	// price=90, change=-10 → previous=100, pct_drop=10% → threshold=5% → 10 <= -(-5) → yes
	alert := PriceAlert{Condition: PriceAlertPctDropIntraday, Threshold: "5.0000"}
	listing := Listing{Price: "90", Change: "-10"}
	assert.True(t, alertSatisfied(alert, listing))
}

func TestAlertSatisfied_UnknownCondition_ReturnsFalse(t *testing.T) {
	t.Parallel()
	alert := PriceAlert{Condition: PriceAlertCondition("UNKNOWN"), Threshold: "50.0000"}
	listing := Listing{Price: "45.0", Change: "-5.0"}
	assert.False(t, alertSatisfied(alert, listing))
}

// ---------------------------------------------------------------------------
// normalizeNotificationType
// ---------------------------------------------------------------------------

func TestNormalizeNotificationType_ValidTypes(t *testing.T) {
	t.Parallel()
	cases := []string{"EMAIL", "PUSH", "IN_APP", "ALL", "email", "push"}
	for _, c := range cases {
		result, err := normalizeNotificationType(c)
		require.NoError(t, err, "type: %s", c)
		assert.NotEmpty(t, result)
	}
}

func TestNormalizeNotificationType_Invalid_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := normalizeNotificationType("UNKNOWN_TYPE")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// periodStart
// ---------------------------------------------------------------------------

func TestPeriodStart_AllVariants(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	cases := map[string]time.Time{
		"WEEK":       now.AddDate(0, 0, -7),
		"MONTH":      now.AddDate(0, -1, 0),
		"YEAR":       now.AddDate(-1, 0, 0),
		"FIVE_YEARS": now.AddDate(-5, 0, 0),
		"ALL":        time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		"DEFAULT":    now.AddDate(0, 0, -1),
	}
	for period, expected := range cases {
		result := periodStart(now, period)
		assert.Equal(t, expected, result, "period=%s", period)
	}
}
