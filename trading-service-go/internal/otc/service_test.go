package otc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"banka1/trading-service-go/internal/clients"
	"banka1/trading-service-go/internal/portfolio"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// ============================ stubs =======================================

// fakeTxRunner just runs fn with a nil tx — the stubs ignore the Querier arg.
func fakeTxRunner(ctx context.Context, fn func(pgx.Tx) error) error {
	return fn(nil)
}

// failTxRunner refuses to begin a tx.
func failTxRunner(ctx context.Context, fn func(pgx.Tx) error) error {
	return errors.New("tx begin failed")
}

type stubRepo struct {
	offer    *OtcOffer
	offerErr error

	activeOffers    []OtcOffer
	activeOffersErr error

	contract    *OptionContract
	contractErr error

	insertOfferErr    error
	updateOfferErr    error
	insertContractErr error
	updateStatusErr   error
	setExercisedErr   error
	historyErr        error

	sumActive    int64
	sumActiveErr error

	buyerContracts  []OptionContract
	sellerContracts []OptionContract
	contractsErr    error

	staleContracts    []OptionContract
	staleErr          error
	reminderContracts []OptionContract
	reminderErr       error

	reminderInserted bool
	reminderInsErr   error

	historyRows []NegotiationHistory
	historyFErr error

	// recorders
	insertedContract  *OptionContract
	updatedStatuses   []string
	exercisedAtCalled bool
	insertedHistory   []*NegotiationHistory
}

func (s *stubRepo) Pool() *pgxpool.Pool { return nil }

func (s *stubRepo) InsertOffer(_ context.Context, _ Querier, o *OtcOffer) error {
	if s.insertOfferErr != nil {
		return s.insertOfferErr
	}
	o.ID = 100
	return nil
}
func (s *stubRepo) FindOfferByID(_ context.Context, _ Querier, _ int64) (*OtcOffer, error) {
	return s.offer, s.offerErr
}
func (s *stubRepo) FindOfferByIDForUpdate(_ context.Context, _ Querier, _ int64) (*OtcOffer, error) {
	if s.offerErr != nil {
		return nil, s.offerErr
	}
	if s.offer == nil {
		return nil, ErrNotFound
	}
	cp := *s.offer
	return &cp, nil
}
func (s *stubRepo) UpdateOffer(_ context.Context, _ Querier, _ *OtcOffer) error {
	return s.updateOfferErr
}
func (s *stubRepo) FindActiveOffersForUser(_ context.Context, _ int64) ([]OtcOffer, error) {
	return s.activeOffers, s.activeOffersErr
}
func (s *stubRepo) InsertOptionContract(_ context.Context, _ Querier, c *OptionContract) error {
	if s.insertContractErr != nil {
		return s.insertContractErr
	}
	c.ID = 555
	s.insertedContract = c
	return nil
}
func (s *stubRepo) FindOptionContractByID(_ context.Context, _ Querier, _ int64) (*OptionContract, error) {
	return s.contract, s.contractErr
}
func (s *stubRepo) FindOptionContractByIDForUpdate(_ context.Context, _ Querier, _ int64) (*OptionContract, error) {
	if s.contractErr != nil {
		return nil, s.contractErr
	}
	if s.contract == nil {
		return nil, ErrNotFound
	}
	cp := *s.contract
	return &cp, nil
}
func (s *stubRepo) UpdateOptionContractStatus(_ context.Context, _ Querier, _ int64, status string) error {
	if s.updateStatusErr != nil {
		return s.updateStatusErr
	}
	s.updatedStatuses = append(s.updatedStatuses, status)
	return nil
}
func (s *stubRepo) SetOptionContractExercisedAt(_ context.Context, _ Querier, _ int64, _ time.Time) error {
	s.exercisedAtCalled = true
	return s.setExercisedErr
}
func (s *stubRepo) SumActiveBySellerAndTicker(_ context.Context, _ Querier, _ int64, _ string) (int64, error) {
	return s.sumActive, s.sumActiveErr
}
func (s *stubRepo) FindContractsByBuyerIDAndStatus(_ context.Context, _ int64, _ string) ([]OptionContract, error) {
	return s.buyerContracts, s.contractsErr
}
func (s *stubRepo) FindContractsBySellerIDAndStatus(_ context.Context, _ int64, _ string) ([]OptionContract, error) {
	return s.sellerContracts, s.contractsErr
}
func (s *stubRepo) FindContractsByStatusAndSettlementDateBefore(_ context.Context, _ string, _ time.Time) ([]OptionContract, error) {
	return s.staleContracts, s.staleErr
}
func (s *stubRepo) FindContractsByStatusAndSettlementDate(_ context.Context, _ string, _ time.Time) ([]OptionContract, error) {
	return s.reminderContracts, s.reminderErr
}
func (s *stubRepo) InsertExpiryReminderIfAbsent(_ context.Context, _ Querier, _ int64, _ int) (bool, error) {
	return s.reminderInserted, s.reminderInsErr
}
func (s *stubRepo) InsertHistory(_ context.Context, _ Querier, h *NegotiationHistory) error {
	if s.historyErr != nil {
		return s.historyErr
	}
	s.insertedHistory = append(s.insertedHistory, h)
	return nil
}
func (s *stubRepo) HistoryForUser(_ context.Context, _ int64, _ *string, _ *int64, _, _ *time.Time) ([]NegotiationHistory, error) {
	return s.historyRows, s.historyFErr
}

type stubPortfolio struct {
	byUser       []portfolio.Portfolio
	byUserErr    error
	byID         *portfolio.Portfolio
	byIDErr      error
	byUserList   *portfolio.Portfolio
	byUserLstErr error
	publicStocks []portfolio.Portfolio
	publicErr    error

	updatePublicErr   error
	updateReservedErr error

	updatedPublic   bool
	updatedReserved bool
}

func (s *stubPortfolio) Pool() *pgxpool.Pool { return nil }
func (s *stubPortfolio) FindByUserID(_ context.Context, _ portfolio.Querier, _ int64) ([]portfolio.Portfolio, error) {
	return s.byUser, s.byUserErr
}
func (s *stubPortfolio) FindByID(_ context.Context, _ portfolio.Querier, _ int64) (*portfolio.Portfolio, error) {
	return s.byID, s.byIDErr
}
func (s *stubPortfolio) FindByUserIDAndListingID(_ context.Context, _ portfolio.Querier, _, _ int64) (*portfolio.Portfolio, error) {
	return s.byUserList, s.byUserLstErr
}
func (s *stubPortfolio) UpdatePublic(_ context.Context, _ portfolio.Querier, _ int64, _ int, _ bool) error {
	s.updatedPublic = true
	return s.updatePublicErr
}
func (s *stubPortfolio) UpdateReservedAndPublic(_ context.Context, _ portfolio.Querier, _ int64, _, _ int) error {
	s.updatedReserved = true
	return s.updateReservedErr
}
func (s *stubPortfolio) FindAllPublicStocks(_ context.Context, _ portfolio.Querier) ([]portfolio.Portfolio, error) {
	return s.publicStocks, s.publicErr
}

type stubMarket struct {
	listings map[int64]*clients.StockListing
	err      error
}

func (s *stubMarket) GetListing(_ context.Context, id int64) (*clients.StockListing, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.listings[id], nil
}

type stubCustomer struct {
	customers map[int64]*clients.Customer
	err       error
}

func (s *stubCustomer) GetCustomer(_ context.Context, id int64) (*clients.Customer, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.customers[id], nil
}

type stubEmployee struct {
	ids []int64
}

func (s *stubEmployee) ActuaryClientIDs(_ context.Context) []int64 { return s.ids }

// capturePublisher records published saga events.
type capturePublisher struct {
	premium []PremiumTransferRequestedEvent
	exer    []ExerciseRequestedEvent
	err     error
}

func (p *capturePublisher) PublishPremiumTransferRequested(_ context.Context, e PremiumTransferRequestedEvent) error {
	p.premium = append(p.premium, e)
	return p.err
}
func (p *capturePublisher) PublishExerciseRequested(_ context.Context, e ExerciseRequestedEvent) error {
	p.exer = append(p.exer, e)
	return p.err
}

// captureNotifier records notifier calls.
type captureNotifier struct {
	NoopNotifier
	counter  int
	accepted int
	canceled []string
}

func (n *captureNotifier) CounterOffered(context.Context, *OtcOffer, int64) { n.counter++ }
func (n *captureNotifier) Accepted(context.Context, *OtcOffer, int64)       { n.accepted++ }
func (n *captureNotifier) Canceled(_ context.Context, _ *OtcOffer, _ int64, ev string) {
	n.canceled = append(n.canceled, ev)
}

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// spyNotifier records ExpiryReminder calls.
type spyNotifier struct {
	NoopNotifier
	reminders []*OptionContract
}

func (s *spyNotifier) ExpiryReminder(_ context.Context, c *OptionContract, _ int) {
	s.reminders = append(s.reminders, c)
}

func ptr[T any](v T) *T { return &v }

// newSvc builds a Service wired to stubs with the fake tx runner.
func newSvc(repo *stubRepo, pf *stubPortfolio, mk *stubMarket, cu *stubCustomer, em *stubEmployee,
	pub *capturePublisher, nt OtcNotifier) *Service {
	if mk == nil {
		mk = &stubMarket{}
	}
	if cu == nil {
		cu = &stubCustomer{}
	}
	if em == nil {
		em = &stubEmployee{}
	}
	if pub == nil {
		pub = &capturePublisher{}
	}
	if nt == nil {
		nt = NoopNotifier{}
	}
	return &Service{
		repo: repo, portfolio: pf, market: mk, customer: cu, employee: em,
		publisher: pub, notifier: nt, logger: discard(), runInTx: fakeTxRunner,
	}
}

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// ============================ tests =======================================

func TestCreateOffer(t *testing.T) {
	repo := &stubRepo{}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	in := CreateOfferInput{StockTicker: "AAPL", SellerID: 20, Amount: 5,
		PricePerStock: dec("100"), Premium: dec("5"), SettlementDate: time.Now()}
	dto, err := svc.CreateOffer(context.Background(), 10, in, ptr("Buyer"))
	if err != nil {
		t.Fatal(err)
	}
	if dto.ID != 100 || dto.BuyerID != 10 || dto.SellerID != 20 || dto.Status != OfferPendingSeller {
		t.Errorf("dto wrong: %+v", dto)
	}
	if len(repo.insertedHistory) != 1 || repo.insertedHistory[0].EventType != EventCreated {
		t.Errorf("history not recorded: %+v", repo.insertedHistory)
	}
}

func TestCreateOffer_InsertError(t *testing.T) {
	repo := &stubRepo{insertOfferErr: errors.New("boom")}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	_, err := svc.CreateOffer(context.Background(), 10, CreateOfferInput{PricePerStock: dec("0"), Premium: dec("0")}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCounterOffer_ByBuyer(t *testing.T) {
	repo := &stubRepo{offer: &OtcOffer{ID: 1, BuyerID: 10, SellerID: 20, Status: OfferPendingBuyer,
		PricePerStock: dec("100"), Premium: dec("5")}}
	nt := &captureNotifier{}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nt)
	dto, err := svc.CounterOffer(context.Background(), 1, 10, CounterOfferInput{Amount: 7,
		PricePerStock: dec("110"), Premium: dec("6"), SettlementDate: time.Now()}, ptr("Buyer"))
	if err != nil {
		t.Fatal(err)
	}
	if dto.Status != OfferPendingSeller {
		t.Errorf("buyer counter should flip to PENDING_SELLER, got %s", dto.Status)
	}
	if nt.counter != 1 {
		t.Errorf("CounterOffered notification not fired")
	}
}

func TestCounterOffer_BySeller(t *testing.T) {
	repo := &stubRepo{offer: &OtcOffer{ID: 1, BuyerID: 10, SellerID: 20, Status: OfferPendingSeller,
		PricePerStock: dec("100"), Premium: dec("5")}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	dto, err := svc.CounterOffer(context.Background(), 1, 20, CounterOfferInput{PricePerStock: dec("1"), Premium: dec("1")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dto.Status != OfferPendingBuyer {
		t.Errorf("seller counter should flip to PENDING_BUYER, got %s", dto.Status)
	}
}

func TestCounterOffer_NotParticipant(t *testing.T) {
	repo := &stubRepo{offer: &OtcOffer{ID: 1, BuyerID: 10, SellerID: 20, Status: OfferPendingSeller,
		PricePerStock: dec("0"), Premium: dec("0")}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	_, err := svc.CounterOffer(context.Background(), 1, 99, CounterOfferInput{PricePerStock: dec("0"), Premium: dec("0")}, nil)
	if err == nil {
		t.Fatal("expected conflict for non-participant")
	}
}

func TestCounterOffer_FinalStatus(t *testing.T) {
	for _, st := range []string{OfferAccepted, OfferRejected, OfferExpired} {
		repo := &stubRepo{offer: &OtcOffer{ID: 1, BuyerID: 10, SellerID: 20, Status: st,
			PricePerStock: dec("0"), Premium: dec("0")}}
		svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
		_, err := svc.CounterOffer(context.Background(), 1, 10, CounterOfferInput{PricePerStock: dec("0"), Premium: dec("0")}, nil)
		if err == nil {
			t.Errorf("status %s should block counter", st)
		}
	}
}

func TestCounterOffer_NotFound(t *testing.T) {
	repo := &stubRepo{offer: nil}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	_, err := svc.CounterOffer(context.Background(), 1, 10, CounterOfferInput{PricePerStock: dec("0"), Premium: dec("0")}, nil)
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestAccept_Success(t *testing.T) {
	ticker := "AAPL"
	repo := &stubRepo{
		offer:     &OtcOffer{ID: 1, BuyerID: 10, SellerID: 20, Amount: 5, Status: OfferPendingSeller, StockTicker: ticker, PricePerStock: dec("100"), Premium: dec("5")},
		sumActive: 0,
	}
	pf := &stubPortfolio{
		byUser: []portfolio.Portfolio{{ID: 9, UserID: 20, ListingID: 1, Quantity: 100, IsPublic: true, PublicQuantity: 50, AveragePurchasePrice: dec("0")}},
	}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{1: {ID: 1, Ticker: &ticker}}}
	pub := &capturePublisher{}
	nt := &captureNotifier{}
	svc := newSvc(repo, pf, mk, nil, nil, pub, nt)
	dto, err := svc.Accept(context.Background(), 1, 20, ptr("Seller"))
	if err != nil {
		t.Fatal(err)
	}
	if dto.Status != OfferAccepted {
		t.Errorf("status = %s want ACCEPTED", dto.Status)
	}
	if repo.insertedContract == nil || repo.insertedContract.Status != ContractPendingPremium {
		t.Errorf("contract not inserted PENDING_PREMIUM: %+v", repo.insertedContract)
	}
	if len(pub.premium) != 1 || pub.premium[0].ContractID != 555 {
		t.Errorf("premium saga not published: %+v", pub.premium)
	}
	if nt.accepted != 1 {
		t.Errorf("Accepted notification not fired")
	}
}

func TestAccept_InvariantViolation(t *testing.T) {
	ticker := "AAPL"
	repo := &stubRepo{
		offer:     &OtcOffer{ID: 1, BuyerID: 10, SellerID: 20, Amount: 60, Status: OfferPendingSeller, StockTicker: ticker, PricePerStock: dec("0"), Premium: dec("0")},
		sumActive: 0,
	}
	pf := &stubPortfolio{
		byUser: []portfolio.Portfolio{{ID: 9, UserID: 20, ListingID: 1, Quantity: 50, AveragePurchasePrice: dec("0")}},
	}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{1: {ID: 1, Ticker: &ticker}}}
	svc := newSvc(repo, pf, mk, nil, nil, nil, nil)
	_, err := svc.Accept(context.Background(), 1, 20, nil)
	if err == nil {
		t.Fatal("expected invariant violation (400)")
	}
}

func TestAccept_WrongTurn(t *testing.T) {
	repo := &stubRepo{offer: &OtcOffer{ID: 1, BuyerID: 10, SellerID: 20, Status: OfferPendingSeller, PricePerStock: dec("0"), Premium: dec("0")}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	// buyer tries to accept while it is the seller's turn
	_, err := svc.Accept(context.Background(), 1, 10, nil)
	if err == nil {
		t.Fatal("buyer accepting on seller's turn should error")
	}
}

func TestAccept_NotActive(t *testing.T) {
	repo := &stubRepo{offer: &OtcOffer{ID: 1, BuyerID: 10, SellerID: 20, Status: OfferAccepted, PricePerStock: dec("0"), Premium: dec("0")}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	_, err := svc.Accept(context.Background(), 1, 20, nil)
	if err == nil {
		t.Fatal("accepting a non-active offer should error")
	}
}

func TestReject(t *testing.T) {
	repo := &stubRepo{offer: &OtcOffer{ID: 1, BuyerID: 10, SellerID: 20, Status: OfferPendingSeller, PricePerStock: dec("0"), Premium: dec("0")}}
	nt := &captureNotifier{}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nt)
	dto, err := svc.Reject(context.Background(), 1, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dto.Status != OfferRejected {
		t.Errorf("status = %s want REJECTED", dto.Status)
	}
	if len(nt.canceled) != 1 || nt.canceled[0] != "REJECTED" {
		t.Errorf("canceled notification wrong: %v", nt.canceled)
	}
}

func TestReject_NotParticipant(t *testing.T) {
	repo := &stubRepo{offer: &OtcOffer{ID: 1, BuyerID: 10, SellerID: 20, Status: OfferPendingSeller, PricePerStock: dec("0"), Premium: dec("0")}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	_, err := svc.Reject(context.Background(), 1, 99, nil)
	if err == nil {
		t.Fatal("non-participant reject should error")
	}
}

func TestWithdraw_Buyer(t *testing.T) {
	repo := &stubRepo{offer: &OtcOffer{ID: 1, BuyerID: 10, SellerID: 20, Status: OfferPendingSeller, PricePerStock: dec("0"), Premium: dec("0")}}
	nt := &captureNotifier{}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nt)
	dto, err := svc.Withdraw(context.Background(), 1, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dto.Status != OfferWithdrawn {
		t.Errorf("status = %s want WITHDRAWN", dto.Status)
	}
	if len(nt.canceled) != 1 || nt.canceled[0] != "WITHDRAWN" {
		t.Errorf("canceled notification wrong: %v", nt.canceled)
	}
}

func TestWithdraw_BuyerWrongStatus(t *testing.T) {
	repo := &stubRepo{offer: &OtcOffer{ID: 1, BuyerID: 10, SellerID: 20, Status: OfferPendingBuyer, PricePerStock: dec("0"), Premium: dec("0")}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	_, err := svc.Withdraw(context.Background(), 1, 10, nil)
	if err == nil {
		t.Fatal("buyer can only withdraw while PENDING_SELLER")
	}
}

func TestWithdraw_Seller(t *testing.T) {
	repo := &stubRepo{offer: &OtcOffer{ID: 1, BuyerID: 10, SellerID: 20, Status: OfferPendingBuyer, PricePerStock: dec("0"), Premium: dec("0")}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	dto, err := svc.Withdraw(context.Background(), 1, 20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dto.Status != OfferWithdrawn {
		t.Errorf("status = %s want WITHDRAWN", dto.Status)
	}
}

func TestWithdraw_NotParticipant(t *testing.T) {
	repo := &stubRepo{offer: &OtcOffer{ID: 1, BuyerID: 10, SellerID: 20, Status: OfferPendingSeller, PricePerStock: dec("0"), Premium: dec("0")}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	_, err := svc.Withdraw(context.Background(), 1, 99, nil)
	if err == nil {
		t.Fatal("non-participant withdraw should error")
	}
}

func TestActiveForUser(t *testing.T) {
	repo := &stubRepo{activeOffers: []OtcOffer{
		{ID: 1, Status: OfferPendingSeller, PricePerStock: dec("1"), Premium: dec("1")},
		{ID: 2, Status: OfferPendingBuyer, PricePerStock: dec("2"), Premium: dec("2")},
	}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	out, err := svc.ActiveForUser(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Errorf("len = %d want 2", len(out))
	}
}

func TestActiveForUser_Error(t *testing.T) {
	repo := &stubRepo{activeOffersErr: errors.New("db")}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	if _, err := svc.ActiveForUser(context.Background(), 10); err == nil {
		t.Fatal("expected error")
	}
}

func TestMyContracts_NoFilterDedup(t *testing.T) {
	repo := &stubRepo{
		buyerContracts:  []OptionContract{{ID: 1, Status: ContractActive, PricePerStock: dec("1")}},
		sellerContracts: []OptionContract{{ID: 1, Status: ContractActive, PricePerStock: dec("1")}},
	}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	out, err := svc.MyContracts(context.Background(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	// id 1 deduped across buyer+seller, returned once per status loop -> 1 unique
	if len(out) != 1 {
		t.Errorf("expected dedup to 1, got %d", len(out))
	}
}

func TestMyContracts_WithFilter(t *testing.T) {
	repo := &stubRepo{
		buyerContracts: []OptionContract{{ID: 7, Status: ContractActive, PricePerStock: dec("1")}},
	}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	out, err := svc.MyContracts(context.Background(), 10, ptr(ContractActive))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].ID != 7 {
		t.Errorf("filter result wrong: %+v", out)
	}
}

func TestMyContracts_Error(t *testing.T) {
	repo := &stubRepo{contractsErr: errors.New("db")}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	if _, err := svc.MyContracts(context.Background(), 10, ptr(ContractActive)); err == nil {
		t.Fatal("expected error")
	}
}

func TestExerciseContract_Success(t *testing.T) {
	repo := &stubRepo{contract: &OptionContract{ID: 5, BuyerID: 10, SellerID: 20, Status: ContractActive,
		StockTicker: "AAPL", Amount: 3, PricePerStock: dec("100"),
		SettlementDate: time.Now().AddDate(0, 0, 10)}}
	pub := &capturePublisher{}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, pub, nil)
	id, err := svc.ExerciseContract(context.Background(), 5, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if id != 5 {
		t.Errorf("id = %d want 5", id)
	}
	if !repo.exercisedAtCalled {
		t.Error("exercised_at not stamped")
	}
	if len(pub.exer) != 1 || pub.exer[0].ContractID != 5 {
		t.Errorf("exercise saga not published: %+v", pub.exer)
	}
}

func TestExerciseContract_NotFound(t *testing.T) {
	repo := &stubRepo{contract: nil}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	_, err := svc.ExerciseContract(context.Background(), 5, 10, nil)
	if err == nil {
		t.Fatal("expected not found")
	}
}

func TestExerciseContract_NotBuyer(t *testing.T) {
	repo := &stubRepo{contract: &OptionContract{ID: 5, BuyerID: 10, Status: ContractActive,
		PricePerStock: dec("0"), SettlementDate: time.Now().AddDate(0, 0, 10)}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	_, err := svc.ExerciseContract(context.Background(), 5, 99, nil)
	if err == nil {
		t.Fatal("non-buyer exercise should error")
	}
}

func TestExerciseContract_NotActive(t *testing.T) {
	repo := &stubRepo{contract: &OptionContract{ID: 5, BuyerID: 10, Status: ContractPendingPremium,
		PricePerStock: dec("0"), SettlementDate: time.Now().AddDate(0, 0, 10)}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	_, err := svc.ExerciseContract(context.Background(), 5, 10, nil)
	if err == nil {
		t.Fatal("non-active exercise should error")
	}
}

func TestExerciseContract_PastSettlement(t *testing.T) {
	repo := &stubRepo{contract: &OptionContract{ID: 5, BuyerID: 10, Status: ContractActive,
		PricePerStock: dec("0"), SettlementDate: time.Now().AddDate(0, 0, -1)}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	_, err := svc.ExerciseContract(context.Background(), 5, 10, nil)
	if err == nil {
		t.Fatal("past-settlement exercise should error")
	}
}

func TestCompletePremiumTransfer(t *testing.T) {
	repo := &stubRepo{contract: &OptionContract{ID: 5, Status: ContractPendingPremium, PricePerStock: dec("0")}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	if err := svc.CompletePremiumTransfer(context.Background(), 5); err != nil {
		t.Fatal(err)
	}
	if len(repo.updatedStatuses) != 1 || repo.updatedStatuses[0] != ContractActive {
		t.Errorf("status not flipped to ACTIVE: %v", repo.updatedStatuses)
	}
}

func TestCompletePremiumTransfer_NotPending_NoOp(t *testing.T) {
	repo := &stubRepo{contract: &OptionContract{ID: 5, Status: ContractActive, PricePerStock: dec("0")}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	if err := svc.CompletePremiumTransfer(context.Background(), 5); err != nil {
		t.Fatal(err)
	}
	if len(repo.updatedStatuses) != 0 {
		t.Errorf("should be no-op, got %v", repo.updatedStatuses)
	}
}

func TestCompletePremiumTransfer_NotFound_NoOp(t *testing.T) {
	repo := &stubRepo{contract: nil}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	if err := svc.CompletePremiumTransfer(context.Background(), 5); err != nil {
		t.Fatal("not found should be a no-op, not an error")
	}
}

func TestFailPremiumTransfer(t *testing.T) {
	ticker := "AAPL"
	repo := &stubRepo{contract: &OptionContract{ID: 5, SellerID: 20, StockTicker: ticker, Amount: 3, Status: ContractPendingPremium, PricePerStock: dec("0")}}
	pf := &stubPortfolio{byUser: []portfolio.Portfolio{{ID: 9, UserID: 20, ListingID: 1, Quantity: 10, ReservedQuantity: 3, AveragePurchasePrice: dec("0")}}}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{1: {ID: 1, Ticker: &ticker}}}
	svc := newSvc(repo, pf, mk, nil, nil, nil, nil)
	if err := svc.FailPremiumTransfer(context.Background(), 5, "rejected"); err != nil {
		t.Fatal(err)
	}
	if len(repo.updatedStatuses) != 1 || repo.updatedStatuses[0] != ContractCanceled {
		t.Errorf("status not CANCELED: %v", repo.updatedStatuses)
	}
	if !pf.updatedReserved {
		t.Error("reserved stock not released")
	}
}

func TestFailPremiumTransfer_NotPending_NoOp(t *testing.T) {
	repo := &stubRepo{contract: &OptionContract{ID: 5, Status: ContractActive, PricePerStock: dec("0")}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	if err := svc.FailPremiumTransfer(context.Background(), 5, "x"); err != nil {
		t.Fatal(err)
	}
	if len(repo.updatedStatuses) != 0 {
		t.Errorf("should be no-op: %v", repo.updatedStatuses)
	}
}

func TestCompleteExercise(t *testing.T) {
	repo := &stubRepo{contract: &OptionContract{ID: 5, Status: ContractActive, PricePerStock: dec("0")}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	if err := svc.CompleteExercise(context.Background(), 5); err != nil {
		t.Fatal(err)
	}
	if len(repo.updatedStatuses) != 1 || repo.updatedStatuses[0] != ContractExercised {
		t.Errorf("status not EXERCISED: %v", repo.updatedStatuses)
	}
}

func TestCompleteExercise_NotActive_NoOp(t *testing.T) {
	repo := &stubRepo{contract: &OptionContract{ID: 5, Status: ContractExercised, PricePerStock: dec("0")}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	if err := svc.CompleteExercise(context.Background(), 5); err != nil {
		t.Fatal(err)
	}
	if len(repo.updatedStatuses) != 0 {
		t.Errorf("should be no-op: %v", repo.updatedStatuses)
	}
}

func TestRevertExercise_NotActive_NoOp(t *testing.T) {
	repo := &stubRepo{contract: &OptionContract{ID: 5, Status: ContractExercised, PricePerStock: dec("0")}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	// status != ACTIVE -> early return before tx.Exec, so nil tx is never touched
	if err := svc.RevertExercise(context.Background(), 5); err != nil {
		t.Fatal(err)
	}
}

func TestRevertExercise_NotFound_NoOp(t *testing.T) {
	repo := &stubRepo{contract: nil}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	if err := svc.RevertExercise(context.Background(), 5); err != nil {
		t.Fatal("not found should be no-op")
	}
}

func TestGetMyPositions(t *testing.T) {
	ticker := "AAPL"
	pf := &stubPortfolio{byUser: []portfolio.Portfolio{
		{ID: 1, ListingID: 1, ListingType: "STOCK", IsPublic: true, Quantity: 10, PublicQuantity: 5, AveragePurchasePrice: dec("0")},
		{ID: 2, ListingID: 2, ListingType: "STOCK", IsPublic: false, Quantity: 10, AveragePurchasePrice: dec("0")}, // not public -> excluded
		{ID: 3, ListingID: 3, ListingType: "OPTION", IsPublic: true, AveragePurchasePrice: dec("0")},               // not stock -> excluded
	}}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{1: {ID: 1, Ticker: &ticker}}}
	svc := newSvc(&stubRepo{}, pf, mk, nil, nil, nil, nil)
	out, err := svc.GetMyPositions(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].ID != 1 {
		t.Errorf("expected only the public stock position: %+v", out)
	}
}

func TestAddPosition(t *testing.T) {
	pf := &stubPortfolio{byUserList: &portfolio.Portfolio{ID: 9, UserID: 10, ListingType: "STOCK", Quantity: 10, ReservedQuantity: 2, AveragePurchasePrice: dec("0")}}
	svc := newSvc(&stubRepo{}, pf, nil, nil, nil, nil, nil)
	dto, err := svc.AddPosition(context.Background(), 10, 1, 5)
	if err != nil {
		t.Fatal(err)
	}
	if dto.PublicQuantity != 5 {
		t.Errorf("public quantity = %d want 5", dto.PublicQuantity)
	}
	if !pf.updatedPublic {
		t.Error("UpdatePublic not called")
	}
}

func TestAddPosition_NotFound(t *testing.T) {
	pf := &stubPortfolio{byUserList: nil}
	svc := newSvc(&stubRepo{}, pf, nil, nil, nil, nil, nil)
	if _, err := svc.AddPosition(context.Background(), 10, 1, 5); err == nil {
		t.Fatal("expected not found")
	}
}

func TestAddPosition_NotStock(t *testing.T) {
	pf := &stubPortfolio{byUserList: &portfolio.Portfolio{ID: 9, UserID: 10, ListingType: "OPTION", AveragePurchasePrice: dec("0")}}
	svc := newSvc(&stubRepo{}, pf, nil, nil, nil, nil, nil)
	if _, err := svc.AddPosition(context.Background(), 10, 1, 5); err == nil {
		t.Fatal("non-stock position should error")
	}
}

func TestAddPosition_TooMuch(t *testing.T) {
	pf := &stubPortfolio{byUserList: &portfolio.Portfolio{ID: 9, UserID: 10, ListingType: "STOCK", Quantity: 10, ReservedQuantity: 8, AveragePurchasePrice: dec("0")}}
	svc := newSvc(&stubRepo{}, pf, nil, nil, nil, nil, nil)
	if _, err := svc.AddPosition(context.Background(), 10, 1, 5); err == nil {
		t.Fatal("exposing more than available should error")
	}
}

func TestUpdatePosition(t *testing.T) {
	pf := &stubPortfolio{byID: &portfolio.Portfolio{ID: 9, UserID: 10, Quantity: 20, ReservedQuantity: 3, IsPublic: true, AveragePurchasePrice: dec("0")}}
	svc := newSvc(&stubRepo{}, pf, nil, nil, nil, nil, nil)
	dto, err := svc.UpdatePosition(context.Background(), 10, 9, 10)
	if err != nil {
		t.Fatal(err)
	}
	if dto.PublicQuantity != 10 {
		t.Errorf("public quantity = %d want 10", dto.PublicQuantity)
	}
}

func TestUpdatePosition_BelowReserved(t *testing.T) {
	pf := &stubPortfolio{byID: &portfolio.Portfolio{ID: 9, UserID: 10, Quantity: 20, ReservedQuantity: 5, AveragePurchasePrice: dec("0")}}
	svc := newSvc(&stubRepo{}, pf, nil, nil, nil, nil, nil)
	if _, err := svc.UpdatePosition(context.Background(), 10, 9, 3); err == nil {
		t.Fatal("dropping below reserved should error")
	}
}

func TestUpdatePosition_NotOwned(t *testing.T) {
	pf := &stubPortfolio{byID: &portfolio.Portfolio{ID: 9, UserID: 99, AveragePurchasePrice: dec("0")}}
	svc := newSvc(&stubRepo{}, pf, nil, nil, nil, nil, nil)
	if _, err := svc.UpdatePosition(context.Background(), 10, 9, 1); err == nil {
		t.Fatal("position owned by another user should error")
	}
}

func TestRemovePosition(t *testing.T) {
	pf := &stubPortfolio{byID: &portfolio.Portfolio{ID: 9, UserID: 10, ReservedQuantity: 0, AveragePurchasePrice: dec("0")}}
	svc := newSvc(&stubRepo{}, pf, nil, nil, nil, nil, nil)
	if err := svc.RemovePosition(context.Background(), 10, 9); err != nil {
		t.Fatal(err)
	}
	if !pf.updatedPublic {
		t.Error("UpdatePublic not called on remove")
	}
}

func TestRemovePosition_Reserved(t *testing.T) {
	pf := &stubPortfolio{byID: &portfolio.Portfolio{ID: 9, UserID: 10, ReservedQuantity: 3, AveragePurchasePrice: dec("0")}}
	svc := newSvc(&stubRepo{}, pf, nil, nil, nil, nil, nil)
	if err := svc.RemovePosition(context.Background(), 10, 9); err == nil {
		t.Fatal("removing with reserved shares should error")
	}
}

func TestGetPublicStocks_ExcludesOwn(t *testing.T) {
	ticker := "AAPL"
	pf := &stubPortfolio{publicStocks: []portfolio.Portfolio{
		{UserID: 10, ListingID: 1, PublicQuantity: 5, AveragePurchasePrice: dec("0")}, // own -> excluded
		{UserID: 20, ListingID: 1, PublicQuantity: 7, AveragePurchasePrice: dec("0")},
	}}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{1: {ID: 1, Ticker: &ticker}}}
	cu := &stubCustomer{customers: map[int64]*clients.Customer{20: {ID: 20, FirstName: ptr("A"), LastName: ptr("B")}}}
	svc := newSvc(&stubRepo{}, pf, mk, cu, nil, nil, nil)
	out, err := svc.GetPublicStocks(context.Background(), 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Ticker != "AAPL" || len(out[0].Sellers) != 1 || out[0].Sellers[0].SellerID != 20 {
		t.Errorf("public stocks wrong: %+v", out)
	}
}

func TestGetPublicStocks_SupervisorView(t *testing.T) {
	ticker := "AAPL"
	pf := &stubPortfolio{publicStocks: []portfolio.Portfolio{
		{UserID: 20, ListingID: 1, PublicQuantity: 7, AveragePurchasePrice: dec("0")}, // actuary client
		{UserID: 30, ListingID: 1, PublicQuantity: 7, AveragePurchasePrice: dec("0")}, // not actuary -> excluded
	}}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{1: {ID: 1, Ticker: &ticker}}}
	em := &stubEmployee{ids: []int64{20}}
	svc := newSvc(&stubRepo{}, pf, mk, &stubCustomer{}, em, nil, nil)
	out, err := svc.GetPublicStocks(context.Background(), 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || len(out[0].Sellers) != 1 || out[0].Sellers[0].SellerID != 20 {
		t.Errorf("supervisor view should only show actuary stocks: %+v", out)
	}
}

func TestHistoryForUser(t *testing.T) {
	repo := &stubRepo{historyRows: []NegotiationHistory{
		{ID: 1, OfferID: 2, EventType: EventCreated},
	}}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	out, err := svc.HistoryForUser(context.Background(), 10, nil, nil, &from, &to)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].ID != 1 {
		t.Errorf("history wrong: %+v", out)
	}
}

func TestExpireOverdueContracts(t *testing.T) {
	ticker := "AAPL"
	repo := &stubRepo{staleContracts: []OptionContract{
		{ID: 1, SellerID: 20, StockTicker: ticker, Amount: 3, Status: ContractActive, PricePerStock: dec("0"),
			SettlementDate: time.Now().AddDate(0, 0, -2)},
	}}
	pf := &stubPortfolio{byUser: []portfolio.Portfolio{{ID: 9, UserID: 20, ListingID: 1, Quantity: 10, ReservedQuantity: 3, AveragePurchasePrice: dec("0")}}}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{1: {ID: 1, Ticker: &ticker}}}
	svc := newSvc(repo, pf, mk, nil, nil, nil, nil)
	if err := svc.ExpireOverdueContracts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.updatedStatuses) != 1 || repo.updatedStatuses[0] != ContractExpired {
		t.Errorf("contract not expired: %v", repo.updatedStatuses)
	}
}

func TestExpireOverdueContracts_NothingToExpire(t *testing.T) {
	repo := &stubRepo{staleContracts: nil}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	if err := svc.ExpireOverdueContracts(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestExpireOverdueContracts_FindError(t *testing.T) {
	repo := &stubRepo{staleErr: errors.New("db")}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	if err := svc.ExpireOverdueContracts(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestSendExpiryReminders(t *testing.T) {
	spy := &spyNotifier{}
	repo := &stubRepo{
		reminderContracts: []OptionContract{{ID: 1, Status: ContractActive, PricePerStock: dec("0")}},
		reminderInserted:  true,
	}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, spy)
	if err := svc.SendExpiryReminders(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	if len(spy.reminders) != 1 {
		t.Errorf("reminder not sent: %d", len(spy.reminders))
	}
}

func TestSendExpiryReminders_AlreadySent(t *testing.T) {
	spy := &spyNotifier{}
	repo := &stubRepo{
		reminderContracts: []OptionContract{{ID: 1, Status: ContractActive, PricePerStock: dec("0")}},
		reminderInserted:  false, // marker already existed
	}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, spy)
	if err := svc.SendExpiryReminders(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	if len(spy.reminders) != 0 {
		t.Errorf("reminder should be skipped when marker exists: %d", len(spy.reminders))
	}
}

func TestSendExpiryReminders_FindError(t *testing.T) {
	repo := &stubRepo{reminderErr: errors.New("db")}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	if err := svc.SendExpiryReminders(context.Background(), 3); err == nil {
		t.Fatal("expected error")
	}
}

func TestTxBeginFailure(t *testing.T) {
	repo := &stubRepo{}
	svc := newSvc(repo, &stubPortfolio{}, nil, nil, nil, nil, nil)
	svc.runInTx = failTxRunner
	_, err := svc.CreateOffer(context.Background(), 10, CreateOfferInput{PricePerStock: dec("0"), Premium: dec("0")}, nil)
	if err == nil {
		t.Fatal("tx begin failure should propagate")
	}
}

func TestResolveClientName(t *testing.T) {
	cu := &stubCustomer{customers: map[int64]*clients.Customer{
		10: {ID: 10, FirstName: ptr("Jovan"), LastName: ptr("Jovanovic")},
	}}
	svc := newSvc(&stubRepo{}, &stubPortfolio{}, nil, cu, nil, nil, nil)
	name := svc.resolveClientName(context.Background(), 10)
	if name == nil || *name != "Jovan Jovanovic" {
		t.Errorf("name = %v want 'Jovan Jovanovic'", name)
	}
	// missing customer -> nil
	if svc.resolveClientName(context.Background(), 99) != nil {
		t.Error("missing customer should give nil name")
	}
}

func TestResolveTicker(t *testing.T) {
	ticker := "AAPL"
	mk := &stubMarket{listings: map[int64]*clients.StockListing{1: {ID: 1, Ticker: &ticker}}}
	svc := newSvc(&stubRepo{}, &stubPortfolio{}, mk, nil, nil, nil, nil)
	if got := svc.resolveTicker(context.Background(), 1); got != "AAPL" {
		t.Errorf("ticker = %q want AAPL", got)
	}
	if got := svc.resolveTicker(context.Background(), 99); got != "" {
		t.Errorf("missing listing should give empty ticker, got %q", got)
	}
}
