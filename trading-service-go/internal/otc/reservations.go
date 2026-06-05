package otc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/clients"
	"banka1/trading-service-go/internal/portfolio"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// ReservationService backs the PUBLIC /stocks/internal endpoints called directly
// by saga-orchestrator-service during the OTC_EXERCISE saga (reserve seller
// stock, transfer ownership to the buyer, and the compensations release/reverse).
// Mirrors trading-service StockReservationService over stock_reservations +
// stock_ownership_transfers + the shared `portfolio` table. Every method is one
// RunInTx (Java @Transactional). Portfolio rows are taken FOR UPDATE in a
// deterministic order (seller listing before buyer listing) to avoid deadlocks —
// a Go-port hardening; Java holds no portfolio row lock here.
type ReservationService struct {
	pool      *pgxpool.Pool
	portfolio reservationPortfolioRepo
	market    marketLister
	logger    *slog.Logger
	runInTx   qRunner
}

// reservationPortfolioRepo abstracts the subset of *portfolio.Repository the
// reservation flow uses. *portfolio.Repository satisfies it.
type reservationPortfolioRepo interface {
	FindByUserID(ctx context.Context, q portfolio.Querier, userID int64) ([]portfolio.Portfolio, error)
	FindByUserIDAndListingIDForUpdate(ctx context.Context, q portfolio.Querier, userID, listingID int64) (*portfolio.Portfolio, error)
	UpdateReservedQuantity(ctx context.Context, q portfolio.Querier, id int64, reserved int) error
	UpdateQuantityAndReserved(ctx context.Context, q portfolio.Querier, id int64, quantity, reserved int) error
	UpdateQuantity(ctx context.Context, q portfolio.Querier, id int64, quantity int) error
	Insert(ctx context.Context, q portfolio.Querier, userID, listingID int64, listingType string, quantity int, avg decimal.Decimal) error
}

// NewReservationService wires the reservation service over the pool + shared
// portfolio repo + market client (ticker resolution).
func NewReservationService(pool *pgxpool.Pool, portfolioRepo *portfolio.Repository, market *clients.MarketClient, logger *slog.Logger) *ReservationService {
	return &ReservationService{pool: pool, portfolio: portfolioRepo, market: market, logger: logger,
		runInTx: poolQRunner(pool)}
}

// Reserve mirrors StockReservationService.reserve. Reserves `amount` of the
// seller's `stockTicker` shares (reservedQuantity += amount) and records a HELD
// stock_reservations row. When the correlationId marks an OTC exercise and the
// seller already holds ≥ amount reserved (from accept-time), the existing
// reservation is consumed instead of double-reserving.
func (s *ReservationService) Reserve(ctx context.Context, sellerID int64, stockTicker string, amount int, correlationID string) (*ReservationResponse, error) {
	var resp *ReservationResponse
	err := s.runInTx(ctx, func(tx reservationQuerier) error {
		listingID, found, err := s.resolveListingByTicker(ctx, tx, sellerID, stockTicker)
		if err != nil {
			return err
		}
		if !found {
			return api.NewOtcError(http.StatusConflict,
				"Korisnik "+itoa(sellerID)+" nema portfolio poziciju za ticker "+stockTicker)
		}
		p, err := s.portfolio.FindByUserIDAndListingIDForUpdate(ctx, tx, sellerID, listingID)
		if err != nil {
			return err
		}
		if p == nil {
			return api.NewOtcError(http.StatusConflict,
				"Korisnik "+itoa(sellerID)+" nema portfolio poziciju za ticker "+stockTicker)
		}
		reserved := p.ReservedQuantity
		available := p.Quantity - reserved
		consumeExisting := isOtcExercise(correlationID) && reserved >= amount
		if available < amount && !consumeExisting {
			return api.NewOtcError(http.StatusConflict,
				"Prodavac "+itoa(sellerID)+" nema dovoljno slobodnih "+stockTicker+
					" akcija: available="+itoa(int64(available))+" requested="+itoa(int64(amount)))
		}
		if !consumeExisting {
			if err := s.portfolio.UpdateReservedQuantity(ctx, tx, p.ID, reserved+amount); err != nil {
				return err
			}
		}
		reservationID := newUUIDv4()
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_reservations
				(reservation_id, correlation_id, seller_id, listing_id, stock_ticker, amount, status)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, 'HELD')`,
			reservationID, correlationID, sellerID, listingID, stockTicker, amount); err != nil {
			return err
		}
		s.logger.Info("otc stock reserved", "seller", sellerID, "ticker", stockTicker,
			"amount", amount, "reservationId", reservationID, "consumedExisting", consumeExisting)
		resp = &ReservationResponse{ReservationID: reservationID, Status: ReservationHeld}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Release mirrors StockReservationService.release: the reservation compensation.
// Idempotent — a missing row returns UNKNOWN, an already-non-HELD row returns its
// current status, neither touching the portfolio.
func (s *ReservationService) Release(ctx context.Context, reservationID, correlationID string) (*ReservationResponse, error) {
	var resp *ReservationResponse
	err := s.runInTx(ctx, func(tx reservationQuerier) error {
		var (
			sellerID  int64
			listingID int64
			amount    int
			status    string
		)
		err := tx.QueryRow(ctx,
			`SELECT seller_id, listing_id, amount, status FROM stock_reservations WHERE reservation_id = $1::uuid`,
			reservationID).Scan(&sellerID, &listingID, &amount, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			s.logger.Warn("otc release: reservation not found — duplicate compensation?",
				"reservationId", reservationID, "correlationId", correlationID)
			resp = &ReservationResponse{ReservationID: reservationID, Status: ReservationUnknown}
			return nil
		}
		if err != nil {
			return err
		}
		if status != ReservationHeld {
			resp = &ReservationResponse{ReservationID: reservationID, Status: status}
			return nil
		}
		p, err := s.portfolio.FindByUserIDAndListingIDForUpdate(ctx, tx, sellerID, listingID)
		if err != nil {
			return err
		}
		if p != nil {
			if err := s.portfolio.UpdateReservedQuantity(ctx, tx, p.ID, maxInt(0, p.ReservedQuantity-amount)); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx,
			`UPDATE stock_reservations SET status='RELEASED', released_at=NOW() WHERE reservation_id = $1::uuid AND status='HELD'`,
			reservationID); err != nil {
			return err
		}
		s.logger.Info("otc stock reservation released", "reservationId", reservationID)
		resp = &ReservationResponse{ReservationID: reservationID, Status: ReservationReleased}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// TransferOwnership mirrors StockReservationService.transferOwnership: settles a
// HELD reservation — decrement the seller (quantity + reserved), upsert the buyer
// (+quantity as a new lot), mark the reservation COMMITTED, and record a COMPLETED
// stock_ownership_transfers row.
func (s *ReservationService) TransferOwnership(ctx context.Context, reservationID string, buyerID int64, correlationID string) (*OwnershipTransferResponse, error) {
	var resp *OwnershipTransferResponse
	err := s.runInTx(ctx, func(tx reservationQuerier) error {
		var (
			sellerID  int64
			listingID int64
			ticker    string
			amount    int
			status    string
		)
		err := tx.QueryRow(ctx,
			`SELECT seller_id, listing_id, stock_ticker, amount, status FROM stock_reservations WHERE reservation_id = $1::uuid`,
			reservationID).Scan(&sellerID, &listingID, &ticker, &amount, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return api.NewOtcError(http.StatusConflict, "Stock reservation "+reservationID+" not found")
		}
		if err != nil {
			return err
		}
		if status != ReservationHeld {
			return api.NewOtcError(http.StatusConflict, "Stock reservation "+reservationID+" is not HELD: "+status)
		}

		// Lock seller then buyer (deterministic order) before mutating either.
		seller, err := s.portfolio.FindByUserIDAndListingIDForUpdate(ctx, tx, sellerID, listingID)
		if err != nil {
			return err
		}
		if seller == nil {
			return api.NewOtcError(http.StatusConflict,
				"Seller "+itoa(sellerID)+" portfolio for listing "+itoa(listingID)+" not found")
		}
		buyer, err := s.portfolio.FindByUserIDAndListingIDForUpdate(ctx, tx, buyerID, listingID)
		if err != nil {
			return err
		}

		// Decrement seller.
		if err := s.portfolio.UpdateQuantityAndReserved(ctx, tx, seller.ID,
			seller.Quantity-amount, maxInt(0, seller.ReservedQuantity-amount)); err != nil {
			return err
		}
		// Upsert buyer (+amount as a new lot at the seller's avg price).
		if buyer == nil {
			if err := s.portfolio.Insert(ctx, tx, buyerID, listingID, seller.ListingType, amount, seller.AveragePurchasePrice); err != nil {
				return err
			}
		} else if err := s.portfolio.UpdateQuantity(ctx, tx, buyer.ID, buyer.Quantity+amount); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx,
			`UPDATE stock_reservations SET status='COMMITTED' WHERE reservation_id = $1::uuid`, reservationID); err != nil {
			return err
		}
		transferID := newUUIDv4()
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_ownership_transfers
				(transfer_id, reservation_id, correlation_id, seller_id, buyer_id, listing_id, stock_ticker, amount, status)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, 'COMPLETED')`,
			transferID, reservationID, correlationID, sellerID, buyerID, listingID, ticker, amount); err != nil {
			return err
		}
		s.logger.Info("otc ownership transferred", "ticker", ticker, "amount", amount,
			"seller", sellerID, "buyer", buyerID, "transferId", transferID)
		resp = &OwnershipTransferResponse{OwnershipTransferID: transferID, Status: TransferCompleted}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ReverseOwnership mirrors StockReservationService.reverseOwnership: the transfer
// compensation. Idempotent — a missing or already-non-COMPLETED transfer is a
// no-op. Restores the seller (quantity + reserved) and removes the lot from the
// buyer (quantity).
func (s *ReservationService) ReverseOwnership(ctx context.Context, ownershipTransferID, correlationID string) error {
	return s.runInTx(ctx, func(tx reservationQuerier) error {
		var (
			sellerID  int64
			buyerID   int64
			listingID int64
			amount    int
			status    string
		)
		err := tx.QueryRow(ctx,
			`SELECT seller_id, buyer_id, listing_id, amount, status FROM stock_ownership_transfers WHERE transfer_id = $1::uuid`,
			ownershipTransferID).Scan(&sellerID, &buyerID, &listingID, &amount, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			s.logger.Warn("otc reverse ownership: transfer not found",
				"transferId", ownershipTransferID, "correlationId", correlationID)
			return nil
		}
		if err != nil {
			return err
		}
		if status != TransferCompleted {
			s.logger.Info("otc reverse ownership: transfer already in terminal state — no-op",
				"transferId", ownershipTransferID, "status", status)
			return nil
		}

		// Lock seller then buyer (same order as transfer) before mutating.
		seller, err := s.portfolio.FindByUserIDAndListingIDForUpdate(ctx, tx, sellerID, listingID)
		if err != nil {
			return err
		}
		buyer, err := s.portfolio.FindByUserIDAndListingIDForUpdate(ctx, tx, buyerID, listingID)
		if err != nil {
			return err
		}
		// Reverse buyer (remove the lot).
		if buyer != nil {
			if err := s.portfolio.UpdateQuantity(ctx, tx, buyer.ID, maxInt(0, buyer.Quantity-amount)); err != nil {
				return err
			}
		}
		// Restore seller quantity only. The reservation was COMMITTED by TransferOwnership
		// so reserved_quantity is already 0 at this point — incrementing it here would
		// create a phantom reservation that double-counts when C2 (ReleaseStocks) runs.
		if seller != nil {
			if err := s.portfolio.UpdateQuantity(ctx, tx, seller.ID, seller.Quantity+amount); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx,
			`UPDATE stock_ownership_transfers SET status='REVERSED', reversed_at=NOW() WHERE transfer_id = $1::uuid`,
			ownershipTransferID); err != nil {
			return err
		}
		s.logger.Info("otc ownership reversed", "transferId", ownershipTransferID)
		return nil
	})
}

// resolveListingByTicker mirrors findPortfolioByTicker: scan the user's positions
// and return the listingId whose market listing ticker matches (case-insensitive).
// Market-lookup failures skip that position (defensive, matches Java).
func (s *ReservationService) resolveListingByTicker(ctx context.Context, q portfolio.Querier, userID int64, ticker string) (int64, bool, error) {
	positions, err := s.portfolio.FindByUserID(ctx, q, userID)
	if err != nil {
		return 0, false, err
	}
	for _, p := range positions {
		listing, lerr := s.market.GetListing(ctx, p.ListingID)
		if lerr != nil || listing == nil || listing.Ticker == nil {
			continue
		}
		if strings.EqualFold(ticker, *listing.Ticker) {
			return p.ListingID, true, nil
		}
	}
	return 0, false, nil
}

// isOtcExercise mirrors StockReservationService.isOtcExercise: the correlationId
// the exercise saga uses is prefixed "otc-exercise-".
func isOtcExercise(correlationID string) bool {
	return strings.HasPrefix(correlationID, "otc-exercise-")
}

// newUUIDv4 generates a random RFC 4122 v4 UUID using crypto/rand (matches the
// funds liquidation helper — avoids a direct google/uuid dependency).
func newUUIDv4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString(b[:])
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return hex.EncodeToString(b[0:4]) + "-" +
		hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" +
		hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:16])
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
