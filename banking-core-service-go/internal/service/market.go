package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"banka1/banking-core-service-go/internal/config"
	"banka1/banking-core-service-go/internal/decimal"
)

type MarketClient struct {
	cfg        config.Config
	client     *http.Client
	tokenCache *ServiceTokenCache
}

type ConversionResponse struct {
	FromCurrency string          `json:"fromCurrency"`
	ToCurrency   string          `json:"toCurrency"`
	FromAmount   decimal.Decimal `json:"fromAmount"`
	ToAmount     decimal.Decimal `json:"toAmount"`
	Rate         decimal.Decimal `json:"rate"`
	Commission   decimal.Decimal `json:"commission"`
}

func NewMarketClient(cfg config.Config, client *http.Client) *MarketClient {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &MarketClient{cfg: cfg, client: client, tokenCache: NewServiceTokenCache(cfg)}
}

func (m *MarketClient) Convert(ctx context.Context, amount decimal.Decimal, from, to string) (ConversionResponse, error) {
	return m.get(ctx, "/calculate", amount, from, to)
}

func (m *MarketClient) ConvertNoCommission(ctx context.Context, amount decimal.Decimal, from, to string) (ConversionResponse, error) {
	return m.get(ctx, "/internal/calculate/no-commission", amount, from, to)
}

func (m *MarketClient) get(ctx context.Context, path string, amount decimal.Decimal, from, to string) (ConversionResponse, error) {
	base := strings.TrimRight(m.cfg.MarketServiceURL, "/")
	u, err := url.Parse(base + path)
	if err != nil {
		return ConversionResponse{}, err
	}
	q := u.Query()
	q.Set("fromCurrency", from)
	q.Set("toCurrency", to)
	q.Set("amount", amount.String())
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return ConversionResponse{}, err
	}
	if token, err := m.tokenCache.Token(); err == nil {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return ConversionResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ConversionResponse{}, BadRequest("Market-service FX poziv nije uspeo: HTTP %d", resp.StatusCode)
	}
	var out ConversionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ConversionResponse{}, err
	}
	return out, nil
}
