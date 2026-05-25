package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/shared/auth"
)

// newIssuer returns a test S2SIssuer that signs tokens with "test-secret".
func newIssuer() *auth.S2SIssuer {
	return auth.NewS2SIssuer("banka1", "interbank-service", []string{"SERVICE"}, "test-secret", time.Hour)
}

// hasBearer checks that the Authorization header carries a Bearer JWT (begins with "Bearer eyJ").
func hasBearer(t *testing.T, r *http.Request) {
	t.Helper()
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer eyJ") {
		t.Errorf("expected Bearer JWT in Authorization header, got %q", h)
	}
}

// ---------------------------------------------------------------------------
// BankingCoreClient tests
// ---------------------------------------------------------------------------

func TestBankingCoreClient_ResolveAccount_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/interbank/account-resolve" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("num") != "111000000000000001" {
			t.Errorf("unexpected num query param %q", r.URL.Query().Get("num"))
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		hasBearer(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ownerType":"CLIENT","ownerId":1,"currency":"USD","availableBalance":"500.00"}`))
	}))
	defer srv.Close()

	c := NewBankingCoreClient(srv.URL, newIssuer(), 5*time.Second)
	info, err := c.ResolveAccount(context.Background(), "111000000000000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Currency != "USD" {
		t.Errorf("currency want USD, got %s", info.Currency)
	}
	if info.OwnerType != "CLIENT" {
		t.Errorf("ownerType want CLIENT, got %s", info.OwnerType)
	}
	if info.OwnerID != 1 {
		t.Errorf("ownerID want 1, got %d", info.OwnerID)
	}
	if !info.AvailableBalance.Equal(decimal.NewFromInt(500)) {
		t.Errorf("availableBalance want 500, got %s", info.AvailableBalance.String())
	}
}

func TestBankingCoreClient_ResolveAccount_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewBankingCoreClient(srv.URL, newIssuer(), 5*time.Second)
	_, err := c.ResolveAccount(context.Background(), "111999999999999999")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestBankingCoreClient_FindAccountByOwnerAndCurrency_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/interbank/account-by-owner" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("ownerId") != "7" {
			t.Errorf("unexpected ownerId %q", r.URL.Query().Get("ownerId"))
		}
		if r.URL.Query().Get("currency") != "EUR" {
			t.Errorf("unexpected currency %q", r.URL.Query().Get("currency"))
		}
		hasBearer(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accountNumber":"111000000000000007"}`))
	}))
	defer srv.Close()

	c := NewBankingCoreClient(srv.URL, newIssuer(), 5*time.Second)
	num, err := c.FindAccountByOwnerAndCurrency(context.Background(), 7, "EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if num != "111000000000000007" {
		t.Errorf("accountNumber want 111000000000000007, got %s", num)
	}
}

func TestBankingCoreClient_FindAccountByOwnerAndCurrency_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewBankingCoreClient(srv.URL, newIssuer(), 5*time.Second)
	_, err := c.FindAccountByOwnerAndCurrency(context.Background(), 999, "USD")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestBankingCoreClient_ReserveMonas_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/interbank/reserve-monas" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		hasBearer(t, r)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["accountNum"] != "111000000000000001" {
			t.Errorf("accountNum=%v", body["accountNum"])
		}
		if body["currency"] != "USD" {
			t.Errorf("currency=%v", body["currency"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"reservationId":"res-uuid-1"}`))
	}))
	defer srv.Close()

	c := NewBankingCoreClient(srv.URL, newIssuer(), 5*time.Second)
	rid, err := c.ReserveMonas(context.Background(), "111000000000000001", "USD", decimal.NewFromInt(100), 222, "tx-local-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rid != "res-uuid-1" {
		t.Errorf("reservationId want res-uuid-1, got %s", rid)
	}
}

func TestBankingCoreClient_CommitMonas_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/interbank/reservations/res-uuid-1/commit-monas" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		hasBearer(t, r)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewBankingCoreClient(srv.URL, newIssuer(), 5*time.Second)
	if err := c.CommitMonas(context.Background(), "res-uuid-1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBankingCoreClient_ReleaseMonas_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/interbank/reservations/res-uuid-2" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		hasBearer(t, r)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewBankingCoreClient(srv.URL, newIssuer(), 5*time.Second)
	if err := c.ReleaseMonas(context.Background(), "res-uuid-2"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBankingCoreClient_Upstream5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`internal error`))
	}))
	defer srv.Close()

	c := NewBankingCoreClient(srv.URL, newIssuer(), 5*time.Second)
	_, err := c.ResolveAccount(context.Background(), "111000000000000001")
	if !errors.Is(err, ErrUpstream) {
		t.Errorf("expected ErrUpstream for 500, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TradingClient tests
// ---------------------------------------------------------------------------

func TestTradingClient_GetPublicStocks_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/interbank/public-stocks" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		hasBearer(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"ticker":"AAPL","sellers":[{"routingNumber":111,"id":"C-1"}],"quantity":100}]`))
	}))
	defer srv.Close()

	c := NewTradingClient(srv.URL, newIssuer(), 5*time.Second)
	stocks, err := c.GetPublicStocks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stocks) != 1 {
		t.Fatalf("expected 1 stock, got %d", len(stocks))
	}
	if stocks[0].Ticker != "AAPL" {
		t.Errorf("ticker want AAPL, got %s", stocks[0].Ticker)
	}
	if stocks[0].Quantity != 100 {
		t.Errorf("quantity want 100, got %d", stocks[0].Quantity)
	}
	if len(stocks[0].Sellers) != 1 || stocks[0].Sellers[0].ID != "C-1" {
		t.Errorf("sellers unexpected: %+v", stocks[0].Sellers)
	}
}

func TestTradingClient_ReserveStock_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/interbank/reserve-stock" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		hasBearer(t, r)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["ticker"] != "AAPL" {
			t.Errorf("ticker=%v", body["ticker"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"reservationId":"stock-res-1"}`))
	}))
	defer srv.Close()

	c := NewTradingClient(srv.URL, newIssuer(), 5*time.Second)
	rid, err := c.ReserveStock(context.Background(), 1, "AAPL", 10, 222, "tx-local-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rid != "stock-res-1" {
		t.Errorf("reservationId want stock-res-1, got %s", rid)
	}
}

func TestTradingClient_CommitStock_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/interbank/reservations/stock-res-1/commit-stock" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewTradingClient(srv.URL, newIssuer(), 5*time.Second)
	if err := c.CommitStock(context.Background(), "stock-res-1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTradingClient_ReleaseStock_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/interbank/reservations/stock-res-2" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewTradingClient(srv.URL, newIssuer(), 5*time.Second)
	if err := c.ReleaseStock(context.Background(), "stock-res-2"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTradingClient_ReserveOption_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Path: /internal/interbank/options/neg-handshake-s9/reserve
		if !strings.Contains(r.URL.Path, "neg-handshake-s9") {
			t.Errorf("unexpected path %q — should contain negotiation id", r.URL.Path)
		}
		if !strings.HasSuffix(r.URL.Path, "/reserve") {
			t.Errorf("path should end with /reserve, got %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		hasBearer(t, r)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["ticker"] != "AAPL" {
			t.Errorf("ticker=%v", body["ticker"])
		}
		if body["sellerForeignId"] != "C-15" {
			t.Errorf("sellerForeignId=%v", body["sellerForeignId"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewTradingClient(srv.URL, newIssuer(), 5*time.Second)
	id := protocol.ForeignBankId{RoutingNumber: 111, Id: "neg-handshake-s9"}
	if err := c.ReserveOption(context.Background(), id, "C-15", "AAPL", 10); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTradingClient_ExerciseOption_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/exercise") {
			t.Errorf("path should end with /exercise, got %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		hasBearer(t, r)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewTradingClient(srv.URL, newIssuer(), 5*time.Second)
	id := protocol.ForeignBankId{RoutingNumber: 111, Id: "neg-1"}
	if err := c.ExerciseOption(context.Background(), id); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTradingClient_ReleaseOption_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/release") {
			t.Errorf("path should end with /release, got %q", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		hasBearer(t, r)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewTradingClient(srv.URL, newIssuer(), 5*time.Second)
	id := protocol.ForeignBankId{RoutingNumber: 111, Id: "neg-2"}
	if err := c.ReleaseOption(context.Background(), id); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTradingClient_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewTradingClient(srv.URL, newIssuer(), 5*time.Second)
	id := protocol.ForeignBankId{RoutingNumber: 111, Id: "neg-missing"}
	err := c.ExerciseOption(context.Background(), id)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// UserClient tests
// ---------------------------------------------------------------------------

func TestUserClient_ResolveUser_Client(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/interbank/user/CLIENT/15" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		hasBearer(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"firstName":"Mile","lastName":"Interbank","displayName":"Mile Interbank"}`))
	}))
	defer srv.Close()

	c := NewUserClient(srv.URL, newIssuer(), 5*time.Second)
	d, err := c.ResolveUser(context.Background(), "CLIENT", 15)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.DisplayName != "Mile Interbank" {
		t.Errorf("displayName want 'Mile Interbank', got %q", d.DisplayName)
	}
	if d.FirstName != "Mile" {
		t.Errorf("firstName want 'Mile', got %q", d.FirstName)
	}
}

func TestUserClient_ResolveUser_Employee(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/interbank/user/EMPLOYEE/3" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		hasBearer(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"firstName":"Marko","lastName":"Markovic","displayName":"Marko Markovic"}`))
	}))
	defer srv.Close()

	c := NewUserClient(srv.URL, newIssuer(), 5*time.Second)
	d, err := c.ResolveUser(context.Background(), "EMPLOYEE", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.DisplayName != "Marko Markovic" {
		t.Errorf("displayName want 'Marko Markovic', got %q", d.DisplayName)
	}
}

func TestUserClient_ResolveUser_LowercaseTypeNormalized(t *testing.T) {
	// Verify that "client" (lowercase) is normalized to "CLIENT" in the path.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/interbank/user/CLIENT/1" {
			t.Errorf("expected CLIENT (uppercase) in path, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"firstName":"X","lastName":"Y","displayName":"X Y"}`))
	}))
	defer srv.Close()

	c := NewUserClient(srv.URL, newIssuer(), 5*time.Second)
	_, err := c.ResolveUser(context.Background(), "client", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUserClient_ResolveUser_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewUserClient(srv.URL, newIssuer(), 5*time.Second)
	_, err := c.ResolveUser(context.Background(), "CLIENT", 9999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUserClient_ResolveUser_400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`bad user type`))
	}))
	defer srv.Close()

	c := NewUserClient(srv.URL, newIssuer(), 5*time.Second)
	_, err := c.ResolveUser(context.Background(), "INVALID", 1)
	if !errors.Is(err, ErrUpstream) {
		t.Errorf("expected ErrUpstream for 400, got %v", err)
	}
}
