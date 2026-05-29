package tax

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"banka1/trading-service-go/internal/clients"
	"banka1/trading-service-go/internal/order"

	"github.com/shopspring/decimal"
)

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newTestService(rate string, market *clients.MarketClient) *Service {
	return &Service{taxRate: dec(rate), market: market, logger: quietLogger()}
}

// fakeDoer returns a canned JSON response for every request (used to stub the
// market-service FX call in OTC tax tests).
type fakeDoer struct {
	status int
	body   string
}

func (f fakeDoer) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: f.status,
		Body:       io.NopCloser(strings.NewReader(f.body)),
		Header:     make(http.Header),
	}, nil
}

var (
	taxStart = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	taxEnd   = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	inWindow = time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
)

func sellTx(id int64, qty int, ppu string, ts time.Time) order.Transaction {
	return order.Transaction{ID: id, OrderID: id * 10, Quantity: qty, PricePerUnit: dec(ppu), Timestamp: ts}
}

func TestAllocateSellTaxLots_FIFOAcrossLots(t *testing.T) {
	s := newTestService("0.15", nil)
	lots := []buyLot{
		{buyTransactionID: 1, remainingQuantity: 5, purchasePricePerUnit: dec("100"), sourceAccountID: 11},
		{buyTransactionID: 2, remainingQuantity: 5, purchasePricePerUnit: dec("120"), sourceAccountID: 12},
	}
	sellOrder := &order.Order{UserID: 7, ListingID: 99, AccountID: 70, Direction: order.DirectionSell}
	charges := []taxChargeEntry{}
	s.allocateSellTaxLots(&charges, &lots, sellOrder, sellTx(500, 8, "150", inWindow), taxStart, taxEnd, nil, "USD")

	if len(charges) != 2 {
		t.Fatalf("expected 2 charges, got %d", len(charges))
	}
	// Lot A: 5 shares, gain/share 50 -> taxable 250 -> tax 37.50, buyTx 1, src 11.
	if !charges[0].taxAmount.Equal(dec("37.5")) || charges[0].buyTransactionID != 1 || charges[0].sourceAccountID != 11 {
		t.Errorf("lot A charge wrong: %+v", charges[0])
	}
	// Lot B: 3 shares, gain/share 30 -> taxable 90 -> tax 13.50, buyTx 2, src 12.
	if !charges[1].taxAmount.Equal(dec("13.5")) || charges[1].buyTransactionID != 2 || charges[1].sourceAccountID != 12 {
		t.Errorf("lot B charge wrong: %+v", charges[1])
	}
	// Lot A fully consumed; lot B has 2 remaining.
	if len(lots) != 1 || lots[0].remainingQuantity != 2 {
		t.Errorf("expected lot B with 2 remaining, got %+v", lots)
	}
	if charges[0].userID != 7 || charges[0].listingID != 99 || charges[0].currency != "USD" {
		t.Errorf("charge metadata wrong: %+v", charges[0])
	}
}

func TestAllocateSellTaxLots_PortfolioAverageFallback(t *testing.T) {
	s := newTestService("0.15", nil)
	lots := []buyLot{} // no buy history
	sellOrder := &order.Order{UserID: 7, ListingID: 99, AccountID: 70, Direction: order.DirectionSell}
	charges := []taxChargeEntry{}
	avg := dec("100")
	s.allocateSellTaxLots(&charges, &lots, sellOrder, sellTx(501, 4, "150", inWindow), taxStart, taxEnd, &avg, "USD")

	if len(charges) != 1 {
		t.Fatalf("expected 1 fallback charge, got %d", len(charges))
	}
	// gain/share 50 * 4 = 200 -> tax 30.00; buyTx -1; src = sell order account.
	if !charges[0].taxAmount.Equal(dec("30")) {
		t.Errorf("fallback tax wrong: %s", charges[0].taxAmount)
	}
	if charges[0].buyTransactionID != -1 {
		t.Errorf("fallback buyTransactionId should be -1, got %d", charges[0].buyTransactionID)
	}
	if charges[0].sourceAccountID != 70 {
		t.Errorf("fallback sourceAccountId should be the sell order account (70), got %d", charges[0].sourceAccountID)
	}
}

func TestAllocateSellTaxLots_NoFallbackWhenAvgZeroOrNil(t *testing.T) {
	s := newTestService("0.15", nil)
	sellOrder := &order.Order{UserID: 7, ListingID: 99, AccountID: 70, Direction: order.DirectionSell}

	// nil fallback -> no charge for the unmatched quantity.
	lots := []buyLot{}
	charges := []taxChargeEntry{}
	s.allocateSellTaxLots(&charges, &lots, sellOrder, sellTx(502, 4, "150", inWindow), taxStart, taxEnd, nil, "USD")
	if len(charges) != 0 {
		t.Errorf("nil fallback should yield no charge, got %d", len(charges))
	}

	// zero average -> no charge (Java requires fallback > 0).
	zero := dec("0")
	charges = []taxChargeEntry{}
	s.allocateSellTaxLots(&charges, &lots, sellOrder, sellTx(503, 4, "150", inWindow), taxStart, taxEnd, &zero, "USD")
	if len(charges) != 0 {
		t.Errorf("zero fallback should yield no charge, got %d", len(charges))
	}
}

func TestAllocateSellTaxLots_LossIsNotTaxed(t *testing.T) {
	s := newTestService("0.15", nil)
	lots := []buyLot{{buyTransactionID: 1, remainingQuantity: 10, purchasePricePerUnit: dec("200"), sourceAccountID: 11}}
	sellOrder := &order.Order{UserID: 7, ListingID: 99, AccountID: 70, Direction: order.DirectionSell}
	charges := []taxChargeEntry{}
	s.allocateSellTaxLots(&charges, &lots, sellOrder, sellTx(504, 10, "150", inWindow), taxStart, taxEnd, nil, "USD")
	if len(charges) != 0 {
		t.Errorf("a loss must not be taxed, got %d charges", len(charges))
	}
	// The lot is still consumed even though no tax is due.
	if len(lots) != 0 {
		t.Errorf("lot should be fully consumed, got %+v", lots)
	}
}

func TestAllocateSellTaxLots_OutsideWindowNoCharge(t *testing.T) {
	s := newTestService("0.15", nil)
	lots := []buyLot{{buyTransactionID: 1, remainingQuantity: 10, purchasePricePerUnit: dec("100"), sourceAccountID: 11}}
	sellOrder := &order.Order{UserID: 7, ListingID: 99, AccountID: 70, Direction: order.DirectionSell}
	charges := []taxChargeEntry{}
	before := time.Date(2026, 4, 30, 23, 59, 0, 0, time.UTC) // before start
	s.allocateSellTaxLots(&charges, &lots, sellOrder, sellTx(505, 5, "150", before), taxStart, taxEnd, nil, "USD")
	if len(charges) != 0 {
		t.Errorf("sell outside [start,end) must not be taxed, got %d", len(charges))
	}
	// End is exclusive: a sell exactly at end is not taxed.
	lots = []buyLot{{buyTransactionID: 1, remainingQuantity: 10, purchasePricePerUnit: dec("100"), sourceAccountID: 11}}
	charges = []taxChargeEntry{}
	s.allocateSellTaxLots(&charges, &lots, sellOrder, sellTx(506, 5, "150", taxEnd), taxStart, taxEnd, nil, "USD")
	if len(charges) != 0 {
		t.Errorf("end is exclusive: sell at end must not be taxed, got %d", len(charges))
	}
}

func TestAllocateSellTaxLots_RoundingHalfUp(t *testing.T) {
	s := newTestService("0.15", nil)
	// gain/share 0.1 * 1 share = 0.1 taxable; 0.1 * 0.15 = 0.015 -> HALF_UP scale 2 = 0.02.
	lots := []buyLot{{buyTransactionID: 1, remainingQuantity: 1, purchasePricePerUnit: dec("0"), sourceAccountID: 11}}
	sellOrder := &order.Order{UserID: 7, ListingID: 99, AccountID: 70, Direction: order.DirectionSell}
	charges := []taxChargeEntry{}
	s.allocateSellTaxLots(&charges, &lots, sellOrder, sellTx(507, 1, "0.1", inWindow), taxStart, taxEnd, nil, "USD")
	if len(charges) != 1 || !charges[0].taxAmount.Equal(dec("0.02")) {
		t.Fatalf("expected tax 0.02 (HALF_UP), got %+v", charges)
	}
}

func TestCalculateOtcTaxInRsd(t *testing.T) {
	// profit = (150-100)*10 = 500; tax = 75.00 USD; FX stub returns 8775 RSD.
	market := clients.NewMarketClient("http://market", nil, fakeDoer{status: 200, body: `{"convertedAmount": 8775.00}`})
	s := newTestService("0.15", market)
	got, err := s.calculateOtcTaxInRsd(context.Background(), OtcTaxEntry{
		ContractID: 1, SellPricePerStock: dec("150"), AveragePurchasePrice: dec("100"), Amount: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(dec("8775")) {
		t.Errorf("expected 8775 RSD, got %s", got)
	}
}

func TestCalculateOtcTaxInRsd_LossNoFX(t *testing.T) {
	// profit <= 0 -> 0, without touching the market client (nil here).
	s := newTestService("0.15", nil)
	got, err := s.calculateOtcTaxInRsd(context.Background(), OtcTaxEntry{
		ContractID: 1, SellPricePerStock: dec("90"), AveragePurchasePrice: dec("100"), Amount: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("a loss must yield zero OTC tax, got %s", got)
	}
}

func TestTrackingMetricsStatus(t *testing.T) {
	cases := []struct {
		name           string
		debt, paid     string
		failed         bool
		expectedStatus string
	}{
		{"failed wins", "100", "0", true, "FAILED"},
		{"partially paid", "100", "50", false, "PARTIALLY_PAID"},
		{"pending", "100", "0", false, "PENDING"},
		{"paid", "0", "50", false, "PAID"},
		{"active", "0", "0", false, "ACTIVE"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newMetrics()
			m.addDebt(dec(c.debt))
			m.addPaid(dec(c.paid))
			if c.failed {
				m.markFailed()
			}
			if got := m.status(); got != c.expectedStatus {
				t.Errorf("status() = %q, want %q", got, c.expectedStatus)
			}
		})
	}
}

func TestBuildFullName(t *testing.T) {
	first, last := "  Petar ", " Petrović "
	if got := buildFullName(&first, &last); got != "Petar Petrović" {
		t.Errorf("buildFullName trimmed/joined wrong: %q", got)
	}
	only := "Mika"
	if got := buildFullName(&only, nil); got != "Mika" {
		t.Errorf("buildFullName with nil last: %q", got)
	}
	if got := buildFullName(nil, nil); got != "" {
		t.Errorf("buildFullName all nil should be empty, got %q", got)
	}
}
