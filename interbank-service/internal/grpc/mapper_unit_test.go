package grpc

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	commonv1 "github.com/raf-si-2025/banka-1-go/proto/banka1/common/v1"
	interbankv1 "github.com/raf-si-2025/banka-1-go/proto/banka1/interbank/v1"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/service"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---------------------------------------------------------------------------
// proto → domain mappers
// ---------------------------------------------------------------------------

func TestUnit_MapProtoForeignBankId(t *testing.T) {
	if got := mapProtoForeignBankId(nil); got != (protocol.ForeignBankId{}) {
		t.Errorf("nil → %+v", got)
	}
	got := mapProtoForeignBankId(&commonv1.ForeignBankId{RoutingNumber: 222, Id: "C-7"})
	if got.RoutingNumber != 222 || got.Id != "C-7" {
		t.Errorf("got %+v", got)
	}
}

func TestUnit_MapProtoMonetaryValue(t *testing.T) {
	mv, err := mapProtoMonetaryValue(nil)
	if err != nil || mv != (protocol.MonetaryValue{}) {
		t.Errorf("nil → %+v %v", mv, err)
	}
	mv, err = mapProtoMonetaryValue(&commonv1.MonetaryValue{Currency: "USD", Amount: "150.50"})
	if err != nil || mv.Currency != "USD" || !mv.Amount.Equal(decimal.RequireFromString("150.50")) {
		t.Errorf("got %+v %v", mv, err)
	}
	if _, err := mapProtoMonetaryValue(&commonv1.MonetaryValue{Amount: "bad"}); err == nil {
		t.Error("expected error for bad amount")
	}
}

func TestUnit_MapProtoTxAccount(t *testing.T) {
	if _, err := mapProtoTxAccount(nil); err == nil {
		t.Error("nil should error")
	}
	person, err := mapProtoTxAccount(&interbankv1.TxAccount{Body: &interbankv1.TxAccount_Person{Person: &commonv1.ForeignBankId{Id: "C-1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if pa, ok := person.(*protocol.PersonAccount); !ok || pa.Id.Id != "C-1" {
		t.Errorf("person: %+v", person)
	}
	real, err := mapProtoTxAccount(&interbankv1.TxAccount{Body: &interbankv1.TxAccount_AccountNum{AccountNum: "111"}})
	if err != nil {
		t.Fatal(err)
	}
	if ra, ok := real.(*protocol.RealAccount); !ok || ra.Num != "111" {
		t.Errorf("real: %+v", real)
	}
	opt, err := mapProtoTxAccount(&interbankv1.TxAccount{Body: &interbankv1.TxAccount_Option{Option: &commonv1.ForeignBankId{Id: "neg-1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if oa, ok := opt.(*protocol.OptionPseudoAccount); !ok || oa.Id.Id != "neg-1" {
		t.Errorf("option: %+v", opt)
	}
	if _, err := mapProtoTxAccount(&interbankv1.TxAccount{}); err == nil {
		t.Error("empty body should error")
	}
}

func TestUnit_MapProtoAsset(t *testing.T) {
	if _, err := mapProtoAsset(nil); err == nil {
		t.Error("nil should error")
	}
	monas, err := mapProtoAsset(&interbankv1.Asset{Body: &interbankv1.Asset_Monas{Monas: &commonv1.MonetaryValue{Currency: "EUR"}}})
	if err != nil {
		t.Fatal(err)
	}
	if ma, ok := monas.(*protocol.MonasAsset); !ok || ma.Currency != "EUR" {
		t.Errorf("monas: %+v", monas)
	}
	if _, err := mapProtoAsset(&interbankv1.Asset{Body: &interbankv1.Asset_Monas{Monas: nil}}); err == nil {
		t.Error("nil monas inner should error")
	}
	stock, err := mapProtoAsset(&interbankv1.Asset{Body: &interbankv1.Asset_Stock{Stock: &commonv1.StockDescription{Ticker: "AAPL"}}})
	if err != nil {
		t.Fatal(err)
	}
	if sa, ok := stock.(*protocol.StockAsset); !ok || sa.Ticker != "AAPL" {
		t.Errorf("stock: %+v", stock)
	}
	if _, err := mapProtoAsset(&interbankv1.Asset{Body: &interbankv1.Asset_Stock{Stock: nil}}); err == nil {
		t.Error("nil stock inner should error")
	}
	opt, err := mapProtoAsset(&interbankv1.Asset{Body: &interbankv1.Asset_Option{Option: &commonv1.OptionDescription{
		NegotiationId:  &commonv1.ForeignBankId{Id: "neg-1"},
		Stock:          &commonv1.StockDescription{Ticker: "AAPL"},
		PricePerUnit:   &commonv1.MonetaryValue{Currency: "USD", Amount: "200.00"},
		SettlementDate: "2026-12-01T00:00:00Z",
		Amount:         10,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if oa, ok := opt.(*protocol.OptionAsset); !ok || oa.NegotiationId.Id != "neg-1" || oa.Amount != 10 {
		t.Errorf("option: %+v", opt)
	}
	if _, err := mapProtoAsset(&interbankv1.Asset{Body: &interbankv1.Asset_Option{Option: nil}}); err == nil {
		t.Error("nil option inner should error")
	}
	if _, err := mapProtoAsset(&interbankv1.Asset{Body: &interbankv1.Asset_Option{Option: &commonv1.OptionDescription{
		PricePerUnit: &commonv1.MonetaryValue{Amount: "bad"},
	}}}); err == nil {
		t.Error("bad option price should error")
	}
	if _, err := mapProtoAsset(&interbankv1.Asset{}); err == nil {
		t.Error("empty asset body should error")
	}
}

func TestUnit_MapProtoPosting(t *testing.T) {
	if _, err := mapProtoPosting(nil); err == nil {
		t.Error("nil should error")
	}
	p, err := mapProtoPosting(&interbankv1.Posting{
		Account: &interbankv1.TxAccount{Body: &interbankv1.TxAccount_AccountNum{AccountNum: "111"}},
		Amount:  "-100.00",
		Asset:   &interbankv1.Asset{Body: &interbankv1.Asset_Monas{Monas: &commonv1.MonetaryValue{Currency: "USD"}}},
	})
	if err != nil || !p.Amount.Equal(decimal.RequireFromString("-100.00")) {
		t.Errorf("got %+v %v", p, err)
	}
	if _, err := mapProtoPosting(&interbankv1.Posting{Amount: "x"}); err == nil {
		t.Error("bad amount should error")
	}
	if _, err := mapProtoPosting(&interbankv1.Posting{Amount: "1", Account: &interbankv1.TxAccount{}}); err == nil {
		t.Error("bad account should error")
	}
	if _, err := mapProtoPosting(&interbankv1.Posting{
		Amount:  "1",
		Account: &interbankv1.TxAccount{Body: &interbankv1.TxAccount_AccountNum{AccountNum: "1"}},
		Asset:   &interbankv1.Asset{},
	}); err == nil {
		t.Error("bad asset should error")
	}
}

func TestUnit_MapProtoTx(t *testing.T) {
	if _, err := mapProtoTx(nil); err == nil {
		t.Error("nil should error")
	}
	tx, err := mapProtoTx(&interbankv1.InterbankTransactionPayload{
		TransactionId: &commonv1.ForeignBankId{Id: "tx-1"},
		Message:       "hello",
		Postings: []*interbankv1.Posting{{
			Account: &interbankv1.TxAccount{Body: &interbankv1.TxAccount_AccountNum{AccountNum: "1"}},
			Amount:  "100",
			Asset:   &interbankv1.Asset{Body: &interbankv1.Asset_Monas{Monas: &commonv1.MonetaryValue{Currency: "USD"}}},
		}},
	})
	if err != nil || tx.TransactionId.Id != "tx-1" || len(tx.Postings) != 1 {
		t.Errorf("got %+v %v", tx, err)
	}
	if _, err := mapProtoTx(&interbankv1.InterbankTransactionPayload{Postings: []*interbankv1.Posting{{Amount: "bad"}}}); err == nil {
		t.Error("bad posting should error")
	}
}

func TestUnit_MapProtoOtcOfferFromCreate(t *testing.T) {
	settle := time.Now().Add(720 * time.Hour).UTC().Format(time.RFC3339)
	offer, err := mapProtoOtcOfferFromCreate(&interbankv1.CreateNegotiationRequest{
		BuyerId:          &commonv1.ForeignBankId{Id: "C-2"},
		StockDescription: &commonv1.StockDescription{Ticker: "AAPL"},
		PricePerUnit:     &commonv1.MonetaryValue{Currency: "USD", Amount: "150.00"},
		Premium:          &commonv1.MonetaryValue{Currency: "USD", Amount: "10.00"},
		Amount:           10,
		SettlementDate:   settle,
	})
	if err != nil || offer.Stock.Ticker != "AAPL" || offer.Amount != 10 {
		t.Errorf("got %+v %v", offer, err)
	}
	if _, err := mapProtoOtcOfferFromCreate(&interbankv1.CreateNegotiationRequest{SettlementDate: "bad"}); err == nil {
		t.Error("bad date should error")
	}
	if _, err := mapProtoOtcOfferFromCreate(&interbankv1.CreateNegotiationRequest{SettlementDate: settle, PricePerUnit: &commonv1.MonetaryValue{Amount: "bad"}}); err == nil {
		t.Error("bad price should error")
	}
	if _, err := mapProtoOtcOfferFromCreate(&interbankv1.CreateNegotiationRequest{SettlementDate: settle, Premium: &commonv1.MonetaryValue{Amount: "bad"}}); err == nil {
		t.Error("bad premium should error")
	}
}

func TestUnit_MapProtoOtcOfferFromPut(t *testing.T) {
	settle := time.Now().Add(720 * time.Hour).UTC().Format(time.RFC3339)
	offer, err := mapProtoOtcOfferFromPut(&interbankv1.PutCounterRequest{
		StockDescription: &commonv1.StockDescription{Ticker: "MSFT"},
		PricePerUnit:     &commonv1.MonetaryValue{Currency: "USD", Amount: "150.00"},
		Premium:          &commonv1.MonetaryValue{Currency: "USD", Amount: "10.00"},
		Amount:           5,
		SettlementDate:   settle,
	})
	if err != nil || offer.Stock.Ticker != "MSFT" || offer.Amount != 5 {
		t.Errorf("got %+v %v", offer, err)
	}
	if _, err := mapProtoOtcOfferFromPut(&interbankv1.PutCounterRequest{SettlementDate: "bad"}); err == nil {
		t.Error("bad date should error")
	}
	if _, err := mapProtoOtcOfferFromPut(&interbankv1.PutCounterRequest{SettlementDate: settle, PricePerUnit: &commonv1.MonetaryValue{Amount: "bad"}}); err == nil {
		t.Error("bad price should error")
	}
	if _, err := mapProtoOtcOfferFromPut(&interbankv1.PutCounterRequest{SettlementDate: settle, Premium: &commonv1.MonetaryValue{Amount: "bad"}}); err == nil {
		t.Error("bad premium should error")
	}
}

// ---------------------------------------------------------------------------
// domain → proto mappers
// ---------------------------------------------------------------------------

func TestUnit_MapForeignBankIdToProto(t *testing.T) {
	got := mapForeignBankIdToProto(protocol.ForeignBankId{RoutingNumber: 111, Id: "C-1"})
	if got.GetRoutingNumber() != 111 || got.GetId() != "C-1" {
		t.Errorf("got %+v", got)
	}
}

func TestUnit_MapMonetaryValueToProto(t *testing.T) {
	got := mapMonetaryValueToProto(protocol.MonetaryValue{Currency: "USD", Amount: decimal.RequireFromString("12.34")})
	if got.GetCurrency() != "USD" || got.GetAmount() != "12.34" {
		t.Errorf("got %+v", got)
	}
}

func TestUnit_MapVoteToProto(t *testing.T) {
	yes := mapVoteToProto(protocol.TransactionVote{Vote: protocol.VoteYes})
	if yes.GetVote() != interbankv1.TransactionVote_VOTE_YES {
		t.Errorf("yes: %v", yes.GetVote())
	}
	no := mapVoteToProto(protocol.TransactionVote{Vote: protocol.VoteNo, Reasons: []protocol.NoVoteReason{{Reason: protocol.ReasonUnbalancedTx}}})
	if no.GetVote() != interbankv1.TransactionVote_VOTE_NO || len(no.GetReasons()) != 1 {
		t.Errorf("no: %+v", no)
	}
	unspec := mapVoteToProto(protocol.TransactionVote{Vote: "WEIRD"})
	if unspec.GetVote() != interbankv1.TransactionVote_VOTE_UNSPECIFIED {
		t.Errorf("unspec: %v", unspec.GetVote())
	}
}

func TestUnit_MapNoVoteReasonToProto(t *testing.T) {
	posting := protocol.Posting{Account: &protocol.RealAccount{Num: "1"}, Amount: decimal.RequireFromString("1"), Asset: &protocol.MonasAsset{Currency: "USD"}}
	r := mapNoVoteReasonToProto(protocol.NoVoteReason{Reason: protocol.ReasonNoSuchAccount, Posting: &posting})
	if r.GetReason() != interbankv1.NoVoteReason_REASON_NO_SUCH_ACCOUNT || r.GetPosting() == nil {
		t.Errorf("with posting: %+v", r)
	}
	r2 := mapNoVoteReasonToProto(protocol.NoVoteReason{Reason: protocol.ReasonUnbalancedTx})
	if r2.GetPosting() != nil {
		t.Errorf("expected nil posting")
	}
}

func TestUnit_MapReasonStringToProto(t *testing.T) {
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
			t.Errorf("%q → %v want %v", in, got, want)
		}
	}
}

func TestUnit_MapTxAccountToProto(t *testing.T) {
	if mapTxAccountToProto(&protocol.PersonAccount{Id: protocol.ForeignBankId{Id: "C-1"}}).GetPerson() == nil {
		t.Error("expected person body")
	}
	if mapTxAccountToProto(&protocol.RealAccount{Num: "111"}).GetAccountNum() != "111" {
		t.Error("expected account num")
	}
	if mapTxAccountToProto(&protocol.OptionPseudoAccount{Id: protocol.ForeignBankId{Id: "neg-1"}}).GetOption() == nil {
		t.Error("expected option body")
	}
	if mapTxAccountToProto(nil) == nil {
		t.Error("expected non-nil empty struct")
	}
}

func TestUnit_MapAssetToProto(t *testing.T) {
	if mapAssetToProto(&protocol.MonasAsset{Currency: "USD"}).GetMonas().GetCurrency() != "USD" {
		t.Error("monas")
	}
	if mapAssetToProto(&protocol.StockAsset{Ticker: "AAPL"}).GetStock().GetTicker() != "AAPL" {
		t.Error("stock")
	}
	opt := mapAssetToProto(&protocol.OptionAsset{OptionDescription: protocol.OptionDescription{
		NegotiationId:  protocol.ForeignBankId{Id: "neg-1"},
		Stock:          protocol.StockDescription{Ticker: "AAPL"},
		PricePerUnit:   protocol.MonetaryValue{Currency: "USD", Amount: decimal.RequireFromString("200.00")},
		SettlementDate: "2026-12-01T00:00:00Z",
		Amount:         10,
	}})
	if opt.GetOption().GetStock().GetTicker() != "AAPL" || opt.GetOption().GetAmount() != 10 {
		t.Errorf("option: %+v", opt)
	}
	if mapAssetToProto(nil) == nil {
		t.Error("expected non-nil empty struct")
	}
}

func TestUnit_MapPostingToProto(t *testing.T) {
	p := mapPostingToProto(protocol.Posting{
		Account: &protocol.RealAccount{Num: "111"},
		Amount:  decimal.RequireFromString("-100.00"),
		Asset:   &protocol.MonasAsset{Currency: "USD"},
	})
	if p.GetAccount().GetAccountNum() != "111" {
		t.Errorf("account: %+v", p.GetAccount())
	}
}

func TestUnit_MapNegotiationDtoToProto(t *testing.T) {
	settle := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	out := mapNegotiationDtoToProto(protocol.ForeignBankId{RoutingNumber: 111, Id: "neg-1"}, service.OtcNegotiationDto{
		Stock:          protocol.StockDescription{Ticker: "AAPL"},
		SettlementDate: settle,
		PricePerUnit:   protocol.MonetaryValue{Currency: "USD", Amount: decimal.RequireFromString("150.00")},
		Premium:        protocol.MonetaryValue{Currency: "USD", Amount: decimal.RequireFromString("10.00")},
		Amount:         10,
		IsOngoing:      true,
	})
	if out.GetId().GetId() != "neg-1" || out.GetStockDescription().GetTicker() != "AAPL" || !out.GetIsOngoing() {
		t.Errorf("out: %+v", out)
	}
	if out.GetSettlementDate() != "2026-12-01T00:00:00Z" {
		t.Errorf("settlementDate: %q", out.GetSettlementDate())
	}
}

// ---------------------------------------------------------------------------
// helpers in handlers.go
// ---------------------------------------------------------------------------

func TestUnit_MapServiceError(t *testing.T) {
	cases := []struct {
		err  error
		want codes.Code
	}{
		{service.ErrNegotiationNotFound, codes.NotFound},
		{service.ErrNegotiationInvalid, codes.InvalidArgument},
		{service.ErrNegotiationClosed, codes.FailedPrecondition},
		{service.ErrTurnViolation, codes.Aborted},
		{service.ErrSenderNotParty, codes.PermissionDenied},
		{service.ErrInterbankProtocol, codes.Internal},
		{errInjected, codes.Internal},
	}
	for _, c := range cases {
		got := mapServiceError(c.err, "msg")
		if status.Code(got) != c.want {
			t.Errorf("%v → %v want %v", c.err, status.Code(got), c.want)
		}
	}
}

var errInjected = errInjectedErr("boom")

type errInjectedErr string

func (e errInjectedErr) Error() string { return string(e) }

func TestUnit_MarshalVoteToJSON(t *testing.T) {
	if got := marshalVoteToJSON(protocol.TransactionVote{Vote: protocol.VoteYes}); got != `{"vote":"YES"}` {
		t.Errorf("yes → %q", got)
	}
	if got := marshalVoteToJSON(protocol.TransactionVote{Vote: protocol.VoteNo}); got != `{"vote":"NO"}` {
		t.Errorf("no → %q", got)
	}
	if got := marshalVoteToJSON(protocol.TransactionVote{}); got != `{"vote":"NO"}` {
		t.Errorf("empty → %q", got)
	}
}

func TestUnit_UnmarshalVoteFromJSON(t *testing.T) {
	var v protocol.TransactionVote
	if err := unmarshalVoteFromJSON(`{"vote":"YES"}`, &v); err != nil || v.Vote != protocol.VoteYes {
		t.Errorf("yes: %q %v", v.Vote, err)
	}
	v = protocol.TransactionVote{}
	if err := unmarshalVoteFromJSON(`{"vote":"NO"}`, &v); err != nil || v.Vote != protocol.VoteNo {
		t.Errorf("no: %q %v", v.Vote, err)
	}
	v = protocol.TransactionVote{}
	if err := unmarshalVoteFromJSON(`{"vote":"???"}`, &v); err == nil {
		t.Error("unrecognised should error")
	}
}

func TestUnit_IntPtr(t *testing.T) {
	p := intPtr(204)
	if p == nil || *p != 204 {
		t.Errorf("intPtr(204) → %v", p)
	}
}

// keep store import used for potential future expansion
var _ = store.DirectionInbound
