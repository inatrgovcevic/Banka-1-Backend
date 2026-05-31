package fx

import (
	"context"
	"testing"
	"time"

	"banka1/market-service-go/internal/platform"
	"github.com/shopspring/decimal"
)

func TestCalculateWithoutCommission(t *testing.T) {
	svc := NewService(platform.Config{FX: platform.FXConfig{CommissionPercentage: "0.70"}}, &Repository{})
	resp, err := svc.Calculate(context.Background(), "USD", "USD", "100.00", "2026-05-26", false)
	if err != nil {
		t.Fatalf("calculate same currency: %v", err)
	}
	if !resp.Rate.Equal(decimal.NewFromInt(1)) || !resp.Commission.Equal(decimal.Zero) || !resp.ToAmount.Equal(decimal.RequireFromString("100")) {
		t.Fatalf("unexpected same-currency conversion: %+v", resp)
	}
	if resp.Date != "2026-05-26" {
		t.Fatalf("expected date 2026-05-26, got %q", resp.Date)
	}
}

func TestBaseCurrency(t *testing.T) {
	now := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	rate := baseCurrency(now)
	if rate.CurrencyCode != "RSD" || !rate.BuyingRate.Equal(decimal.NewFromInt(1)) || !rate.SellingRate.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("unexpected base currency payload: %+v", rate)
	}
}
