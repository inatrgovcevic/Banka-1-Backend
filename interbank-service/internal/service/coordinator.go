package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/shopspring/decimal"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"
)

// ---------------------------------------------------------------------------
// OutboundClient interface (Task 18 will provide the concrete implementation)
// ---------------------------------------------------------------------------

// OutboundClient is consumed by Coordinator to perform 2PC messaging toward
// the partner bank. The concrete implementation (InterbankClient, Task 18)
// signs requests with the partner's outbound token and POSTs to their
// /interbank endpoint. Tests use a fake.
type OutboundClient interface {
	// SendNewTx sends a NEW_TX message to the partner bank and returns their vote.
	SendNewTx(ctx context.Context, partnerRouting int, tx protocol.InterbankTransactionPayload) (protocol.TransactionVote, error)
	// SendCommitTx sends a COMMIT_TX message. Returns nil on 204.
	SendCommitTx(ctx context.Context, partnerRouting int, txID protocol.ForeignBankId) error
	// SendRollbackTx sends a ROLLBACK_TX message. Returns nil on 204.
	SendRollbackTx(ctx context.Context, partnerRouting int, txID protocol.ForeignBankId) error
}

// ---------------------------------------------------------------------------
// ContractStoreIface — persistence seam for Coordinator
// ---------------------------------------------------------------------------

// ContractStoreIface is the subset of store.ContractStore used by Coordinator.
type ContractStoreIface interface {
	Insert(ctx context.Context, c *store.Contract) error
}

// ---------------------------------------------------------------------------
// BankingCoreAccountResolver — resolve local user → 18-digit account number
// ---------------------------------------------------------------------------

// BankingCoreAccountResolver is a subset of BankingCoreReserver used by Coordinator
// to look up real account numbers for local parties.
type BankingCoreAccountResolver interface {
	FindAccountByOwnerAndCurrency(ctx context.Context, ownerID int64, currency string) (string, error)
}

// ---------------------------------------------------------------------------
// Coordinator
// ---------------------------------------------------------------------------

// Coordinator orchestrates the synchronous 2PC for §3.6 GET /negotiations/{rn}/{id}/accept.
// Corresponds to Java InterbankCoordinatorService.
//
// Flow per spec §3.6 + §7:
//  1. Build 4-posting transaction (buyer premium debit, seller premium credit,
//     seller Option pseudo-account credit, buyer Person Option debit).
//  2. PrepareLocal (local side reservation).
//  3. OutboundClient.SendNewTx (partner side prepare).
//  4. On partner NO or error: rollback local + rollback partner; return error.
//  5. CommitLocal (finalize local reservations).
//  6. MarkClosed negotiation + insert contract ACTIVE.
//  7. OutboundClient.SendCommitTx (best-effort; retry scheduler covers failures).
type Coordinator struct {
	myRouting   int
	executor    *Executor
	outbound    OutboundClient
	negStore    NegotiationStoreIface
	contractSt  ContractStoreIface
	bcResolver  BankingCoreAccountResolver
	log         *slog.Logger
}

// NewCoordinator constructs a Coordinator. bcResolver may be nil — in that case
// MONAS account resolution falls back to TxAccount.Person (partner resolves).
func NewCoordinator(
	myRouting int,
	executor *Executor,
	outbound OutboundClient,
	negStore NegotiationStoreIface,
	contractSt ContractStoreIface,
	bcResolver BankingCoreAccountResolver,
	log *slog.Logger,
) *Coordinator {
	if log == nil {
		log = slog.Default()
	}
	return &Coordinator{
		myRouting:  myRouting,
		executor:   executor,
		outbound:   outbound,
		negStore:   negStore,
		contractSt: contractSt,
		bcResolver: bcResolver,
		log:        log,
	}
}

// AcceptNegotiation implements CoordinatorIface.AcceptNegotiation.
// The negotiation entity must have is_ongoing=true and settlement in the future;
// all turn checks have already been performed in OtcNegotiationService.
func (c *Coordinator) AcceptNegotiation(ctx context.Context, neg *store.Negotiation) error {
	partnerRouting := neg.BuyerRouting
	if partnerRouting == c.myRouting {
		partnerRouting = neg.SellerRouting
	}

	// Total premium = premium_amount (per-unit) × amount.
	totalPremium := neg.PremiumAmount.Mul(decimal.NewFromInt(int64(neg.Amount)))
	premCurrency := neg.PremiumCurrency
	strikeCurrency := neg.PriceCurrency

	optionDesc := protocol.OptionDescription{
		NegotiationId: protocol.ForeignBankId{RoutingNumber: c.myRouting, Id: neg.ID},
		Stock:         protocol.StockDescription{Ticker: neg.StockTicker},
		PricePerUnit:  protocol.MonetaryValue{Currency: strikeCurrency, Amount: neg.PriceAmount},
		SettlementDate: neg.SettlementDate.UTC().Format(time.RFC3339),
		Amount:        neg.Amount,
	}

	// Resolve MONAS accounts: our local parties get real 18-digit numbers;
	// partner-side parties become TxAccount.Person (partner resolves internally).
	buyerCashAcc := c.resolveMonasAccount(ctx, neg.BuyerRouting, neg.BuyerID, premCurrency)
	sellerCashAcc := c.resolveMonasAccount(ctx, neg.SellerRouting, neg.SellerID, premCurrency)

	postings := []protocol.Posting{
		// a) Buyer credit premium (-totalPremium): premium leaves buyer account.
		{
			Account: buyerCashAcc,
			Amount:  totalPremium.Neg(),
			Asset:   &protocol.MonasAsset{Currency: premCurrency},
		},
		// b) Seller debit premium (+totalPremium): seller receives premium.
		{
			Account: sellerCashAcc,
			Amount:  totalPremium,
			Asset:   &protocol.MonasAsset{Currency: premCurrency},
		},
		// c) Seller Option pseudo-account credit (-1 option unit).
		{
			Account: &protocol.OptionPseudoAccount{Id: protocol.ForeignBankId{RoutingNumber: c.myRouting, Id: neg.ID}},
			Amount:  decimal.NewFromFloat(-1),
			Asset:   &protocol.OptionAsset{OptionDescription: optionDesc},
		},
		// d) Buyer Person account debit (+1 option unit): buyer receives option.
		{
			Account: &protocol.PersonAccount{Id: protocol.ForeignBankId{RoutingNumber: neg.BuyerRouting, Id: neg.BuyerID}},
			Amount:  decimal.NewFromFloat(1),
			Asset:   &protocol.OptionAsset{OptionDescription: optionDesc},
		},
	}

	txID := protocol.ForeignBankId{RoutingNumber: c.myRouting, Id: generateTxID()}
	tx := protocol.InterbankTransactionPayload{
		TransactionId:  txID,
		Postings:       postings,
		Message:        "OTC accept for negotiation " + neg.ID,
		PaymentCode:    "289",
		PaymentPurpose: "OTC premium + option transfer",
	}

	// 1. PrepareLocal.
	localVote, err := c.executor.PrepareLocal(ctx, tx)
	if err != nil {
		return fmt.Errorf("%w: local prepare infra failure: %v", ErrInterbankProtocol, err)
	}
	if localVote.Vote != protocol.VoteYes {
		return fmt.Errorf("%w: local prepare rejected: %v", ErrInterbankProtocol, localVote.Reasons)
	}

	// 2. Partner prepare.
	partnerVote, err := c.outbound.SendNewTx(ctx, partnerRouting, tx)
	if err != nil {
		c.safeRollbackLocal(ctx, txID)
		return fmt.Errorf("%w: partner prepare exception: %v", ErrInterbankProtocol, err)
	}
	if partnerVote.Vote != protocol.VoteYes {
		c.safeRollbackLocal(ctx, txID)
		c.safeRollbackPartner(ctx, partnerRouting, txID)
		return fmt.Errorf("%w: partner rejected: %v", ErrInterbankProtocol, partnerVote.Reasons)
	}

	// 3. CommitLocal.
	if err := c.executor.CommitLocal(ctx, txID); err != nil {
		// Catastrophic: we committed locally but might fail to tell partner.
		c.safeRollbackPartner(ctx, partnerRouting, txID)
		return fmt.Errorf("%w: catastrophic commitLocal failure: %v", ErrInterbankProtocol, err)
	}

	// 4. Flip negotiation + create contract.
	if err := c.negStore.MarkClosed(ctx, neg.ID); err != nil {
		c.log.WarnContext(ctx, "coordinator: failed to mark negotiation closed",
			"neg", neg.ID, "err", err)
		// Non-fatal; 2PC is committed. Continue.
	}
	contract := &store.Contract{
		ID:                       generateContractID(),
		NegotiationID:            neg.ID,
		BuyerRouting:             neg.BuyerRouting,
		BuyerID:                  neg.BuyerID,
		SellerRouting:            neg.SellerRouting,
		SellerID:                 neg.SellerID,
		StockTicker:              neg.StockTicker,
		Amount:                   neg.Amount,
		StrikeCurrency:           neg.PriceCurrency,
		StrikeAmount:             neg.PriceAmount,
		SettlementDate:           neg.SettlementDate,
		Status:                   store.ContractStatusActive,
		OptionPseudoOwnerRouting: neg.BuyerRouting,
		OptionPseudoOwnerID:      neg.BuyerID,
	}
	if err := c.contractSt.Insert(ctx, contract); err != nil {
		c.log.ErrorContext(ctx, "coordinator: failed to insert contract",
			"neg", neg.ID, "err", err)
		// Non-fatal for the 2PC protocol; contract will be missing but 2PC committed.
	}

	// 5. Partner commit — CRITICAL for 204 response; best-effort (retry scheduler covers failures).
	if err := c.outbound.SendCommitTx(ctx, partnerRouting, txID); err != nil {
		c.log.WarnContext(ctx, "coordinator: sendCommitTx to partner failed — retry scheduler will pick it up",
			"partnerRouting", partnerRouting, "txID", txID, "err", err)
	}

	c.log.InfoContext(ctx, "accepted negotiation — contract ACTIVE",
		"neg", neg.ID, "contract", contract.ID, "tx", txID)
	return nil
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

func (c *Coordinator) resolveMonasAccount(ctx context.Context, userRouting int, userID string, currency string) protocol.TxAccount {
	if userRouting != c.myRouting || c.bcResolver == nil {
		return &protocol.PersonAccount{Id: protocol.ForeignBankId{RoutingNumber: userRouting, Id: userID}}
	}
	// Strip C-/E- prefix to get numeric owner id.
	numericPart := userID
	if len(userID) > 2 && (userID[:2] == "C-" || userID[:2] == "E-") {
		numericPart = userID[2:]
	}
	ownerID, err := parseBigInt(numericPart)
	if err != nil {
		c.log.WarnContext(ctx, "coordinator: parse owner id failed; fallback to Person",
			"userID", userID, "err", err)
		return &protocol.PersonAccount{Id: protocol.ForeignBankId{RoutingNumber: userRouting, Id: userID}}
	}
	accountNum, err := c.bcResolver.FindAccountByOwnerAndCurrency(ctx, ownerID, currency)
	if err != nil || accountNum == "" {
		c.log.WarnContext(ctx, "coordinator: resolve MONAS account failed; fallback to Person",
			"userID", userID, "currency", currency, "err", err)
		return &protocol.PersonAccount{Id: protocol.ForeignBankId{RoutingNumber: userRouting, Id: userID}}
	}
	return &protocol.RealAccount{Num: accountNum}
}

func (c *Coordinator) safeRollbackLocal(ctx context.Context, txID protocol.ForeignBankId) {
	if err := c.executor.RollbackLocal(ctx, txID); err != nil {
		c.log.WarnContext(ctx, "coordinator: safeRollbackLocal failed", "txID", txID, "err", err)
	}
}

func (c *Coordinator) safeRollbackPartner(ctx context.Context, partnerRouting int, txID protocol.ForeignBankId) {
	if err := c.outbound.SendRollbackTx(ctx, partnerRouting, txID); err != nil {
		c.log.WarnContext(ctx, "coordinator: safeRollbackPartner failed",
			"partnerRouting", partnerRouting, "txID", txID, "err", err)
	}
}

func generateTxID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "tx-" + hex.EncodeToString(b)
}

func generateContractID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "otc-" + hex.EncodeToString(b)
}

// parseBigInt parses a numeric string into int64. Used for owner ID extraction.
func parseBigInt(s string) (int64, error) {
	n := new(big.Int)
	if _, ok := n.SetString(s, 10); !ok {
		return 0, fmt.Errorf("not a valid integer: %q", s)
	}
	if !n.IsInt64() {
		return 0, fmt.Errorf("integer too large for int64: %q", s)
	}
	return n.Int64(), nil
}

