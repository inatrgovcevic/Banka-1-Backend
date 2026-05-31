package http

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"banka1/market-service-go/internal/platform"
)

func TestNewAppRespectsDisabledScheduler(t *testing.T) {
	cfg := platform.Config{
		Stock: platform.StockConfig{
			RefreshEnabled:  false,
			RefreshInterval: time.Minute,
		},
		FX: platform.FXConfig{
			FetchOnStartup: false,
			FetchCron:      "",
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := NewApp(ctx, cfg, nil, slog.Default())
	if err != nil {
		t.Fatalf("NewApp failed when scheduler/fetch disabled: %v", err)
	}
	defer app.Close()

	if app.MarketService == nil || app.FXService == nil || app.PriceFeed == nil {
		t.Fatal("NewApp must wire MarketService, FXService and PriceFeed even with scheduler disabled")
	}
}

func TestNewAppDoesNotCallProviderOnStartupWhenDisabled(t *testing.T) {
	cfg := platform.Config{
		Stock: platform.StockConfig{
			RefreshEnabled: false,
		},
		FX: platform.FXConfig{
			FetchOnStartup: false,
			FetchCron:      "",
			TwelveDataBaseURL: "http://127.0.0.1:1",
			TwelveDataAPIKey:  "",
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := NewApp(ctx, cfg, nil, slog.Default())
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}
	defer app.Close()
}
