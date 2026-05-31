package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/shopspring/decimal"
)

type ExchangeClient struct {
	baseURL string
	http    *http.Client
}

func NewExchangeClient() *ExchangeClient {
	baseURL := os.Getenv("SERVICES_EXCHANGE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8083"
	}

	return &ExchangeClient{
		baseURL: baseURL,
		http:    &http.Client{},
	}
}

type ConversionResponse struct {
	FromCurrency string          `json:"fromCurrency"`
	ToCurrency   string          `json:"toCurrency"`
	FromAmount   decimal.Decimal `json:"fromAmount"`
	ToAmount     decimal.Decimal `json:"toAmount"`
	Rate         decimal.Decimal `json:"rate"`
	Commission   decimal.Decimal `json:"commission"`
}

func (c *ExchangeClient) Calculate(
	fromCurrency string,
	toCurrency string,
	amount decimal.Decimal,
) (ConversionResponse, error) {

	url := fmt.Sprintf(
		"%s/calculate?fromCurrency=%s&toCurrency=%s&amount=%s",
		c.baseURL,
		fromCurrency,
		toCurrency,
		amount.String(),
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return ConversionResponse{}, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return ConversionResponse{}, err
	}
	defer resp.Body.Close()

	var result ConversionResponse

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return ConversionResponse{}, err
	}

	return result, nil
}
