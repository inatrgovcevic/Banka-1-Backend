package portfolio

import (
	"context"
	"fmt"
	"strings"
	"time"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/clients"

	gpdb "banka1/go-platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// optionContractShares mirrors PortfolioServiceImpl.OPTION_CONTRACT_SHARES: one
// option contract covers 100 underlying shares.
const optionContractShares = 100

// TaxReporter supplies the tax figures the portfolio summary surfaces
// (yearlyTaxPaid/monthlyTaxDue), mirroring PortfolioServiceImpl's TaxService
// dependency. Declared here (not imported from the tax package) so portfolio does
// not import tax — tax imports portfolio for the average_purchase_price lookup, so
// the dependency must point one way to avoid an import cycle.
type TaxReporter interface {
	CurrentYearPaidTax(ctx context.Context, userID int64) (decimal.Decimal, error)
	CurrentMonthUnpaidTax(ctx context.Context, userID int64) (decimal.Decimal, error)
}

// portfolioRepo abstracts *Repository so the Service can be unit-tested with a
// stub. *Repository satisfies this interface.
type portfolioRepo interface {
	Pool() *pgxpool.Pool
	FindByUserID(ctx context.Context, q Querier, userID int64) ([]Portfolio, error)
	FindByID(ctx context.Context, q Querier, id int64) (*Portfolio, error)
	FindByUserIDAndListingID(ctx context.Context, q Querier, userID, listingID int64) (*Portfolio, error)
	UpdatePublic(ctx context.Context, q Querier, id int64, publicQuantity int, isPublic bool) error
	Insert(ctx context.Context, q Querier, userID, listingID int64, listingType string, quantity int, avg decimal.Decimal) error
	UpdateQuantityAndAvg(ctx context.Context, q Querier, id int64, quantity int, avg decimal.Decimal) error
	UpdateQuantity(ctx context.Context, q Querier, id int64, quantity int) error
	Delete(ctx context.Context, q Querier, id int64) error
}

// marketLister / accountMover abstract the client subsets the service uses.
type marketLister interface {
	GetListing(ctx context.Context, id int64) (*clients.StockListing, error)
	Calculate(ctx context.Context, from, to string, amount decimal.Decimal) (*clients.ExchangeRate, error)
}

type accountMover interface {
	GetBankAccount(ctx context.Context, currency string) (*clients.BankAccount, error)
	GetAccountDetailsByID(ctx context.Context, accountID int64) (*clients.AccountDetails, error)
	GetGovernmentBankAccountRsd(ctx context.Context) (*clients.AccountDetails, error)
	Transaction(ctx context.Context, payment clients.Payment) error
}

// txRunner runs fn in a transaction; production wraps gpdb.RunInTx, tests fake it.
type txRunner func(ctx context.Context, fn func(pgx.Tx) error) error

// Service implements the /portfolio operations. Mirrors PortfolioServiceImpl.
type Service struct {
	repo    portfolioRepo
	market  marketLister
	account accountMover
	tax     TaxReporter
	runInTx txRunner
}

func NewService(repo *Repository, market *clients.MarketClient, account *clients.AccountClient, tax TaxReporter) *Service {
	return &Service{repo: repo, market: market, account: account, tax: tax,
		runInTx: func(ctx context.Context, fn func(pgx.Tx) error) error {
			return gpdb.RunInTx(ctx, repo.Pool(), pgx.TxOptions{}, fn)
		}}
}

// GetPortfolio returns the holdings enriched with market data plus totalProfit,
// yearlyTaxPaid, and monthlyTaxDue. The tax fields come from the tax service
// (TaxReporter), mirroring PortfolioServiceImpl. Note: CurrentMonthUnpaidTax does
// strict FX conversion, so — exactly like the Java endpoint — a conversion failure
// surfaces here as a 409 (propagated to the HTTP layer).
func (s *Service) GetPortfolio(ctx context.Context, userID int64) (api.PortfolioSummaryResponse, error) {
	holdings, err := s.repo.FindByUserID(ctx, s.repo.Pool(), userID)
	if err != nil {
		return api.PortfolioSummaryResponse{}, err
	}

	// Fetch each distinct listing once (mirrors the toMap over distinct listingIds).
	listings := make(map[int64]*clients.StockListing)
	for _, h := range holdings {
		if _, ok := listings[h.ListingID]; ok {
			continue
		}
		listing, err := s.market.GetListing(ctx, h.ListingID)
		if err != nil {
			return api.PortfolioSummaryResponse{}, err
		}
		listings[h.ListingID] = listing
	}

	responses := make([]api.PortfolioResponse, 0, len(holdings))
	totalProfit := decimal.Zero
	for _, h := range holdings {
		resp, err := mapToResponse(h, listings[h.ListingID])
		if err != nil {
			return api.PortfolioSummaryResponse{}, err
		}
		responses = append(responses, resp)
		totalProfit = totalProfit.Add(resp.Profit)
	}

	yearlyTaxPaid, err := s.tax.CurrentYearPaidTax(ctx, userID)
	if err != nil {
		return api.PortfolioSummaryResponse{}, err
	}
	monthlyTaxDue, err := s.tax.CurrentMonthUnpaidTax(ctx, userID)
	if err != nil {
		return api.PortfolioSummaryResponse{}, err
	}

	return api.PortfolioSummaryResponse{
		Holdings:      responses,
		TotalProfit:   totalProfit,
		YearlyTaxPaid: yearlyTaxPaid,
		MonthlyTaxDue: monthlyTaxDue,
	}, nil
}

// SetPublicQuantity advertises STOCK units for OTC. Not transactional (mirrors
// Java: plain findById + save). publicQuantity has no bean validation; null and
// out-of-range are rejected here.
func (s *Service) SetPublicQuantity(ctx context.Context, userID, portfolioID int64, req api.SetPublicQuantityRequest) error {
	p, err := s.getOwnedPortfolio(ctx, s.repo.Pool(), userID, portfolioID)
	if err != nil {
		return err
	}
	if p.ListingType != "STOCK" {
		return api.NewOrderError(400, "Only STOCK positions can be made public")
	}
	if req.PublicQuantity == nil || *req.PublicQuantity < 0 {
		return api.NewOrderError(400, "Public quantity cannot be negative")
	}
	reserved := p.ReservedQuantity
	available := p.Quantity - reserved
	if *req.PublicQuantity > available {
		return api.NewOrderError(400, fmt.Sprintf(
			"Public quantity cannot exceed available quantity (owned: %d, reserved: %d, max: %d)",
			p.Quantity, reserved, available))
	}
	return s.repo.UpdatePublic(ctx, s.repo.Pool(), portfolioID, *req.PublicQuantity, *req.PublicQuantity > 0)
}

// ExerciseOption exercises an OPTION position. isAgent mirrors AuthenticatedUser
// .isAgent() (AGENT/SUPERVISOR/ADMIN). Runs in one transaction; external account
// transfers happen mid-transaction exactly as Java's @Transactional method does.
func (s *Service) ExerciseOption(ctx context.Context, userID int64, isAgent bool, portfolioID int64) error {
	if !isAgent {
		return api.NewOrderError(403, "Only actuaries can exercise options")
	}
	return s.runInTx(ctx, func(tx pgx.Tx) error {
		optionPortfolio, err := s.getOwnedPortfolio(ctx, tx, userID, portfolioID)
		if err != nil {
			return err
		}
		if optionPortfolio.ListingType != "OPTION" {
			return api.NewOrderError(400, "Only OPTION positions can be exercised")
		}

		optionListing, err := s.market.GetListing(ctx, optionPortfolio.ListingID)
		if err != nil {
			return err
		}
		if optionListing.SettlementDate == nil || optionListing.StrikePrice == nil || optionListing.OptionType == nil {
			return api.NewOrderError(409, "Option listing is missing required exercise metadata")
		}
		if optionListing.Price == nil {
			return fmt.Errorf("portfolio: option listing %d has no price", optionPortfolio.ListingID)
		}

		marketPrice := *optionListing.Price
		strikePrice := *optionListing.StrikePrice
		optionType := *optionListing.OptionType

		settle, err := time.Parse("2006-01-02", *optionListing.SettlementDate)
		if err != nil {
			return fmt.Errorf("portfolio: option listing %d has invalid settlement date %q: %w", optionPortfolio.ListingID, *optionListing.SettlementDate, err)
		}
		now := time.Now()
		settlementAtStartOfDay := time.Date(settle.Year(), settle.Month(), settle.Day(), 0, 0, 0, 0, now.Location())
		if settlementAtStartOfDay.Before(now) {
			return api.NewOrderError(409, "Option already expired")
		}

		inTheMoney := marketPrice.GreaterThan(strikePrice)
		if optionType != "CALL" {
			inTheMoney = marketPrice.LessThan(strikePrice)
		}
		if !inTheMoney {
			return api.NewOrderError(409, "Option is not in-the-money")
		}

		exercisedShares := optionPortfolio.Quantity * optionContractShares
		if exercisedShares <= 0 {
			return api.NewOrderError(409, "Option position has no exercisable contracts")
		}

		underlyingListing, err := s.resolveUnderlying(ctx, optionListing)
		if err != nil {
			return err
		}
		settlementAmount := strikePrice.Mul(decimal.NewFromInt(int64(exercisedShares))).Round(2)

		// PUT pre-check (mirrors validateUnderlyingPositionForExercise).
		if optionType == "PUT" {
			up, err := s.repo.FindByUserIDAndListingID(ctx, tx, userID, underlyingListing.ID)
			if err != nil {
				return err
			}
			if up == nil || up.Quantity < exercisedShares {
				return api.NewOrderError(409, "Insufficient underlying stock quantity for PUT exercise")
			}
		}

		if err := s.moveExerciseFunds(ctx, userID, optionListing.Currency(), settlementAmount, optionType); err != nil {
			return err
		}
		if err := s.updateUnderlyingPortfolio(ctx, tx, userID, underlyingListing, exercisedShares, strikePrice, optionType); err != nil {
			return err
		}
		return s.repo.Delete(ctx, tx, optionPortfolio.ID)
	})
}

func (s *Service) getOwnedPortfolio(ctx context.Context, q Querier, userID, portfolioID int64) (*Portfolio, error) {
	p, err := s.repo.FindByID(ctx, q, portfolioID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, api.NewOrderError(404, "Portfolio not found")
	}
	if p.UserID != userID {
		return nil, api.NewOrderError(403, "Portfolio does not belong to the authenticated user")
	}
	return p, nil
}

func (s *Service) resolveUnderlying(ctx context.Context, optionListing *clients.StockListing) (*clients.StockListing, error) {
	if optionListing.UnderlyingListingID == nil {
		return nil, api.NewOrderError(409, "Option listing is missing underlying listing metadata")
	}
	return s.market.GetListing(ctx, *optionListing.UnderlyingListingID)
}

// moveExerciseFunds settles the exercise cash leg via account-service. CALL: the
// actuary pays strike to the state account; PUT: the state account pays the
// actuary. Amounts are converted to the market account currency (RSD) as in Java.
func (s *Service) moveExerciseFunds(ctx context.Context, userID int64, currency string, settlementAmount decimal.Decimal, optionType string) error {
	bankAccount, err := s.account.GetBankAccount(ctx, currency)
	if err != nil {
		return err
	}
	userSettlement, err := s.account.GetAccountDetailsByID(ctx, bankAccount.ResolvedID())
	if err != nil {
		return err
	}
	marketAccount, err := s.account.GetGovernmentBankAccountRsd(ctx)
	if err != nil {
		return err
	}
	targetAmount, err := s.convertAmount(ctx, currency, marketAccount.CurrencyOrEmpty(), settlementAmount)
	if err != nil {
		return err
	}

	var payment clients.Payment
	if optionType == "CALL" {
		payment = clients.Payment{
			FromAccountNumber: userSettlement.Number(),
			ToAccountNumber:   marketAccount.Number(),
			FromAmount:        settlementAmount,
			ToAmount:          targetAmount,
			Commission:        decimal.Zero,
			ClientID:          userID,
		}
	} else {
		payment = clients.Payment{
			FromAccountNumber: marketAccount.Number(),
			ToAccountNumber:   userSettlement.Number(),
			FromAmount:        targetAmount,
			ToAmount:          settlementAmount,
			Commission:        decimal.Zero,
			ClientID:          userID,
		}
	}
	return s.account.Transaction(ctx, payment)
}

// updateUnderlyingPortfolio applies the share movement: CALL adds/merges the
// underlying position (weighted average, scale 4 HALF_UP), PUT deducts and
// deletes on zero. Mirrors updateUnderlyingPortfolio.
func (s *Service) updateUnderlyingPortfolio(ctx context.Context, tx pgx.Tx, userID int64, underlying *clients.StockListing, exercisedShares int, strikePrice decimal.Decimal, optionType string) error {
	up, err := s.repo.FindByUserIDAndListingID(ctx, tx, userID, underlying.ID)
	if err != nil {
		return err
	}

	if optionType == "CALL" {
		if up == nil {
			listingType := "STOCK"
			if underlying.ListingType != nil {
				listingType = *underlying.ListingType
			}
			return s.repo.Insert(ctx, tx, userID, underlying.ID, listingType, exercisedShares, strikePrice)
		}
		totalValue := up.AveragePurchasePrice.Mul(decimal.NewFromInt(int64(up.Quantity))).
			Add(strikePrice.Mul(decimal.NewFromInt(int64(exercisedShares))))
		newQuantity := up.Quantity + exercisedShares
		newAvg := totalValue.DivRound(decimal.NewFromInt(int64(newQuantity)), 4)
		return s.repo.UpdateQuantityAndAvg(ctx, tx, up.ID, newQuantity, newAvg)
	}

	// PUT
	if up == nil || up.Quantity < exercisedShares {
		return api.NewOrderError(409, "Insufficient underlying stock quantity for PUT exercise")
	}
	newQuantity := up.Quantity - exercisedShares
	if newQuantity == 0 {
		return s.repo.Delete(ctx, tx, up.ID)
	}
	return s.repo.UpdateQuantity(ctx, tx, up.ID, newQuantity)
}

// convertAmount mirrors PortfolioServiceImpl.convertAmount: no-op when currencies
// match or are empty, otherwise market-service FX with graceful pass-through if
// the conversion result is missing.
func (s *Service) convertAmount(ctx context.Context, from, to string, amount decimal.Decimal) (decimal.Decimal, error) {
	if from == "" || to == "" || strings.EqualFold(from, to) {
		return amount, nil
	}
	conversion, err := s.market.Calculate(ctx, from, to, amount)
	if err != nil {
		return decimal.Zero, err
	}
	if conversion == nil || conversion.Converted() == nil {
		return amount, nil
	}
	return *conversion.Converted(), nil
}

func mapToResponse(p Portfolio, listing *clients.StockListing) (api.PortfolioResponse, error) {
	if listing == nil || listing.Price == nil {
		// Java would NPE on a null price -> 500. Surface a generic error so the
		// HTTP layer returns the same 500 "Unexpected server error".
		return api.PortfolioResponse{}, fmt.Errorf("portfolio: listing %d has no price", p.ListingID)
	}
	currentPrice := *listing.Price
	profit := currentPrice.Sub(p.AveragePurchasePrice).Mul(decimal.NewFromInt(int64(p.Quantity)))

	resp := api.PortfolioResponse{
		ID:                   p.ID,
		ListingID:            p.ListingID,
		ListingType:          p.ListingType,
		Ticker:               listing.Ticker,
		Quantity:             p.Quantity,
		PublicQuantity:       p.PublicQuantity,
		LastModified:         api.NewLocalDateTime(p.LastModified),
		CurrentPrice:         currentPrice,
		AveragePurchasePrice: p.AveragePurchasePrice,
		Profit:               profit,
	}
	if p.ListingType == "OPTION" {
		exercisable := isOptionExercisable(listing)
		resp.Exercisable = &exercisable
	}
	return resp, nil
}

// isOptionExercisable mirrors PortfolioServiceImpl.isOptionExercisable: in-the-money
// and not expired (settlement date strictly after today).
func isOptionExercisable(listing *clients.StockListing) bool {
	if listing == nil || listing.SettlementDate == nil || listing.OptionType == nil ||
		listing.Price == nil || listing.StrikePrice == nil {
		return false
	}
	settle, err := time.Parse("2006-01-02", *listing.SettlementDate)
	if err != nil {
		return false
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	settleDate := time.Date(settle.Year(), settle.Month(), settle.Day(), 0, 0, 0, 0, now.Location())
	if !settleDate.After(today) {
		return false
	}
	if *listing.OptionType == "CALL" {
		return listing.Price.GreaterThan(*listing.StrikePrice)
	}
	return listing.Price.LessThan(*listing.StrikePrice)
}
