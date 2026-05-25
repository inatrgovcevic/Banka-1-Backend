package protocol

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
)

// canonical NEW_TX wire payload per spec §6
const newTxJSON = `{
  "idempotenceKey": {"routingNumber": 222, "locallyGeneratedKey": "wire-snap-01"},
  "messageType": "NEW_TX",
  "message": {
    "transactionId": {"routingNumber": 222, "id": "test-tx-01"},
    "postings": [
      {"account": {"type": "ACCOUNT", "num": "222000000000000001"},
       "amount": "-100.00",
       "asset": {"type": "MONAS", "asset": {"currency": "USD"}}},
      {"account": {"type": "ACCOUNT", "num": "111000001234567890"},
       "amount": "100.00",
       "asset": {"type": "MONAS", "asset": {"currency": "USD"}}}
    ],
    "message": "wire snapshot 01"
  }
}`

func TestEnvelope_NewTx_Unmarshal(t *testing.T) {
	var msg InterbankMessagePayload
	if err := json.Unmarshal([]byte(newTxJSON), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.MessageType != MessageTypeNewTx {
		t.Errorf("messageType=%v want NEW_TX", msg.MessageType)
	}
	if msg.IdempotenceKey.RoutingNumber != 222 {
		t.Errorf("idem.routing=%d", msg.IdempotenceKey.RoutingNumber)
	}
	if msg.IdempotenceKey.LocallyGeneratedKey != "wire-snap-01" {
		t.Errorf("idem.key=%q", msg.IdempotenceKey.LocallyGeneratedKey)
	}
	tx, ok := msg.Message.(*InterbankTransactionPayload)
	if !ok {
		t.Fatalf("message not InterbankTransactionPayload: %T", msg.Message)
	}
	if tx.TransactionId.RoutingNumber != 222 || tx.TransactionId.Id != "test-tx-01" {
		t.Errorf("tx id=%+v", tx.TransactionId)
	}
	if len(tx.Postings) != 2 {
		t.Errorf("postings=%d want 2", len(tx.Postings))
	}
	// posting[0]
	acc0, ok := tx.Postings[0].Account.(*RealAccount)
	if !ok {
		t.Errorf("posting[0].account not RealAccount: %T", tx.Postings[0].Account)
	} else if acc0.Num != "222000000000000001" {
		t.Errorf("posting[0].account.num=%q", acc0.Num)
	}
	if tx.Postings[0].Amount.String() != "-100" {
		t.Errorf("posting[0].amount=%s want -100", tx.Postings[0].Amount.String())
	}
	monas, ok := tx.Postings[0].Asset.(*MonasAsset)
	if !ok {
		t.Errorf("posting[0].asset not MonasAsset: %T", tx.Postings[0].Asset)
	} else if monas.Currency != "USD" {
		t.Errorf("currency=%q", monas.Currency)
	}
}

func TestEnvelope_NewTx_RoundTrip(t *testing.T) {
	var orig InterbankMessagePayload
	if err := json.Unmarshal([]byte(newTxJSON), &orig); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var rt InterbankMessagePayload
	if err := json.Unmarshal(out, &rt); err != nil {
		t.Fatalf("rt unmarshal: %v", err)
	}
	if rt.IdempotenceKey.LocallyGeneratedKey != orig.IdempotenceKey.LocallyGeneratedKey {
		t.Errorf("round-trip key mismatch")
	}
	if rt.MessageType != orig.MessageType {
		t.Errorf("round-trip messageType mismatch")
	}
	rtTx := rt.Message.(*InterbankTransactionPayload)
	origTx := orig.Message.(*InterbankTransactionPayload)
	if rtTx.TransactionId != origTx.TransactionId {
		t.Errorf("round-trip txId mismatch")
	}
	if len(rtTx.Postings) != len(origTx.Postings) {
		t.Errorf("round-trip postings count mismatch")
	}
}

func TestPosting_Amount_BigDecimal_PlainSerialization(t *testing.T) {
	raw := []byte(`{"account":{"type":"ACCOUNT","num":"111000000000000001"},"amount":"0.00000001","asset":{"type":"MONAS","asset":{"currency":"USD"}}}`)
	var p Posting
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !p.Amount.Equal(decimal.RequireFromString("0.00000001")) {
		t.Errorf("amount=%s want 0.00000001", p.Amount.String())
	}
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(out, []byte(`"amount":"0.00000001"`)) {
		t.Errorf("marshal lost precision or used scientific notation: %s", out)
	}
}

func TestAsset_PolymorphicMarshalUnmarshal(t *testing.T) {
	// MONAS
	monasJSON := []byte(`{"type":"MONAS","asset":{"currency":"EUR"}}`)
	a1, err := UnmarshalAsset(monasJSON)
	if err != nil {
		t.Fatalf("monas unmarshal: %v", err)
	}
	if a1.Type() != AssetTypeMonas {
		t.Errorf("got type=%v", a1.Type())
	}
	out1, _ := json.Marshal(a1)
	if !bytes.Contains(out1, []byte(`"type":"MONAS"`)) || !bytes.Contains(out1, []byte(`"asset":`)) {
		t.Errorf("monas remarshal: %s", out1)
	}
	// STOCK
	stockJSON := []byte(`{"type":"STOCK","asset":{"ticker":"AAPL"}}`)
	a2, err := UnmarshalAsset(stockJSON)
	if err != nil {
		t.Fatalf("stock unmarshal: %v", err)
	}
	if a2.Type() != AssetTypeStock {
		t.Errorf("got type=%v", a2.Type())
	}
	// OPTION (full nested)
	optJSON := []byte(`{"type":"OPTION","asset":{"negotiationId":{"routingNumber":111,"id":"neg-1"},"stock":{"ticker":"AAPL"},"pricePerUnit":{"currency":"USD","amount":"200.00"},"settlementDate":"2026-12-01T00:00:00Z","amount":10}}`)
	a3, err := UnmarshalAsset(optJSON)
	if err != nil {
		t.Fatalf("option unmarshal: %v", err)
	}
	if a3.Type() != AssetTypeOption {
		t.Errorf("got type=%v", a3.Type())
	}
	o := a3.(*OptionAsset)
	if o.NegotiationId.Id != "neg-1" {
		t.Errorf("option negotiationId=%+v", o.NegotiationId)
	}
}

func TestTxAccount_PolymorphicMarshalUnmarshal(t *testing.T) {
	// PERSON
	personJSON := []byte(`{"type":"PERSON","id":{"routingNumber":111,"id":"C-7"}}`)
	acc1, err := UnmarshalTxAccount(personJSON)
	if err != nil {
		t.Fatalf("person unmarshal: %v", err)
	}
	p := acc1.(*PersonAccount)
	if p.Id.Id != "C-7" {
		t.Errorf("person id=%+v", p.Id)
	}
	// ACCOUNT (18-digit num)
	accountJSON := []byte(`{"type":"ACCOUNT","num":"111000001234567890"}`)
	acc2, err := UnmarshalTxAccount(accountJSON)
	if err != nil {
		t.Fatalf("account unmarshal: %v", err)
	}
	a := acc2.(*RealAccount)
	if a.Num != "111000001234567890" {
		t.Errorf("account num=%q", a.Num)
	}
	// OPTION pseudo
	optJSON := []byte(`{"type":"OPTION","id":{"routingNumber":111,"id":"neg-1"}}`)
	acc3, err := UnmarshalTxAccount(optJSON)
	if err != nil {
		t.Fatalf("option unmarshal: %v", err)
	}
	o := acc3.(*OptionPseudoAccount)
	if o.Id.Id != "neg-1" {
		t.Errorf("option id=%+v", o.Id)
	}
}

func TestCommitRollback_Unmarshal(t *testing.T) {
	commitJSON := `{
      "idempotenceKey": {"routingNumber": 222, "locallyGeneratedKey": "commit-01"},
      "messageType": "COMMIT_TX",
      "message": {"transactionId": {"routingNumber": 222, "id": "tx-01"}}
    }`
	var msg InterbankMessagePayload
	if err := json.Unmarshal([]byte(commitJSON), &msg); err != nil {
		t.Fatalf("commit unmarshal: %v", err)
	}
	if msg.MessageType != MessageTypeCommitTx {
		t.Errorf("messageType=%v", msg.MessageType)
	}
	cb, ok := msg.Message.(*CommitTransactionBody)
	if !ok {
		t.Fatalf("not CommitTransactionBody: %T", msg.Message)
	}
	if cb.TransactionId.Id != "tx-01" {
		t.Errorf("commit txId=%+v", cb.TransactionId)
	}

	rollbackJSON := `{
      "idempotenceKey": {"routingNumber": 222, "locallyGeneratedKey": "rb-01"},
      "messageType": "ROLLBACK_TX",
      "message": {"transactionId": {"routingNumber": 222, "id": "tx-01"}}
    }`
	var rb InterbankMessagePayload
	if err := json.Unmarshal([]byte(rollbackJSON), &rb); err != nil {
		t.Fatalf("rollback unmarshal: %v", err)
	}
	if rb.MessageType != MessageTypeRollbackTx {
		t.Errorf("messageType=%v", rb.MessageType)
	}
}

func TestTransactionVote_YesNoSerialization(t *testing.T) {
	yes := TransactionVote{Vote: VoteYes}
	out, _ := json.Marshal(yes)
	if !bytes.Contains(out, []byte(`"vote":"YES"`)) {
		t.Errorf("yes vote: %s", out)
	}
	// reasons omitempty when empty
	if bytes.Contains(out, []byte(`"reasons"`)) {
		t.Errorf("expected reasons omitted for YES vote: %s", out)
	}

	no := TransactionVote{
		Vote: VoteNo,
		Reasons: []NoVoteReason{
			{Reason: "INSUFFICIENT_ASSET"},
		},
	}
	outNo, _ := json.Marshal(no)
	if !bytes.Contains(outNo, []byte(`"vote":"NO"`)) || !bytes.Contains(outNo, []byte(`"reason":"INSUFFICIENT_ASSET"`)) {
		t.Errorf("no vote: %s", outNo)
	}
	// posting omitted when nil
	if bytes.Contains(outNo, []byte(`"posting":`)) {
		t.Errorf("expected posting omitted when nil: %s", outNo)
	}
}
