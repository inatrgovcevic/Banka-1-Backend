package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

// The /public-stock seller entry must serialize with wire keys "seller" and
// "amount" — matching Banka 2's PublicStock.Seller(seller, amount) record AND
// Banka 1's own live api.SellerRow handler. The legacy keys "sellerId" /
// "quantity" silently produced zero-value sellers when decoding a partner's
// real /public-stock response (OutboundFetchPublicStock). This guards the fix.
func TestPublicStockSellerRef_WireKeys(t *testing.T) {
	ref := PublicStockSellerRef{
		SellerID: ForeignBankId{RoutingNumber: 222, Id: "C-2"},
		Quantity: decimal.NewFromInt(50),
	}
	b, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"seller"`) || !strings.Contains(s, `"amount"`) {
		t.Errorf("expected wire keys seller+amount, got %s", s)
	}
	if strings.Contains(s, `"sellerId"`) || strings.Contains(s, `"quantity"`) {
		t.Errorf("must not emit legacy keys sellerId/quantity, got %s", s)
	}

	// CRITICAL: decode a SCALED Banka-2-shaped amount (1.0000). Banka 2's live
	// /public-stock emits scaled decimals; a Go int could not decode 1.0000 and
	// would fail the whole response. decimal.Decimal must accept it.
	var back PublicStockSellerRef
	if err := json.Unmarshal([]byte(`{"seller":{"routingNumber":222,"id":"C-2"},"amount":1.0000}`), &back); err != nil {
		t.Fatalf("unmarshal scaled amount 1.0000: %v", err)
	}
	if back.SellerID.RoutingNumber != 222 || back.SellerID.Id != "C-2" || !back.Quantity.Equal(decimal.NewFromInt(1)) {
		t.Errorf("round-trip mismatch: %+v", back)
	}
}
