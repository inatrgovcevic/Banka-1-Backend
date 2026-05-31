package market

import (
	"context"
	"errors"
	"testing"
	"time"

	"banka1/market-service-go/internal/platform"
	"log/slog"
	"github.com/shopspring/decimal"
)

type fakePriceFeedCache struct {
	item     *StockPriceSnapshot
	getErr   error
	setErr   error
	setCalls int
}

func (f *fakePriceFeedCache) Get(context.Context, string) (*StockPriceSnapshot, error) {
	return f.item, f.getErr
}

func (f *fakePriceFeedCache) Set(context.Context, string, StockPriceSnapshot, time.Duration) error {
	f.setCalls++
	return f.setErr
}

func TestPriceFeedFallsBackToDevSnapshotWhenProviderUnavailable(t *testing.T) {
	service := NewPriceFeedService(platform.Config{Stock: platform.StockConfig{PriceFeedTTL: time.Second}}, slog.Default())
	snapshot, ok := service.GetSingle(context.Background(), "aapl")
	if !ok {
		t.Fatal("expected snapshot")
	}
	if snapshot.Ticker != "AAPL" || !snapshot.CurrentPrice.Equal(decimal.RequireFromString("150.25")) || !snapshot.ChangePercent.Equal(decimal.RequireFromString("0.50")) {
		t.Fatalf("unexpected fallback snapshot: %+v", snapshot)
	}
}

func TestGroupOptionsSeparatesCallsAndPutsBySettlementDate(t *testing.T) {
	options := []OptionRow{
		{ID: 1, Ticker: "AAPL240621C00180000", OptionType: "CALL", StrikePrice: "180", ImpliedVolatility: "0.20", OpenInterest: 10, LastPrice: "2", Bid: "1.9", Ask: "2.1", Volume: 3, SettlementDate: time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)},
		{ID: 2, Ticker: "AAPL240621P00170000", OptionType: "PUT", StrikePrice: "170", ImpliedVolatility: "0.19", OpenInterest: 12, LastPrice: "1.5", Bid: "1.4", Ask: "1.6", Volume: 4, SettlementDate: time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)},
	}
	grouped := groupOptions(options, calculateDollarVolume("1", 175))
	if len(grouped) != 1 || len(grouped[0].Calls) != 1 || len(grouped[0].Puts) != 1 {
		t.Fatalf("unexpected grouped options: %+v", grouped)
	}
}

func TestPriceFeedReturnsRedisHitBeforeFallback(t *testing.T) {
	cache := &fakePriceFeedCache{
		item: &StockPriceSnapshot{
			Ticker:        "AAPL",
			CurrentPrice:  decimal.RequireFromString("111.11"),
			OpenPrice:     decimal.RequireFromString("110.00"),
			PreviousClose: decimal.RequireFromString("109.50"),
			ChangePercent: decimal.RequireFromString("1.47"),
			Volume:        99,
			Currency:      "USD",
			Timestamp:     time.Now().UTC(),
		},
	}
	service := NewPriceFeedService(platform.Config{Stock: platform.StockConfig{PriceFeedTTL: time.Second}}, slog.Default())
	service.SetCache(cache)
	snapshot, ok := service.GetSingle(context.Background(), "AAPL")
	if !ok || !snapshot.CurrentPrice.Equal(decimal.RequireFromString("111.11")) {
		t.Fatalf("expected redis hit snapshot, got %+v", snapshot)
	}
}

func TestPriceFeedFallsBackToLocalCacheWhenRedisUnavailable(t *testing.T) {
	cache := &fakePriceFeedCache{getErr: errors.New("redis down"), setErr: errors.New("redis down")}
	service := NewPriceFeedService(platform.Config{Stock: platform.StockConfig{PriceFeedTTL: time.Minute}}, slog.Default())
	service.SetCache(cache)
	first, _ := service.GetSingle(context.Background(), "AAPL")
	second, _ := service.GetSingle(context.Background(), "AAPL")
	if cache.setCalls == 0 {
		t.Fatal("expected redis set attempt before local fallback")
	}
	if !first.CurrentPrice.Equal(second.CurrentPrice) {
		t.Fatalf("expected local-cache hit after redis failure: first=%+v second=%+v", first, second)
	}
}

func TestPriceFeedRedisMissFallsThroughToProvider(t *testing.T) {
	cache := &fakePriceFeedCache{item: nil}
	service := NewPriceFeedService(platform.Config{Stock: platform.StockConfig{PriceFeedTTL: time.Minute}}, slog.Default())
	service.SetCache(cache)
	snapshot, ok := service.GetSingle(context.Background(), "AAPL")
	if !ok {
		t.Fatal("expected snapshot from provider/dev-mock fallback after redis miss")
	}
	if snapshot.Ticker != "AAPL" {
		t.Fatalf("expected upper-cased ticker, got %q", snapshot.Ticker)
	}
	if cache.setCalls != 1 {
		t.Fatalf("expected dev-mock write to redis after miss, got setCalls=%d", cache.setCalls)
	}
}

func TestPriceFeedCurrentMultiTickerNormalizesAndFiltersEmpty(t *testing.T) {
	service := NewPriceFeedService(platform.Config{Stock: platform.StockConfig{PriceFeedTTL: time.Minute}}, slog.Default())
	snapshots := service.GetCurrent(context.Background(), []string{"aapl", " msft ", "", "GOOG"})
	if len(snapshots) != 3 {
		t.Fatalf("expected 3 non-empty snapshots, got %d", len(snapshots))
	}
	for _, snap := range snapshots {
		if snap.Ticker != "AAPL" && snap.Ticker != "MSFT" && snap.Ticker != "GOOG" {
			t.Fatalf("ticker should be normalised to upper case, got %q", snap.Ticker)
		}
		if snap.Currency != "USD" {
			t.Fatalf("expected currency USD, got %q", snap.Currency)
		}
	}
}

func TestPriceFeedRedisDisabledUsesLocalCache(t *testing.T) {
	service := NewPriceFeedService(platform.Config{Stock: platform.StockConfig{PriceFeedTTL: time.Minute}}, slog.Default())
	if service.cache2 != nil {
		t.Fatalf("expected nil redis cache when RedisHost is empty, got %T", service.cache2)
	}
	first, _ := service.GetSingle(context.Background(), "TSLA")
	second, _ := service.GetSingle(context.Background(), "TSLA")
	if !first.CurrentPrice.Equal(second.CurrentPrice) {
		t.Fatalf("expected stable local cache hit, got %+v vs %+v", first, second)
	}
}
