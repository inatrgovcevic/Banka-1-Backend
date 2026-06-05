package order

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/clients"

	"github.com/shopspring/decimal"
)

// stubService creates a bare Service with nil dependencies for testing
// early-exit paths (paths that return before touching any dependency).
func stubService() *Service {
	return &Service{
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		notifier: NoopNotifier{},
		funds:    NoopFundCallback{},
	}
}

func intPtr(n int) *int { return &n }
func int64Ptr(n int64) *int64 { return &n }
func strPtr(s string) *string { return &s }
func decPtr(s string) *decimal.Decimal { d := decimal.RequireFromString(s); return &d }

// ---- CreateBuyOrder early exits (before market call) ----

func TestCreateBuyOrder_InvalidCommonRequest_NilListingID(t *testing.T) {
	svc := stubService()
	req := api.CreateBuyOrderRequest{ListingID: nil, Quantity: intPtr(1)}
	_, err := svc.CreateBuyOrder(context.Background(), AuthUser{}, req)
	if err == nil {
		t.Error("expected error for nil listingID")
	}
}

func TestCreateBuyOrder_InvalidCommonRequest_ZeroQuantity(t *testing.T) {
	svc := stubService()
	req := api.CreateBuyOrderRequest{ListingID: int64Ptr(1), Quantity: intPtr(0)}
	_, err := svc.CreateBuyOrder(context.Background(), AuthUser{}, req)
	if err == nil {
		t.Error("expected error for zero quantity")
	}
}

func TestCreateBuyOrder_InvalidPurchaseFor(t *testing.T) {
	svc := stubService()
	req := api.CreateBuyOrderRequest{
		ListingID:   int64Ptr(1),
		Quantity:    intPtr(5),
		PurchaseFor: strPtr("GARBAGE"),
	}
	_, err := svc.CreateBuyOrder(context.Background(), AuthUser{}, req)
	if err == nil {
		t.Error("expected error for invalid purchaseFor")
	}
}

func TestCreateBuyOrder_ClientBuyingForBank_403(t *testing.T) {
	svc := stubService()
	req := api.CreateBuyOrderRequest{
		ListingID:   int64Ptr(1),
		Quantity:    intPtr(5),
		PurchaseFor: strPtr("BANK"),
	}
	user := AuthUser{Roles: []string{"CLIENT"}}
	_, err := svc.CreateBuyOrder(context.Background(), user, req)
	if err == nil {
		t.Error("expected 403 for client buying for bank")
	}
}

func TestCreateBuyOrder_FundOrderWithoutFundID_400(t *testing.T) {
	svc := stubService()
	req := api.CreateBuyOrderRequest{
		ListingID:   int64Ptr(1),
		Quantity:    intPtr(5),
		PurchaseFor: strPtr("INVESTMENT_FUND"),
		FundID:      nil,
		AccountID:   int64Ptr(1),
	}
	user := AuthUser{Roles: []string{"SUPERVISOR"}} // not client
	_, err := svc.CreateBuyOrder(context.Background(), user, req)
	if err == nil {
		t.Error("expected 400 for fund order without fundId")
	}
}

func TestCreateBuyOrder_FundOrderWithoutAccountID_400(t *testing.T) {
	svc := stubService()
	req := api.CreateBuyOrderRequest{
		ListingID:   int64Ptr(1),
		Quantity:    intPtr(5),
		PurchaseFor: strPtr("INVESTMENT_FUND"),
		FundID:      int64Ptr(10),
		AccountID:   nil,
	}
	user := AuthUser{Roles: []string{"SUPERVISOR"}}
	_, err := svc.CreateBuyOrder(context.Background(), user, req)
	if err == nil {
		t.Error("expected 400 for fund order without accountId")
	}
}

// ---- CreateSellOrder early exits ----

func TestCreateSellOrder_InvalidCommonRequest(t *testing.T) {
	svc := stubService()
	req := api.CreateSellOrderRequest{ListingID: nil, Quantity: intPtr(1)}
	_, err := svc.CreateSellOrder(context.Background(), AuthUser{}, req)
	if err == nil {
		t.Error("expected error for nil listingID")
	}
}

// ---- validateTradingAccess ----

func TestValidateTradingAccess_NonClient_OK(t *testing.T) {
	svc := stubService()
	user := AuthUser{Roles: []string{"AGENT"}}
	listing := &clients.StockListing{}
	if err := svc.validateTradingAccess(user, listing); err != nil {
		t.Errorf("non-client should always have access, got %v", err)
	}
}

func TestValidateTradingAccess_ClientNoPermission_403(t *testing.T) {
	svc := stubService()
	user := AuthUser{Roles: []string{"CLIENT"}}
	listing := &clients.StockListing{}
	err := svc.validateTradingAccess(user, listing)
	if err == nil {
		t.Error("client without trading permission should get 403")
	}
}

func TestValidateTradingAccess_ClientWithPermission_Stock_OK(t *testing.T) {
	svc := stubService()
	user := AuthUser{Roles: []string{"CLIENT"}, Permissions: []string{"SECURITIES_TRADE"}}
	typ := "STOCK"
	listing := &clients.StockListing{ListingType: &typ}
	if err := svc.validateTradingAccess(user, listing); err != nil {
		t.Errorf("client with trading permission for STOCK should succeed: %v", err)
	}
}

func TestValidateTradingAccess_ClientWithPermission_Futures_OK(t *testing.T) {
	svc := stubService()
	user := AuthUser{Roles: []string{"CLIENT"}, Permissions: []string{"SECURITIES_TRADE"}}
	typ := "FUTURES"
	listing := &clients.StockListing{ListingType: &typ}
	if err := svc.validateTradingAccess(user, listing); err != nil {
		t.Errorf("client with trading permission for FUTURES should succeed: %v", err)
	}
}

func TestValidateTradingAccess_ClientWithPermission_Currency_403(t *testing.T) {
	svc := stubService()
	user := AuthUser{Roles: []string{"CLIENT"}, Permissions: []string{"SECURITIES_TRADE"}}
	typ := "FOREX"
	listing := &clients.StockListing{ListingType: &typ}
	err := svc.validateTradingAccess(user, listing)
	if err == nil {
		t.Error("client cannot trade FOREX, should get 403")
	}
}


// ---- decimalMin/decimalMax in execution ----

func TestDecimalMin_FirstSmaller(t *testing.T) {
	a, b := decimal.NewFromFloat(1), decimal.NewFromFloat(2)
	if !decimalMin(a, b).Equal(a) {
		t.Error("expected a < b → return a")
	}
}

func TestDecimalMax_SecondBigger(t *testing.T) {
	a, b := decimal.NewFromFloat(1), decimal.NewFromFloat(2)
	if !decimalMax(a, b).Equal(b) {
		t.Error("expected a < b → return b")
	}
}

func TestDecimalMaxZero_Negative(t *testing.T) {
	a := decimal.NewFromFloat(-5)
	if !decimalMaxZero(a).IsZero() {
		t.Error("max(negative, 0) should be 0")
	}
}

func TestDecimalMaxZero_Positive(t *testing.T) {
	a := decimal.NewFromFloat(7)
	if !decimalMaxZero(a).Equal(a) {
		t.Error("max(positive, 0) should be positive")
	}
}
