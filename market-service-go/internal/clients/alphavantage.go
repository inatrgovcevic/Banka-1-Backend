package clients

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type AlphaVantageClient struct {
	baseURL string
	apiKey  string
	client  HTTPDoer
}

type Quote struct {
	Symbol           string
	Open             decimal.Decimal
	Price            decimal.Decimal
	PreviousClose    decimal.Decimal
	Change           decimal.Decimal
	ChangePercent    decimal.Decimal
	Volume           int64
	LatestTradingDay time.Time
	Bid              decimal.Decimal
	Ask              decimal.Decimal
}

type ForexExchangeRate struct {
	FromCurrencyCode string
	ToCurrencyCode   string
	ExchangeRate     decimal.Decimal
	LastRefreshed    time.Time
}

type CompanyOverview struct {
	Name              string
	SharesOutstanding int64
	DividendYield     decimal.Decimal
}

type DailyValue struct {
	Date       time.Time
	ClosePrice decimal.Decimal
	Volume     int64
}

func NewAlphaVantageClient(baseURL, apiKey string, client HTTPDoer) *AlphaVantageClient {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &AlphaVantageClient{baseURL: baseURL, apiKey: apiKey, client: client}
}

func (c *AlphaVantageClient) FetchQuote(ctx context.Context, ticker string) (*Quote, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, nil
	}
	body, err := c.fetch(ctx, map[string]string{
		"function": "GLOBAL_QUOTE",
		"symbol":   ticker,
		"apikey":   c.apiKey,
	})
	if err != nil {
		return nil, err
	}
	var payload map[string]map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	quoteRaw := payload["Global Quote"]
	if len(quoteRaw) == 0 {
		return nil, nil
	}
	price, err := decimal.NewFromString(quoteRaw["05. price"])
	if err != nil {
		return nil, nil
	}
	open, _ := decimal.NewFromString(quoteRaw["02. open"])
	prev, _ := decimal.NewFromString(quoteRaw["08. previous close"])
	change, _ := decimal.NewFromString(quoteRaw["09. change"])
	changePct, _ := decimal.NewFromString(strings.TrimSuffix(quoteRaw["10. change percent"], "%"))
	volume, _ := strconv.ParseInt(strings.TrimSpace(quoteRaw["06. volume"]), 10, 64)
	day, _ := time.Parse("2006-01-02", quoteRaw["07. latest trading day"])
	return &Quote{
		Symbol:           quoteRaw["01. symbol"],
		Open:             open,
		Price:            price,
		PreviousClose:    prev,
		Change:           change,
		ChangePercent:    changePct,
		Volume:           volume,
		LatestTradingDay: day,
		Bid:              price,
		Ask:              price,
	}, nil
}

func (c *AlphaVantageClient) FetchExchangeRate(ctx context.Context, from, to string) (*ForexExchangeRate, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, nil
	}
	body, err := c.fetch(ctx, map[string]string{
		"function":      "CURRENCY_EXCHANGE_RATE",
		"from_currency": from,
		"to_currency":   to,
		"apikey":        c.apiKey,
	})
	if err != nil {
		return nil, err
	}
	var payload map[string]map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	raw := payload["Realtime Currency Exchange Rate"]
	if len(raw) == 0 {
		return nil, nil
	}
	rate, err := decimal.NewFromString(raw["5. Exchange Rate"])
	if err != nil {
		return nil, nil
	}
	lastRefreshed, _ := time.Parse("2006-01-02 15:04:05", raw["6. Last Refreshed"])
	return &ForexExchangeRate{
		FromCurrencyCode: raw["1. From_Currency Code"],
		ToCurrencyCode:   raw["3. To_Currency Code"],
		ExchangeRate:     rate,
		LastRefreshed:    lastRefreshed,
	}, nil
}

func (c *AlphaVantageClient) FetchCompanyOverview(ctx context.Context, ticker string) (*CompanyOverview, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, nil
	}
	body, err := c.fetch(ctx, map[string]string{
		"function": "OVERVIEW",
		"symbol":   ticker,
		"apikey":   c.apiKey,
	})
	if err != nil {
		return nil, err
	}
	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if len(payload) == 0 || payload["Symbol"] == "" {
		return nil, nil
	}
	shares, _ := strconv.ParseInt(payload["SharesOutstanding"], 10, 64)
	yield, _ := decimal.NewFromString(payload["DividendYield"])
	return &CompanyOverview{Name: payload["Name"], SharesOutstanding: shares, DividendYield: yield}, nil
}

func (c *AlphaVantageClient) FetchDaily(ctx context.Context, ticker string) ([]DailyValue, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, nil
	}
	body, err := c.fetch(ctx, map[string]string{
		"function": "TIME_SERIES_DAILY",
		"symbol":   ticker,
		"apikey":   c.apiKey,
		"outputsize": "compact",
	})
	if err != nil {
		return nil, err
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	var series map[string]map[string]string
	if err := json.Unmarshal(payload["Time Series (Daily)"], &series); err != nil {
		return nil, nil
	}
	out := make([]DailyValue, 0, len(series))
	for dayText, row := range series {
		day, err := time.Parse("2006-01-02", dayText)
		if err != nil {
			continue
		}
		closePrice, err := decimal.NewFromString(row["4. close"])
		if err != nil {
			continue
		}
		volume, _ := strconv.ParseInt(row["5. volume"], 10, 64)
		out = append(out, DailyValue{Date: day, ClosePrice: closePrice, Volume: volume})
	}
	return out, nil
}

func (c *AlphaVantageClient) fetch(ctx context.Context, query map[string]string) ([]byte, error) {
	if c.baseURL == "" {
		return nil, errors.New("missing base URL")
	}
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/query"
	params := endpoint.Query()
	for key, value := range query {
		params.Set(key, value)
	}
	endpoint.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("alpha vantage status %d", resp.StatusCode)
	}
	return ioReadAll(resp.Body)
}

