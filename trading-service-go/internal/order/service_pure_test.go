package order

import (
	"testing"
	"time"

	"banka1/trading-service-go/internal/clients"

	"github.com/shopspring/decimal"
)

// ---- parsePurchaseFor ----

func TestParsePurchaseFor_Nil(t *testing.T) {
	got, err := parsePurchaseFor(nil)
	if err != nil || got != "" {
		t.Errorf("nil: got %q, %v", got, err)
	}
}

func TestParsePurchaseFor_Empty(t *testing.T) {
	s := "  "
	got, err := parsePurchaseFor(&s)
	if err != nil || got != "" {
		t.Errorf("empty: got %q, %v", got, err)
	}
}

func TestParsePurchaseFor_Bank(t *testing.T) {
	s := "bank"
	got, err := parsePurchaseFor(&s)
	if err != nil || got != PurchaseForBank {
		t.Errorf("bank: got %q, %v", got, err)
	}
}

func TestParsePurchaseFor_InvestmentFund(t *testing.T) {
	s := "investment_fund"
	got, err := parsePurchaseFor(&s)
	if err != nil || got != PurchaseForInvestmentFund {
		t.Errorf("investment_fund: got %q, %v", got, err)
	}
}

func TestParsePurchaseFor_Invalid(t *testing.T) {
	s := "GARBAGE"
	_, err := parsePurchaseFor(&s)
	if err == nil {
		t.Error("expected error for invalid purchaseFor")
	}
}

// ---- validateCommonRequest ----

func TestValidateCommonRequest_NilListingID(t *testing.T) {
	qty := 1
	if err := validateCommonRequest(nil, &qty, nil, nil); err == nil {
		t.Error("expected error for nil listingID")
	}
}

func TestValidateCommonRequest_NilQuantity(t *testing.T) {
	id := int64(1)
	if err := validateCommonRequest(&id, nil, nil, nil); err == nil {
		t.Error("expected error for nil quantity")
	}
}

func TestValidateCommonRequest_ZeroQuantity(t *testing.T) {
	id, qty := int64(1), 0
	if err := validateCommonRequest(&id, &qty, nil, nil); err == nil {
		t.Error("expected error for zero quantity")
	}
}

func TestValidateCommonRequest_NegativeLimitValue(t *testing.T) {
	id, qty := int64(1), 5
	lim := decimal.NewFromFloat(-1)
	if err := validateCommonRequest(&id, &qty, &lim, nil); err == nil {
		t.Error("expected error for negative limit")
	}
}

func TestValidateCommonRequest_NegativeStopValue(t *testing.T) {
	id, qty := int64(1), 5
	stop := decimal.NewFromFloat(-0.5)
	if err := validateCommonRequest(&id, &qty, nil, &stop); err == nil {
		t.Error("expected error for negative stop")
	}
}

func TestValidateCommonRequest_Valid(t *testing.T) {
	id, qty := int64(5), 10
	lim := decimal.NewFromFloat(100)
	stop := decimal.NewFromFloat(90)
	if err := validateCommonRequest(&id, &qty, &lim, &stop); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateCommonRequest_NilOptionals(t *testing.T) {
	id, qty := int64(5), 10
	if err := validateCommonRequest(&id, &qty, nil, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---- formatEmployeeName ----

func TestFormatEmployeeName_BothNames(t *testing.T) {
	f, l := "Marko", "Marković"
	emp := &clients.Employee{Ime: &f, Prezime: &l}
	got := formatEmployeeName(emp)
	if got == nil || *got != "Marko Marković" {
		t.Errorf("got %v, want Marko Marković", got)
	}
}

func TestFormatEmployeeName_OnlyFirst(t *testing.T) {
	f := "Ana"
	emp := &clients.Employee{Ime: &f}
	got := formatEmployeeName(emp)
	if got == nil || *got != "Ana" {
		t.Errorf("got %v, want Ana", got)
	}
}

func TestFormatEmployeeName_EmptyNames_FallsToUsername(t *testing.T) {
	u := "user123"
	emp := &clients.Employee{Username: &u}
	got := formatEmployeeName(emp)
	if got == nil || *got != "user123" {
		t.Errorf("got %v, want user123", got)
	}
}

// ---- hasPastSettlementDate ----

func TestHasPastSettlementDate_Nil(t *testing.T) {
	listing := &clients.StockListing{SettlementDate: nil}
	if hasPastSettlementDate(listing) {
		t.Error("nil settlement date should return false")
	}
}

func TestHasPastSettlementDate_InvalidFormat(t *testing.T) {
	s := "not-a-date"
	listing := &clients.StockListing{SettlementDate: &s}
	if hasPastSettlementDate(listing) {
		t.Error("invalid format should return false")
	}
}

func TestHasPastSettlementDate_PastDate(t *testing.T) {
	s := "2020-01-01"
	listing := &clients.StockListing{SettlementDate: &s}
	if !hasPastSettlementDate(listing) {
		t.Error("2020-01-01 should be in the past")
	}
}

func TestHasPastSettlementDate_FutureDate(t *testing.T) {
	s := "2099-12-31"
	listing := &clients.StockListing{SettlementDate: &s}
	if hasPastSettlementDate(listing) {
		t.Error("future date should not be in the past")
	}
}

// ---- boolValue ----

func TestBoolValue_Nil(t *testing.T) {
	if boolValue(nil) {
		t.Error("nil should be false")
	}
}

func TestBoolValue_True(t *testing.T) {
	b := true
	if !boolValue(&b) {
		t.Error("true should be true")
	}
}

func TestBoolValue_False(t *testing.T) {
	b := false
	if boolValue(&b) {
		t.Error("false should be false")
	}
}

// ---- PurchaseFor constants ----

func TestPurchaseForConstants(t *testing.T) {
	if PurchaseForBank == "" || PurchaseForInvestmentFund == "" {
		t.Error("constants must not be empty")
	}
	if PurchaseForBank == PurchaseForInvestmentFund {
		t.Error("constants must be distinct")
	}
}

// ---- NoopFundCallback ----

func TestNoopFundCallback(t *testing.T) {
	n := NoopFundCallback{}
	if err := n.AddHolding(nil, 1, "AAPL", 5, decimal.NewFromFloat(100)); err != nil {
		t.Errorf("AddHolding: %v", err)
	}
	if err := n.DebitLiquidity(nil, 1, decimal.NewFromFloat(50), "test"); err != nil {
		t.Errorf("DebitLiquidity: %v", err)
	}
}

// ---- AuthUser ----

func TestAuthUser_IsClient(t *testing.T) {
	u := AuthUser{Roles: []string{"CLIENT"}}
	if !u.IsClient() {
		t.Error("CLIENT role should be client")
	}
}

func TestAuthUser_IsAgent(t *testing.T) {
	u := AuthUser{Roles: []string{"AGENT"}}
	if !u.IsAgent() {
		t.Error("AGENT role should be agent")
	}
}

func TestAuthUser_HasTradingPermission(t *testing.T) {
	u := AuthUser{Permissions: []string{"SECURITIES_TRADE"}}
	if !u.HasTradingPermission() {
		t.Error("should have trading permission with SECURITIES_TRADE")
	}
}

func TestAuthUser_HasTradingPermission_ClientTrading(t *testing.T) {
	u := AuthUser{Roles: []string{"CLIENT_TRADING"}}
	if !u.HasTradingPermission() {
		t.Error("CLIENT_TRADING role should have trading permission")
	}
}

func TestAuthUser_NoTradingPermission(t *testing.T) {
	u := AuthUser{Permissions: []string{"READ_ONLY"}}
	if u.HasTradingPermission() {
		t.Error("READ_ONLY should not have trading permission")
	}
}

func TestAuthUser_HasRole_CaseInsensitive(t *testing.T) {
	u := AuthUser{Roles: []string{"supervisor"}}
	if !u.HasRole("SUPERVISOR") {
		t.Error("role match should be case-insensitive")
	}
}

// ---- Order time constants ----

func TestExecutionDelayConstants(t *testing.T) {
	if initialExecutionDelay <= 0 {
		t.Error("initialExecutionDelay must be positive")
	}
	if retryDelayOnError <= 0 {
		t.Error("retryDelayOnError must be positive")
	}
}

// ---- hasPastSettlementDate with today's date ----

func TestHasPastSettlementDate_Today(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	listing := &clients.StockListing{SettlementDate: &today}
	// today is NOT before today, so should return false
	if hasPastSettlementDate(listing) {
		t.Error("today should not be 'past'")
	}
}
