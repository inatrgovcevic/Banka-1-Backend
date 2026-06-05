package otc

import (
	"context"
	"errors"
	"testing"

	"banka1/trading-service-go/internal/clients"
	"banka1/trading-service-go/internal/portfolio"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
)

// seqQuerier returns prepared QueryRow rows / Exec tags in sequence.
type seqQuerier struct {
	rows     []*fakeRow
	rowIdx   int
	execTag  pgconn.CommandTag
	execErr  error
	queryErr error
}

func (q *seqQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return q.execTag, q.execErr
}
func (q *seqQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &fakeRows{}, q.queryErr
}
func (q *seqQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if q.rowIdx >= len(q.rows) {
		return &fakeRow{err: pgx.ErrNoRows}
	}
	r := q.rows[q.rowIdx]
	q.rowIdx++
	return r
}

// stubResPortfolio stubs reservationPortfolioRepo.
type stubResPortfolio struct {
	byUser  []portfolio.Portfolio
	forUp   *portfolio.Portfolio
	forUpB  *portfolio.Portfolio // second FindForUpdate (buyer)
	forUpN  int
	errAny  error
	updates []string
}

func (s *stubResPortfolio) FindByUserID(_ context.Context, _ portfolio.Querier, _ int64) ([]portfolio.Portfolio, error) {
	return s.byUser, s.errAny
}
func (s *stubResPortfolio) FindByUserIDAndListingIDForUpdate(_ context.Context, _ portfolio.Querier, _, _ int64) (*portfolio.Portfolio, error) {
	if s.errAny != nil {
		return nil, s.errAny
	}
	s.forUpN++
	if s.forUpN == 1 {
		return s.forUp, nil
	}
	return s.forUpB, nil
}
func (s *stubResPortfolio) UpdateReservedQuantity(_ context.Context, _ portfolio.Querier, _ int64, _ int) error {
	s.updates = append(s.updates, "reserved")
	return nil
}
func (s *stubResPortfolio) UpdateQuantityAndReserved(_ context.Context, _ portfolio.Querier, _ int64, _, _ int) error {
	s.updates = append(s.updates, "qty_reserved")
	return nil
}
func (s *stubResPortfolio) UpdateQuantity(_ context.Context, _ portfolio.Querier, _ int64, _ int) error {
	s.updates = append(s.updates, "qty")
	return nil
}
func (s *stubResPortfolio) Insert(_ context.Context, _ portfolio.Querier, _, _ int64, _ string, _ int, _ decimal.Decimal) error {
	s.updates = append(s.updates, "insert")
	return nil
}

func newResSvc(pf reservationPortfolioRepo, mk marketLister, q reservationQuerier) *ReservationService {
	if mk == nil {
		mk = &stubMarket{}
	}
	return &ReservationService{
		portfolio: pf, market: mk, logger: discard(),
		runInTx: func(ctx context.Context, fn func(reservationQuerier) error) error {
			return fn(q)
		},
	}
}

func TestReservation_Reserve_Success(t *testing.T) {
	ticker := "AAPL"
	pf := &stubResPortfolio{
		byUser: []portfolio.Portfolio{{ID: 9, UserID: 20, ListingID: 1, Quantity: 10, ReservedQuantity: 0, AveragePurchasePrice: dec("0")}},
		forUp:  &portfolio.Portfolio{ID: 9, UserID: 20, ListingID: 1, Quantity: 10, ReservedQuantity: 0, AveragePurchasePrice: dec("0")},
	}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{1: {ID: 1, Ticker: &ticker}}}
	q := &seqQuerier{execTag: tag("INSERT 0 1")}
	svc := newResSvc(pf, mk, q)
	resp, err := svc.Reserve(context.Background(), 20, "AAPL", 5, "corr-1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != ReservationHeld {
		t.Errorf("status = %s want HELD", resp.Status)
	}
}

func TestReservation_Reserve_NoPortfolio(t *testing.T) {
	pf := &stubResPortfolio{byUser: nil}
	svc := newResSvc(pf, &stubMarket{}, &seqQuerier{})
	if _, err := svc.Reserve(context.Background(), 20, "AAPL", 5, "c"); err == nil {
		t.Fatal("expected conflict when no portfolio matches ticker")
	}
}

func TestReservation_Reserve_Insufficient(t *testing.T) {
	ticker := "AAPL"
	pf := &stubResPortfolio{
		byUser: []portfolio.Portfolio{{ID: 9, UserID: 20, ListingID: 1, Quantity: 3, AveragePurchasePrice: dec("0")}},
		forUp:  &portfolio.Portfolio{ID: 9, UserID: 20, ListingID: 1, Quantity: 3, AveragePurchasePrice: dec("0")},
	}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{1: {ID: 1, Ticker: &ticker}}}
	svc := newResSvc(pf, mk, &seqQuerier{})
	if _, err := svc.Reserve(context.Background(), 20, "AAPL", 5, "c"); err == nil {
		t.Fatal("expected insufficient-stock error")
	}
}

func TestReservation_Reserve_ConsumeExisting(t *testing.T) {
	ticker := "AAPL"
	// otc-exercise correlation + reserved >= amount -> consume, no new reserve
	pf := &stubResPortfolio{
		byUser: []portfolio.Portfolio{{ID: 9, UserID: 20, ListingID: 1, Quantity: 10, ReservedQuantity: 5, AveragePurchasePrice: dec("0")}},
		forUp:  &portfolio.Portfolio{ID: 9, UserID: 20, ListingID: 1, Quantity: 10, ReservedQuantity: 5, AveragePurchasePrice: dec("0")},
	}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{1: {ID: 1, Ticker: &ticker}}}
	svc := newResSvc(pf, mk, &seqQuerier{execTag: tag("INSERT 0 1")})
	resp, err := svc.Reserve(context.Background(), 20, "AAPL", 5, "otc-exercise-1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != ReservationHeld {
		t.Errorf("status = %s", resp.Status)
	}
	for _, u := range pf.updates {
		if u == "reserved" {
			t.Error("consumeExisting should not call UpdateReservedQuantity")
		}
	}
}

func TestReservation_Release_Success(t *testing.T) {
	pf := &stubResPortfolio{forUp: &portfolio.Portfolio{ID: 9, ReservedQuantity: 5, AveragePurchasePrice: dec("0")}}
	// first QueryRow: reservation row (seller_id, listing_id, amount, status)
	q := &seqQuerier{
		rows:    []*fakeRow{{vals: []any{int64(20), int64(1), 5, ReservationHeld}}},
		execTag: tag("UPDATE 1"),
	}
	svc := newResSvc(pf, nil, q)
	resp, err := svc.Release(context.Background(), "res-id", "corr")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != ReservationReleased {
		t.Errorf("status = %s want RELEASED", resp.Status)
	}
}

func TestReservation_Release_NotFound(t *testing.T) {
	q := &seqQuerier{rows: []*fakeRow{{err: pgx.ErrNoRows}}}
	svc := newResSvc(&stubResPortfolio{}, nil, q)
	resp, err := svc.Release(context.Background(), "res-id", "corr")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != ReservationUnknown {
		t.Errorf("status = %s want UNKNOWN", resp.Status)
	}
}

func TestReservation_Release_AlreadyReleased(t *testing.T) {
	q := &seqQuerier{rows: []*fakeRow{{vals: []any{int64(20), int64(1), 5, ReservationReleased}}}}
	svc := newResSvc(&stubResPortfolio{}, nil, q)
	resp, err := svc.Release(context.Background(), "res-id", "corr")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != ReservationReleased {
		t.Errorf("non-HELD should return its current status, got %s", resp.Status)
	}
}

func TestReservation_TransferOwnership_NewBuyer(t *testing.T) {
	pf := &stubResPortfolio{
		forUp:  &portfolio.Portfolio{ID: 9, UserID: 20, ListingID: 1, Quantity: 10, ReservedQuantity: 5, ListingType: "STOCK", AveragePurchasePrice: dec("100")},
		forUpB: nil, // buyer has no position -> Insert
	}
	q := &seqQuerier{
		rows:    []*fakeRow{{vals: []any{int64(20), int64(1), "AAPL", 5, ReservationHeld}}},
		execTag: tag("UPDATE 1"),
	}
	svc := newResSvc(pf, nil, q)
	resp, err := svc.TransferOwnership(context.Background(), "res-id", 10, "corr")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != TransferCompleted {
		t.Errorf("status = %s want COMPLETED", resp.Status)
	}
	if !containsStr(pf.updates, "insert") {
		t.Error("expected buyer Insert")
	}
}

func TestReservation_TransferOwnership_ExistingBuyer(t *testing.T) {
	pf := &stubResPortfolio{
		forUp:  &portfolio.Portfolio{ID: 9, UserID: 20, ListingID: 1, Quantity: 10, ReservedQuantity: 5, ListingType: "STOCK", AveragePurchasePrice: dec("100")},
		forUpB: &portfolio.Portfolio{ID: 8, UserID: 10, ListingID: 1, Quantity: 2, AveragePurchasePrice: dec("90")},
	}
	q := &seqQuerier{
		rows:    []*fakeRow{{vals: []any{int64(20), int64(1), "AAPL", 5, ReservationHeld}}},
		execTag: tag("UPDATE 1"),
	}
	svc := newResSvc(pf, nil, q)
	resp, err := svc.TransferOwnership(context.Background(), "res-id", 10, "corr")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != TransferCompleted {
		t.Errorf("status = %s", resp.Status)
	}
}

func TestReservation_TransferOwnership_NotFound(t *testing.T) {
	q := &seqQuerier{rows: []*fakeRow{{err: pgx.ErrNoRows}}}
	svc := newResSvc(&stubResPortfolio{}, nil, q)
	if _, err := svc.TransferOwnership(context.Background(), "res-id", 10, "corr"); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestReservation_TransferOwnership_NotHeld(t *testing.T) {
	q := &seqQuerier{rows: []*fakeRow{{vals: []any{int64(20), int64(1), "AAPL", 5, ReservationCommitted}}}}
	svc := newResSvc(&stubResPortfolio{}, nil, q)
	if _, err := svc.TransferOwnership(context.Background(), "res-id", 10, "corr"); err == nil {
		t.Fatal("expected not-HELD error")
	}
}

func TestReservation_ReverseOwnership_Success(t *testing.T) {
	pf := &stubResPortfolio{
		forUp:  &portfolio.Portfolio{ID: 9, UserID: 20, ListingID: 1, Quantity: 5, AveragePurchasePrice: dec("0")},
		forUpB: &portfolio.Portfolio{ID: 8, UserID: 10, ListingID: 1, Quantity: 5, AveragePurchasePrice: dec("0")},
	}
	q := &seqQuerier{
		rows:    []*fakeRow{{vals: []any{int64(20), int64(10), int64(1), 5, TransferCompleted}}},
		execTag: tag("UPDATE 1"),
	}
	svc := newResSvc(pf, nil, q)
	if err := svc.ReverseOwnership(context.Background(), "transfer-id", "corr"); err != nil {
		t.Fatal(err)
	}
}

func TestReservation_ReverseOwnership_NotFound(t *testing.T) {
	q := &seqQuerier{rows: []*fakeRow{{err: pgx.ErrNoRows}}}
	svc := newResSvc(&stubResPortfolio{}, nil, q)
	if err := svc.ReverseOwnership(context.Background(), "transfer-id", "corr"); err != nil {
		t.Fatal("not found should be no-op")
	}
}

func TestReservation_ReverseOwnership_AlreadyReversed(t *testing.T) {
	q := &seqQuerier{rows: []*fakeRow{{vals: []any{int64(20), int64(10), int64(1), 5, TransferReversed}}}}
	svc := newResSvc(&stubResPortfolio{}, nil, q)
	if err := svc.ReverseOwnership(context.Background(), "transfer-id", "corr"); err != nil {
		t.Fatal("already-reversed should be no-op")
	}
}

func TestReservation_ResolveListingByTicker_NoMatch(t *testing.T) {
	other := "MSFT"
	pf := &stubResPortfolio{byUser: []portfolio.Portfolio{{ID: 9, ListingID: 2, AveragePurchasePrice: dec("0")}}}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{2: {ID: 2, Ticker: &other}}}
	svc := newResSvc(pf, mk, &seqQuerier{})
	_, found, err := svc.resolveListingByTicker(context.Background(), &fakeQuerier{}, 20, "AAPL")
	if err != nil || found {
		t.Errorf("no match expected, got found=%v err=%v", found, err)
	}
}

func TestReservation_FindByUserError(t *testing.T) {
	pf := &stubResPortfolio{errAny: errors.New("db")}
	svc := newResSvc(pf, &stubMarket{}, &seqQuerier{})
	if _, err := svc.Reserve(context.Background(), 20, "AAPL", 5, "c"); err == nil {
		t.Fatal("expected db error")
	}
}

func containsStr(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}
