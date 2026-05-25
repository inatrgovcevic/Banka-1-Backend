package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"
)

// ErrAlreadyTerminal indicates a commit/rollback was attempted on a tx whose
// status is already terminal (COMMITTED, ROLLED_BACK, or FAILED).
var ErrAlreadyTerminal = errors.New("service: transaction already terminal")

// ---------------------------------------------------------------------------
// Interfaces
// ---------------------------------------------------------------------------

// ExecutorStore is the persistence seam used by Executor. Production wiring
// adapts *store.TransactionStore; tests use an in-memory fake.
type ExecutorStore interface {
	// PersistPrepared inserts a new PREPARED transaction row.
	// Populates t.ID and t.CreatedAt on success.
	PersistPrepared(ctx context.Context, t *store.Transaction) error

	// FindTx returns (nil, nil) when the transaction is not found.
	FindTx(ctx context.Context, routing int, id string) (*store.Transaction, error)

	// UpdateTxStatus flips the status column for the given (routing, id) pair.
	UpdateTxStatus(ctx context.Context, routing int, id, status string) error
}

// BankingCoreReserver extends BankingCoreReader with reservation actions toward
// banking-core's internal interbank endpoints.
type BankingCoreReserver interface {
	BankingCoreReader // ResolveAccount, FindAccountByOwnerAndCurrency
	// ReserveMonas places an amount reservation on the given account.
	// Returns the reservation UUID on success.
	ReserveMonas(ctx context.Context, accountNum, currency string, amount decimal.Decimal, txIDRouting int, txIDLocal string) (string, error)
	// CommitMonas permanently debits a previously reserved amount.
	CommitMonas(ctx context.Context, reservationID string) error
	// ReleaseMonas frees a previously reserved amount back to available balance.
	ReleaseMonas(ctx context.Context, reservationID string) error
}

// TradingReserver provides stock and option reservation actions toward
// trading-service's internal interbank endpoints.
type TradingReserver interface {
	// ReserveStock creates a stock reservation for a pending inter-bank transaction.
	// Returns the reservation UUID on success.
	ReserveStock(ctx context.Context, sellerUserID int64, ticker string, quantity int, txIDRouting int, txIDLocal string) (string, error)
	// CommitStock permanently transfers the reserved stock.
	CommitStock(ctx context.Context, reservationID string) error
	// ReleaseStock frees the stock reservation.
	ReleaseStock(ctx context.Context, reservationID string) error
	// ReserveOption marks an option contract as reserved (idempotent per spec §3.6).
	ReserveOption(ctx context.Context, negotiationID protocol.ForeignBankId, sellerForeignID, ticker string, quantity int) error
	// ExerciseOption marks an option contract as exercised (spec §2.7.2).
	ExerciseOption(ctx context.Context, negotiationID protocol.ForeignBankId) error
	// ReleaseOption frees an option reservation.
	ReleaseOption(ctx context.Context, negotiationID protocol.ForeignBankId) error
}

// OptionNegotiationLookup resolves an option negotiation to its seller foreign-id.
// Used during reservePosting for the OPTION branch. In production this is
// implemented by a thin adapter over *store.NegotiationStore.
type OptionNegotiationLookup interface {
	// FindSellerID returns the seller's foreign-id string for the given negotiation id.
	// Returns a non-nil error (and empty string) when the negotiation is not found.
	FindSellerID(ctx context.Context, negID string) (string, error)
}

// ---------------------------------------------------------------------------
// Executor struct
// ---------------------------------------------------------------------------

// Executor orchestrates the 2PC prepare/commit/rollback lifecycle for
// incoming inter-bank transactions per spec §7.
//
// Critical invariant: Spring @Transactional (Java) maps to our per-method
// approach: all outbound REST reservation calls are NOT inside the same DB
// transaction as PersistPrepared. We use saga-style LIFO compensation instead.
type Executor struct {
	myRouting int
	store     ExecutorStore
	bc        BankingCoreReserver
	td        TradingReserver
	negLookup OptionNegotiationLookup
	validator *Validator
	log       *slog.Logger
}

// NewExecutor constructs an Executor. negLookup is used only during
// reservePosting for the OPTION branch (looking up the seller's foreign-id).
// A nil negLookup means no OPTION reservations will succeed; pass a real
// adapter in production.
func NewExecutor(
	myRouting int,
	s ExecutorStore,
	bc BankingCoreReserver,
	td TradingReserver,
	negLookup OptionNegotiationLookup,
	log *slog.Logger,
) *Executor {
	if log == nil {
		log = slog.Default()
	}
	// The Validator uses negLookup (which implements NegotiationReader) for
	// option validation checks. bc provides BankingCoreReader.
	var negReader NegotiationReader
	if negLookup != nil {
		if nr, ok := negLookup.(NegotiationReader); ok {
			negReader = nr
		}
	}
	return &Executor{
		myRouting: myRouting,
		store:     s,
		bc:        bc,
		td:        td,
		negLookup: negLookup,
		validator: NewValidator(myRouting, negReader, bc, td),
		log:       log,
	}
}

// ---------------------------------------------------------------------------
// PrepareLocal
// ---------------------------------------------------------------------------

// PrepareLocal validates and reserves resources for an incoming NEW_TX message.
// Returns a YES vote and persists a PREPARED row on success.
// Returns a NO vote (never an error) when any validation rule is violated.
// Returns an error (not a vote) on unexpected infrastructure failures; in that
// case compensation has already been attempted for any partial reservations.
func (e *Executor) PrepareLocal(ctx context.Context, tx protocol.InterbankTransactionPayload) (protocol.TransactionVote, error) {
	// 1. Balance check — pure local computation.
	if reasons := e.validator.BalanceCheck(tx.Postings); len(reasons) > 0 {
		return protocol.TransactionVote{Vote: protocol.VoteNo, Reasons: reasons}, nil
	}

	// 2. Filter postings that belong to our side.
	var ours []protocol.Posting
	for _, p := range tx.Postings {
		if e.isOurs(p) {
			ours = append(ours, p)
		}
	}

	// 3. Validate each of our postings (no side effects; NO vote on any failure).
	var reasons []protocol.NoVoteReason
	for _, p := range ours {
		r, err := e.validator.ValidatePosting(ctx, p)
		if err != nil {
			return protocol.TransactionVote{}, fmt.Errorf("executor: validate posting: %w", err)
		}
		if r != nil {
			reasons = append(reasons, *r)
		}
	}
	if len(reasons) > 0 {
		return protocol.TransactionVote{Vote: protocol.VoteNo, Reasons: reasons}, nil
	}

	// 4. Reserve — saga-style, LIFO compensation on failure.
	var refs []store.ReservationRef
	for _, p := range ours {
		if !p.Amount.IsNegative() {
			// Only outgoing (debit) postings need reservation.
			continue
		}
		ref, err := e.reservePosting(ctx, p, tx.TransactionId)
		if err != nil {
			e.compensate(ctx, refs)
			return protocol.TransactionVote{}, fmt.Errorf("executor: reserve posting: %w", err)
		}
		refs = append(refs, ref)
	}

	// 5. Build message meta JSON.
	metaJSON, _ := json.Marshal(map[string]any{
		"message":        tx.Message,
		"callNumber":     tx.CallNumber,
		"paymentCode":    tx.PaymentCode,
		"paymentPurpose": tx.PaymentPurpose,
	})

	// 6. Marshal postings for storage.
	postingsJSON, _ := json.Marshal(tx.Postings)

	// 7. Persist PREPARED row.
	row := &store.Transaction{
		TransactionIdRouting: tx.TransactionId.RoutingNumber,
		TransactionIdLocal:   tx.TransactionId.Id,
		Status:               store.TxStatusPrepared,
		PostingsJSON:         string(postingsJSON),
		ReservationRefs:      refs,
		MessageMeta:          string(metaJSON),
	}
	if err := e.store.PersistPrepared(ctx, row); err != nil {
		e.compensate(ctx, refs)
		return protocol.TransactionVote{}, fmt.Errorf("executor: persist prepared: %w", err)
	}

	return protocol.TransactionVote{Vote: protocol.VoteYes}, nil
}

// ---------------------------------------------------------------------------
// CommitLocal
// ---------------------------------------------------------------------------

// CommitLocal finalizes a PREPARED transaction. Idempotent.
// Returns nil when:
//   - tx is already COMMITTED (no-op)
//   - tx is not found (no-op; partner is protocol master)
//
// Returns ErrAlreadyTerminal when tx is in ROLLED_BACK or FAILED state.
func (e *Executor) CommitLocal(ctx context.Context, txID protocol.ForeignBankId) error {
	tx, err := e.store.FindTx(ctx, txID.RoutingNumber, txID.Id)
	if err != nil {
		return fmt.Errorf("executor: find tx for commit: %w", err)
	}
	if tx == nil {
		e.log.WarnContext(ctx, "commitLocal: tx not found — partner is master, no-op",
			"txID", txID)
		return nil
	}
	if tx.Status == store.TxStatusCommitted {
		e.log.DebugContext(ctx, "commitLocal: already committed — idempotent no-op",
			"txID", txID)
		return nil
	}
	if tx.Status != store.TxStatusPrepared {
		return fmt.Errorf("%w: status=%s txID=%v", ErrAlreadyTerminal, tx.Status, txID)
	}

	for _, ref := range tx.ReservationRefs {
		if err := e.commitRef(ctx, ref); err != nil {
			// Best-effort status flip to FAILED; log and propagate.
			if updateErr := e.store.UpdateTxStatus(ctx, txID.RoutingNumber, txID.Id, store.TxStatusFailed); updateErr != nil {
				e.log.ErrorContext(ctx, "commitLocal: failed to flip status to FAILED",
					"txID", txID, "err", updateErr)
			}
			return fmt.Errorf("executor: commit ref %+v: %w", ref, err)
		}
	}

	return e.store.UpdateTxStatus(ctx, txID.RoutingNumber, txID.Id, store.TxStatusCommitted)
}

// ---------------------------------------------------------------------------
// RollbackLocal
// ---------------------------------------------------------------------------

// RollbackLocal releases all reservations for a PREPARED transaction. Idempotent.
// Best-effort: logs warnings but does not return errors for individual release
// failures (partner is protocol master).
func (e *Executor) RollbackLocal(ctx context.Context, txID protocol.ForeignBankId) error {
	tx, err := e.store.FindTx(ctx, txID.RoutingNumber, txID.Id)
	if err != nil {
		return fmt.Errorf("executor: find tx for rollback: %w", err)
	}
	if tx == nil {
		e.log.WarnContext(ctx, "rollbackLocal: tx not found — best-effort no-op",
			"txID", txID)
		return nil
	}
	if tx.Status != store.TxStatusPrepared {
		e.log.WarnContext(ctx, "rollbackLocal: tx not in PREPARED state — no-op",
			"txID", txID, "status", tx.Status)
		return nil
	}

	// Release LIFO (reverse order).
	e.compensate(ctx, tx.ReservationRefs)

	return e.store.UpdateTxStatus(ctx, txID.RoutingNumber, txID.Id, store.TxStatusRolledBack)
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

// isOurs returns true when a posting's account side belongs to our routing number.
// "Ours" means:
//   - RealAccount: prefix of 18-digit number matches our routing (e.g. "111...")
//   - PersonAccount: id.RoutingNumber == myRouting
//   - OptionPseudoAccount: id.RoutingNumber == myRouting
func (e *Executor) isOurs(p protocol.Posting) bool {
	switch acc := p.Account.(type) {
	case *protocol.RealAccount:
		return e.validator.accountIsOurs(acc.Num)
	case *protocol.PersonAccount:
		return acc.Id.RoutingNumber == e.myRouting
	case *protocol.OptionPseudoAccount:
		return acc.Id.RoutingNumber == e.myRouting
	}
	return false
}

// reservePosting dispatches to the appropriate downstream reservation call
// based on the asset type. Returns the ReservationRef for bookkeeping.
func (e *Executor) reservePosting(ctx context.Context, p protocol.Posting, txID protocol.ForeignBankId) (store.ReservationRef, error) {
	switch asset := p.Asset.(type) {

	case *protocol.MonasAsset:
		return e.reserveMonasPosting(ctx, p, asset, txID)

	case *protocol.StockAsset:
		return e.reserveStockPosting(ctx, p, asset, txID)

	case *protocol.OptionAsset:
		return e.reserveOptionPosting(ctx, p, asset, txID)

	default:
		return store.ReservationRef{}, fmt.Errorf("executor: unknown asset type %T", p.Asset)
	}
}

// reserveMonasPosting handles MONAS (monetary asset) reservation.
// Both RealAccount and PersonAccount are supported (Person → resolve to account number first).
func (e *Executor) reserveMonasPosting(ctx context.Context, p protocol.Posting, asset *protocol.MonasAsset, txID protocol.ForeignBankId) (store.ReservationRef, error) {
	if e.bc == nil {
		return store.ReservationRef{}, errors.New("executor: BankingCoreReserver is nil")
	}

	var accountNum string
	switch acc := p.Account.(type) {
	case *protocol.RealAccount:
		accountNum = acc.Num

	case *protocol.PersonAccount:
		// Person+MONAS: spec §2.6 allows opaque person id; resolve to real account.
		if acc.Id.RoutingNumber != e.myRouting {
			// Partner-side person — skip reservation (partner reserves their own side).
			return store.ReservationRef{}, nil
		}
		ownerID, err := parseOwnerID(acc.Id.Id)
		if err != nil {
			return store.ReservationRef{}, fmt.Errorf("executor: parse owner id %q: %w", acc.Id.Id, err)
		}
		num, err := e.bc.FindAccountByOwnerAndCurrency(ctx, ownerID, asset.Currency)
		if err != nil || num == "" {
			return store.ReservationRef{}, fmt.Errorf("executor: resolve person %d to %s account: %w", ownerID, asset.Currency, err)
		}
		accountNum = num

	default:
		return store.ReservationRef{}, fmt.Errorf("executor: MONAS posting with unexpected account type %T", p.Account)
	}

	resID, err := e.bc.ReserveMonas(ctx, accountNum, asset.Currency, p.Amount.Abs(), txID.RoutingNumber, txID.Id)
	if err != nil {
		return store.ReservationRef{}, fmt.Errorf("executor: ReserveMonas account=%s: %w", accountNum, err)
	}
	return store.ReservationRef{Kind: store.RefKindMonas, ReservationID: resID}, nil
}

// reserveStockPosting handles STOCK reservation (only PersonAccount sellers are valid here).
func (e *Executor) reserveStockPosting(ctx context.Context, p protocol.Posting, asset *protocol.StockAsset, txID protocol.ForeignBankId) (store.ReservationRef, error) {
	if e.td == nil {
		return store.ReservationRef{}, errors.New("executor: TradingReserver is nil")
	}

	person, ok := p.Account.(*protocol.PersonAccount)
	if !ok {
		return store.ReservationRef{}, fmt.Errorf("executor: STOCK posting requires PersonAccount, got %T", p.Account)
	}
	if person.Id.RoutingNumber != e.myRouting {
		// Partner-side stock seller — partner reserves their side, skip.
		return store.ReservationRef{}, nil
	}

	sellerUserID, err := parseOwnerID(person.Id.Id)
	if err != nil {
		return store.ReservationRef{}, fmt.Errorf("executor: parse stock seller id %q: %w", person.Id.Id, err)
	}

	resID, err := e.td.ReserveStock(ctx, sellerUserID, asset.Ticker, int(p.Amount.Abs().IntPart()), txID.RoutingNumber, txID.Id)
	if err != nil {
		return store.ReservationRef{}, fmt.Errorf("executor: ReserveStock seller=%d ticker=%s: %w", sellerUserID, asset.Ticker, err)
	}
	return store.ReservationRef{Kind: store.RefKindStock, ReservationID: resID}, nil
}

// reserveOptionPosting handles OPTION reservation (OptionPseudoAccount only).
// The negotiation id IS the reservation key — there is no separate reservation UUID.
// Per spec §3.6: the option pseudo-account.id is the negotiationId; the seller is
// looked up from our negotiation store (pseudo.id() is NOT the seller).
func (e *Executor) reserveOptionPosting(ctx context.Context, p protocol.Posting, asset *protocol.OptionAsset, txID protocol.ForeignBankId) (store.ReservationRef, error) {
	if e.td == nil {
		return store.ReservationRef{}, errors.New("executor: TradingReserver is nil")
	}

	pseudo, ok := p.Account.(*protocol.OptionPseudoAccount)
	if !ok {
		return store.ReservationRef{}, fmt.Errorf("executor: OPTION posting requires OptionPseudoAccount, got %T", p.Account)
	}
	if pseudo.Id.RoutingNumber != e.myRouting {
		// Partner-side option pseudo-account — skip.
		return store.ReservationRef{}, nil
	}

	negID := asset.NegotiationId.Id

	// Resolve seller from our negotiation mirror.
	// pseudo.Id.Id holds the negotiationId (NOT the seller); per Java comment in reservePosting.
	sellerForeignID := ""
	if e.negLookup != nil {
		sid, err := e.negLookup.FindSellerID(ctx, negID)
		if err != nil {
			return store.ReservationRef{}, fmt.Errorf("executor: resolve option seller for neg %q: %w", negID, err)
		}
		sellerForeignID = sid
	}

	err := e.td.ReserveOption(ctx, asset.NegotiationId, sellerForeignID, asset.Stock.Ticker, asset.Amount)
	if err != nil {
		return store.ReservationRef{}, fmt.Errorf("executor: ReserveOption neg=%s: %w", negID, err)
	}

	negRouting := pseudo.Id.RoutingNumber
	return store.ReservationRef{
		Kind:               store.RefKindOption,
		NegotiationRouting: &negRouting,
		NegotiationID:      &negID,
	}, nil
}

// commitRef finalizes a single reservation ref.
func (e *Executor) commitRef(ctx context.Context, ref store.ReservationRef) error {
	switch ref.Kind {
	case store.RefKindMonas:
		if e.bc == nil {
			return errors.New("executor: BankingCoreReserver is nil for MONAS commit")
		}
		return e.bc.CommitMonas(ctx, ref.ReservationID)

	case store.RefKindStock:
		if e.td == nil {
			return errors.New("executor: TradingReserver is nil for STOCK commit")
		}
		return e.td.CommitStock(ctx, ref.ReservationID)

	case store.RefKindOption:
		if e.td == nil {
			return errors.New("executor: TradingReserver is nil for OPTION exercise")
		}
		if ref.NegotiationRouting == nil || ref.NegotiationID == nil {
			return errors.New("executor: OPTION reservation ref missing negotiation fields")
		}
		return e.td.ExerciseOption(ctx, protocol.ForeignBankId{
			RoutingNumber: *ref.NegotiationRouting,
			Id:            *ref.NegotiationID,
		})

	default:
		e.log.Warn("executor: commitRef: unknown ref kind — skipping", "kind", ref.Kind)
		return nil
	}
}

// releaseRef frees a single reservation ref. Best-effort — logs failures but
// does not return them (caller continues with remaining refs).
func (e *Executor) releaseRef(ctx context.Context, ref store.ReservationRef) {
	var err error
	switch ref.Kind {
	case store.RefKindMonas:
		if e.bc != nil {
			err = e.bc.ReleaseMonas(ctx, ref.ReservationID)
		}
	case store.RefKindStock:
		if e.td != nil {
			err = e.td.ReleaseStock(ctx, ref.ReservationID)
		}
	case store.RefKindOption:
		if e.td != nil && ref.NegotiationRouting != nil && ref.NegotiationID != nil {
			err = e.td.ReleaseOption(ctx, protocol.ForeignBankId{
				RoutingNumber: *ref.NegotiationRouting,
				Id:            *ref.NegotiationID,
			})
		}
	default:
		e.log.Warn("executor: releaseRef: unknown ref kind — skipping", "kind", ref.Kind)
		return
	}
	if err != nil {
		e.log.Warn("executor: releaseRef: stuck reservation",
			"kind", ref.Kind,
			"reservationID", ref.ReservationID,
			"err", err)
	}
}

// compensate releases all refs in LIFO order. Best-effort: individual release
// errors are logged but do not interrupt the sweep.
func (e *Executor) compensate(ctx context.Context, refs []store.ReservationRef) {
	for i := len(refs) - 1; i >= 0; i-- {
		e.releaseRef(ctx, refs[i])
	}
}
