package clients

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestTwelveDataFetchExchangeRateParsesPayload(t *testing.T) {
	client := NewTwelveDataClient("https://example.com", "key", roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("symbol") != "USD/RSD" {
			t.Fatalf("unexpected symbol query: %s", req.URL.RawQuery)
		}
		body := `{"symbol":"USD/RSD","rate":"107.55","timestamp":1780000000}`
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
	if rate == nil || rate.Rate.String() != "107.55" || rate.FromCurrency != "USD" || rate.ToCurrency != "RSD" {
		t.Fatalf("unexpected rate: %+v", rate)
	}
}
