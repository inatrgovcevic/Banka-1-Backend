package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/shared/auth"
)

// MarketServiceClient calls the market-service (which consolidates the
// exchange-service).
//
// Design note: The Java MarketServiceClient had dead stock reservation /
// ownership transfer methods that were superseded by TradingServiceClient
// (PR_15 C15.3). The Go port retains ONLY the one method still in use:
// ConvertCurrencyNoCommission — used by OtcPremiumTransferSaga to convert the
// USD premium to RSD before the internal banking-core transfer.
//
// Endpoint:
//
//	GET /internal/calculate/no-commission?fromCurrency=&toCurrency=&amount=
type MarketServiceClient struct {
	baseURL string
	issuer  *auth.S2SIssuer
	hc      *http.Client
}

// NewMarketServiceClient builds a client. If timeout <= 0 it defaults to 5s.
func NewMarketServiceClient(baseURL string, issuer *auth.S2SIssuer, timeout time.Duration) *MarketServiceClient {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &MarketServiceClient{
		baseURL: baseURL,
		issuer:  issuer,
		hc:      &http.Client{Timeout: timeout},
	}
}

// ConversionResult is the response from the no-commission conversion endpoint.
type ConversionResult struct {
	FromCurrency string          `json:"fromCurrency"`
	ToCurrency   string          `json:"toCurrency"`
	FromAmount   decimal.Decimal `json:"fromAmount"`
	ToAmount     decimal.Decimal `json:"toAmount"`
	Rate         decimal.Decimal `json:"rate"`
	Commission   decimal.Decimal `json:"commission"`
}

// ConvertCurrencyNoCommission converts amount from fromCurrency to toCurrency
// using the exchange-service mid-rate (no commission charged). This is used
// in OtcPremiumTransferSaga to convert a USD premium to RSD so that the
// banking-core default RSD accounts can be used for the transfer.
//
// Returns the converted decimal amount directly (saga.MarketActions interface).
func (c *MarketServiceClient) ConvertCurrencyNoCommission(
	ctx context.Context,
	fromCurrency, toCurrency string,
	amount decimal.Decimal,
) (decimal.Decimal, error) {
	u := fmt.Sprintf(
		"%s/internal/calculate/no-commission?fromCurrency=%s&toCurrency=%s&amount=%s",
		c.baseURL,
		url.QueryEscape(fromCurrency),
		url.QueryEscape(toCurrency),
		url.QueryEscape(amount.String()),
	)
	req, err := buildReq(ctx, http.MethodGet, u, nil)
	if err != nil {
		return decimal.Zero, fmt.Errorf("market.ConvertCurrencyNoCommission: build: %w", err)
	}
	tok, err := c.issuer.IssueToken()
	if err != nil {
		return decimal.Zero, fmt.Errorf("market.ConvertCurrencyNoCommission: issue S2S token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	var result ConversionResult
	if err := execReq(c.hc, req, &result); err != nil {
		return decimal.Zero, fmt.Errorf("market.ConvertCurrencyNoCommission(%s->%s): %w", fromCurrency, toCurrency, err)
	}
	if result.ToAmount.IsZero() {
		return decimal.Zero, fmt.Errorf("market.ConvertCurrencyNoCommission: exchange service returned zero toAmount for %s->%s", fromCurrency, toCurrency)
	}
	return result.ToAmount, nil
}
