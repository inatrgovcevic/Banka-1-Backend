package grpc

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	commonv1 "github.com/raf-si-2025/banka-1-go/proto/banka1/common/v1"
	interbankv1 "github.com/raf-si-2025/banka-1-go/proto/banka1/interbank/v1"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/service"
)

func TestMapProtoForeignBankId(t *testing.T) {
	if got := mapProtoForeignBankId(nil); got != (protocol.ForeignBankId{}) {
		t.Errorf("nil → %+v, want zero", got)
	}
	got := mapProtoForeignBankId(&commonv1.ForeignBankId{RoutingNumber: 222, Id: "C-7"})
	if got.RoutingNumber != 222 || got.Id != "C-7" {
		t.Errorf("got %+v", got)
	}
}

func TestMapProtoMonetaryValue(t *testing.T) {
	mv, err := mapProtoMonetaryValue(nil)
	if err != nil || mv != (protocol.MonetaryValue{}) {
		t.Errorf("nil → %+v, %v", mv, err)
	}

	mv, err = mapProtoMonetaryValue(&commonv1.MonetaryValue{Currency: "USD", Amount: "150.50"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if mv.Currency != "USD" || !mv.Amount.Equal(decimal.RequireFromString("150.50")) {
		t.Errorf("got %+v", mv)
	}

	if _, err := mapProtoMonetaryValue(&commonv1.MonetaryValue{Currency: "USD", Amount: "not-a-number"}); err == nil {
		t.Error("expected error for invalid amount")
	}
}

func TestMapProtoTxAccount(t *testing.T) {
	if _, err := mapProtoTxAccount(nil); err == nil {
		t.Error("nil should error")
	}

	person, err := mapProtoTxAccount(&interbankv1.TxAccount{
		Body: &interbankv1.TxAccount_Person{Person: &commonv1.ForeignBankId{RoutingNumber: 111, Id: "C-1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pa, ok := person.(*protocol.PersonAccount); !ok || pa.Id.Id != "C-1" {
		t.Errorf("person: %+v", person)
	}

	real, err := mapProtoTxAccount(&interbankv1.TxAccount{
		Body: &interbankv1.TxAccount_AccountNum{AccountNum: "111000000000000001"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ra, ok := real.(*protocol.RealAccount); !ok || ra.Num != "111000000000000001" {
		t.Errorf("real: %+v", real)
	}

	opt, err := mapProtoTxAccount(&interbankv1.TxAccount{
		Body: &interbankv1.TxAccount_Option{Option: &commonv1.ForeignBankId{RoutingNumber: 111, Id: "neg-1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if oa, ok := opt.(*protocol.OptionPseudoAccount); !ok || oa.Id.Id != "neg-1" {
		t.Errorf("option: %+v", opt)
	}

	// Unknown oneof (empty body)
	if _, err := mapProtoTxAccount(&interbankv1.TxAccount{}); err == nil {
		t.Error("empty body should error")
	}
}

func TestMapProtoAsset(t *testing.T) {
	if _, err := mapProtoAsset(nil); err == nil {
		t.Error("nil should error")
	}

	monas, err := mapProtoAsset(&interbankv1.Asset{
		Body: &interbankv1.Asset_Monas{Monas: &commonv1.MonetaryValue{Currency: "EUR", Amount: "0"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ma, ok := monas.(*protocol.MonasAsset); !ok || ma.Currency != "EUR" {
		t.Errorf("monas: %+v", monas)
	}

	// nil Monas inner
	if _, err := mapProtoAsset(&interbankv1.Asset{Body: &interbankv1.Asset_Monas{Monas: nil}}); err == nil {
		t.Error("nil monas inner should error")
	}

	stock, err := mapProtoAsset(&interbankv1.Asset{
		Body: &interbankv1.Asset_Stock{Stock: &commonv1.StockDescription{Ticker: "AAPL"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sa, ok := stock.(*protocol.StockAsset); !ok || sa.Ticker != "AAPL" {
		t.Errorf("stock: %+v", stock)
	}

	if _, err := mapProtoAsset(&interbankv1.Asset{Body: &interbankv1.Asset_Stock{Stock: nil}}); err == nil {
		t.Error("nil stock inner should error")
	}

	opt, err := mapProtoAsset(&interbankv1.Asset{
		Body: &interbankv1.Asset_Option{Option: &commonv1.OptionDescription{
			NegotiationId:  &commonv1.ForeignBankId{RoutingNumber: 111, Id: "neg-1"},
			Stock:          &commonv1.StockDescription{Ticker: "AAPL"},
			PricePerUnit:   &commonv1.MonetaryValue{Currency: "USD", Amount: "200.00"},
			SettlementDate: "2026-12-01T00:00:00Z",
			Amount:         10,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	oa, ok := opt.(*protocol.OptionAsset)
	if !ok || oa.NegotiationId.Id != "neg-1" || oa.Stock.Ticker != "AAPL" || oa.Amount != 10 {
		t.Errorf("option: %+v", opt)
	}

	if _, err := mapProtoAsset(&interbankv1.Asset{Body: &interbankv1.Asset_Option{Option: nil}}); err == nil {
		t.Error("nil option inner should error")
	}

	// option with bad price
	if _, err := mapProtoAsset(&interbankv1.Asset{
		Body: &interbankv1.Asset_Option{Option: &commonv1.OptionDescription{
			PricePerUnit: &commonv1.MonetaryValue{Currency: "USD", Amount: "bad"},
		}},
	}); err == nil {
		t.Error("bad option price should error")
	}

	// unknown oneof
	if _, err := mapProtoAsset(&interbankv1.Asset{}); err == nil {
		t.Error("empty asset body should error")
	}
}

func TestMapProtoPosting(t *testing.T) {
	if _, err := mapProtoPosting(nil); err == nil {
		t.Error("nil should error")
	}

	p, err := mapProtoPosting(&interbankv1.Posting{
		Account: &interbankv1.TxAccount{Body: &interbankv1.TxAccount_AccountNum{AccountNum: "111000000000000001"}},
		Amount:  "-100.00",
		Asset:   &interbankv1.Asset{Body: &interbankv1.Asset_Monas{Monas: &commonv1.MonetaryValue{Currency: "USD", Amount: "0"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Amount.Equal(decimal.RequireFromString("-100.00")) {
		t.Errorf("amount %v", p.Amount)
	}

	// bad amount
	if _, err := mapProtoPosting(&interbankv1.Posting{Amount: "x"}); err == nil {
		t.Error("bad amount should error")
	}
	// bad account
	if _, err := mapProtoPosting(&interbankv1.Posting{Amount: "1", Account: &interbankv1.TxAccount{}}); err == nil {
		t.Error("bad account should error")
	}
	// bad asset
	if _, err := mapProtoPosting(&interbankv1.Posting{
		Amount:  "1",
		Account: &interbankv1.TxAccount{Body: &interbankv1.TxAccount_AccountNum{AccountNum: "1"}},
		Asset:   &interbankv1.Asset{},
	}); err == nil {
		t.Error("bad asset should error")
	}
}

func TestMapProtoTx(t *testing.T) {
	if _, err := mapProtoTx(nil); err == nil {
		t.Error("nil should error")
	}

	tx, err := mapProtoTx(&interbankv1.InterbankTransactionPayload{
		TransactionId:  &commonv1.ForeignBankId{RoutingNumber: 222, Id: "tx-1"},
		Message:        "hello",
		CallNumber:     "97",
		PaymentCode:    "289",
		PaymentPurpose: "transfer",
		Postings: []*interbankv1.Posting{
			{
				Account: &interbankv1.TxAccount{Body: &interbankv1.TxAccount_AccountNum{AccountNum: "1"}},
				Amount:  "100",
				Asset:   &interbankv1.Asset{Body: &interbankv1.Asset_Monas{Monas: &commonv1.MonetaryValue{Currency: "USD", Amount: "0"}}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tx.TransactionId.Id != "tx-1" || tx.Message != "hello" || len(tx.Postings) != 1 {
		t.Errorf("tx: %+v", tx)
	}

	// posting error propagates with index
	if _, err := mapProtoTx(&interbankv1.InterbankTransactionPayload{
		Postings: []*interbankv1.Posting{{Amount: "bad"}},
	}); err == nil {
		t.Error("bad posting should error")
	}
}

func TestMapProtoOtcOfferFromCreate(t *testing.T) {
	settle := time.Now().Add(720 * time.Hour).UTC().Format(time.RFC3339)
	offer, err := mapProtoOtcOfferFromCreate(&interbankv1.CreateNegotiationRequest{
		BuyerId:          &commonv1.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
		SellerId:         &commonv1.ForeignBankId{RoutingNumber: 111, Id: "C-5"},
		LastModifiedBy:   &commonv1.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
		StockDescription: &commonv1.StockDescription{Ticker: "AAPL"},
		PricePerUnit:     &commonv1.MonetaryValue{Currency: "USD", Amount: "150.00"},
		Premium:          &commonv1.MonetaryValue{Currency: "USD", Amount: "10.00"},
		Amount:           10,
		SettlementDate:   settle,
	})
	if err != nil {
		t.Fatal(err)
	}
	if offer.Stock.Ticker != "AAPL" || offer.Amount != 10 || offer.BuyerID.Id != "C-2" {
		t.Errorf("offer: %+v", offer)
	}

	// bad date
	if _, err := mapProtoOtcOfferFromCreate(&interbankv1.CreateNegotiationRequest{SettlementDate: "not-a-date"}); err == nil {
		t.Error("bad date should error")
	}
	// bad price
	if _, err := mapProtoOtcOfferFromCreate(&interbankv1.CreateNegotiationRequest{
		SettlementDate: settle,
		PricePerUnit:   &commonv1.MonetaryValue{Currency: "USD", Amount: "bad"},
	}); err == nil {
		t.Error("bad price should error")
	}
	// bad premium
	if _, err := mapProtoOtcOfferFromCreate(&interbankv1.CreateNegotiationRequest{
		SettlementDate: settle,
		Premium:        &commonv1.MonetaryValue{Currency: "USD", Amount: "bad"},
	}); err == nil {
		t.Error("bad premium should error")
	}
}

func TestMapProtoOtcOfferFromPut(t *testing.T) {
	settle := time.Now().Add(720 * time.Hour).UTC().Format(time.RFC3339)
	offer, err := mapProtoOtcOfferFromPut(&interbankv1.PutCounterRequest{
		BuyerId:          &commonv1.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
		SellerId:         &commonv1.ForeignBankId{RoutingNumber: 111, Id: "C-5"},
		LastModifiedBy:   &commonv1.ForeignBankId{RoutingNumber: 111, Id: "C-5"},
		StockDescription: &commonv1.StockDescription{Ticker: "MSFT"},
		PricePerUnit:     &commonv1.MonetaryValue{Currency: "USD", Amount: "150.00"},
		Premium:          &commonv1.MonetaryValue{Currency: "USD", Amount: "10.00"},
		Amount:           5,
		SettlementDate:   settle,
	})
	if err != nil {
		t.Fatal(err)
	}
	if offer.Stock.Ticker != "MSFT" || offer.Amount != 5 || offer.LastModifiedBy.Id != "C-5" {
		t.Errorf("offer: %+v", offer)
	}

	if _, err := mapProtoOtcOfferFromPut(&interbankv1.PutCounterRequest{SettlementDate: "bad"}); err == nil {
		t.Error("bad date should error")
	}
	if _, err := mapProtoOtcOfferFromPut(&interbankv1.PutCounterRequest{
		SettlementDate: settle,
		PricePerUnit:   &commonv1.MonetaryValue{Currency: "USD", Amount: "bad"},
	}); err == nil {
		t.Error("bad price should error")
	}
	if _, err := mapProtoOtcOfferFromPut(&interbankv1.PutCounterRequest{
		SettlementDate: settle,
		Premium:        &commonv1.MonetaryValue{Currency: "USD", Amount: "bad"},
	}); err == nil {
		t.Error("bad premium should error")
	}
}

func TestMapForeignBankIdToProto(t *testing.T) {
	got := mapForeignBankIdToProto(protocol.ForeignBankId{RoutingNumber: 111, Id: "C-1"})
	if got.GetRoutingNumber() != 111 || got.GetId() != "C-1" {
		t.Errorf("got %+v", got)
	}
}

func TestMapMonetaryValueToProto(t *testing.T) {
	got := mapMonetaryValueToProto(protocol.MonetaryValue{Currency: "USD", Amount: decimal.RequireFromString("12.34")})
	if got.GetCurrency() != "USD" || got.GetAmount() != "12.34" {
		t.Errorf("got %+v", got)
	}
}

func TestMapVoteToProto(t *testing.T) {
	yes := mapVoteToProto(protocol.TransactionVote{Vote: protocol.VoteYes})
	if yes.GetVote() != interbankv1.TransactionVote_VOTE_YES {
		t.Errorf("yes: %v", yes.GetVote())
	}

	no := mapVoteToProto(protocol.TransactionVote{
		Vote:    protocol.VoteNo,
		Reasons: []protocol.NoVoteReason{{Reason: protocol.ReasonUnbalancedTx}},
	})
	if no.GetVote() != interbankv1.TransactionVote_VOTE_NO || len(no.GetReasons()) != 1 {
		t.Errorf("no: %+v", no)
	}

	unspec := mapVoteToProto(protocol.TransactionVote{Vote: "WEIRD"})
	if unspec.GetVote() != interbankv1.TransactionVote_VOTE_UNSPECIFIED {
		t.Errorf("unspec: %v", unspec.GetVote())
	}
}

func TestMapNoVoteReasonToProto(t *testing.T) {
	posting := protocol.Posting{
		Account: &protocol.RealAccount{Num: "1"},
		Amount:  decimal.RequireFromString("1"),
		Asset:   &protocol.MonasAsset{Currency: "USD"},
	}
	r := mapNoVoteReasonToProto(protocol.NoVoteReason{Reason: protocol.ReasonNoSuchAccount, Posting: &posting})
	if r.GetReason() != interbankv1.NoVoteReason_REASON_NO_SUCH_ACCOUNT || r.GetPosting() == nil {
		t.Errorf("with posting: %+v", r)
	}

	r2 := mapNoVoteReasonToProto(protocol.NoVoteReason{Reason: protocol.ReasonUnbalancedTx})
	if r2.GetPosting() != nil {
		t.Errorf("expected nil posting, got %+v", r2.GetPosting())
	}
}

func TestMapReasonStringToProto(t *testing.T) {
	cases := map[string]interbankv1.NoVoteReason_Reason{
		protocol.ReasonUnbalancedTx:              interbankv1.NoVoteReason_REASON_UNBALANCED_TX,
		protocol.ReasonNoSuchAccount:             interbankv1.NoVoteReason_REASON_NO_SUCH_ACCOUNT,
		protocol.ReasonNoSuchAsset:               interbankv1.NoVoteReason_REASON_NO_SUCH_ASSET,
		protocol.ReasonUnacceptableAsset:         interbankv1.NoVoteReason_REASON_UNACCEPTABLE_ASSET,
		protocol.ReasonInsufficientAsset:         interbankv1.NoVoteReason_REASON_INSUFFICIENT_ASSET,
		protocol.ReasonOptionAmountIncorrect:     interbankv1.NoVoteReason_REASON_OPTION_AMOUNT_INCORRECT,
		protocol.ReasonOptionUsedOrExpired:       interbankv1.NoVoteReason_REASON_OPTION_USED_OR_EXPIRED,
		protocol.ReasonOptionNegotiationNotFound: interbankv1.NoVoteReason_REASON_OPTION_NEGOTIATION_NOT_FOUND,
		"UNKNOWN_REASON":                         interbankv1.NoVoteReason_REASON_UNSPECIFIED,
	}
	for in, want := range cases {
		if got := mapReasonStringToProto(in); got != want {
			t.Errorf("%q → %v, want %v", in, got, want)
		}
	}
}

func TestMapTxAccountToProto(t *testing.T) {
	person := mapTxAccountToProto(&protocol.PersonAccount{Id: protocol.ForeignBankId{RoutingNumber: 1, Id: "C-1"}})
	if person.GetPerson() == nil {
		t.Error("expected person body")
	}
	real := mapTxAccountToProto(&protocol.RealAccount{Num: "111"})
	if real.GetAccountNum() != "111" {
		t.Errorf("real: %+v", real)
	}
	opt := mapTxAccountToProto(&protocol.OptionPseudoAccount{Id: protocol.ForeignBankId{Id: "neg-1"}})
	if opt.GetOption() == nil {
		t.Error("expected option body")
	}
	// default: nil interface
	def := mapTxAccountToProto(nil)
	if def == nil {
		t.Error("expected non-nil empty struct")
	}
}

func TestMapAssetToProto(t *testing.T) {
	monas := mapAssetToProto(&protocol.MonasAsset{Currency: "USD"})
	if monas.GetMonas().GetCurrency() != "USD" {
		t.Errorf("monas: %+v", monas)
	}
	stock := mapAssetToProto(&protocol.StockAsset{Ticker: "AAPL"})
	if stock.GetStock().GetTicker() != "AAPL" {
		t.Errorf("stock: %+v", stock)
	}
	opt := mapAssetToProto(&protocol.OptionAsset{OptionDescription: protocol.OptionDescription{
		NegotiationId:  protocol.ForeignBankId{RoutingNumber: 111, Id: "neg-1"},
		Stock:          protocol.StockDescription{Ticker: "AAPL"},
		PricePerUnit:   protocol.MonetaryValue{Currency: "USD", Amount: decimal.RequireFromString("200.00")},
		SettlementDate: "2026-12-01T00:00:00Z",
		Amount:         10,
	}})
	if opt.GetOption().GetStock().GetTicker() != "AAPL" || opt.GetOption().GetAmount() != 10 {
		t.Errorf("option: %+v", opt)
	}
	def := mapAssetToProto(nil)
	if def == nil {
		t.Error("expected non-nil empty struct")
	}
}

func TestMapPostingToProto(t *testing.T) {
	p := mapPostingToProto(protocol.Posting{
		Account: &protocol.RealAccount{Num: "111"},
		Amount:  decimal.RequireFromString("-100.00"),
		Asset:   &protocol.MonasAsset{Currency: "USD"},
	})
	if p.GetAmount() != "-100" && p.GetAmount() != "-100.00" {
		t.Errorf("amount: %q", p.GetAmount())
	}
	if p.GetAccount().GetAccountNum() != "111" {
		t.Errorf("account: %+v", p.GetAccount())
	}
}

func TestMapNegotiationDtoToProto(t *testing.T) {
	settle := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	out := mapNegotiationDtoToProto(
		protocol.ForeignBankId{RoutingNumber: 111, Id: "neg-1"},
		service.OtcNegotiationDto{
			Stock:          protocol.StockDescription{Ticker: "AAPL"},
			SettlementDate: settle,
			PricePerUnit:   protocol.MonetaryValue{Currency: "USD", Amount: decimal.RequireFromString("150.00")},
			Premium:        protocol.MonetaryValue{Currency: "USD", Amount: decimal.RequireFromString("10.00")},
			BuyerID:        protocol.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
			SellerID:       protocol.ForeignBankId{RoutingNumber: 111, Id: "C-5"},
			Amount:         10,
			LastModifiedBy: protocol.ForeignBankId{RoutingNumber: 222, Id: "C-2"},
			IsOngoing:      true,
		},
	)
	if out.GetId().GetId() != "neg-1" || out.GetStockDescription().GetTicker() != "AAPL" {
		t.Errorf("out: %+v", out)
	}
	if !out.GetIsOngoing() || out.GetAmount() != 10 {
		t.Errorf("out: %+v", out)
	}
	if out.GetSettlementDate() != "2026-12-01T00:00:00Z" {
		t.Errorf("settlementDate: %q", out.GetSettlementDate())
	}
}
