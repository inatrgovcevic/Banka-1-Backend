package market

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestCalculateChangePercent(t *testing.T) {
	got, ok := calculateChangePercent("102", "4")
	if !ok {
		t.Fatal("expected change percent to be available")
	}
	want := decimal.RequireFromString("4.0816")
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestEvaluatePhase(t *testing.T) {
	preOpen := "08:00:00"
	preClose := "09:30:00"
	postOpen := "16:00:00"
	postClose := "20:00:00"
	exchange := &StockExchange{
		TimeZone:            "UTC",
		OpenTime:            "09:30:00",
		CloseTime:           "16:00:00",
		PreMarketOpenTime:   &preOpen,
		PreMarketCloseTime:  &preClose,
		PostMarketOpenTime:  &postOpen,
		PostMarketCloseTime: &postClose,
		IsActive:            true,
	}
	open, afterHours, phase := evaluatePhase(time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC), exchange)
	if !open || afterHours || phase != MarketPhaseRegularMarket {
		t.Fatalf("unexpected regular phase: open=%v afterHours=%v phase=%s", open, afterHours, phase)
	}
}
