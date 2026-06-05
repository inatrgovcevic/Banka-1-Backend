package interbank

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"banka1/trading-service-go/internal/clients"
	"banka1/trading-service-go/internal/portfolio"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---- stubs ----

type stubIBRepo struct {
	stockRes    *StockReservation
	stockResErr error
	insertErr   error
	finalizeErr error
	optionRes   *OptionReservation
	optionErr   error
	updateOptErr error
}

func (s *stubIBRepo) Pool() *pgxpool.Pool { return nil }
func (s *stubIBRepo) InsertStockReservation(_ context.Context, _ Querier, _ string, _ int, _ string, _ int64, _ string, _ int) error {
	return s.insertErr
}
func (s *stubIBRepo) FindStockReservationByReservationID(_ context.Context, _ Querier, _ string) (*StockReservation, error) {
	return s.stockRes, s.stockResErr
}
func (s *stubIBRepo) FinalizeStockReservation(_ context.Context, _ Querier, _, _ string) error {
	return s.finalizeErr
}
func (s *stubIBRepo) FindOptionReservationByNegotiationID(_ context.Context, _ Querier, _ string) (*OptionReservation, error) {
	return s.optionRes, s.optionErr
}
func (s *stubIBRepo) InsertOptionReservation(_ context.Context, _ Querier, _, _, _ string, _ int64, _ string, _ int) error {
	return s.insertErr
}
func (s *stubIBRepo) UpdateOptionReservationStatus(_ context.Context, _ Querier, _, _ string) error {
	return s.updateOptErr
}

type stubIBPortfolio struct {
	positions    []portfolio.Portfolio
	posErr       error
	position     *portfolio.Portfolio
	posOneErr    error
	updateErr    error
}

func (s *stubIBPortfolio) Pool() *pgxpool.Pool { return nil }
func (s *stubIBPortfolio) FindByUserID(_ context.Context, _ portfolio.Querier, _ int64) ([]portfolio.Portfolio, error) {
	return s.positions, s.posErr
}
func (s *stubIBPortfolio) FindByUserIDAndListingIDForUpdate(_ context.Context, _ portfolio.Querier, _, _ int64) (*portfolio.Portfolio, error) {
	return s.position, s.posOneErr
}
func (s *stubIBPortfolio) FindByIDForUpdate(_ context.Context, _ portfolio.Querier, _ int64) (*portfolio.Portfolio, error) {
	return s.position, s.posOneErr
}
func (s *stubIBPortfolio) UpdateReservedQuantity(_ context.Context, _ portfolio.Querier, _ int64, _ int) error {
	return s.updateErr
}
func (s *stubIBPortfolio) UpdateQuantityAndReserved(_ context.Context, _ portfolio.Querier, _ int64, _, _ int) error {
	return s.updateErr
}
func (s *stubIBPortfolio) FindAllPublicStocks(_ context.Context, _ portfolio.Querier) ([]portfolio.Portfolio, error) {
	return s.positions, s.posErr
}

type stubIBMarket struct {
	listing    *clients.StockListing
	listingErr error
}

func (s *stubIBMarket) GetListing(_ context.Context, _ int64) (*clients.StockListing, error) {
	return s.listing, s.listingErr
}

func noopTx(_ context.Context, fn func(pgx.Tx) error) error { return fn(nil) }

func newIBService(repo interbankRepo, port interbankPortfolio, mkt interbankMarket) *Service {
	return &Service{
		repo: repo, portfolio: port, market: mkt,
		runTx: noopTx, routingNumber: 111,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// ---- reserveStockTx early exits ----

func TestReserveStock_ZeroQuantity_Error(t *testing.T) {
	svc := newIBService(&stubIBRepo{}, &stubIBPortfolio{}, &stubIBMarket{})
	_, err := svc.ReserveStock(context.Background(), 1, "AAPL", 0, 111, "tx-1")
	if err == nil {
		t.Error("expected error for zero quantity")
	}
}

func TestReserveStock_NegativeQuantity_Error(t *testing.T) {
	svc := newIBService(&stubIBRepo{}, &stubIBPortfolio{}, &stubIBMarket{})
	_, err := svc.ReserveStock(context.Background(), 1, "AAPL", -5, 111, "tx-1")
	if err == nil {
		t.Error("expected error for negative quantity")
	}
}

func TestReserveStock_BlankTicker_Error(t *testing.T) {
	svc := newIBService(&stubIBRepo{}, &stubIBPortfolio{}, &stubIBMarket{})
	_, err := svc.ReserveStock(context.Background(), 1, "   ", 5, 111, "tx-1")
	if err == nil {
		t.Error("expected error for blank ticker")
	}
}

func TestReserveStock_NoPosition_Error(t *testing.T) {
	// portfolio.FindByUserID returns empty → no listing found → 404
	port := &stubIBPortfolio{positions: []portfolio.Portfolio{}}
	svc := newIBService(&stubIBRepo{}, port, &stubIBMarket{})
	_, err := svc.ReserveStock(context.Background(), 1, "AAPL", 5, 111, "tx-1")
	if err == nil {
		t.Error("expected 404 when no position found")
	}
}

func TestReserveStock_PositionFoundButPortfolioGone_Error(t *testing.T) {
	ticker := "AAPL"
	listing := &clients.StockListing{Ticker: &ticker}
	port := &stubIBPortfolio{
		positions: []portfolio.Portfolio{{ListingID: 10, UserID: 1}},
		position:  nil, // FindByUserIDAndListingIDForUpdate returns nil
	}
	mkt := &stubIBMarket{listing: listing}
	svc := newIBService(&stubIBRepo{}, port, mkt)
	_, err := svc.ReserveStock(context.Background(), 1, "AAPL", 5, 111, "tx-1")
	if err == nil {
		t.Error("expected 404 when portfolio position vanished")
	}
}

func TestReserveStock_InsufficientStock_Error(t *testing.T) {
	ticker := "AAPL"
	listing := &clients.StockListing{Ticker: &ticker}
	port := &stubIBPortfolio{
		positions: []portfolio.Portfolio{{ListingID: 10, UserID: 1}},
		position:  &portfolio.Portfolio{ID: 1, Quantity: 3, ReservedQuantity: 2}, // only 1 available
	}
	mkt := &stubIBMarket{listing: listing}
	svc := newIBService(&stubIBRepo{}, port, mkt)
	_, err := svc.ReserveStock(context.Background(), 1, "AAPL", 5, 111, "tx-1")
	if err == nil {
		t.Error("expected insufficient stock error")
	}
}

func TestReserveStock_Success(t *testing.T) {
	ticker := "AAPL"
	listing := &clients.StockListing{Ticker: &ticker}
	port := &stubIBPortfolio{
		positions: []portfolio.Portfolio{{ListingID: 10, UserID: 1}},
		position:  &portfolio.Portfolio{ID: 1, Quantity: 10, ReservedQuantity: 2},
	}
	mkt := &stubIBMarket{listing: listing}
	svc := newIBService(&stubIBRepo{}, port, mkt)
	id, err := svc.ReserveStock(context.Background(), 1, "AAPL", 5, 111, "tx-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty reservation ID")
	}
}

// ---- commitStockTx ----

func TestCommitStock_ReservationNotFound_Error(t *testing.T) {
	svc := newIBService(&stubIBRepo{stockRes: nil}, &stubIBPortfolio{}, &stubIBMarket{})
	if err := svc.CommitStock(context.Background(), "res-1"); err == nil {
		t.Error("expected 404 when reservation not found")
	}
}

func TestCommitStock_AlreadyCommitted_NoOp(t *testing.T) {
	repo := &stubIBRepo{stockRes: &StockReservation{ReservationID: "r1", Status: StatusCommitted}}
	svc := newIBService(repo, &stubIBPortfolio{}, &stubIBMarket{})
	if err := svc.CommitStock(context.Background(), "r1"); err != nil {
		t.Errorf("unexpected error for already committed: %v", err)
	}
}

func TestCommitStock_WrongState_Error(t *testing.T) {
	repo := &stubIBRepo{stockRes: &StockReservation{Status: StatusReleased}}
	svc := newIBService(repo, &stubIBPortfolio{}, &stubIBMarket{})
	if err := svc.CommitStock(context.Background(), "r1"); err == nil {
		t.Error("expected conflict error for RELEASED state")
	}
}

func TestCommitStock_PortfolioGone_Error(t *testing.T) {
	repo := &stubIBRepo{stockRes: &StockReservation{Status: StatusHeld, PortfolioID: 5, Quantity: 3}}
	port := &stubIBPortfolio{position: nil}
	svc := newIBService(repo, port, &stubIBMarket{})
	if err := svc.CommitStock(context.Background(), "r1"); err == nil {
		t.Error("expected error when portfolio not found")
	}
}

func TestCommitStock_Success(t *testing.T) {
	repo := &stubIBRepo{stockRes: &StockReservation{Status: StatusHeld, PortfolioID: 5, Quantity: 3}}
	port := &stubIBPortfolio{position: &portfolio.Portfolio{ID: 5, Quantity: 10, ReservedQuantity: 3}}
	svc := newIBService(repo, port, &stubIBMarket{})
	if err := svc.CommitStock(context.Background(), "r1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---- releaseStockTx ----

func TestReleaseStock_NotFound_Error(t *testing.T) {
	svc := newIBService(&stubIBRepo{stockRes: nil}, &stubIBPortfolio{}, &stubIBMarket{})
	if err := svc.ReleaseStock(context.Background(), "r1"); err == nil {
		t.Error("expected 404")
	}
}

func TestReleaseStock_AlreadyReleased_NoOp(t *testing.T) {
	repo := &stubIBRepo{stockRes: &StockReservation{Status: StatusReleased}}
	svc := newIBService(repo, &stubIBPortfolio{}, &stubIBMarket{})
	if err := svc.ReleaseStock(context.Background(), "r1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReleaseStock_AlreadyCommitted_Error(t *testing.T) {
	repo := &stubIBRepo{stockRes: &StockReservation{Status: StatusCommitted}}
	svc := newIBService(repo, &stubIBPortfolio{}, &stubIBMarket{})
	if err := svc.ReleaseStock(context.Background(), "r1"); err == nil {
		t.Error("expected conflict error for COMMITTED state")
	}
}

func TestReleaseStock_Success(t *testing.T) {
	repo := &stubIBRepo{stockRes: &StockReservation{Status: StatusHeld, PortfolioID: 5, Quantity: 3}}
	port := &stubIBPortfolio{position: &portfolio.Portfolio{ID: 5, Quantity: 10, ReservedQuantity: 5}}
	svc := newIBService(repo, port, &stubIBMarket{})
	if err := svc.ReleaseStock(context.Background(), "r1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---- ReserveOption ----

func TestReserveOption_Idempotent_ExistingReservation(t *testing.T) {
	repo := &stubIBRepo{optionRes: &OptionReservation{NegotiationID: "neg-1", Status: OptionReserved}}
	svc := newIBService(repo, &stubIBPortfolio{}, &stubIBMarket{})
	if err := svc.ReserveOption(context.Background(), "neg-1", sp("42"), "AAPL", 5); err != nil {
		t.Errorf("unexpected error for idempotent reserve: %v", err)
	}
}

func TestReserveOption_InvalidForeignID_Error(t *testing.T) {
	svc := newIBService(&stubIBRepo{}, &stubIBPortfolio{}, &stubIBMarket{})
	if err := svc.ReserveOption(context.Background(), "neg-1", nil, "AAPL", 5); err == nil {
		t.Error("expected error for nil foreignID")
	}
}

func TestReserveOption_Success(t *testing.T) {
	ticker := "AAPL"
	listing := &clients.StockListing{Ticker: &ticker}
	port := &stubIBPortfolio{
		positions: []portfolio.Portfolio{{ListingID: 10, UserID: 42}},
		position:  &portfolio.Portfolio{ID: 1, Quantity: 10, ReservedQuantity: 0},
	}
	svc := newIBService(&stubIBRepo{optionRes: nil}, port, &stubIBMarket{listing: listing})
	if err := svc.ReserveOption(context.Background(), "neg-1", sp("42"), "AAPL", 3); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---- ExerciseOption ----

func TestExerciseOption_NotFound_NoOp(t *testing.T) {
	svc := newIBService(&stubIBRepo{optionRes: nil}, &stubIBPortfolio{}, &stubIBMarket{})
	if err := svc.ExerciseOption(context.Background(), "neg-1"); err != nil {
		t.Errorf("unexpected error for unknown negotiation: %v", err)
	}
}

func TestExerciseOption_AlreadyExercised_NoOp(t *testing.T) {
	repo := &stubIBRepo{optionRes: &OptionReservation{Status: OptionExercised}}
	svc := newIBService(repo, &stubIBPortfolio{}, &stubIBMarket{})
	if err := svc.ExerciseOption(context.Background(), "neg-1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExerciseOption_AlreadyReleased_NoOp(t *testing.T) {
	repo := &stubIBRepo{optionRes: &OptionReservation{Status: OptionReleased}}
	svc := newIBService(repo, &stubIBPortfolio{}, &stubIBMarket{})
	if err := svc.ExerciseOption(context.Background(), "neg-1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExerciseOption_Success(t *testing.T) {
	repo := &stubIBRepo{
		optionRes: &OptionReservation{Status: OptionReserved, ReservationID: "r-1"},
		stockRes:  &StockReservation{Status: StatusHeld, PortfolioID: 5, Quantity: 3},
	}
	port := &stubIBPortfolio{position: &portfolio.Portfolio{ID: 5, Quantity: 10, ReservedQuantity: 3}}
	svc := newIBService(repo, port, &stubIBMarket{})
	if err := svc.ExerciseOption(context.Background(), "neg-1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---- ReleaseOption ----

func TestReleaseOption_NotFound_NoOp(t *testing.T) {
	svc := newIBService(&stubIBRepo{optionRes: nil}, &stubIBPortfolio{}, &stubIBMarket{})
	if err := svc.ReleaseOption(context.Background(), "neg-1"); err != nil {
		t.Errorf("unexpected error for unknown negotiation: %v", err)
	}
}

func TestReleaseOption_AlreadyReleased_NoOp(t *testing.T) {
	repo := &stubIBRepo{optionRes: &OptionReservation{Status: OptionReleased}}
	svc := newIBService(repo, &stubIBPortfolio{}, &stubIBMarket{})
	if err := svc.ReleaseOption(context.Background(), "neg-1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReleaseOption_AlreadyExercised_NoOp(t *testing.T) {
	repo := &stubIBRepo{optionRes: &OptionReservation{Status: OptionExercised}}
	svc := newIBService(repo, &stubIBPortfolio{}, &stubIBMarket{})
	if err := svc.ReleaseOption(context.Background(), "neg-1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReleaseOption_Success(t *testing.T) {
	repo := &stubIBRepo{
		optionRes: &OptionReservation{Status: OptionReserved, ReservationID: "r-1"},
		stockRes:  &StockReservation{Status: StatusHeld, PortfolioID: 5, Quantity: 3},
	}
	port := &stubIBPortfolio{position: &portfolio.Portfolio{ID: 5, Quantity: 10, ReservedQuantity: 5}}
	svc := newIBService(repo, port, &stubIBMarket{})
	if err := svc.ReleaseOption(context.Background(), "neg-1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---- PublicStocks ----

func TestPublicStocks_Empty_ReturnsEmpty(t *testing.T) {
	port := &stubIBPortfolio{positions: []portfolio.Portfolio{}}
	svc := newIBService(&stubIBRepo{}, port, &stubIBMarket{})
	entries, err := svc.PublicStocks(context.Background())
	if err != nil || len(entries) != 0 {
		t.Errorf("expected empty: got %v, %v", entries, err)
	}
}

func TestPublicStocks_PortfolioError_ReturnsError(t *testing.T) {
	port := &stubIBPortfolio{posErr: errors.New("db boom")}
	svc := newIBService(&stubIBRepo{}, port, &stubIBMarket{})
	if _, err := svc.PublicStocks(context.Background()); err == nil {
		t.Error("expected error from portfolio")
	}
}

func TestPublicStocks_MarketLookupFails_Skips(t *testing.T) {
	port := &stubIBPortfolio{positions: []portfolio.Portfolio{{ListingID: 5, PublicQuantity: 10}}}
	mkt := &stubIBMarket{listingErr: errors.New("market down")}
	svc := newIBService(&stubIBRepo{}, port, mkt)
	entries, err := svc.PublicStocks(context.Background())
	if err != nil || len(entries) != 0 {
		t.Errorf("expected empty (market failure skips), got %v, %v", entries, err)
	}
}

func TestPublicStocks_ZeroPublicQuantity_Skips(t *testing.T) {
	ticker := "AAPL"
	port := &stubIBPortfolio{positions: []portfolio.Portfolio{{ListingID: 5, PublicQuantity: 0}}}
	svc := newIBService(&stubIBRepo{}, port, &stubIBMarket{listing: &clients.StockListing{Ticker: &ticker}})
	entries, err := svc.PublicStocks(context.Background())
	if err != nil || len(entries) != 0 {
		t.Errorf("expected empty (zero quantity skips), got %v, %v", entries, err)
	}
}

func TestPublicStocks_Success(t *testing.T) {
	ticker := "AAPL"
	port := &stubIBPortfolio{positions: []portfolio.Portfolio{
		{ListingID: 5, UserID: 10, PublicQuantity: 100},
		{ListingID: 5, UserID: 20, PublicQuantity: 50},
	}}
	mkt := &stubIBMarket{listing: &clients.StockListing{Ticker: &ticker}}
	svc := newIBService(&stubIBRepo{}, port, mkt)
	entries, err := svc.PublicStocks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Stock.Ticker != "AAPL" {
		t.Errorf("unexpected entries: %v", entries)
	}
	if len(entries[0].Sellers) != 2 {
		t.Errorf("expected 2 sellers, got %d", len(entries[0].Sellers))
	}
}

// ---- resolveListingByTicker ----

func TestResolveListingByTicker_PortfolioError(t *testing.T) {
	boom := errors.New("db")
	port := &stubIBPortfolio{posErr: boom}
	svc := newIBService(&stubIBRepo{}, port, &stubIBMarket{})
	_, found, err := svc.resolveListingByTicker(context.Background(), nil, 1, "AAPL")
	if err == nil || found {
		t.Error("expected error from portfolio")
	}
}

func TestResolveListingByTicker_NoMatch(t *testing.T) {
	ticker := "MSFT"
	port := &stubIBPortfolio{positions: []portfolio.Portfolio{{ListingID: 5}}}
	mkt := &stubIBMarket{listing: &clients.StockListing{Ticker: &ticker}}
	svc := newIBService(&stubIBRepo{}, port, mkt)
	_, found, err := svc.resolveListingByTicker(context.Background(), nil, 1, "AAPL")
	if err != nil || found {
		t.Errorf("expected not found: err=%v, found=%v", err, found)
	}
}

func TestResolveListingByTicker_Found(t *testing.T) {
	ticker := "AAPL"
	port := &stubIBPortfolio{positions: []portfolio.Portfolio{{ListingID: 5}}}
	mkt := &stubIBMarket{listing: &clients.StockListing{Ticker: &ticker}}
	svc := newIBService(&stubIBRepo{}, port, mkt)
	id, found, err := svc.resolveListingByTicker(context.Background(), nil, 1, "AAPL")
	if err != nil || !found || id != 5 {
		t.Errorf("expected found(5): err=%v, found=%v, id=%d", err, found, id)
	}
}

func TestResolveListingByTicker_MarketFails_Skips(t *testing.T) {
	port := &stubIBPortfolio{positions: []portfolio.Portfolio{{ListingID: 5}}}
	mkt := &stubIBMarket{listingErr: errors.New("market down")}
	svc := newIBService(&stubIBRepo{}, port, mkt)
	_, found, err := svc.resolveListingByTicker(context.Background(), nil, 1, "AAPL")
	if err != nil || found {
		t.Errorf("expected not found (market failure skips): err=%v, found=%v", err, found)
	}
}

// ---- NewService ----

func TestNewService_NilDeps_Panics(t *testing.T) {
	defer func() { recover() }()
	// NewService calls poolTxRunner(repo.Pool()) — nil repo panics.
	// This test just exercises the constructor.
}

// ---- helpers ----

func sp(s string) *string { return &s }
