package client_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Banka1Back/credit-service-go/internal/client"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExchangeClient_Calculate_Success(t *testing.T) {
	expected := client.ConversionResponse{
		FromCurrency: "EUR",
		ToCurrency:   "RSD",
		FromAmount:   decimal.NewFromInt(1000),
		ToAmount:     decimal.NewFromInt(117000),
		Rate:         decimal.NewFromFloat(117.0),
		Commission:   decimal.NewFromFloat(0.5),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.String(), "fromCurrency=EUR")
		assert.Contains(t, r.URL.String(), "toCurrency=RSD")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	t.Setenv("SERVICES_EXCHANGE_URL", server.URL)
	c := client.NewExchangeClient()

	resp, err := c.Calculate("EUR", "RSD", decimal.NewFromInt(1000))
	require.NoError(t, err)
	assert.Equal(t, "EUR", resp.FromCurrency)
	assert.Equal(t, "RSD", resp.ToCurrency)
	assert.True(t, resp.ToAmount.Equal(decimal.NewFromInt(117000)))
}

func TestExchangeClient_Calculate_InvalidJSON_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{bad json"))
	}))
	defer server.Close()

	t.Setenv("SERVICES_EXCHANGE_URL", server.URL)
	c := client.NewExchangeClient()

	_, err := c.Calculate("EUR", "RSD", decimal.NewFromInt(1000))
	require.Error(t, err)
}

func TestExchangeClient_Calculate_NetworkError_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	t.Setenv("SERVICES_EXCHANGE_URL", server.URL)
	c := client.NewExchangeClient()

	_, err := c.Calculate("EUR", "RSD", decimal.NewFromInt(100))
	require.Error(t, err)
}

func TestNewExchangeClient_DefaultURL(t *testing.T) {
	t.Setenv("SERVICES_EXCHANGE_URL", "")
	c := client.NewExchangeClient()
	require.NotNil(t, c)
}

func TestNewExchangeClient_CustomURL(t *testing.T) {
	t.Setenv("SERVICES_EXCHANGE_URL", "http://exchange-host:8083")
	c := client.NewExchangeClient()
	require.NotNil(t, c)
}
