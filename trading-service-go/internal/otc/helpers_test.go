package otc

import (
	"regexp"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestItoa(t *testing.T) {
	if itoa(0) != "0" || itoa(-5) != "-5" || itoa(12345) != "12345" {
		t.Errorf("itoa wrong")
	}
}

func TestResolveActorName(t *testing.T) {
	name := "Jovan"
	empty := ""
	cases := []struct {
		id   int64
		name *string
		want string
	}{
		{5, &name, "Jovan"},
		{5, &empty, "user#5"},
		{7, nil, "user#7"},
	}
	for _, c := range cases {
		if got := resolveActorName(c.id, c.name); got != c.want {
			t.Errorf("resolveActorName(%d,%v) = %q want %q", c.id, c.name, got, c.want)
		}
	}
}

func TestMinMaxInt(t *testing.T) {
	if minInt(3, 5) != 3 || minInt(5, 3) != 3 || minInt(4, 4) != 4 {
		t.Errorf("minInt wrong")
	}
	if maxInt(3, 5) != 5 || maxInt(5, 3) != 5 || maxInt(4, 4) != 4 {
		t.Errorf("maxInt wrong")
	}
}

func TestPointerHelpers(t *testing.T) {
	if *intPtr(7) != 7 {
		t.Error("intPtr")
	}
	if *strPtr("x") != "x" {
		t.Error("strPtr")
	}
	d := decimal.RequireFromString("1.5")
	if !decPtrOf(d).Equal(d) {
		t.Error("decPtrOf")
	}
}

func TestExposeTooMuchMsg(t *testing.T) {
	msg := exposeTooMuchMsg(10, 8, 2, 6)
	for _, want := range []string{"10", "8", "2", "6"} {
		if !regexp.MustCompile(want).MatchString(msg) {
			t.Errorf("message %q missing %q", msg, want)
		}
	}
}

func TestIsOtcExercise(t *testing.T) {
	if !isOtcExercise("otc-exercise-123") {
		t.Error("should match otc-exercise- prefix")
	}
	if isOtcExercise("otc-premium-123") {
		t.Error("premium correlation should not match")
	}
	if isOtcExercise("") {
		t.Error("empty should not match")
	}
}

func TestNewUUIDv4(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		u := newUUIDv4()
		if !re.MatchString(u) {
			t.Fatalf("uuid %q does not match v4 format", u)
		}
		if seen[u] {
			t.Fatalf("duplicate uuid %q", u)
		}
		seen[u] = true
	}
}

func TestToDto(t *testing.T) {
	mb := "Jovan"
	o := &OtcOffer{
		ID: 1, StockTicker: "AAPL", BuyerID: 10, SellerID: 20, Amount: 5,
		PricePerStock:  decimal.RequireFromString("100.00"),
		Premium:        decimal.RequireFromString("5.00"),
		SettlementDate: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Status:         OfferAccepted, ModifiedBy: &mb,
		LastModified: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
	}
	dto := toDto(o)
	if dto.ID != 1 || dto.StockTicker != "AAPL" || dto.Amount != 5 || dto.Status != OfferAccepted {
		t.Errorf("dto wrong: %+v", dto)
	}
	if dto.ModifiedBy == nil || *dto.ModifiedBy != "Jovan" {
		t.Errorf("modifiedBy wrong: %v", dto.ModifiedBy)
	}
}

func TestToContractDto(t *testing.T) {
	exAt := time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC)
	c := &OptionContract{
		ID: 2, OfferID: 1, StockTicker: "NVDA", BuyerID: 10, SellerID: 20, Amount: 3,
		PricePerStock:  decimal.RequireFromString("850.00"),
		SettlementDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Status:         ContractActive,
		CreatedAt:      time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		ExercisedAt:    &exAt,
	}
	dto := toContractDto(c)
	if dto.ID != 2 || dto.OfferID != 1 || dto.Amount != 3 || dto.Status != ContractActive {
		t.Errorf("dto wrong: %+v", dto)
	}
}

func TestToContractDto_NilExercisedAt(t *testing.T) {
	c := &OptionContract{ID: 3, PricePerStock: decimal.Zero, Status: ContractPendingPremium}
	dto := toContractDto(c)
	if dto.ID != 3 || dto.Status != ContractPendingPremium {
		t.Errorf("dto wrong: %+v", dto)
	}
}

func TestToHistoryResponse(t *testing.T) {
	actorID := int64(10)
	actorName := "Jovan"
	oldAmt, newAmt := 5, 7
	oldStatus, newStatus := OfferPendingSeller, OfferPendingBuyer
	oldP := decimal.RequireFromString("100")
	settle := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	h := &NegotiationHistory{
		ID: 1, OfferID: 2, BuyerID: 10, SellerID: 20, ActorID: &actorID, ActorName: &actorName,
		EventType: EventCounterOffered, StockTicker: "AAPL",
		OldAmount: &oldAmt, NewAmount: &newAmt, OldPricePerStock: &oldP,
		OldStatus: &oldStatus, NewStatus: &newStatus,
		OldSettlementDate: &settle, NewSettlementDate: &settle,
		ChangedAt: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
	}
	resp := toHistoryResponse(h)
	if resp.ID != 1 || resp.OfferID != 2 || resp.EventType != EventCounterOffered {
		t.Errorf("resp wrong: %+v", resp)
	}
	if resp.ActorName == nil || *resp.ActorName != "Jovan" {
		t.Errorf("actorName wrong: %v", resp.ActorName)
	}
	if resp.OldAmount == nil || *resp.OldAmount != 5 || resp.NewAmount == nil || *resp.NewAmount != 7 {
		t.Errorf("amounts wrong")
	}
}

func TestDecPtr(t *testing.T) {
	if got, err := decPtr(nil); err != nil || got != nil {
		t.Errorf("nil input should give nil, no error: %v %v", got, err)
	}
	s := "12.50"
	got, err := decPtr(&s)
	if err != nil || got == nil || !got.Equal(decimal.RequireFromString("12.50")) {
		t.Errorf("decPtr(%q) = %v, %v", s, got, err)
	}
	bad := "not-a-number"
	if _, err := decPtr(&bad); err == nil {
		t.Error("invalid decimal should error")
	}
}

func TestAllContractStatuses(t *testing.T) {
	want := []string{ContractPendingPremium, ContractActive, ContractExercised, ContractExpired, ContractCanceled}
	if len(allContractStatuses) != len(want) {
		t.Fatalf("len = %d want %d", len(allContractStatuses), len(want))
	}
	for i, s := range want {
		if allContractStatuses[i] != s {
			t.Errorf("allContractStatuses[%d] = %q want %q", i, allContractStatuses[i], s)
		}
	}
}

func TestTruncateToDate(t *testing.T) {
	got := truncateToDate(time.Date(2024, 3, 15, 14, 30, 59, 999, time.UTC))
	want := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) || got.Location() != time.UTC {
		t.Errorf("truncateToDate = %v want %v (UTC)", got, want)
	}
}

func TestExpiryRoutingConstant(t *testing.T) {
	if routingOtcExpiry != "otc.expiry_reminder" {
		t.Errorf("routingOtcExpiry = %q", routingOtcExpiry)
	}
}
