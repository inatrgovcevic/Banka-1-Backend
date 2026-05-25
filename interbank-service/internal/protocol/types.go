// Package protocol defines wire-format DTOs for the inter-bank protocol.
// Byte-equivalent to the Java side (interbank-service.protocol.dto). The wire
// reference is dokumenta/Inter-Bank-Protokol-Implementacija.md §6.
//
// Polymorphic types Asset and TxAccount use a nested {"type":"X","asset":{...}}
// JSON shape matching the Java @JsonTypeInfo(use=Id.NAME,property="type") with
// a record field named "asset". Custom UnmarshalJSON / MarshalJSON for those
// types lives in asset.go and tx_account.go.
//
// BigDecimal-valued fields use shopspring/decimal whose default JSON form is a
// quoted string (compatible with Java WRITE_BIGDECIMAL_AS_PLAIN + string
// representation). UnmarshalJSON accepts both quoted strings and bare numbers.
package protocol

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

// ForeignBankId identifies a remote-bank-owned entity. The id is opaque to us.
// Prefix conventions on our side: C-{n} client, E-{n} employee, N-{n} neg, T-{n} tx, O-{n} option.
type ForeignBankId struct {
	RoutingNumber int    `json:"routingNumber" validate:"required"`
	Id            string `json:"id"            validate:"required,max=64"`
}

// IdempotenceKey appears on every InterbankMessagePayload.
// LocallyGeneratedKey: caller-generated, max 64 UTF-8 bytes per spec §2.2.
type IdempotenceKey struct {
	RoutingNumber       int    `json:"routingNumber"       validate:"required"`
	LocallyGeneratedKey string `json:"locallyGeneratedKey" validate:"required,max=64"`
}

// MonetaryValue carries currency + amount (BigDecimal as plain string on the wire).
// Java: record MonetaryValue(CurrencyCode currency, BigDecimal amount)
type MonetaryValue struct {
	Currency string          `json:"currency" validate:"required,oneof=RSD EUR USD CHF JPY AUD CAD GBP"`
	Amount   decimal.Decimal `json:"amount"   validate:"required"`
}

// StockDescription identifies a tradeable stock by ticker.
// Java: record StockDescription(String ticker)
type StockDescription struct {
	Ticker string `json:"ticker" validate:"required,max=16"`
}

// OptionDescription describes an option contract referenced by negotiationId.
// Java: record OptionDescription(ForeignBankId negotiationId, StockDescription stock,
//   MonetaryValue pricePerUnit, OffsetDateTime settlementDate, int amount)
// settlementDate serializes as RFC3339 UTC string.
type OptionDescription struct {
	NegotiationId  ForeignBankId    `json:"negotiationId"  validate:"required"`
	Stock          StockDescription `json:"stock"          validate:"required"`
	PricePerUnit   MonetaryValue    `json:"pricePerUnit"   validate:"required"`
	SettlementDate string           `json:"settlementDate" validate:"required"` // RFC3339 UTC string
	Amount         int              `json:"amount"         validate:"required,min=1"`
}

// Posting is one entry in an inter-bank transaction. Amount is signed.
// Postings must sum to zero per asset key for the transaction to be balanced.
// Java: record Posting(TxAccount account, BigDecimal amount, Asset asset)
type Posting struct {
	Account TxAccount       `json:"account" validate:"required"`
	Amount  decimal.Decimal `json:"amount"  validate:"required"`
	Asset   Asset           `json:"asset"   validate:"required"`
}

// UnmarshalJSON dispatches the polymorphic Account and Asset fields.
func (p *Posting) UnmarshalJSON(data []byte) error {
	var raw struct {
		Account json.RawMessage `json:"account"`
		Amount  decimal.Decimal `json:"amount"`
		Asset   json.RawMessage `json:"asset"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Amount = raw.Amount
	acc, err := UnmarshalTxAccount(raw.Account)
	if err != nil {
		return err
	}
	p.Account = acc
	ast, err := UnmarshalAsset(raw.Asset)
	if err != nil {
		return err
	}
	p.Asset = ast
	return nil
}

// MarshalJSON for Posting emits account, amount, and asset with their custom shapes.
func (p Posting) MarshalJSON() ([]byte, error) {
	// We need to emit the polymorphic account and asset with their own MarshalJSON.
	// Use a raw message trick to avoid recursion.
	accBytes, err := json.Marshal(p.Account)
	if err != nil {
		return nil, err
	}
	astBytes, err := json.Marshal(p.Asset)
	if err != nil {
		return nil, err
	}
	type postingWire struct {
		Account json.RawMessage `json:"account"`
		Amount  decimal.Decimal `json:"amount"`
		Asset   json.RawMessage `json:"asset"`
	}
	return json.Marshal(postingWire{
		Account: accBytes,
		Amount:  p.Amount,
		Asset:   astBytes,
	})
}

// InterbankTransactionPayload is the body of a NEW_TX message.
// Java: record InterbankTransactionPayload(List<Posting> postings,
//   ForeignBankId transactionId, String message, String callNumber,
//   String paymentCode, String paymentPurpose)
type InterbankTransactionPayload struct {
	TransactionId  ForeignBankId `json:"transactionId"            validate:"required"`
	Postings       []Posting     `json:"postings"                 validate:"required,min=1,dive"`
	Message        string        `json:"message,omitempty"`
	CallNumber     string        `json:"callNumber,omitempty"`
	PaymentCode    string        `json:"paymentCode,omitempty"`
	PaymentPurpose string        `json:"paymentPurpose,omitempty"`
}

// CommitTransactionBody is the body of a COMMIT_TX message.
// Java: record CommitTransactionBody(ForeignBankId transactionId)
type CommitTransactionBody struct {
	TransactionId ForeignBankId `json:"transactionId" validate:"required"`
}

// RollbackTransactionBody is the body of a ROLLBACK_TX message.
// Java: record RollbackTransactionBody(ForeignBankId transactionId)
type RollbackTransactionBody struct {
	TransactionId ForeignBankId `json:"transactionId" validate:"required"`
}

// TransactionVote is the response to NEW_TX.
// Java: record TransactionVote(Vote vote, List<NoVoteReason> reasons) with @JsonInclude(NON_NULL)
type TransactionVote struct {
	Vote    string         `json:"vote"`
	Reasons []NoVoteReason `json:"reasons,omitempty"`
}

// NoVoteReason explains one negative vote.
// Posting is omitted (nil) for global reasons (e.g. UNBALANCED_TX).
// Java: record NoVoteReason(Reason reason, Posting posting) with @JsonInclude(NON_NULL)
type NoVoteReason struct {
	Reason  string   `json:"reason"`
	Posting *Posting `json:"posting,omitempty"`
}

// Vote constants. Java: TransactionVote.Vote enum { YES, NO }
const (
	VoteYes = "YES"
	VoteNo  = "NO"
)

// Reason constants — all 8 NoVoteReason.Reason values per spec §2.12.1.
// Java: NoVoteReason.Reason enum
const (
	ReasonUnbalancedTx              = "UNBALANCED_TX"
	ReasonNoSuchAccount             = "NO_SUCH_ACCOUNT"
	ReasonNoSuchAsset               = "NO_SUCH_ASSET"
	ReasonUnacceptableAsset         = "UNACCEPTABLE_ASSET"
	ReasonInsufficientAsset         = "INSUFFICIENT_ASSET"
	ReasonOptionAmountIncorrect     = "OPTION_AMOUNT_INCORRECT"
	ReasonOptionUsedOrExpired       = "OPTION_USED_OR_EXPIRED"
	ReasonOptionNegotiationNotFound = "OPTION_NEGOTIATION_NOT_FOUND"
)
