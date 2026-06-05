package client_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Banka1Back/credit-service-go/internal/client"
	"Banka1Back/credit-service-go/internal/model"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountClient_GetDetails_Success(t *testing.T) {
	expected := client.AccountDetailsResponse{
		OwnerID:  42,
		Currency: model.CurrencyRSD,
		Email:    "user@test.com",
		Username: "testuser",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	t.Setenv("SERVICES_ACCOUNT_URL", server.URL)
	c := client.NewAccountClient()

	resp, err := c.GetDetails("1234567890123456789")
	require.NoError(t, err)
	assert.Equal(t, int64(42), resp.OwnerID)
	assert.Equal(t, model.CurrencyRSD, resp.Currency)
	assert.Equal(t, "user@test.com", resp.Email)
	assert.Equal(t, "testuser", resp.Username)
}

func TestAccountClient_GetDetails_NonSuccessStatus_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("SERVICES_ACCOUNT_URL", server.URL)
	c := client.NewAccountClient()

	_, err := c.GetDetails("1234567890123456789")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account-service get details failed")
}

func TestAccountClient_GetDetails_InvalidJSON_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not-json{"))
	}))
	defer server.Close()

	t.Setenv("SERVICES_ACCOUNT_URL", server.URL)
	c := client.NewAccountClient()

	_, err := c.GetDetails("123")
	require.Error(t, err)
}

func TestAccountClient_GetDetails_ServerError_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("SERVICES_ACCOUNT_URL", server.URL)
	c := client.NewAccountClient()

	_, err := c.GetDetails("123")
	require.Error(t, err)
}

func TestAccountClient_TransactionFromBank_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("SERVICES_ACCOUNT_URL", server.URL)
	c := client.NewAccountClient()

	err := c.TransactionFromBank("1234567890123456789", decimal.NewFromInt(1000))
	require.NoError(t, err)
}

func TestAccountClient_TransactionFromBank_FailedStatus_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	t.Setenv("SERVICES_ACCOUNT_URL", server.URL)
	c := client.NewAccountClient()

	err := c.TransactionFromBank("1234567890123456789", decimal.NewFromInt(1000))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transactionFromBank failed")
}

func TestAccountClient_TransactionFromBank_InternalError_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("SERVICES_ACCOUNT_URL", server.URL)
	c := client.NewAccountClient()

	err := c.TransactionFromBank("123", decimal.NewFromInt(500))
	require.Error(t, err)
}

func TestAccountClient_TransactionToBank_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("SERVICES_ACCOUNT_URL", server.URL)
	c := client.NewAccountClient()

	err := c.TransactionToBank("1234567890123456789", decimal.NewFromInt(2000))
	require.NoError(t, err)
}

func TestAccountClient_TransactionToBank_FailedStatus_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	t.Setenv("SERVICES_ACCOUNT_URL", server.URL)
	c := client.NewAccountClient()

	err := c.TransactionToBank("1234567890123456789", decimal.NewFromInt(2000))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transactionToBank failed")
}

func TestNewAccountClient_DefaultURL(t *testing.T) {
	t.Setenv("SERVICES_ACCOUNT_URL", "")
	c := client.NewAccountClient()
	require.NotNil(t, c)
}

func TestNewAccountClient_CustomURL(t *testing.T) {
	t.Setenv("SERVICES_ACCOUNT_URL", "http://custom-host:9090")
	c := client.NewAccountClient()
	require.NotNil(t, c)
}
