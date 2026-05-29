package order

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"banka1/trading-service-go/internal/clients"

	"github.com/shopspring/decimal"
)

func dp(s string) *decimal.Decimal {
	d := decimal.RequireFromString(s)
	return &d
}

func ip(i int) *int { return &i }

func eq(t *testing.T, got decimal.Decimal, want string) {
	t.Helper()
	if !got.Equal(decimal.RequireFromString(want)) {
		t.Errorf("got %s, want %s", got.String(), want)
	}
}

// --- stub market client for commission cap conversion --------------------

type stubDoer struct{ body string }

func (d stubDoer) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(d.body)),
		Header:     make(http.Header),
	}, nil
}

func newTestService(marketBody string) *Service {
	return &Service{
		market: clients.NewMarketClient("http://stub", nil, stubDoer{body: marketBody}),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestDetermineOrderType(t *testing.T) {
	cases := []struct {
		limit, stop *decimal.Decimal
		want        string
	}{
		{nil, nil, TypeMarket},
		{dp("10"), nil, TypeLimit},
		{nil, dp("10"), TypeStop},
		{dp("10"), dp("9"), TypeStopLimit},
	}
	for _, c := range cases {
		if got := determineOrderType(c.limit, c.stop); got != c.want {
			t.Errorf("determineOrderType(%v,%v)=%s want %s", c.limit, c.stop, got, c.want)
		}
	}
}

func TestCommissionRatesAndRounding(t *testing.T) {
	// Same currency (USD): no HTTP — cap is the raw $7/$12.
	s := newTestService(`{"convertedAmount":"7"}`)
	ctx := context.Background()

	// MARKET 0.14: 100 * 0.14 = 14.00, above the $7 cap → 7.
	eq(t, s.commission(ctx, TypeMarket, decimal.RequireFromString("100"), "USD"), "7")
	// MARKET below cap: 10 * 0.14 = 1.40.
	eq(t, s.commission(ctx, TypeMarket, decimal.RequireFromString("10"), "USD"), "1.4")
	// HALF_UP at scale 2: 3.21 * 0.14 = 0.4494 -> 0.45.
	eq(t, s.commission(ctx, TypeMarket, decimal.RequireFromString("3.21"), "USD"), "0.45")
	// LIMIT 0.24, below cap: 10 * 0.24 = 2.40.
	eq(t, s.commission(ctx, TypeLimit, decimal.RequireFromString("10"), "USD"), "2.4")
	// LIMIT above cap $12: 1000 * 0.24 = 240 -> 12.
	eq(t, s.commission(ctx, TypeLimit, decimal.RequireFromString("1000"), "USD"), "12")
	// STOP prices like MARKET (0.14); STOP_LIMIT like LIMIT (0.24).
	eq(t, s.commission(ctx, TypeStop, decimal.RequireFromString("10"), "USD"), "1.4")
	eq(t, s.commission(ctx, TypeStopLimit, decimal.RequireFromString("10"), "USD"), "2.4")
}

func TestCommissionCapCrossCurrency(t *testing.T) {
	// Cross-currency cap: $7 -> 700 RSD via market.Calculate. A huge base caps to 700.
	s := newTestService(`{"convertedAmount":"700"}`)
	eq(t, s.commission(context.Background(), TypeMarket, decimal.RequireFromString("100000"), "RSD"), "700")
}

func TestCalculateApproximatePrice(t *testing.T) {
	listing := &clients.StockListing{ContractSize: ip(10), Ask: dp("5"), Bid: dp("4")}
	// MARKET BUY uses ask: 5 * 10 * 3 = 150.
	got, err := calculateApproximatePrice(TypeMarket, DirectionBuy, listing, 3, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, got, "150")
	// LIMIT uses the limit value: 7 * 10 * 2 = 140.
	got, err = calculateApproximatePrice(TypeLimit, DirectionBuy, listing, 2, dp("7"), nil)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, got, "140")
}

func TestCalculateExecutionPricePerUnit(t *testing.T) {
	listing := &clients.StockListing{Ask: dp("48"), Bid: dp("52")}
	// LIMIT BUY: min(limit, ask).
	if p, ok := calculateExecutionPricePerUnit(&Order{OrderType: TypeLimit, Direction: DirectionBuy, LimitValue: dp("50")}, listing); !ok || !p.Equal(decimal.RequireFromString("48")) {
		t.Errorf("LIMIT BUY: got %s ok=%v want 48", p, ok)
	}
	if p, ok := calculateExecutionPricePerUnit(&Order{OrderType: TypeLimit, Direction: DirectionBuy, LimitValue: dp("40")}, listing); !ok || !p.Equal(decimal.RequireFromString("40")) {
		t.Errorf("LIMIT BUY cap: got %s want 40", p)
	}
	// LIMIT SELL: max(limit, bid).
	if p, ok := calculateExecutionPricePerUnit(&Order{OrderType: TypeLimit, Direction: DirectionSell, LimitValue: dp("50")}, listing); !ok || !p.Equal(decimal.RequireFromString("52")) {
		t.Errorf("LIMIT SELL: got %s want 52", p)
	}
	// MARKET BUY uses ask.
	if p, ok := calculateExecutionPricePerUnit(&Order{OrderType: TypeMarket, Direction: DirectionBuy}, listing); !ok || !p.Equal(decimal.RequireFromString("48")) {
		t.Errorf("MARKET BUY: got %s want 48", p)
	}
}

func TestIsStopActivated(t *testing.T) {
	// BUY: ask >= stop activates.
	if !isStopActivated(&Order{Direction: DirectionBuy, StopValue: dp("100")}, decimal.RequireFromString("100")) {
		t.Error("BUY stop should activate at ask == stop")
	}
	if isStopActivated(&Order{Direction: DirectionBuy, StopValue: dp("100")}, decimal.RequireFromString("99")) {
		t.Error("BUY stop should not activate below stop")
	}
	// SELL: bid < stop activates.
	if !isStopActivated(&Order{Direction: DirectionSell, StopValue: dp("100")}, decimal.RequireFromString("99")) {
		t.Error("SELL stop should activate below stop")
	}
	if isStopActivated(&Order{Direction: DirectionSell, StopValue: dp("100")}, decimal.RequireFromString("100")) {
		t.Error("SELL stop should not activate at bid == stop")
	}
}

func TestCurrentExecutableCapacity(t *testing.T) {
	v := func(n int64) *int64 { return &n }
	if c := currentExecutableCapacity(&Order{RemainingPortions: 10}, &clients.StockListing{Volume: v(5)}); c != 5 {
		t.Errorf("min(volume=5, remaining=10) want 5 got %d", c)
	}
	if c := currentExecutableCapacity(&Order{RemainingPortions: 3}, &clients.StockListing{Volume: v(20)}); c != 3 {
		t.Errorf("min(volume=20, remaining=3) want 3 got %d", c)
	}
	if c := currentExecutableCapacity(&Order{RemainingPortions: 7}, &clients.StockListing{}); c != 7 {
		t.Errorf("nil volume defaults to remaining, want 7 got %d", c)
	}
}

func TestDetermineExecutionQuantity(t *testing.T) {
	// AON fills the whole remainder.
	if q := determineExecutionQuantity(&Order{AllOrNone: true, RemainingPortions: 9}, 3); q != 9 {
		t.Errorf("AON want 9 got %d", q)
	}
	// Non-AON: random 1..capacity.
	for i := 0; i < 1000; i++ {
		q := determineExecutionQuantity(&Order{RemainingPortions: 100}, 5)
		if q < 1 || q > 5 {
			t.Fatalf("random fill out of [1,5]: %d", q)
		}
	}
}

func TestCalculateInitialMarginCost(t *testing.T) {
	stock := &clients.StockListing{Price: dp("100"), ListingType: sp("STOCK"), ContractSize: ip(1)}
	got, err := calculateInitialMarginCost(stock, 2) // 100*0.5=50; *1.1=55; *2=110.00
	if err != nil {
		t.Fatal(err)
	}
	eq(t, got, "110")
	withMaint := &clients.StockListing{Price: dp("100"), MaintenanceMargin: dp("10"), ContractSize: ip(1)}
	got, err = calculateInitialMarginCost(withMaint, 2) // 10*1.1*2 = 22.00
	if err != nil {
		t.Fatal(err)
	}
	eq(t, got, "22")
}

func TestParseStatusFilter(t *testing.T) {
	if f, ok := ParseStatusFilter(""); !ok || f != "ALL" {
		t.Errorf("empty -> ALL, got %s ok=%v", f, ok)
	}
	if f, ok := ParseStatusFilter("pending"); !ok || f != StatusPending {
		t.Errorf("case-insensitive PENDING, got %s ok=%v", f, ok)
	}
	if _, ok := ParseStatusFilter("BOGUS"); ok {
		t.Error("unknown filter should be rejected")
	}
}

func TestAuthUserRolesAndPermissions(t *testing.T) {
	client := AuthUser{Roles: []string{"CLIENT_TRADING"}, Permissions: []string{"MARGIN_TRADE"}}
	if !client.IsClient() || client.IsAgent() {
		t.Error("CLIENT_TRADING is a client, not an agent")
	}
	if !client.HasTradingPermission() {
		t.Error("CLIENT_TRADING grants trading")
	}
	if !client.HasMarginPermission() {
		t.Error("MARGIN_TRADE grants margin")
	}
	agent := AuthUser{Roles: []string{"SUPERVISOR"}}
	if agent.IsClient() || !agent.IsAgent() {
		t.Error("SUPERVISOR is an agent, not a client")
	}
	if agent.HasMarginPermission() {
		t.Error("no permissions -> no margin")
	}
}

func TestDecimalHelpers(t *testing.T) {
	eq(t, decimalMin(decimal.RequireFromString("3"), decimal.RequireFromString("5")), "3")
	eq(t, decimalMax(decimal.RequireFromString("3"), decimal.RequireFromString("5")), "5")
	eq(t, decimalMaxZero(decimal.RequireFromString("-2")), "0")
	eq(t, decimalMaxZero(decimal.RequireFromString("4")), "4")
}

func sp(s string) *string { return &s }
