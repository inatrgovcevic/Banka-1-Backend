package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/shared/auth"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/client"
)

// testIssuer returns a dummy S2SIssuer suitable for tests — the token is
// never validated server-side in these httptest servers.
func testIssuer() *auth.S2SIssuer {
	return auth.NewS2SIssuer("test", "saga-test", []string{"SERVICE"}, "test-secret", 10*time.Minute)
}

// ---------------------------------------------------------------------------
// BankingCoreClient
// ---------------------------------------------------------------------------

func TestBankingCoreClient_ReserveFunds_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/transactions/internal/reserve-funds" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("X-Correlation-Id") != "corr-1" {
			t.Errorf("missing X-Correlation-Id")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(client.ReservationResult{ReservationID: "res-abc", Status: "HELD"})
	}))
	defer srv.Close()

	c := client.NewBankingCoreClient(srv.URL, testIssuer(), 5*time.Second)
	id, err := c.ReserveFunds(context.Background(), 1, decimal.RequireFromString("500.00"), "corr-1")
	if err != nil {
		t.Fatalf("ReserveFunds error: %v", err)
	}
	if id != "res-abc" {
		t.Errorf("reservationId=%q, want res-abc", id)
	}
}

func TestBankingCoreClient_ReserveFunds_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"insufficient funds"}`))
	}))
	defer srv.Close()

	c := client.NewBankingCoreClient(srv.URL, testIssuer(), 5*time.Second)
	_, err := c.ReserveFunds(context.Background(), 1, decimal.RequireFromString("100"), "corr-2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestBankingCoreClient_ReleaseFunds_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := client.NewBankingCoreClient(srv.URL, testIssuer(), 5*time.Second)
	if err := c.ReleaseFunds(context.Background(), "res-xyz", "corr-3"); err != nil {
		t.Fatalf("ReleaseFunds error: %v", err)
	}
}

func TestBankingCoreClient_InternalTransfer_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/transactions/internal/transfer" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(client.TransferResult{TransferID: "tr-001", Status: "COMPLETED"})
	}))
	defer srv.Close()

	c := client.NewBankingCoreClient(srv.URL, testIssuer(), 5*time.Second)
	id, err := c.InternalTransfer(context.Background(),
		"111000100000000001", "111000200000000002",
		decimal.RequireFromString("1000.00"), "corr-4",
	)
	if err != nil {
		t.Fatalf("InternalTransfer error: %v", err)
	}
	if id != "tr-001" {
		t.Errorf("transferId=%q, want tr-001", id)
	}
}

func TestBankingCoreClient_ResolveDefaultAccountNumber_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"accountNumber": "111000111111111101"})
	}))
	defer srv.Close()

	c := client.NewBankingCoreClient(srv.URL, testIssuer(), 5*time.Second)
	acct, err := c.ResolveDefaultAccountNumber(context.Background(), 42)
	if err != nil {
		t.Fatalf("ResolveDefaultAccountNumber error: %v", err)
	}
	if acct != "111000111111111101" {
		t.Errorf("account=%q, want 111000111111111101", acct)
	}
}

func TestBankingCoreClient_ResolveDefaultAccountNumber_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := client.NewBankingCoreClient(srv.URL, testIssuer(), 5*time.Second)
	_, err := c.ResolveDefaultAccountNumber(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

// ---------------------------------------------------------------------------
// TradingServiceClient
// ---------------------------------------------------------------------------

func TestTradingServiceClient_ReserveStocks_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stocks/internal/reserve" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(client.StockReservationResult{ReservationID: "sr-99", Status: "RESERVED"})
	}))
	defer srv.Close()

	tc := client.NewTradingServiceClient(srv.URL, testIssuer(), 5*time.Second)
	id, err := tc.ReserveStocks(context.Background(), 5, "AAPL", 10, "corr-5")
	if err != nil {
		t.Fatalf("ReserveStocks error: %v", err)
	}
	if id != "sr-99" {
		t.Errorf("reservationId=%q, want sr-99", id)
	}
}

func TestTradingServiceClient_TransferOwnership_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(client.OwnershipTransferResult{OwnershipTransferID: "ot-77"})
	}))
	defer srv.Close()

	tc := client.NewTradingServiceClient(srv.URL, testIssuer(), 5*time.Second)
	id, err := tc.TransferOwnership(context.Background(), "sr-99", 7, "corr-6")
	if err != nil {
		t.Fatalf("TransferOwnership error: %v", err)
	}
	if id != "ot-77" {
		t.Errorf("ownershipTransferId=%q, want ot-77", id)
	}
}

func TestTradingServiceClient_LiquidateForFund_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(client.LiquidationResult{
			LiquidationID:    "liq-11",
			LiquidatedAmount: decimal.RequireFromString("5000.00"),
			HoldingsSold:     3,
		})
	}))
	defer srv.Close()

	tc := client.NewTradingServiceClient(srv.URL, testIssuer(), 5*time.Second)
	// LiquidateForFund now returns (liquidationID string, error) to satisfy TradingActions.
	liqID, err := tc.LiquidateForFund(context.Background(), 1, decimal.RequireFromString("5000"), "corr-7")
	if err != nil {
		t.Fatalf("LiquidateForFund error: %v", err)
	}
	if liqID != "liq-11" {
		t.Errorf("liquidationId=%q, want liq-11", liqID)
	}
}

// ---------------------------------------------------------------------------
// MarketServiceClient
// ---------------------------------------------------------------------------

func TestMarketServiceClient_ConvertCurrencyNoCommission_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/calculate/no-commission" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(client.ConversionResult{
			FromCurrency: "USD",
			ToCurrency:   "RSD",
			FromAmount:   decimal.RequireFromString("100"),
			ToAmount:     decimal.RequireFromString("11000"),
			Rate:         decimal.RequireFromString("110"),
		})
	}))
	defer srv.Close()

	mc := client.NewMarketServiceClient(srv.URL, testIssuer(), 5*time.Second)
	// ConvertCurrencyNoCommission now returns (decimal.Decimal, error) to satisfy MarketActions.
	toAmount, err := mc.ConvertCurrencyNoCommission(context.Background(), "USD", "RSD", decimal.RequireFromString("100"))
	if err != nil {
		t.Fatalf("ConvertCurrencyNoCommission error: %v", err)
	}
	if !toAmount.Equal(decimal.RequireFromString("11000")) {
		t.Errorf("toAmount=%s, want 11000", toAmount)
	}
}

func TestMarketServiceClient_ConvertCurrencyNoCommission_ZeroToAmountError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(client.ConversionResult{
			FromCurrency: "USD",
			ToCurrency:   "RSD",
			ToAmount:     decimal.Zero,
		})
	}))
	defer srv.Close()

	mc := client.NewMarketServiceClient(srv.URL, testIssuer(), 5*time.Second)
	_, err := mc.ConvertCurrencyNoCommission(context.Background(), "USD", "RSD", decimal.RequireFromString("100"))
	if err == nil {
		t.Fatal("expected error for zero toAmount, got nil")
	}
}
