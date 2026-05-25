package protocol

import (
	"encoding/json"
	"fmt"
)

// TxAccountType is the discriminator for the TxAccount oneof.
// Java: @JsonTypeInfo property="type" with @JsonSubTypes PERSON/ACCOUNT/OPTION
type TxAccountType string

const (
	TxAccountTypePerson  TxAccountType = "PERSON"
	TxAccountTypeAccount TxAccountType = "ACCOUNT"
	TxAccountTypeOption  TxAccountType = "OPTION"
)

// TxAccount is one of *PersonAccount / *RealAccount / *OptionPseudoAccount.
//
// Wire shapes matching the Java sealed interface:
//
//	PERSON  → {"type":"PERSON","id":{"routingNumber":111,"id":"C-7"}}
//	ACCOUNT → {"type":"ACCOUNT","num":"111000001234567890"}
//	OPTION  → {"type":"OPTION","id":{"routingNumber":111,"id":"neg-1"}}
type TxAccount interface {
	Type() TxAccountType
	isTxAccount()
}

// PersonAccount references a foreign-bank-resolved entity (a person or company).
// The receiver bank resolves this opaque id to their actual MONAS account.
// Java: record Person(ForeignBankId id)
type PersonAccount struct {
	Id ForeignBankId `json:"id" validate:"required"`
}

func (PersonAccount) Type() TxAccountType { return TxAccountTypePerson }
func (PersonAccount) isTxAccount()        {}

// RealAccount is an 18-digit bank account number.
// Java: record Account(String num)  @Pattern(regexp="\\d{18}")
type RealAccount struct {
	Num string `json:"num" validate:"required,len=18,numeric"`
}

func (RealAccount) Type() TxAccountType { return TxAccountTypeAccount }
func (RealAccount) isTxAccount()        {}

// OptionPseudoAccount references the option contract's pseudo-account (spec §2.7.2).
// id is the negotiation id at the bank that issued the option.
// Java: record Option(ForeignBankId id)
type OptionPseudoAccount struct {
	Id ForeignBankId `json:"id" validate:"required"`
}

func (OptionPseudoAccount) Type() TxAccountType { return TxAccountTypeOption }
func (OptionPseudoAccount) isTxAccount()        {}

// UnmarshalTxAccount dispatches on the type discriminator to the appropriate
// concrete TxAccount implementation.
func UnmarshalTxAccount(raw json.RawMessage) (TxAccount, error) {
	// First parse just the type discriminator and the two possible structural fields.
	var env struct {
		Type TxAccountType   `json:"type"`
		Id   json.RawMessage `json:"id,omitempty"`
		Num  string          `json:"num,omitempty"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("txaccount envelope: %w", err)
	}
	switch env.Type {
	case TxAccountTypePerson:
		var id ForeignBankId
		if err := json.Unmarshal(env.Id, &id); err != nil {
			return nil, fmt.Errorf("person id: %w", err)
		}
		return &PersonAccount{Id: id}, nil
	case TxAccountTypeAccount:
		return &RealAccount{Num: env.Num}, nil
	case TxAccountTypeOption:
		var id ForeignBankId
		if err := json.Unmarshal(env.Id, &id); err != nil {
			return nil, fmt.Errorf("option id: %w", err)
		}
		return &OptionPseudoAccount{Id: id}, nil
	default:
		return nil, fmt.Errorf("unknown txaccount type %q", env.Type)
	}
}

// MarshalJSON for each variant emits the canonical wire shape.

func (p *PersonAccount) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type TxAccountType `json:"type"`
		Id   ForeignBankId `json:"id"`
	}{Type: TxAccountTypePerson, Id: p.Id})
}

func (a *RealAccount) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type TxAccountType `json:"type"`
		Num  string        `json:"num"`
	}{Type: TxAccountTypeAccount, Num: a.Num})
}

func (o *OptionPseudoAccount) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type TxAccountType `json:"type"`
		Id   ForeignBankId `json:"id"`
	}{Type: TxAccountTypeOption, Id: o.Id})
}
