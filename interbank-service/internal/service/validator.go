// Package service contains the core business logic for interbank-service:
// validation, 2PC execution, OTC negotiation lifecycle, outbound coordination.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
)

// ErrAccountNotFound is the sentinel that BankingCoreReader implementations
// must return when a lookup misses. Validator translates this to NO_SUCH_ACCOUNT.
var ErrAccountNotFound = errors.New("service: account not found")

// AccountInfo is the minimal view of an account that the validator needs.
type AccountInfo struct {
	OwnerType        string
	OwnerID          int64
	Currency         string
	AvailableBalance decimal.Decimal
}

// NegotiationLite is the minimal view of a negotiation row that the validator needs.
type NegotiationLite struct {
	IsOngoing      bool
	SettlementDate time.Time
	Amount         int
	PricePerUnit   decimal.Decimal
}

// BankingCoreReader is what the validator needs from banking-core internal endpoints.
type BankingCoreReader interface {
	// ResolveAccount looks up an account by 18-digit number.
	// Returns ErrAccountNotFound if the account does not exist.
	ResolveAccount(ctx context.Context, num string) (*AccountInfo, error)
	// FindAccountByOwnerAndCurrency resolves (ownerID, currency) to an account number.
	// Returns ErrAccountNotFound if no matching account exists.
	FindAccountByOwnerAndCurrency(ctx context.Context, ownerID int64, currency string) (string, error)
}

// TradingReader is what the validator needs from trading-service internal endpoints.
// Currently unused by the validator (option lookups use our own store).
// Kept for future expansion (e.g. stock-existence checks for NO_SUCH_ASSET on STOCK postings).
type TradingReader interface{}

// NegotiationReader is what the validator needs to validate Option postings.
// In production this is implemented by a store adapter; in tests by a fake.
type NegotiationReader interface {
	// FindNegotiation returns the negotiation for id.
	// Returns (nil, non-nil-error) if not found; (nil, nil) is invalid and treated as not-found.
	FindNegotiation(ctx context.Context, id protocol.ForeignBankId) (*NegotiationLite, error)
}

// Validator runs each NEW_TX posting through the 8 NoVoteReason branches per spec §2.12.1.
type Validator struct {
	myRouting int
	negs      NegotiationReader
	bc        BankingCoreReader
	td        TradingReader
}

// NewValidator constructs a Validator for our routing-number identity.
func NewValidator(myRouting int, negs NegotiationReader, bc BankingCoreReader, td TradingReader) *Validator {
	return &Validator{myRouting: myRouting, negs: negs, bc: bc, td: td}
}

// BalanceCheck verifies that postings sum to zero per asset-key.
// Returns up to one UNBALANCED_TX reason if any asset key is unbalanced; never errors.
func (v *Validator) BalanceCheck(postings []protocol.Posting) []protocol.NoVoteReason {
	sums := make(map[string]decimal.Decimal)
	for _, p := range postings {
		k := assetKey(p.Asset)
		sums[k] = sums[k].Add(p.Amount)
	}
	for _, s := range sums {
		if !s.IsZero() {
			return []protocol.NoVoteReason{{Reason: protocol.ReasonUnbalancedTx}}
		}
	}
	return nil
}

// assetKey derives the balance-check grouping key from an asset.
func assetKey(a protocol.Asset) string {
	switch v := a.(type) {
	case *protocol.MonasAsset:
		return "MONAS:" + v.Currency
	case *protocol.StockAsset:
		return "STOCK:" + v.Ticker
	case *protocol.OptionAsset:
		return fmt.Sprintf("OPTION:%d:%s", v.NegotiationId.RoutingNumber, v.NegotiationId.Id)
	default:
		return "UNKNOWN"
	}
}

// ValidatePosting runs the single-posting validation branches.
// Only validates "ours" postings (account references our routing number).
// Returns (nil, nil) when the posting is partner-side OR is valid.
// Returns (*NoVoteReason, nil) for a business-rule violation.
// Returns (nil, error) for unexpected infrastructure errors.
func (v *Validator) ValidatePosting(ctx context.Context, p protocol.Posting) (*protocol.NoVoteReason, error) {
	switch acc := p.Account.(type) {

	case *protocol.RealAccount:
		return v.validateRealAccount(ctx, acc, p)

	case *protocol.PersonAccount:
		return v.validatePersonAccount(ctx, acc, p)

	case *protocol.OptionPseudoAccount:
		return v.validateOptionPseudoAccount(ctx, acc, p)
	}

	// Unknown account type — not ours to validate.
	return nil, nil
}

// validateRealAccount handles the ACCOUNT type: ACCOUNT+MONAS is normal;
// ACCOUNT+anything-else is UNACCEPTABLE_ASSET.
func (v *Validator) validateRealAccount(ctx context.Context, acc *protocol.RealAccount, p protocol.Posting) (*protocol.NoVoteReason, error) {
	// Only validate accounts that belong to us.
	if !v.accountIsOurs(acc.Num) {
		return nil, nil
	}

	switch monas := p.Asset.(type) {
	case *protocol.MonasAsset:
		if v.bc == nil {
			return &protocol.NoVoteReason{Reason: protocol.ReasonNoSuchAccount, Posting: &p}, nil
		}
		info, err := v.bc.ResolveAccount(ctx, acc.Num)
		if err != nil {
			if errors.Is(err, ErrAccountNotFound) {
				return &protocol.NoVoteReason{Reason: protocol.ReasonNoSuchAccount, Posting: &p}, nil
			}
			return nil, err
		}
		if info.Currency != monas.Currency {
			return &protocol.NoVoteReason{Reason: protocol.ReasonNoSuchAsset, Posting: &p}, nil
		}
		if p.Amount.IsNegative() && info.AvailableBalance.LessThan(p.Amount.Abs()) {
			return &protocol.NoVoteReason{Reason: protocol.ReasonInsufficientAsset, Posting: &p}, nil
		}
		return nil, nil

	default:
		// ACCOUNT + STOCK, ACCOUNT + OPTION, etc.
		return &protocol.NoVoteReason{Reason: protocol.ReasonUnacceptableAsset, Posting: &p}, nil
	}
}

// validatePersonAccount handles the PERSON type. Only validate persons whose
// routing matches ours. PERSON+MONAS → resolve to real account; PERSON+OPTION →
// valid by default (spec §3.6 accept option-leg); PERSON+STOCK → UNACCEPTABLE.
func (v *Validator) validatePersonAccount(ctx context.Context, person *protocol.PersonAccount, p protocol.Posting) (*protocol.NoVoteReason, error) {
	if person.Id.RoutingNumber != v.myRouting {
		// Partner-side person — not our responsibility.
		return nil, nil
	}

	switch monas := p.Asset.(type) {
	case *protocol.MonasAsset:
		ownerID, err := parseOwnerID(person.Id.Id)
		if err != nil {
			return &protocol.NoVoteReason{Reason: protocol.ReasonNoSuchAccount, Posting: &p}, nil
		}
		if v.bc == nil {
			return &protocol.NoVoteReason{Reason: protocol.ReasonNoSuchAccount, Posting: &p}, nil
		}
		num, err := v.bc.FindAccountByOwnerAndCurrency(ctx, ownerID, monas.Currency)
		if err != nil || num == "" {
			return &protocol.NoVoteReason{Reason: protocol.ReasonNoSuchAccount, Posting: &p}, nil
		}
		info, err := v.bc.ResolveAccount(ctx, num)
		if err != nil {
			if errors.Is(err, ErrAccountNotFound) {
				return &protocol.NoVoteReason{Reason: protocol.ReasonNoSuchAccount, Posting: &p}, nil
			}
			return nil, err
		}
		if p.Amount.IsNegative() && info.AvailableBalance.LessThan(p.Amount.Abs()) {
			return &protocol.NoVoteReason{Reason: protocol.ReasonInsufficientAsset, Posting: &p}, nil
		}
		return nil, nil

	case *protocol.OptionAsset:
		// PERSON + OPTION is the accept option-leg per spec §3.6 — valid.
		_ = monas
		return nil, nil

	default:
		// PERSON + STOCK or other — UNACCEPTABLE_ASSET.
		return &protocol.NoVoteReason{Reason: protocol.ReasonUnacceptableAsset, Posting: &p}, nil
	}
}

// validateOptionPseudoAccount handles the OPTION account type.
// OPTION + non-OPTION asset → UNACCEPTABLE_ASSET.
// OPTION + OPTION asset, routing=ours → run the option-specific checks.
// OPTION + OPTION asset, routing=partner → not ours.
func (v *Validator) validateOptionPseudoAccount(ctx context.Context, opt *protocol.OptionPseudoAccount, p protocol.Posting) (*protocol.NoVoteReason, error) {
	if _, ok := p.Asset.(*protocol.OptionAsset); !ok {
		return &protocol.NoVoteReason{Reason: protocol.ReasonUnacceptableAsset, Posting: &p}, nil
	}
	if opt.Id.RoutingNumber != v.myRouting {
		return nil, nil
	}
	return v.validateOption(ctx, opt.Id, p)
}

// validateOption performs the three option-specific checks:
// OPTION_NEGOTIATION_NOT_FOUND, OPTION_USED_OR_EXPIRED, OPTION_AMOUNT_INCORRECT.
func (v *Validator) validateOption(ctx context.Context, id protocol.ForeignBankId, p protocol.Posting) (*protocol.NoVoteReason, error) {
	if v.negs == nil {
		return &protocol.NoVoteReason{Reason: protocol.ReasonOptionNegotiationNotFound, Posting: &p}, nil
	}
	neg, err := v.negs.FindNegotiation(ctx, id)
	if err != nil || neg == nil {
		return &protocol.NoVoteReason{Reason: protocol.ReasonOptionNegotiationNotFound, Posting: &p}, nil
	}
	if !neg.IsOngoing {
		return &protocol.NoVoteReason{Reason: protocol.ReasonOptionUsedOrExpired, Posting: &p}, nil
	}
	if neg.SettlementDate.Before(time.Now()) {
		return &protocol.NoVoteReason{Reason: protocol.ReasonOptionUsedOrExpired, Posting: &p}, nil
	}
	// Amount check: |amount| ∈ {1, k, k·π}
	abs := p.Amount.Abs()
	k := decimal.NewFromInt(int64(neg.Amount))
	kPi := k.Mul(neg.PricePerUnit)
	one := decimal.NewFromInt(1)
	if !abs.Equal(one) && !abs.Equal(k) && !abs.Equal(kPi) {
		return &protocol.NoVoteReason{Reason: protocol.ReasonOptionAmountIncorrect, Posting: &p}, nil
	}
	return nil, nil
}

// accountIsOurs returns true if the 18-digit account number's routing prefix (first 3 digits)
// matches our routing number.
func (v *Validator) accountIsOurs(num string) bool {
	if len(num) != 18 {
		return false
	}
	prefix := num[:3]
	return prefix == fmt.Sprintf("%03d", v.myRouting)
}

// parseOwnerID extracts the numeric portion of a prefixed id like "C-7" or "E-42".
func parseOwnerID(prefixedID string) (int64, error) {
	if len(prefixedID) < 3 {
		return 0, fmt.Errorf("short id %q", prefixedID)
	}
	if prefixedID[0] != 'C' && prefixedID[0] != 'E' {
		return 0, fmt.Errorf("unexpected prefix in %q", prefixedID)
	}
	parts := strings.SplitN(prefixedID, "-", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("missing dash in %q", prefixedID)
	}
	var x int64
	_, err := fmt.Sscanf(parts[1], "%d", &x)
	return x, err
}
