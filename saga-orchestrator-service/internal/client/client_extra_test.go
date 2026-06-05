package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/client"
)

// ---------------------------------------------------------------------------
// BankingCoreClient — CommitReservation, ReverseTransfer
// ---------------------------------------------------------------------------

func TestBankingCoreClient_CommitReservation_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/commit") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := client.NewBankingCoreClient(srv.URL, testIssuer(), 5*time.Second)
	if err := c.CommitReservation(context.Background(), "res-1", "corr-c1"); err != nil {
		t.Fatalf("CommitReservation error: %v", err)
	}
}

func TestBankingCoreClient_CommitReservation_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := client.NewBankingCoreClient(srv.URL, testIssuer(), 5*time.Second)
	if err := c.CommitReservation(context.Background(), "res-x", "corr-cx"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestBankingCoreClient_ReverseTransfer_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/reverse") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := client.NewBankingCoreClient(srv.URL, testIssuer(), 5*time.Second)
	if err := c.ReverseTransfer(context.Background(), "tr-1", "corr-r1"); err != nil {
		t.Fatalf("ReverseTransfer error: %v", err)
	}
}

func TestBankingCoreClient_ReverseTransfer_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := client.NewBankingCoreClient(srv.URL, testIssuer(), 5*time.Second)
	if err := c.ReverseTransfer(context.Background(), "tr-missing", "corr-rm"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestBankingCoreClient_ReleaseFunds_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := client.NewBankingCoreClient(srv.URL, testIssuer(), 5*time.Second)
	if err := c.ReleaseFunds(context.Background(), "res-bad", "corr-rb"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// NewBankingCoreClient with a non-positive timeout defaults to 10s.
func TestNewBankingCoreClient_DefaultTimeout(t *testing.T) {
	c := client.NewBankingCoreClient("http://example.invalid", testIssuer(), 0)
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

// ---------------------------------------------------------------------------
// TradingServiceClient — ReleaseStocks, ReverseOwnership
// ---------------------------------------------------------------------------

func TestTradingServiceClient_ReleaseStocks_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	tc := client.NewTradingServiceClient(srv.URL, testIssuer(), 5*time.Second)
	if err := tc.ReleaseStocks(context.Background(), "sr-1", "corr-s1"); err != nil {
		t.Fatalf("ReleaseStocks error: %v", err)
	}
}

func TestTradingServiceClient_ReleaseStocks_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tc := client.NewTradingServiceClient(srv.URL, testIssuer(), 5*time.Second)
	if err := tc.ReleaseStocks(context.Background(), "sr-x", "corr-sx"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTradingServiceClient_ReverseOwnership_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/reverse") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tc := client.NewTradingServiceClient(srv.URL, testIssuer(), 5*time.Second)
	if err := tc.ReverseOwnership(context.Background(), "ot-1", "corr-o1"); err != nil {
		t.Fatalf("ReverseOwnership error: %v", err)
	}
}

func TestTradingServiceClient_ReverseOwnership_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	tc := client.NewTradingServiceClient(srv.URL, testIssuer(), 5*time.Second)
	if err := tc.ReverseOwnership(context.Background(), "ot-x", "corr-ox"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ReserveStocks upstream error path.
func TestTradingServiceClient_ReserveStocks_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tc := client.NewTradingServiceClient(srv.URL, testIssuer(), 5*time.Second)
	if _, err := tc.ReserveStocks(context.Background(), 5, "AAPL", 10, "corr"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// LiquidateForFund upstream error path.
func TestTradingServiceClient_LiquidateForFund_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tc := client.NewTradingServiceClient(srv.URL, testIssuer(), 5*time.Second)
	if _, err := tc.LiquidateForFund(context.Background(), 1, decimal.RequireFromString("100"), "corr"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TransferOwnership upstream error path.
func TestTradingServiceClient_TransferOwnership_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tc := client.NewTradingServiceClient(srv.URL, testIssuer(), 5*time.Second)
	if _, err := tc.TransferOwnership(context.Background(), "sr-1", 7, "corr"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNewTradingServiceClient_DefaultTimeout(t *testing.T) {
	tc := client.NewTradingServiceClient("http://example.invalid", testIssuer(), -1)
	if tc == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewMarketServiceClient_DefaultTimeout(t *testing.T) {
	mc := client.NewMarketServiceClient("http://example.invalid", testIssuer(), 0)
	if mc == nil {
		t.Fatal("expected non-nil client")
	}
}

// ConvertCurrencyNoCommission with an upstream error path.
func TestMarketServiceClient_ConvertCurrencyNoCommission_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	mc := client.NewMarketServiceClient(srv.URL, testIssuer(), 5*time.Second)
	_, err := mc.ConvertCurrencyNoCommission(context.Background(), "USD", "RSD", decimal.RequireFromString("100"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
