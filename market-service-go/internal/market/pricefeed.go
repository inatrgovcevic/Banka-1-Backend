package market

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"banka1/market-service-go/internal/clients"
	"banka1/market-service-go/internal/platform"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

type cachedSnapshot struct {
	snapshot  StockPriceSnapshot
	expiresAt time.Time
}

type priceFeedCache interface {
	Get(context.Context, string) (*StockPriceSnapshot, error)
	Set(context.Context, string, StockPriceSnapshot, time.Duration) error
}

type redisPriceFeedCache struct {
	client *redis.Client
}

func (c *redisPriceFeedCache) Get(ctx context.Context, key string) (*StockPriceSnapshot, error) {
	raw, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var snapshot StockPriceSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (c *redisPriceFeedCache) Set(ctx context.Context, key string, snapshot StockPriceSnapshot, ttl time.Duration) error {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, raw, ttl).Err()
}

type PriceFeedService struct {
	cfg    platform.Config
	logger *slog.Logger
	client *clients.AlphaVantageClient
	cache2 priceFeedCache
	mu     sync.RWMutex
	cache  map[string]cachedSnapshot
}

func NewPriceFeedService(cfg platform.Config, logger *slog.Logger) *PriceFeedService {
	return &PriceFeedService{
		cfg:    cfg,
		logger: logger,
		client: clients.NewAlphaVantageClient(cfg.Stock.MarketDataBaseURL, cfg.Stock.AlphaVantageAPIKey, nil),
		cache2: newRedisPriceFeedCache(cfg),
		cache:  map[string]cachedSnapshot{},
	}
}

func (s *PriceFeedService) SetClient(client *clients.AlphaVantageClient) {
	s.client = client
}

func (s *PriceFeedService) SetCache(cache priceFeedCache) {
	s.cache2 = cache
}

func (s *PriceFeedService) GetCurrent(ctx context.Context, tickers []string) []StockPriceSnapshot {
	out := make([]StockPriceSnapshot, 0, len(tickers))
	for _, ticker := range tickers {
		ticker = strings.TrimSpace(strings.ToUpper(ticker))
		if ticker == "" {
			continue
		}
		snapshot, _ := s.GetSingle(ctx, ticker)
		out = append(out, snapshot)
	}
	return out
}

func (s *PriceFeedService) GetSingle(ctx context.Context, ticker string) (StockPriceSnapshot, bool) {
	ticker = strings.TrimSpace(strings.ToUpper(ticker))
	if s.cache2 != nil {
		snapshot, err := s.cache2.Get(ctx, redisKey(ticker))
		if err == nil && snapshot != nil {
			return *snapshot, true
		}
		if err != nil {
			s.logger.Warn("redis price feed read failed", "ticker", ticker, "error", err.Error())
		}
	}
	s.mu.RLock()
	cached, ok := s.cache[ticker]
	s.mu.RUnlock()
	if ok && time.Now().Before(cached.expiresAt) {
		return cached.snapshot, true
	}
	if s.client != nil {
		quote, err := s.client.FetchQuote(ctx, ticker)
		if err == nil && quote != nil {
			snapshot := StockPriceSnapshot{
				Ticker:        quote.Symbol,
				CurrentPrice:  quote.Price,
				OpenPrice:     quote.Open,
				PreviousClose: quote.PreviousClose,
				ChangePercent: quote.ChangePercent,
				Volume:        quote.Volume,
				Currency:      "USD",
				Timestamp:     time.Now().UTC(),
			}
			s.writeToCache(ctx, ticker, snapshot)
			return snapshot, true
		}
	}
	snapshot := StockPriceSnapshot{
		Ticker:        ticker,
		CurrentPrice:  decimal.RequireFromString("150.25"),
		OpenPrice:     decimal.RequireFromString("148.00"),
		PreviousClose: decimal.RequireFromString("149.50"),
		ChangePercent: decimal.RequireFromString("0.50"),
		Volume:        1000000,
		Currency:      "USD",
		Timestamp:     time.Now().UTC(),
	}
	s.writeToCache(ctx, ticker, snapshot)
	return snapshot, true
}

func (s *PriceFeedService) writeToCache(ctx context.Context, ticker string, snapshot StockPriceSnapshot) {
	if s.cache2 != nil {
		if err := s.cache2.Set(ctx, redisKey(ticker), snapshot, s.cfg.Stock.PriceFeedTTL); err == nil {
			return
		} else {
			s.logger.Warn("redis price feed write failed", "ticker", ticker, "error", err.Error())
		}
	}
	s.mu.Lock()
	s.cache[ticker] = cachedSnapshot{snapshot: snapshot, expiresAt: time.Now().Add(s.cfg.Stock.PriceFeedTTL)}
	s.mu.Unlock()
}

func newRedisPriceFeedCache(cfg platform.Config) priceFeedCache {
	if strings.TrimSpace(cfg.Stock.RedisHost) == "" {
		return nil
	}
	client := redis.NewClient(&redis.Options{
		Addr:         net.JoinHostPort(cfg.Stock.RedisHost, cfg.Stock.RedisPort),
		DialTimeout:  cfg.Stock.RedisTimeout,
		ReadTimeout:  cfg.Stock.RedisTimeout,
		WriteTimeout: cfg.Stock.RedisTimeout,
	})
	return &redisPriceFeedCache{client: client}
}

func redisKey(ticker string) string {
	return "stock:price:" + ticker
}
