package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type TwelveDataClient struct {
	baseURL string
	apiKey  string
	client  HTTPDoer
}

type TwelveRate struct {
	FromCurrency string
	ToCurrency   string
	Rate         decimal.Decimal
	Date         time.Time
}

func NewTwelveDataClient(baseURL, apiKey string, client HTTPDoer) *TwelveDataClient {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &TwelveDataClient{baseURL: baseURL, apiKey: apiKey, client: client}
}

func (c *TwelveDataClient) FetchExchangeRate(ctx context.Context, from, to string) (*TwelveRate, error) {
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/exchange_rate"
	query := endpoint.Query()
	query.Set("symbol", from+"/"+to)
	query.Set("apikey", c.apiKey)
	endpoint.RawQuery = query.Encode()
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
		return nil, fmt.Errorf("twelve data status %d", resp.StatusCode)
	}
	body, err := ioReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if raw["code"] != nil || raw["message"] != nil || raw["status"] != nil {
		return nil, fmt.Errorf("%v", raw["message"])
	}
	symbol, _ := raw["symbol"].(string)
	if !strings.EqualFold(symbol, from+"/"+to) {
		return nil, fmt.Errorf("unexpected currency pair %s", symbol)
	}
	rate, err := decimal.NewFromString(fmt.Sprint(raw["rate"]))
	if err != nil {
		return nil, err
	}
	date := time.Now().UTC()
	if raw["timestamp"] != nil {
		if secs, ok := raw["timestamp"].(float64); ok {
			date = time.Unix(int64(secs), 0).UTC()
		}
	}
	return &TwelveRate{FromCurrency: from, ToCurrency: to, Rate: rate, Date: time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)}, nil
}

