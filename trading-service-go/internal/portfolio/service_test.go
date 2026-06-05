package portfolio

import (
	"context"
	"errors"
	"testing"
	"time"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/clients"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// ---- stubs -----------------------------------------------------------------

type stubRepo struct {
	byUser      []Portfolio
	byUserErr   error
	byID        *Portfolio
	byIDErr     error
	byUL        *Portfolio
	byULErr     error
	updates     []string
	updatePubEr error
}

func (s *stubRepo) Pool() *pgxpool.Pool { return nil }
func (s *stubRepo) FindByUserID(_ context.Context, _ Querier, _ int64) ([]Portfolio, error) {
	return s.byUser, s.byUserErr
}
func (s *stubRepo) FindByID(_ context.Context, _ Querier, _ int64) (*Portfolio, error) {
	return s.byID, s.byIDErr
}
func (s *stubRepo) FindByUserIDAndListingID(_ context.Context, _ Querier, _, _ int64) (*Portfolio, error) {
	return s.byUL, s.byULErr
}
func (s *stubRepo) UpdatePublic(_ context.Context, _ Querier, _ int64, _ int, _ bool) error {
	s.updates = append(s.updates, "public")
	return s.updatePubEr
}
func (s *stubRepo) Insert(_ context.Context, _ Querier, _, _ int64, _ string, _ int, _ decimal.Decimal) error {
	s.updates = append(s.updates, "insert")
	return nil
}
func (s *stubRepo) UpdateQuantityAndAvg(_ context.Context, _ Querier, _ int64, _ int, _ decimal.Decimal) error {
	s.updates = append(s.updates, "qtyavg")
	return nil
}
func (s *stubRepo) UpdateQuantity(_ context.Context, _ Querier, _ int64, _ int) error {
	s.updates = append(s.updates, "qty")
	return nil
}
func (s *stubRepo) Delete(_ context.Context, _ Querier, _ int64) error {
	s.updates = append(s.updates, "delete")
	return nil
}

type stubMarket struct {
	listings map[int64]*clients.StockListing
	listErr  error
	rate     *clients.ExchangeRate
}

func (s *stubMarket) GetListing(_ context.Context, id int64) (*clients.StockListing, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.listings[id], nil
}
func (s *stubMarket) Calculate(_ context.Context, _, _ string, _ decimal.Decimal) (*clients.ExchangeRate, error) {
	return s.rate, nil
}

type stubAccount struct {
	bank   *clients.BankAccount
	detail *clients.AccountDetails
	gov    *clients.AccountDetails
	txErr  error
	txDone bool
}

func (s *stubAccount) GetBankAccount(_ context.Context, _ string) (*clients.BankAccount, error) {
	return s.bank, nil
}
func (s *stubAccount) GetAccountDetailsByID(_ context.Context, _ int64) (*clients.AccountDetails, error) {
	return s.detail, nil
}
func (s *stubAccount) GetGovernmentBankAccountRsd(_ context.Context) (*clients.AccountDetails, error) {
	return s.gov, nil
}
func (s *stubAccount) Transaction(_ context.Context, _ clients.Payment) error {
	s.txDone = true
	return s.txErr
}

type stubTax struct {
	paid   decimal.Decimal
	due    decimal.Decimal
	paidEr error
	dueEr  error
}

func (s stubTax) CurrentYearPaidTax(_ context.Context, _ int64) (decimal.Decimal, error) {
	return s.paid, s.paidEr
}
func (s stubTax) CurrentMonthUnpaidTax(_ context.Context, _ int64) (decimal.Decimal, error) {
	return s.due, s.dueEr
}

func newSvc(repo *stubRepo, mk *stubMarket, acc *stubAccount, tx TaxReporter) *Service {
	if mk == nil {
		mk = &stubMarket{}
	}
	if acc == nil {
		acc = &stubAccount{}
	}
	if tx == nil {
		tx = stubTax{paid: decimal.Zero, due: decimal.Zero}
	}
	return &Service{
		repo: repo, market: mk, account: acc, tax: tx,
		runInTx: func(ctx context.Context, fn func(pgx.Tx) error) error { return fn(nil) },
	}
}

func ptr[T any](v T) *T          { return &v }
func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// ---- GetPortfolio ----------------------------------------------------------

func TestGetPortfolio(t *testing.T) {
	ticker := "AAPL"
	repo := &stubRepo{byUser: []Portfolio{
		{ID: 1, UserID: 10, ListingID: 100, ListingType: "STOCK", Quantity: 5, AveragePurchasePrice: d("100")},
	}}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{100: {ID: 100, Ticker: &ticker, Price: ptr(d("120"))}}}
	svc := newSvc(repo, mk, nil, stubTax{paid: d("50"), due: d("10")})
	out, err := svc.GetPortfolio(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Holdings) != 1 {
		t.Fatalf("holdings = %d", len(out.Holdings))
	}
	// profit = (120-100)*5 = 100
	if !out.TotalProfit.Equal(d("100")) {
		t.Errorf("totalProfit = %s want 100", out.TotalProfit)
	}
	if !out.YearlyTaxPaid.Equal(d("50")) || !out.MonthlyTaxDue.Equal(d("10")) {
		t.Errorf("tax fields wrong: %+v", out)
	}
}

func TestGetPortfolio_RepoError(t *testing.T) {
	svc := newSvc(&stubRepo{byUserErr: errors.New("db")}, nil, nil, nil)
	if _, err := svc.GetPortfolio(context.Background(), 10); err == nil {
		t.Error("expected error")
	}
}

func TestGetPortfolio_ListingError(t *testing.T) {
	repo := &stubRepo{byUser: []Portfolio{{ID: 1, ListingID: 100, AveragePurchasePrice: d("1")}}}
	svc := newSvc(repo, &stubMarket{listErr: errors.New("market")}, nil, nil)
	if _, err := svc.GetPortfolio(context.Background(), 10); err == nil {
		t.Error("expected listing error")
	}
}

func TestGetPortfolio_TaxError(t *testing.T) {
	ticker := "AAPL"
	repo := &stubRepo{byUser: []Portfolio{{ID: 1, ListingID: 100, AveragePurchasePrice: d("1")}}}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{100: {ID: 100, Ticker: &ticker, Price: ptr(d("2"))}}}
	svc := newSvc(repo, mk, nil, stubTax{paidEr: errors.New("tax")})
	if _, err := svc.GetPortfolio(context.Background(), 10); err == nil {
		t.Error("expected tax error")
	}
}

// ---- SetPublicQuantity -----------------------------------------------------

func TestSetPublicQuantity(t *testing.T) {
	repo := &stubRepo{byID: &Portfolio{ID: 1, UserID: 10, ListingType: "STOCK", Quantity: 10, ReservedQuantity: 2, AveragePurchasePrice: d("1")}}
	svc := newSvc(repo, nil, nil, nil)
	err := svc.SetPublicQuantity(context.Background(), 10, 1, api.SetPublicQuantityRequest{PublicQuantity: ptr(5)})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetPublicQuantity_NotStock(t *testing.T) {
	repo := &stubRepo{byID: &Portfolio{ID: 1, UserID: 10, ListingType: "OPTION", AveragePurchasePrice: d("1")}}
	svc := newSvc(repo, nil, nil, nil)
	if err := svc.SetPublicQuantity(context.Background(), 10, 1, api.SetPublicQuantityRequest{PublicQuantity: ptr(1)}); err == nil {
		t.Error("non-stock should error")
	}
}

func TestSetPublicQuantity_Negative(t *testing.T) {
	repo := &stubRepo{byID: &Portfolio{ID: 1, UserID: 10, ListingType: "STOCK", Quantity: 10, AveragePurchasePrice: d("1")}}
	svc := newSvc(repo, nil, nil, nil)
	if err := svc.SetPublicQuantity(context.Background(), 10, 1, api.SetPublicQuantityRequest{PublicQuantity: ptr(-1)}); err == nil {
		t.Error("negative should error")
	}
}

func TestSetPublicQuantity_ExceedsAvailable(t *testing.T) {
	repo := &stubRepo{byID: &Portfolio{ID: 1, UserID: 10, ListingType: "STOCK", Quantity: 10, ReservedQuantity: 8, AveragePurchasePrice: d("1")}}
	svc := newSvc(repo, nil, nil, nil)
	if err := svc.SetPublicQuantity(context.Background(), 10, 1, api.SetPublicQuantityRequest{PublicQuantity: ptr(5)}); err == nil {
		t.Error("exceeding available should error")
	}
}

func TestSetPublicQuantity_NotFound(t *testing.T) {
	svc := newSvc(&stubRepo{byID: nil}, nil, nil, nil)
	if err := svc.SetPublicQuantity(context.Background(), 10, 1, api.SetPublicQuantityRequest{PublicQuantity: ptr(1)}); err == nil {
		t.Error("not found should error")
	}
}

func TestSetPublicQuantity_NotOwned(t *testing.T) {
	repo := &stubRepo{byID: &Portfolio{ID: 1, UserID: 99, ListingType: "STOCK", AveragePurchasePrice: d("1")}}
	svc := newSvc(repo, nil, nil, nil)
	if err := svc.SetPublicQuantity(context.Background(), 10, 1, api.SetPublicQuantityRequest{PublicQuantity: ptr(1)}); err == nil {
		t.Error("not owned should error")
	}
}

// ---- ExerciseOption --------------------------------------------------------

func futureDate() string { return time.Now().AddDate(0, 0, 10).Format("2006-01-02") }

func TestExerciseOption_NotAgent(t *testing.T) {
	svc := newSvc(&stubRepo{}, nil, nil, nil)
	if err := svc.ExerciseOption(context.Background(), 10, false, 1); err == nil {
		t.Error("non-agent should be forbidden")
	}
}

func TestExerciseOption_CallSuccess(t *testing.T) {
	settle := futureDate()
	optType := "CALL"
	uListing := int64(200)
	repo := &stubRepo{
		byID: &Portfolio{ID: 1, UserID: 10, ListingID: 100, ListingType: "OPTION", Quantity: 1, AveragePurchasePrice: d("1")},
		byUL: nil, // no existing underlying -> Insert
	}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{
		100: {ID: 100, ListingType: ptr("OPTION"), Price: ptr(d("150")), StrikePrice: ptr(d("100")), OptionType: &optType, SettlementDate: &settle, UnderlyingListingID: &uListing},
		200: {ID: 200, ListingType: ptr("STOCK"), Price: ptr(d("150"))},
	}}
	acc := &stubAccount{
		bank:   &clients.BankAccount{ID: ptr(int64(1))},
		detail: &clients.AccountDetails{AccountNumber: ptr("user-acc"), Currency: ptr("RSD")},
		gov:    &clients.AccountDetails{AccountNumber: ptr("gov-acc"), Currency: ptr("RSD")},
	}
	svc := newSvc(repo, mk, acc, nil)
	if err := svc.ExerciseOption(context.Background(), 10, true, 1); err != nil {
		t.Fatal(err)
	}
	if !acc.txDone {
		t.Error("settlement transaction not performed")
	}
}

func TestExerciseOption_NotOption(t *testing.T) {
	repo := &stubRepo{byID: &Portfolio{ID: 1, UserID: 10, ListingType: "STOCK", AveragePurchasePrice: d("1")}}
	svc := newSvc(repo, nil, nil, nil)
	if err := svc.ExerciseOption(context.Background(), 10, true, 1); err == nil {
		t.Error("non-option should error")
	}
}

func TestExerciseOption_MissingMetadata(t *testing.T) {
	repo := &stubRepo{byID: &Portfolio{ID: 1, UserID: 10, ListingID: 100, ListingType: "OPTION", Quantity: 1, AveragePurchasePrice: d("1")}}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{100: {ID: 100}}}
	svc := newSvc(repo, mk, nil, nil)
	if err := svc.ExerciseOption(context.Background(), 10, true, 1); err == nil {
		t.Error("missing exercise metadata should error")
	}
}

func TestExerciseOption_Expired(t *testing.T) {
	past := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	optType := "CALL"
	repo := &stubRepo{byID: &Portfolio{ID: 1, UserID: 10, ListingID: 100, ListingType: "OPTION", Quantity: 1, AveragePurchasePrice: d("1")}}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{100: {ID: 100, Price: ptr(d("150")), StrikePrice: ptr(d("100")), OptionType: &optType, SettlementDate: &past}}}
	svc := newSvc(repo, mk, nil, nil)
	if err := svc.ExerciseOption(context.Background(), 10, true, 1); err == nil {
		t.Error("expired option should error")
	}
}

func TestExerciseOption_PutSuccess(t *testing.T) {
	settle := futureDate()
	optType := "PUT"
	uListing := int64(200)
	// PUT: strike (100) > market (50) -> ITM. User holds 100 underlying shares.
	repo := &stubRepo{
		byID: &Portfolio{ID: 1, UserID: 10, ListingID: 100, ListingType: "OPTION", Quantity: 1, AveragePurchasePrice: d("1")},
		byUL: &Portfolio{ID: 2, UserID: 10, ListingID: 200, ListingType: "STOCK", Quantity: 100, AveragePurchasePrice: d("90")},
	}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{
		100: {ID: 100, Price: ptr(d("50")), StrikePrice: ptr(d("100")), OptionType: &optType, SettlementDate: &settle, UnderlyingListingID: &uListing},
		200: {ID: 200, ListingType: ptr("STOCK"), Price: ptr(d("50"))},
	}}
	acc := &stubAccount{
		bank:   &clients.BankAccount{ID: ptr(int64(1))},
		detail: &clients.AccountDetails{AccountNumber: ptr("user-acc"), Currency: ptr("RSD")},
		gov:    &clients.AccountDetails{AccountNumber: ptr("gov-acc"), Currency: ptr("RSD")},
	}
	svc := newSvc(repo, mk, acc, nil)
	if err := svc.ExerciseOption(context.Background(), 10, true, 1); err != nil {
		t.Fatal(err)
	}
}

func TestExerciseOption_PutInsufficientUnderlying(t *testing.T) {
	settle := futureDate()
	optType := "PUT"
	uListing := int64(200)
	repo := &stubRepo{
		byID: &Portfolio{ID: 1, UserID: 10, ListingID: 100, ListingType: "OPTION", Quantity: 1, AveragePurchasePrice: d("1")},
		byUL: &Portfolio{ID: 2, UserID: 10, ListingID: 200, Quantity: 5, AveragePurchasePrice: d("90")}, // < 100 shares
	}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{
		100: {ID: 100, Price: ptr(d("50")), StrikePrice: ptr(d("100")), OptionType: &optType, SettlementDate: &settle, UnderlyingListingID: &uListing},
		200: {ID: 200, ListingType: ptr("STOCK"), Price: ptr(d("50"))},
	}}
	svc := newSvc(repo, mk, nil, nil)
	if err := svc.ExerciseOption(context.Background(), 10, true, 1); err == nil {
		t.Error("insufficient underlying for PUT should error")
	}
}

func TestExerciseOption_MissingUnderlying(t *testing.T) {
	settle := futureDate()
	optType := "CALL"
	repo := &stubRepo{byID: &Portfolio{ID: 1, UserID: 10, ListingID: 100, ListingType: "OPTION", Quantity: 1, AveragePurchasePrice: d("1")}}
	// no UnderlyingListingID -> resolveUnderlying errors
	mk := &stubMarket{listings: map[int64]*clients.StockListing{100: {ID: 100, Price: ptr(d("150")), StrikePrice: ptr(d("100")), OptionType: &optType, SettlementDate: &settle}}}
	svc := newSvc(repo, mk, nil, nil)
	if err := svc.ExerciseOption(context.Background(), 10, true, 1); err == nil {
		t.Error("missing underlying listing should error")
	}
}

func TestExerciseOption_NotInTheMoney(t *testing.T) {
	settle := futureDate()
	optType := "CALL"
	repo := &stubRepo{byID: &Portfolio{ID: 1, UserID: 10, ListingID: 100, ListingType: "OPTION", Quantity: 1, AveragePurchasePrice: d("1")}}
	mk := &stubMarket{listings: map[int64]*clients.StockListing{100: {ID: 100, Price: ptr(d("50")), StrikePrice: ptr(d("100")), OptionType: &optType, SettlementDate: &settle}}}
	svc := newSvc(repo, mk, nil, nil)
	if err := svc.ExerciseOption(context.Background(), 10, true, 1); err == nil {
		t.Error("out-of-the-money CALL should error")
	}
}

// ---- pure helpers ----------------------------------------------------------

func TestIsOptionExercisable(t *testing.T) {
	settle := futureDate()
	call := "CALL"
	put := "PUT"
	if !isOptionExercisable(&clients.StockListing{Price: ptr(d("150")), StrikePrice: ptr(d("100")), OptionType: &call, SettlementDate: &settle}) {
		t.Error("ITM CALL should be exercisable")
	}
	if !isOptionExercisable(&clients.StockListing{Price: ptr(d("50")), StrikePrice: ptr(d("100")), OptionType: &put, SettlementDate: &settle}) {
		t.Error("ITM PUT should be exercisable")
	}
	if isOptionExercisable(nil) {
		t.Error("nil listing not exercisable")
	}
	past := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if isOptionExercisable(&clients.StockListing{Price: ptr(d("150")), StrikePrice: ptr(d("100")), OptionType: &call, SettlementDate: &past}) {
		t.Error("expired option not exercisable")
	}
}

func TestMapToResponse_NilPrice(t *testing.T) {
	if _, err := mapToResponse(Portfolio{ID: 1, ListingID: 100}, nil); err == nil {
		t.Error("nil listing should error")
	}
}

func TestMapToResponse_OptionSetsExercisable(t *testing.T) {
	settle := futureDate()
	call := "CALL"
	listing := &clients.StockListing{ID: 100, Price: ptr(d("150")), StrikePrice: ptr(d("100")), OptionType: &call, SettlementDate: &settle}
	resp, err := mapToResponse(Portfolio{ID: 1, ListingID: 100, ListingType: "OPTION", Quantity: 1, AveragePurchasePrice: d("100")}, listing)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Exercisable == nil || !*resp.Exercisable {
		t.Error("expected exercisable=true")
	}
}
