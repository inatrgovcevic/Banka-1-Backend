package protocol

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
)

// TestAssetTypeMarkers exercises the Type() and isAsset() marker methods on all
// three asset variants (value receivers).
func TestAssetTypeMarkers(t *testing.T) {
	var assets = []struct {
		a    Asset
		want AssetType
	}{
		{MonasAsset{Currency: "USD"}, AssetTypeMonas},
		{StockAsset{Ticker: "AAPL"}, AssetTypeStock},
		{OptionAsset{}, AssetTypeOption},
	}
	for _, c := range assets {
		if c.a.Type() != c.want {
			t.Errorf("%T.Type()=%v want %v", c.a, c.a.Type(), c.want)
		}
		c.a.isAsset() // marker, just exercise it
	}
}

// TestTxAccountTypeMarkers exercises Type() and isTxAccount() on all three
// account variants.
func TestTxAccountTypeMarkers(t *testing.T) {
	var accts = []struct {
		a    TxAccount
		want TxAccountType
	}{
		{PersonAccount{}, TxAccountTypePerson},
		{RealAccount{Num: "111000000000000001"}, TxAccountTypeAccount},
		{OptionPseudoAccount{}, TxAccountTypeOption},
	}
	for _, c := range accts {
		if c.a.Type() != c.want {
			t.Errorf("%T.Type()=%v want %v", c.a, c.a.Type(), c.want)
		}
		c.a.isTxAccount()
	}
}

func TestStockAsset_MarshalJSON(t *testing.T) {
	out, err := json.Marshal(&StockAsset{Ticker: "AAPL"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(`"type":"STOCK"`)) || !bytes.Contains(out, []byte(`"ticker":"AAPL"`)) {
		t.Errorf("stock marshal: %s", out)
	}
	// round-trip
	a, err := UnmarshalAsset(out)
	if err != nil {
		t.Fatal(err)
	}
	if sa, ok := a.(*StockAsset); !ok || sa.Ticker != "AAPL" {
		t.Errorf("round-trip: %+v", a)
	}
}

func TestOptionAsset_MarshalJSON(t *testing.T) {
	oa := &OptionAsset{OptionDescription: OptionDescription{
		NegotiationId:  ForeignBankId{RoutingNumber: 111, Id: "neg-1"},
		Stock:          StockDescription{Ticker: "AAPL"},
		PricePerUnit:   MonetaryValue{Currency: "USD", Amount: decimal.RequireFromString("200.00")},
		SettlementDate: "2026-12-01T00:00:00Z",
		Amount:         10,
	}}
	out, err := json.Marshal(oa)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(`"type":"OPTION"`)) || !bytes.Contains(out, []byte(`"negotiationId"`)) {
		t.Errorf("option marshal: %s", out)
	}
	a, err := UnmarshalAsset(out)
	if err != nil {
		t.Fatal(err)
	}
	if ro, ok := a.(*OptionAsset); !ok || ro.NegotiationId.Id != "neg-1" || ro.Amount != 10 {
		t.Errorf("round-trip: %+v", a)
	}
}

func TestPersonAccount_MarshalJSON(t *testing.T) {
	out, err := json.Marshal(&PersonAccount{Id: ForeignBankId{RoutingNumber: 111, Id: "C-7"}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(`"type":"PERSON"`)) || !bytes.Contains(out, []byte(`"id":`)) {
		t.Errorf("person marshal: %s", out)
	}
	a, err := UnmarshalTxAccount(out)
	if err != nil {
		t.Fatal(err)
	}
	if pa, ok := a.(*PersonAccount); !ok || pa.Id.Id != "C-7" {
		t.Errorf("round-trip: %+v", a)
	}
}

func TestRealAccount_MarshalJSON(t *testing.T) {
	out, err := json.Marshal(&RealAccount{Num: "111000001234567890"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(`"type":"ACCOUNT"`)) || !bytes.Contains(out, []byte(`"num":"111000001234567890"`)) {
		t.Errorf("account marshal: %s", out)
	}
}

func TestOptionPseudoAccount_MarshalJSON(t *testing.T) {
	out, err := json.Marshal(&OptionPseudoAccount{Id: ForeignBankId{RoutingNumber: 111, Id: "neg-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(`"type":"OPTION"`)) {
		t.Errorf("option pseudo marshal: %s", out)
	}
	a, err := UnmarshalTxAccount(out)
	if err != nil {
		t.Fatal(err)
	}
	if oa, ok := a.(*OptionPseudoAccount); !ok || oa.Id.Id != "neg-1" {
		t.Errorf("round-trip: %+v", a)
	}
}

// Error-path coverage for the polymorphic unmarshalers.
func TestUnmarshalAsset_Errors(t *testing.T) {
	if _, err := UnmarshalAsset([]byte(`not json`)); err == nil {
		t.Error("bad envelope should error")
	}
	if _, err := UnmarshalAsset([]byte(`{"type":"MONAS","asset":"oops"}`)); err == nil {
		t.Error("bad monas inner should error")
	}
	if _, err := UnmarshalAsset([]byte(`{"type":"STOCK","asset":123}`)); err == nil {
		t.Error("bad stock inner should error")
	}
	if _, err := UnmarshalAsset([]byte(`{"type":"OPTION","asset":"oops"}`)); err == nil {
		t.Error("bad option inner should error")
	}
	if _, err := UnmarshalAsset([]byte(`{"type":"BOGUS"}`)); err == nil {
		t.Error("unknown type should error")
	}
}

func TestUnmarshalTxAccount_Errors(t *testing.T) {
	if _, err := UnmarshalTxAccount([]byte(`not json`)); err == nil {
		t.Error("bad envelope should error")
	}
	if _, err := UnmarshalTxAccount([]byte(`{"type":"PERSON","id":123}`)); err == nil {
		t.Error("bad person id should error")
	}
	if _, err := UnmarshalTxAccount([]byte(`{"type":"OPTION","id":123}`)); err == nil {
		t.Error("bad option id should error")
	}
	if _, err := UnmarshalTxAccount([]byte(`{"type":"BOGUS"}`)); err == nil {
		t.Error("unknown type should error")
	}
}
