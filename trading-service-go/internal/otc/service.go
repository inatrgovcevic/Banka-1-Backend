package otc

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/clients"
	"banka1/trading-service-go/internal/portfolio"

	gpdb "banka1/go-platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// Service is the OTC negotiation + contract engine — the Go port of
// trading-service OtcService (offer state machine, reserved-stock invariant on
// accept, after-commit saga publishes) plus the OtcPortfolioService reserve/
// release helpers and the saga-completion transitions the listeners drive.
type Service struct {
	repo      *Repository
	portfolio *portfolio.Repository
	market    *clients.MarketClient
	customer  *clients.CustomerClient
	employee  *clients.EmployeeClient
	publisher SagaPublisher
	notifier  OtcNotifier
	logger    *slog.Logger
}

// NewService wires the OTC service. publisher publishes the premium/exercise saga
// requests on saga.events; notifier publishes employee.events OTC notifications.
func NewService(repo *Repository, portfolioRepo *portfolio.Repository, market *clients.MarketClient,
	customer *clients.CustomerClient, employee *clients.EmployeeClient,
	publisher SagaPublisher, notifier OtcNotifier, logger *slog.Logger) *Service {
	return &Service{
		repo: repo, portfolio: portfolioRepo, market: market, customer: customer,
		employee: employee, publisher: publisher, notifier: notifier, logger: logger,
	}
}

// CreateOfferInput / CounterOfferInput carry the validated request bodies.
type CreateOfferInput struct {
	StockTicker    string
	SellerID       int64
	Amount         int
	PricePerStock  decimal.Decimal
	Premium        decimal.Decimal
	SettlementDate time.Time
}

type CounterOfferInput struct {
	Amount         int
	PricePerStock  decimal.Decimal
	Premium        decimal.Decimal
	SettlementDate time.Time
}

// ============================ offer state machine =========================

// CreateOffer mirrors OtcService.createOffer: a buyer's initial offer (status
// PENDING_SELLER, modifiedBy = the token name claim). No saga, no notification.
func (s *Service) CreateOffer(ctx context.Context, buyerID int64, in CreateOfferInput, buyerName *string) (*OtcOfferDto, error) {
	offer := &OtcOffer{
		StockTicker:    in.StockTicker,
		BuyerID:        buyerID,
		SellerID:       in.SellerID,
		Amount:         in.Amount,
		PricePerStock:  in.PricePerStock,
		Premium:        in.Premium,
		SettlementDate: in.SettlementDate,
		Status:         OfferPendingSeller,
		ModifiedBy:     buyerName,
	}
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := s.repo.InsertOffer(ctx, tx, offer); err != nil {
			return err
		}
		return s.recordHistory(ctx, tx, nil, offer, EventCreated, buyerID, buyerName)
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info("created OTC offer", "offerId", offer.ID, "ticker", offer.StockTicker,
		"amount", offer.Amount, "buyer", buyerID)
	return toDto(offer), nil
}

// CounterOffer mirrors OtcService.counterOffer: either party replaces the terms
// and flips the turn. ACCEPTED/REJECTED/EXPIRED are final (WITHDRAWN is NOT
// blocked — faithful to Java). The offer row is FOR UPDATE locked.
func (s *Service) CounterOffer(ctx context.Context, offerID, actorID int64, in CounterOfferInput, actorName *string) (*OtcOfferDto, error) {
	var offer *OtcOffer
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		o, err := s.requireOfferForUpdate(ctx, tx, offerID)
		if err != nil {
			return err
		}
		before := *o
		isBuyer := o.BuyerID == actorID
		isSeller := o.SellerID == actorID
		if !isBuyer && !isSeller {
			return api.NewOtcError(http.StatusConflict,
				"Korisnik "+itoa(actorID)+" nije ucesnik OTC ponude "+itoa(offerID))
		}
		if o.Status == OfferAccepted || o.Status == OfferRejected || o.Status == OfferExpired {
			return api.NewOtcError(http.StatusConflict, "Ponuda je vec u finalnom statusu: "+o.Status)
		}
		o.Amount = in.Amount
		o.PricePerStock = in.PricePerStock
		o.Premium = in.Premium
		o.SettlementDate = in.SettlementDate
		o.ModifiedBy = actorName
		if isBuyer {
			o.Status = OfferPendingSeller
		} else {
			o.Status = OfferPendingBuyer
		}
		if err := s.repo.UpdateOffer(ctx, tx, o); err != nil {
			return err
		}
		if err := s.recordHistory(ctx, tx, &before, o, EventCounterOffered, actorID, actorName); err != nil {
			return err
		}
		offer = o
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.notifier.CounterOffered(ctx, offer, actorID)
	return toDto(offer), nil
}

// Accept mirrors OtcService.accept: the party currently on the clock accepts. The
// reserved-stock invariant (sum of the seller's live ACTIVE+PENDING_PREMIUM
// contracts for the ticker + this offer ≤ seller-owned) is enforced under the
// offer FOR UPDATE lock; a violation is InsufficientPublicStock → 400. On success
// an OptionContract (PENDING_PREMIUM) is created, the seller's stock reserved, and
// the premium-transfer saga published after commit.
func (s *Service) Accept(ctx context.Context, offerID, actorID int64, actorName *string) (*OtcOfferDto, error) {
	var offer *OtcOffer
	var contractID int64
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		o, err := s.requireOfferForUpdate(ctx, tx, offerID)
		if err != nil {
			return err
		}
		before := *o
		isSeller := o.SellerID == actorID
		isBuyer := o.BuyerID == actorID
		if !isSeller && !isBuyer {
			return api.NewOtcError(http.StatusConflict,
				"Korisnik "+itoa(actorID)+" nije ucesnik OTC ponude "+itoa(offerID)+".")
		}
		if o.Status == OfferPendingSeller && !isSeller {
			return api.NewOtcError(http.StatusConflict, "Na potezu je prodavac — samo prodavac moze prihvatiti ponudu.")
		}
		if o.Status == OfferPendingBuyer && !isBuyer {
			return api.NewOtcError(http.StatusConflict, "Na potezu je kupac — samo kupac moze prihvatiti kontraponudu.")
		}
		if o.Status != OfferPendingSeller && o.Status != OfferPendingBuyer {
			return api.NewOtcError(http.StatusConflict, "Ponuda nije aktivna (trenutno: "+o.Status+").")
		}

		sellerID := o.SellerID
		requested := int64(o.Amount)
		sellerOwned, err := s.resolveSellerOwnedQuantity(ctx, tx, sellerID, o.StockTicker)
		if err != nil {
			return err
		}
		activeReserved, err := s.repo.SumActiveBySellerAndTicker(ctx, tx, sellerID, o.StockTicker)
		if err != nil {
			return err
		}
		if activeReserved+requested > sellerOwned {
			// InsufficientPublicStockException → 400 (the OTC handler shape).
			return api.NewOtcError(http.StatusBadRequest,
				"Reserved-stock invariant violated for seller "+itoa(sellerID)+" and ticker "+
					o.StockTicker+": active="+itoa(activeReserved)+", requested="+itoa(requested)+
					", owned="+itoa(sellerOwned)+".")
		}

		o.Status = OfferAccepted
		modifiedBy := resolveActorName(actorID, actorName)
		o.ModifiedBy = &modifiedBy
		if err := s.repo.UpdateOffer(ctx, tx, o); err != nil {
			return err
		}

		contract := &OptionContract{
			OfferID:        o.ID,
			StockTicker:    o.StockTicker,
			BuyerID:        o.BuyerID,
			SellerID:       sellerID,
			Amount:         o.Amount,
			PricePerStock:  o.PricePerStock,
			SettlementDate: o.SettlementDate,
			Status:         ContractPendingPremium,
		}
		if err := s.repo.InsertOptionContract(ctx, tx, contract); err != nil {
			return err
		}
		if err := s.reserveForContract(ctx, tx, sellerID, o.StockTicker, o.Amount); err != nil {
			return err
		}
		if err := s.recordHistory(ctx, tx, &before, o, EventAccepted, actorID, &modifiedBy); err != nil {
			return err
		}
		offer = o
		contractID = contract.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	// notification + saga publish are after-commit (best-effort) — mirrors Java
	// registerAfterCommit(publishSagaPremiumTransfer).
	s.notifier.Accepted(ctx, offer, actorID)
	if err := s.publisher.PublishPremiumTransferRequested(ctx, PremiumTransferRequestedEvent{
		ContractID: contractID,
		BuyerID:    offer.BuyerID,
		SellerID:   offer.SellerID,
		Premium:    offer.Premium,
	}); err != nil {
		s.logger.Warn("otc.premium.transfer.requested publish failed (contract stays PENDING_PREMIUM)",
			"contractId", contractID, "error", err)
	}
	return toDto(offer), nil
}

// Reject mirrors OtcService.reject: any participant may reject from any state (no
// status guard in Java).
func (s *Service) Reject(ctx context.Context, offerID, actorID int64, actorName *string) (*OtcOfferDto, error) {
	var offer *OtcOffer
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		o, err := s.requireOfferForUpdate(ctx, tx, offerID)
		if err != nil {
			return err
		}
		before := *o
		if o.BuyerID != actorID && o.SellerID != actorID {
			return api.NewOtcError(http.StatusConflict, "Korisnik "+itoa(actorID)+" nije ucesnik ponude.")
		}
		o.Status = OfferRejected
		modifiedBy := resolveActorName(actorID, actorName)
		o.ModifiedBy = &modifiedBy
		if err := s.repo.UpdateOffer(ctx, tx, o); err != nil {
			return err
		}
		if err := s.recordHistory(ctx, tx, &before, o, EventRejected, actorID, &modifiedBy); err != nil {
			return err
		}
		offer = o
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.notifier.Canceled(ctx, offer, actorID, "REJECTED")
	return toDto(offer), nil
}

// Withdraw mirrors OtcService.withdraw: the party that sent the pending offer
// retracts it (buyer while PENDING_SELLER, seller while PENDING_BUYER).
func (s *Service) Withdraw(ctx context.Context, offerID, actorID int64, actorName *string) (*OtcOfferDto, error) {
	var offer *OtcOffer
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		o, err := s.requireOfferForUpdate(ctx, tx, offerID)
		if err != nil {
			return err
		}
		before := *o
		isBuyer := o.BuyerID == actorID
		isSeller := o.SellerID == actorID
		if !isBuyer && !isSeller {
			return api.NewOtcError(http.StatusConflict, "Korisnik "+itoa(actorID)+" nije ucesnik ponude.")
		}
		if isBuyer && o.Status != OfferPendingSeller {
			return api.NewOtcError(http.StatusConflict, "Kupac moze povuci samo dok je ponuda PENDING_SELLER.")
		}
		if isSeller && o.Status != OfferPendingBuyer {
			return api.NewOtcError(http.StatusConflict, "Prodavac moze povuci samo dok je ponuda PENDING_BUYER.")
		}
		o.Status = OfferWithdrawn
		modifiedBy := resolveActorName(actorID, actorName)
		o.ModifiedBy = &modifiedBy
		if err := s.repo.UpdateOffer(ctx, tx, o); err != nil {
			return err
		}
		if err := s.recordHistory(ctx, tx, &before, o, EventWithdrawn, actorID, &modifiedBy); err != nil {
			return err
		}
		offer = o
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.notifier.Canceled(ctx, offer, actorID, "WITHDRAWN")
	return toDto(offer), nil
}

// ActiveForUser mirrors OtcService.activeForUser: PENDING_BUYER/PENDING_SELLER
// offers where the user is buyer or seller.
func (s *Service) ActiveForUser(ctx context.Context, userID int64) ([]OtcOfferDto, error) {
	offers, err := s.repo.FindActiveOffersForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]OtcOfferDto, 0, len(offers))
	for i := range offers {
		out = append(out, *toDto(&offers[i]))
	}
	return out, nil
}

// ========================== option contracts ==============================

// allContractStatuses is the enum values() order — drives the no-filter
// myContracts union exactly like Java OptionContractStatus.values().
var allContractStatuses = []string{
	ContractPendingPremium, ContractActive, ContractExercised, ContractExpired, ContractCanceled,
}

// MyContracts mirrors OtcService.myContracts: the user's contracts (buyer or
// seller) optionally filtered by status, deduped by id. The per-status,
// buyer-then-seller query structure matches Java's stream concat so — against the
// same Postgres with no ORDER BY — the row order is identical.
func (s *Service) MyContracts(ctx context.Context, userID int64, statusFilter *string) ([]OptionContractDto, error) {
	statuses := allContractStatuses
	if statusFilter != nil {
		statuses = []string{*statusFilter}
	}
	out := make([]OptionContractDto, 0)
	seen := make(map[int64]struct{})
	add := func(cs []OptionContract) {
		for i := range cs {
			if _, dup := seen[cs[i].ID]; dup {
				continue
			}
			seen[cs[i].ID] = struct{}{}
			out = append(out, toContractDto(&cs[i]))
		}
	}
	for _, st := range statuses {
		buyer, err := s.repo.FindContractsByBuyerIDAndStatus(ctx, userID, st)
		if err != nil {
			return nil, err
		}
		add(buyer)
		seller, err := s.repo.FindContractsBySellerIDAndStatus(ctx, userID, st)
		if err != nil {
			return nil, err
		}
		add(seller)
	}
	return out, nil
}

// ExerciseContract mirrors OtcService.exerciseContract: the buyer triggers
// exercise. The contract is stamped exercised_at and stays ACTIVE; the exercise
// saga is published after commit and the completion listener flips it to
// EXERCISED. fi carries optional fault-injection headers (nil for production).
// Returns the contractID so callers can return a correlationId to the client.
func (s *Service) ExerciseContract(ctx context.Context, contractID, buyerID int64, fi *FaultInjection) (int64, error) {
	var event ExerciseRequestedEvent
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		c, err := s.repo.FindOptionContractByIDForUpdate(ctx, tx, contractID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return api.NewOtcError(http.StatusNotFound, "Ugovor "+itoa(contractID)+" ne postoji.")
			}
			return err
		}
		if c.BuyerID != buyerID {
			return api.NewOtcError(http.StatusForbidden, "Samo kupac moze iskoristiti opciju.")
		}
		if c.Status != ContractActive {
			return api.NewOtcError(http.StatusConflict, "Ugovor nije u statusu 'važeći': "+c.Status)
		}
		if !time.Now().Before(c.SettlementDate) {
			return api.NewOtcError(http.StatusConflict, "Rok za iskorišćavanje opcije je prošao.")
		}
		if err := s.repo.SetOptionContractExercisedAt(ctx, tx, contractID, time.Now()); err != nil {
			return err
		}
		event = ExerciseRequestedEvent{
			ContractID:     c.ID,
			BuyerID:        c.BuyerID,
			SellerID:       c.SellerID,
			StockTicker:    c.StockTicker,
			Amount:         c.Amount,
			PricePerStock:  c.PricePerStock,
			FaultInjection: fi,
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if err := s.publisher.PublishExerciseRequested(ctx, event); err != nil {
		s.logger.Warn("otc.exercise.requested publish failed (contract stays ACTIVE)",
			"contractId", event.ContractID, "error", err)
	}
	return contractID, nil
}

// ===================== saga-completion transitions ========================

// CompletePremiumTransfer flips PENDING_PREMIUM → ACTIVE (idempotent). Driven by
// the otc.premium.transfer.completed listener.
func (s *Service) CompletePremiumTransfer(ctx context.Context, contractID int64) error {
	return gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		c, err := s.repo.FindOptionContractByIDForUpdate(ctx, tx, contractID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				s.logger.Warn("otc premium completed: contract not found — skip", "contractId", contractID)
				return nil
			}
			return err
		}
		if c.Status != ContractPendingPremium {
			s.logger.Info("otc premium completed: contract not PENDING_PREMIUM — no-op",
				"contractId", contractID, "status", c.Status)
			return nil
		}
		if err := s.repo.UpdateOptionContractStatus(ctx, tx, contractID, ContractActive); err != nil {
			return err
		}
		s.logger.Info("otc contract PENDING_PREMIUM -> ACTIVE", "contractId", contractID)
		return nil
	})
}

// FailPremiumTransfer flips PENDING_PREMIUM → CANCELED and releases the reserved
// stock (idempotent). Driven by the otc.premium.transfer.failed listener.
func (s *Service) FailPremiumTransfer(ctx context.Context, contractID int64, reason string) error {
	return gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		c, err := s.repo.FindOptionContractByIDForUpdate(ctx, tx, contractID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				s.logger.Warn("otc premium failed: contract not found — skip", "contractId", contractID)
				return nil
			}
			return err
		}
		if c.Status != ContractPendingPremium {
			s.logger.Warn("otc premium failed: contract not PENDING_PREMIUM — cannot cancel",
				"contractId", contractID, "status", c.Status)
			return nil
		}
		if err := s.repo.UpdateOptionContractStatus(ctx, tx, contractID, ContractCanceled); err != nil {
			return err
		}
		if err := s.releaseForContract(ctx, tx, c.SellerID, c.StockTicker, c.Amount); err != nil {
			return err
		}
		s.logger.Info("otc contract CANCELED (premium failed)", "contractId", contractID, "reason", reason)
		return nil
	})
}

// CompleteExercise flips ACTIVE → EXERCISED (idempotent). The ownership transfer +
// reserved-quantity decrement are done separately by the ReservationService when
// the saga calls /stocks/internal. Driven by the otc.exercise.completed listener.
func (s *Service) CompleteExercise(ctx context.Context, contractID int64) error {
	return gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		c, err := s.repo.FindOptionContractByIDForUpdate(ctx, tx, contractID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				s.logger.Warn("otc exercise completed: contract not found", "contractId", contractID)
				return nil
			}
			return err
		}
		if c.Status != ContractActive {
			s.logger.Info("otc exercise completed: contract not ACTIVE — no-op",
				"contractId", contractID, "status", c.Status)
			return nil
		}
		if err := s.repo.UpdateOptionContractStatus(ctx, tx, contractID, ContractExercised); err != nil {
			return err
		}
		s.logger.Info("otc contract ACTIVE -> EXERCISED", "contractId", contractID)
		return nil
	})
}

// RevertExercise resets exercised_at → NULL when the OTC_EXERCISE saga fails
// and compensates. The contract stays ACTIVE so it can be re-exercised.
// Idempotent: a non-ACTIVE contract is a no-op.
func (s *Service) RevertExercise(ctx context.Context, contractID int64) error {
	return gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		c, err := s.repo.FindOptionContractByIDForUpdate(ctx, tx, contractID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				s.logger.Warn("otc exercise revert: contract not found", "contractId", contractID)
				return nil
			}
			return err
		}
		if c.Status != ContractActive {
			s.logger.Info("otc exercise revert: contract not ACTIVE — no-op",
				"contractId", contractID, "status", c.Status)
			return nil
		}
		if _, err := tx.Exec(ctx,
			`UPDATE option_contracts SET exercised_at = NULL WHERE id = $1`, contractID); err != nil {
			return err
		}
		s.logger.Info("otc exercise reverted (ACTIVE, exercised_at cleared)", "contractId", contractID)
		return nil
	})
}

// ============================ OTC positions ===============================

// GetMyPositions mirrors OtcService.getMyPositions: the user's STOCK positions
// currently exposed for OTC (is_public).
func (s *Service) GetMyPositions(ctx context.Context, userID int64) ([]OtcPositionDto, error) {
	positions, err := s.portfolio.FindByUserID(ctx, s.portfolio.Pool(), userID)
	if err != nil {
		return nil, err
	}
	out := make([]OtcPositionDto, 0)
	for i := range positions {
		p := positions[i]
		if p.ListingType == "STOCK" && p.IsPublic {
			out = append(out, s.toPositionDto(ctx, &p))
		}
	}
	return out, nil
}

// AddPosition mirrors OtcService.addPosition: expose `publicQuantity` of a STOCK
// position for OTC discovery (is_public = true).
func (s *Service) AddPosition(ctx context.Context, userID, listingID int64, publicQuantity int) (*OtcPositionDto, error) {
	var result *portfolio.Portfolio
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		p, err := s.portfolio.FindByUserIDAndListingID(ctx, tx, userID, listingID)
		if err != nil {
			return err
		}
		if p == nil {
			return api.NewOtcError(http.StatusNotFound,
				"Portfolio pozicija za listing "+itoa(listingID)+" ne postoji.")
		}
		if p.ListingType != "STOCK" {
			return api.NewOtcError(http.StatusConflict, "Samo STOCK pozicije se mogu izloziti za OTC.")
		}
		reserved := p.ReservedQuantity
		maxAllowed := p.Quantity - reserved
		if publicQuantity > maxAllowed {
			return api.NewOtcError(http.StatusConflict, exposeTooMuchMsg(publicQuantity, p.Quantity, reserved, maxAllowed))
		}
		if err := s.portfolio.UpdatePublic(ctx, tx, p.ID, publicQuantity, true); err != nil {
			return err
		}
		p.IsPublic = true
		p.PublicQuantity = publicQuantity
		result = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	dto := s.toPositionDto(ctx, result)
	return &dto, nil
}

// UpdatePosition mirrors OtcService.updatePosition: change the exposed quantity
// (is_public unchanged). Cannot exceed quantity-reserved nor drop below reserved.
func (s *Service) UpdatePosition(ctx context.Context, userID, positionID int64, publicQuantity int) (*OtcPositionDto, error) {
	var result *portfolio.Portfolio
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		p, err := s.requireOwnedPosition(ctx, tx, userID, positionID)
		if err != nil {
			return err
		}
		reserved := p.ReservedQuantity
		maxAllowed := p.Quantity - reserved
		if publicQuantity > maxAllowed {
			return api.NewOtcError(http.StatusConflict, exposeTooMuchMsg(publicQuantity, p.Quantity, reserved, maxAllowed))
		}
		if publicQuantity < reserved {
			return api.NewOtcError(http.StatusConflict,
				"Nije moguce smanjiti izlozenu kolicinu ispod rezervisane kolicine "+strconv.Itoa(reserved)+".")
		}
		// Java sets only publicQuantity (is_public unchanged) — preserve current flag.
		if err := s.portfolio.UpdatePublic(ctx, tx, p.ID, publicQuantity, p.IsPublic); err != nil {
			return err
		}
		p.PublicQuantity = publicQuantity
		result = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	dto := s.toPositionDto(ctx, result)
	return &dto, nil
}

// RemovePosition mirrors OtcService.removePosition: pull a position off the OTC
// market (is_public = false, public_quantity = 0). Refused while shares are
// reserved.
func (s *Service) RemovePosition(ctx context.Context, userID, positionID int64) error {
	return gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		p, err := s.requireOwnedPosition(ctx, tx, userID, positionID)
		if err != nil {
			return err
		}
		if p.ReservedQuantity > 0 {
			return api.NewOtcError(http.StatusConflict,
				"Nije moguce ukloniti poziciju dok su akcije rezervisane ("+strconv.Itoa(p.ReservedQuantity)+").")
		}
		return s.portfolio.UpdatePublic(ctx, tx, p.ID, 0, false)
	})
}

// GetPublicStocks mirrors OtcService.getPublicStocks: every publicly-exposed STOCK
// position, grouped by ticker. Non-supervisors exclude their own stocks;
// supervisors see only stocks exposed by actuaries (their client ids). The
// insertion-ordered grouping matches Java's LinkedHashMap over the same unordered
// findAllPublicStocks scan.
func (s *Service) GetPublicStocks(ctx context.Context, excludeUserID int64, supervisorView bool) ([]PublicStockDto, error) {
	var actuaryIDs map[int64]struct{}
	if supervisorView {
		ids := s.employee.ActuaryClientIDs(ctx)
		actuaryIDs = make(map[int64]struct{}, len(ids))
		for _, id := range ids {
			actuaryIDs[id] = struct{}{}
		}
	}
	rows, err := s.portfolio.FindAllPublicStocks(ctx, s.portfolio.Pool())
	if err != nil {
		return nil, err
	}
	order := make([]string, 0)
	byTicker := make(map[string][]PublicStockSellerDto)
	for i := range rows {
		p := rows[i]
		if !supervisorView && excludeUserID != 0 && excludeUserID == p.UserID {
			continue
		}
		if supervisorView {
			if _, ok := actuaryIDs[p.UserID]; !ok {
				continue
			}
		}
		qty := p.PublicQuantity
		if qty <= 0 {
			continue
		}
		ticker := s.resolveTicker(ctx, p.ListingID)
		if ticker == "" {
			continue
		}
		name := s.resolveClientName(ctx, p.UserID)
		if _, exists := byTicker[ticker]; !exists {
			order = append(order, ticker)
		}
		byTicker[ticker] = append(byTicker[ticker], PublicStockSellerDto{
			SellerID: p.UserID, SellerName: name, AvailableQuantity: qty,
		})
	}
	out := make([]PublicStockDto, 0, len(order))
	for _, ticker := range order {
		out = append(out, PublicStockDto{Ticker: ticker, Sellers: byTicker[ticker]})
	}
	return out, nil
}

// =============================== helpers ==================================

func (s *Service) requireOfferForUpdate(ctx context.Context, tx pgx.Tx, offerID int64) (*OtcOffer, error) {
	o, err := s.repo.FindOfferByIDForUpdate(ctx, tx, offerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, api.NewOtcError(http.StatusNotFound, "OTC ponuda "+itoa(offerID)+" ne postoji.")
		}
		return nil, err
	}
	return o, nil
}

func (s *Service) requireOwnedPosition(ctx context.Context, tx pgx.Tx, userID, positionID int64) (*portfolio.Portfolio, error) {
	p, err := s.portfolio.FindByID(ctx, tx, positionID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, api.NewOtcError(http.StatusNotFound, "Portfolio pozicija "+itoa(positionID)+" ne postoji.")
	}
	if p.UserID != userID {
		return nil, api.NewOtcError(http.StatusConflict,
			"Pozicija "+itoa(positionID)+" ne pripada korisniku "+itoa(userID)+".")
	}
	return p, nil
}

// recordHistory mirrors OtcNegotiationHistoryService.record: one audit row with
// old (when before != nil) and new field snapshots.
func (s *Service) recordHistory(ctx context.Context, tx pgx.Tx, before, after *OtcOffer, eventType string, actorID int64, actorName *string) error {
	if after == nil {
		return nil
	}
	h := &NegotiationHistory{
		OfferID:     after.ID,
		BuyerID:     after.BuyerID,
		SellerID:    after.SellerID,
		ActorID:     &actorID,
		ActorName:   actorName,
		EventType:   eventType,
		StockTicker: after.StockTicker,
		NewAmount:   intPtr(after.Amount),
		NewPremium:  decPtrOf(after.Premium),
		NewStatus:   strPtr(after.Status),
	}
	pps := after.PricePerStock
	h.NewPricePerStock = &pps
	nsd := after.SettlementDate
	h.NewSettlementDate = &nsd
	if before != nil {
		h.OldAmount = intPtr(before.Amount)
		h.OldPricePerStock = decPtrOf(before.PricePerStock)
		h.OldPremium = decPtrOf(before.Premium)
		osd := before.SettlementDate
		h.OldSettlementDate = &osd
		h.OldStatus = strPtr(before.Status)
	}
	return s.repo.InsertHistory(ctx, tx, h)
}

// reserveForContract mirrors OtcPortfolioService.reserveForContract: reserved +=
// amount, public = max(0, public-amount). No-op (warn) when no matching position.
func (s *Service) reserveForContract(ctx context.Context, q portfolio.Querier, sellerID int64, ticker string, amount int) error {
	p, err := s.findPortfolioByTicker(ctx, q, sellerID, ticker)
	if err != nil {
		return err
	}
	if p == nil {
		s.logger.Warn("otc reserve: no portfolio found", "seller", sellerID, "ticker", ticker)
		return nil
	}
	return s.portfolio.UpdateReservedAndPublic(ctx, q, p.ID, p.ReservedQuantity+amount, maxInt(0, p.PublicQuantity-amount))
}

// releaseForContract mirrors OtcPortfolioService.releaseForContract: reserved =
// max(0, reserved-amount), public = min(quantity, public+amount).
func (s *Service) releaseForContract(ctx context.Context, q portfolio.Querier, sellerID int64, ticker string, amount int) error {
	p, err := s.findPortfolioByTicker(ctx, q, sellerID, ticker)
	if err != nil {
		return err
	}
	if p == nil {
		s.logger.Warn("otc release: no portfolio found", "seller", sellerID, "ticker", ticker)
		return nil
	}
	return s.portfolio.UpdateReservedAndPublic(ctx, q, p.ID,
		maxInt(0, p.ReservedQuantity-amount), minInt(p.Quantity, p.PublicQuantity+amount))
}

// resolveSellerOwnedQuantity mirrors OtcService.resolveSellerOwnedQuantity: sum of
// the seller's shares for a ticker — publicQuantity when the position is exposed,
// else the full quantity. Market-lookup failures skip the position.
func (s *Service) resolveSellerOwnedQuantity(ctx context.Context, q portfolio.Querier, sellerID int64, ticker string) (int64, error) {
	if ticker == "" {
		return 0, nil
	}
	positions, err := s.portfolio.FindByUserID(ctx, q, sellerID)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, p := range positions {
		listing, lerr := s.market.GetListing(ctx, p.ListingID)
		if lerr != nil || listing == nil || listing.Ticker == nil {
			continue
		}
		if !strings.EqualFold(ticker, *listing.Ticker) {
			continue
		}
		if p.IsPublic && p.PublicQuantity > 0 {
			total += int64(p.PublicQuantity)
		} else {
			total += int64(p.Quantity)
		}
	}
	return total, nil
}

// findPortfolioByTicker scans the user's positions and returns the one whose
// market listing ticker matches (case-insensitive), or nil.
func (s *Service) findPortfolioByTicker(ctx context.Context, q portfolio.Querier, userID int64, ticker string) (*portfolio.Portfolio, error) {
	positions, err := s.portfolio.FindByUserID(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	for i := range positions {
		listing, lerr := s.market.GetListing(ctx, positions[i].ListingID)
		if lerr != nil || listing == nil || listing.Ticker == nil {
			continue
		}
		if strings.EqualFold(ticker, *listing.Ticker) {
			return &positions[i], nil
		}
	}
	return nil, nil
}

// resolveTicker mirrors resolveSellerOwnedTicker: listingId → ticker (or "" on
// failure).
func (s *Service) resolveTicker(ctx context.Context, listingID int64) string {
	listing, err := s.market.GetListing(ctx, listingID)
	if err != nil || listing == nil || listing.Ticker == nil {
		return ""
	}
	return *listing.Ticker
}

// resolveClientName mirrors resolveClientName: "First Last" trimmed, or nil when
// the customer cannot be resolved.
func (s *Service) resolveClientName(ctx context.Context, userID int64) *string {
	cust, err := s.customer.GetCustomer(ctx, userID)
	if err != nil || cust == nil {
		return nil
	}
	var first, last string
	if f := cust.First(); f != nil {
		first = *f
	}
	if l := cust.Last(); l != nil {
		last = *l
	}
	name := strings.TrimSpace(first + " " + last)
	return &name
}

func (s *Service) toPositionDto(ctx context.Context, p *portfolio.Portfolio) OtcPositionDto {
	var ticker *string
	if t := s.resolveTicker(ctx, p.ListingID); t != "" {
		ticker = &t
	}
	return OtcPositionDto{
		ID:                p.ID,
		ListingID:         p.ListingID,
		StockTicker:       ticker,
		TotalQuantity:     p.Quantity,
		ReservedQuantity:  p.ReservedQuantity,
		PublicQuantity:    p.PublicQuantity,
		AvailableQuantity: p.Quantity - p.ReservedQuantity,
	}
}

func toDto(o *OtcOffer) *OtcOfferDto {
	return &OtcOfferDto{
		ID:             o.ID,
		StockTicker:    o.StockTicker,
		BuyerID:        o.BuyerID,
		SellerID:       o.SellerID,
		Amount:         o.Amount,
		PricePerStock:  o.PricePerStock,
		Premium:        o.Premium,
		SettlementDate: api.NewLocalDate(o.SettlementDate),
		Status:         o.Status,
		ModifiedBy:     o.ModifiedBy,
		LastModified:   api.NewLocalDateTime(o.LastModified),
	}
}

func toContractDto(c *OptionContract) OptionContractDto {
	return OptionContractDto{
		ID:             c.ID,
		OfferID:        c.OfferID,
		StockTicker:    c.StockTicker,
		BuyerID:        c.BuyerID,
		SellerID:       c.SellerID,
		Amount:         c.Amount,
		PricePerStock:  c.PricePerStock,
		SettlementDate: api.NewLocalDate(c.SettlementDate),
		Status:         c.Status,
		CreatedAt:      api.NewLocalDateTime(c.CreatedAt),
		ExercisedAt:    api.LocalDateTimeFromPtr(c.ExercisedAt),
	}
}

func exposeTooMuchMsg(requested, quantity, reserved, maxAllowed int) string {
	return "Nije moguce izloziti " + strconv.Itoa(requested) + " akcija; posedujete " +
		strconv.Itoa(quantity) + ", od toga " + strconv.Itoa(reserved) + " rezervisano, maksimum za OTC je " +
		strconv.Itoa(maxAllowed) + "."
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// resolveActorName returns the actor's display name from the token claim when
// available, falling back to "user#<id>" so the field is never empty.
func resolveActorName(actorID int64, name *string) string {
	if name != nil && *name != "" {
		return *name
	}
	return "user#" + itoa(actorID)
}
func intPtr(n int) *int                           { return &n }
func strPtr(s string) *string                     { return &s }
func decPtrOf(d decimal.Decimal) *decimal.Decimal { return &d }

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
