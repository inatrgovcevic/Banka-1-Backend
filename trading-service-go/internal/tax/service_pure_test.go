package tax

import (
	"testing"
	"time"

	"banka1/trading-service-go/internal/order"

	"github.com/shopspring/decimal"
)

func taxDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// ---- inTaxWindow ----

func TestInTaxWindow_Inside(t *testing.T) {
	start := taxDate(2024, 1, 1)
	end := taxDate(2024, 2, 1)
	ts := taxDate(2024, 1, 15)
	if !inTaxWindow(ts, start, end) {
		t.Error("expected inside window")
	}
}

func TestInTaxWindow_OnStart(t *testing.T) {
	start := taxDate(2024, 1, 1)
	end := taxDate(2024, 2, 1)
	if !inTaxWindow(start, start, end) {
		t.Error("start is inclusive")
	}
}

func TestInTaxWindow_OnEnd_Exclusive(t *testing.T) {
	start := taxDate(2024, 1, 1)
	end := taxDate(2024, 2, 1)
	if inTaxWindow(end, start, end) {
		t.Error("end is exclusive")
	}
}

func TestInTaxWindow_Before(t *testing.T) {
	start := taxDate(2024, 1, 1)
	end := taxDate(2024, 2, 1)
	if inTaxWindow(taxDate(2023, 12, 31), start, end) {
		t.Error("before start should be outside")
	}
}

func TestInTaxWindow_After(t *testing.T) {
	start := taxDate(2024, 1, 1)
	end := taxDate(2024, 2, 1)
	if inTaxWindow(taxDate(2024, 3, 1), start, end) {
		t.Error("after end should be outside")
	}
}

// ---- startOfDayUTC / firstOfMonthUTC / firstOfYearUTC ----

func TestStartOfDayUTC(t *testing.T) {
	in := time.Date(2024, 5, 15, 14, 30, 59, 0, time.UTC)
	got := startOfDayUTC(in)
	if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 {
		t.Errorf("expected midnight, got %v", got)
	}
	if got.Day() != 15 || got.Month() != 5 {
		t.Errorf("day/month changed: %v", got)
	}
}

func TestFirstOfMonthUTC(t *testing.T) {
	in := time.Date(2024, 3, 25, 10, 0, 0, 0, time.UTC)
	got := firstOfMonthUTC(in)
	if got.Day() != 1 || got.Month() != 3 || got.Year() != 2024 {
		t.Errorf("got %v, want March 1 2024", got)
	}
}

func TestFirstOfYearUTC(t *testing.T) {
	in := time.Date(2024, 7, 4, 0, 0, 0, 0, time.UTC)
	got := firstOfYearUTC(in)
	if got.Day() != 1 || got.Month() != 1 || got.Year() != 2024 {
		t.Errorf("got %v, want Jan 1 2024", got)
	}
}

// ---- sortedInt64Keys ----

func TestSortedInt64Keys_Sorted(t *testing.T) {
	m := map[int64]decimal.Decimal{
		3: decimal.NewFromInt(1),
		1: decimal.NewFromInt(2),
		2: decimal.NewFromInt(3),
	}
	got := sortedInt64Keys(m)
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("not sorted: %v", got)
	}
}

func TestSortedInt64Keys_Empty(t *testing.T) {
	got := sortedInt64Keys(nil)
	if len(got) != 0 {
		t.Error("expected empty")
	}
}

// ---- setKeys ----

func TestSetKeys_ReturnsKeys(t *testing.T) {
	m := map[int64]bool{1: true, 2: true, 3: false}
	got := setKeys(m)
	if len(got) != 3 {
		t.Errorf("got %d keys, want 3", len(got))
	}
}

// ---- paginate ----

func TestPaginate_FirstPage(t *testing.T) {
	all := []int{1, 2, 3, 4, 5}
	got := paginate(all, 0, 2)
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("got %v, want [1 2]", got)
	}
}

func TestPaginate_LastPage(t *testing.T) {
	all := []int{1, 2, 3, 4, 5}
	got := paginate(all, 2, 2)
	if len(got) != 1 || got[0] != 5 {
		t.Errorf("got %v, want [5]", got)
	}
}

func TestPaginate_OutOfRange(t *testing.T) {
	all := []int{1, 2, 3}
	got := paginate(all, 10, 2)
	if len(got) != 0 {
		t.Errorf("expected empty for out-of-range page, got %v", got)
	}
}

func TestPaginate_ZeroSize(t *testing.T) {
	all := []int{1, 2, 3}
	got := paginate(all, 0, 0)
	if len(got) != 3 {
		t.Errorf("zero size should return all: got %v", got)
	}
}

// ---- buildFullName ----

func TestBuildFullName_BothNames(t *testing.T) {
	f, l := "Marko", "Marković"
	got := buildFullName(&f, &l)
	if got != "Marko Marković" {
		t.Errorf("got %q, want %q", got, "Marko Marković")
	}
}

func TestBuildFullName_OnlyFirst(t *testing.T) {
	f := "Ana"
	got := buildFullName(&f, nil)
	if got != "Ana" {
		t.Errorf("got %q, want Ana", got)
	}
}

func TestBuildFullName_NilBoth(t *testing.T) {
	got := buildFullName(nil, nil)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ---- orderDirectionRank ----

func TestOrderDirectionRank_Nil(t *testing.T) {
	if orderDirectionRank(nil) != 2 {
		t.Error("nil order should have rank 2")
	}
}

func TestOrderDirectionRank_Buy(t *testing.T) {
	o := &order.Order{Direction: order.DirectionBuy}
	if orderDirectionRank(o) != 0 {
		t.Error("BUY should have rank 0")
	}
}

func TestOrderDirectionRank_Sell(t *testing.T) {
	o := &order.Order{Direction: order.DirectionSell}
	if orderDirectionRank(o) != 1 {
		t.Error("SELL should have rank 1")
	}
}

func TestOrderDirectionRank_EmptyDirection(t *testing.T) {
	o := &order.Order{Direction: ""}
	if orderDirectionRank(o) != 2 {
		t.Error("empty direction should have rank 2")
	}
}

// ---- belongsToRelevantTaxScope ----

func TestBelongsToRelevantTaxScope_NilOrder(t *testing.T) {
	if belongsToRelevantTaxScope(nil, map[int64]map[int64]bool{}) {
		t.Error("nil order should return false")
	}
}

func TestBelongsToRelevantTaxScope_ZeroUserID(t *testing.T) {
	o := &order.Order{ListingID: 5}
	if belongsToRelevantTaxScope(o, map[int64]map[int64]bool{}) {
		t.Error("zero UserID should return false")
	}
}

func TestBelongsToRelevantTaxScope_ZeroListingID(t *testing.T) {
	o := &order.Order{UserID: 1}
	if belongsToRelevantTaxScope(o, map[int64]map[int64]bool{}) {
		t.Error("zero ListingID should return false")
	}
}

func TestBelongsToRelevantTaxScope_ListingNotFound(t *testing.T) {
	o := &order.Order{UserID: 1, ListingID: 99}
	m := map[int64]map[int64]bool{1: {5: true}}
	if belongsToRelevantTaxScope(o, m) {
		t.Error("listing 99 not in scope")
	}
}

func TestBelongsToRelevantTaxScope_True(t *testing.T) {
	o := &order.Order{UserID: 1, ListingID: 5}
	m := map[int64]map[int64]bool{1: {5: true}}
	if !belongsToRelevantTaxScope(o, m) {
		t.Error("should be in scope")
	}
}

func TestBelongsToRelevantTaxScope_UserNotInMap(t *testing.T) {
	o := &order.Order{UserID: 2, ListingID: 5}
	m := map[int64]map[int64]bool{1: {5: true}}
	if belongsToRelevantTaxScope(o, m) {
		t.Error("user 2 not in map")
	}
}

// ---- resolveChargeAmountRsd ----

func TestResolveChargeAmountRsd_WithRsd(t *testing.T) {
	d := decimal.NewFromFloat(99.5)
	charge := TaxCharge{TaxAmount: decimal.NewFromFloat(50), TaxAmountRsd: &d}
	got := resolveChargeAmountRsd(charge)
	if !got.Equal(d) {
		t.Errorf("got %v, want %v", got, d)
	}
}

func TestResolveChargeAmountRsd_WithoutRsd(t *testing.T) {
	charge := TaxCharge{TaxAmount: decimal.NewFromFloat(50), TaxAmountRsd: nil}
	got := resolveChargeAmountRsd(charge)
	if !got.Equal(decimal.NewFromFloat(50)) {
		t.Errorf("got %v, want 50", got)
	}
}

// ---- taxTrackingMetrics ----

func TestNewMetrics_ZeroValues(t *testing.T) {
	m := newMetrics()
	if !m.debt.IsZero() || !m.currentMonthTax.IsZero() || !m.paidTax.IsZero() {
		t.Error("new metrics should have zero values")
	}
	if m.failed {
		t.Error("new metrics should not be failed")
	}
}

func TestAddDebt(t *testing.T) {
	m := newMetrics()
	m.addDebt(decimal.NewFromFloat(100))
	m.addDebt(decimal.NewFromFloat(50))
	if !m.debt.Equal(decimal.NewFromFloat(150)) {
		t.Errorf("debt = %v, want 150", m.debt)
	}
}

func TestAddCurrentMonthTax(t *testing.T) {
	m := newMetrics()
	m.addCurrentMonthTax(decimal.NewFromFloat(30))
	if !m.currentMonthTax.Equal(decimal.NewFromFloat(30)) {
		t.Errorf("currentMonthTax = %v, want 30", m.currentMonthTax)
	}
}

func TestAddPaid(t *testing.T) {
	m := newMetrics()
	m.addPaid(decimal.NewFromFloat(200))
	if !m.paidTax.Equal(decimal.NewFromFloat(200)) {
		t.Errorf("paidTax = %v, want 200", m.paidTax)
	}
}

func TestMarkFailed(t *testing.T) {
	m := newMetrics()
	m.markFailed()
	if !m.failed {
		t.Error("should be failed")
	}
}

func TestRecordCalculation_Updates(t *testing.T) {
	m := newMetrics()
	t1 := taxDate(2024, 1, 1)
	t2 := taxDate(2024, 2, 1)
	m.recordCalculation(t1)
	if m.lastCalculationDate == nil || !m.lastCalculationDate.Equal(t1) {
		t.Error("should record t1")
	}
	m.recordCalculation(t2)
	if !m.lastCalculationDate.Equal(t2) {
		t.Error("should update to later date t2")
	}
	// Earlier date should not replace.
	m.recordCalculation(t1)
	if !m.lastCalculationDate.Equal(t2) {
		t.Error("earlier date should not replace later")
	}
}

func TestStatus_Failed(t *testing.T) {
	m := newMetrics()
	m.markFailed()
	if m.status() != "FAILED" {
		t.Errorf("status = %q, want FAILED", m.status())
	}
}

func TestStatus_Pending(t *testing.T) {
	m := newMetrics()
	m.addDebt(decimal.NewFromFloat(100))
	if m.status() != "PENDING" {
		t.Errorf("status = %q, want PENDING", m.status())
	}
}

func TestStatus_Paid(t *testing.T) {
	m := newMetrics()
	m.addPaid(decimal.NewFromFloat(100))
	if m.status() != "PAID" {
		t.Errorf("status = %q, want PAID", m.status())
	}
}

func TestStatus_PartiallyPaid(t *testing.T) {
	m := newMetrics()
	m.addDebt(decimal.NewFromFloat(100))
	m.addPaid(decimal.NewFromFloat(50))
	if m.status() != "PARTIALLY_PAID" {
		t.Errorf("status = %q, want PARTIALLY_PAID", m.status())
	}
}

func TestStatus_Active(t *testing.T) {
	m := newMetrics()
	if m.status() != "ACTIVE" {
		t.Errorf("status = %q, want ACTIVE", m.status())
	}
}

// ---- ensureMetrics / metricsOf ----

func TestEnsureMetrics_CreatesNew(t *testing.T) {
	m := map[int64]*taxTrackingMetrics{}
	got := ensureMetrics(m, 42)
	if got == nil {
		t.Error("should return non-nil metrics")
	}
	if m[42] != got {
		t.Error("should store in map")
	}
}

func TestEnsureMetrics_ReturnsExisting(t *testing.T) {
	existing := newMetrics()
	existing.addDebt(decimal.NewFromFloat(100))
	m := map[int64]*taxTrackingMetrics{7: existing}
	got := ensureMetrics(m, 7)
	if got != existing {
		t.Error("should return existing metrics")
	}
}

func TestMetricsOf_NotFound_ReturnsEmpty(t *testing.T) {
	m := map[int64]*taxTrackingMetrics{}
	got := metricsOf(m, 99)
	if got == nil {
		t.Error("should return empty metrics")
	}
	if m[99] != nil {
		t.Error("should not store in map")
	}
}

func TestMetricsOf_Found_ReturnsStored(t *testing.T) {
	stored := newMetrics()
	stored.addDebt(decimal.NewFromFloat(50))
	m := map[int64]*taxTrackingMetrics{1: stored}
	got := metricsOf(m, 1)
	if got != stored {
		t.Error("should return stored metrics")
	}
}
