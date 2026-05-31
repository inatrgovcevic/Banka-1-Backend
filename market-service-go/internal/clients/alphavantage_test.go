package clients

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestAlphaVantageFetchQuoteParsesGlobalQuote(t *testing.T) {
	client := NewAlphaVantageClient("https://example.com", "key", roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("function") != "GLOBAL_QUOTE" {
			t.Fatalf("unexpected function: %s", req.URL.RawQuery)
		}
		body := `{"Global Quote":{"01. symbol":"IBM","02. open":"150.00","05. price":"151.30","06. volume":"1234567","07. latest trading day":"2026-05-08","08. previous close":"150.10","09. change":"1.20","10. change percent":"0.7995%"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	}))

	quote, err := client.FetchQuote(context.Background(), "IBM")
	if err != nil {
		t.Fatalf("FetchQuote returned error: %v", err)
	}
	if quote == nil || quote.Symbol != "IBM" || quote.ChangePercent.String() != "0.7995" || quote.Volume != 1234567 {
		t.Fatalf("unexpected quote: %+v", quote)
	}
}

func TestAlphaVantageFetchExchangeRateParsesPayload(t *testing.T) {
	client := NewAlphaVantageClient("https://example.com", "key", roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"Realtime Currency Exchange Rate":{"1. From_Currency Code":"USD","3. To_Currency Code":"RSD","5. Exchange Rate":"107.1234","6. Last Refreshed":"2026-05-08 14:12:00"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	}))

	rate, err := client.FetchExchangeRate(context.Background(), "USD", "RSD")
	if err != nil {
		t.Fatalf("FetchExchangeRate returned error: %v", err)
	}
	if rate == nil || rate.ExchangeRate.String() != "107.1234" || rate.FromCurrencyCode != "USD" || rate.ToCurrencyCode != "RSD" {
		t.Fatalf("unexpected rate: %+v", rate)
	}
}
