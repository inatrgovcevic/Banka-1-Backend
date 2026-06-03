package dividend

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"banka1/trading-service-go/internal/clients"
	"banka1/trading-service-go/internal/order"
	"banka1/trading-service-go/internal/portfolio"

	gpdb "banka1/go-platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

const (
	rsd           = "RSD"
	monetaryScale = 4
)

var quarters = decimal.NewFromInt(4)

// Service orchestrates the quarterly dividend computation and payout. Mirrors
// DividendDistributionService + DividendPayoutExecutor:
//
//   - gross = quantity * price * (dividendYield / 4), in the listing currency;
//   - personal position: 15% tax in RSD (gross converted to RSD without
//     commission), net to the holder, tax to the state RSD account;
//   - bank-held position (forBank): no tax, full gross is Profit Banke.
//
// Each holder payout runs in its own transaction (the Java executor is a
// separate @Transactional bean for exactly that reason): one failed payout
// rolls back only itself, the rest of the run continues.
type Service struct {
	repo       *Repository
	orders     *order.Repository
	portfolios *portfolio.Repository
	market     *clients.MarketClient
	account    *clients.AccountClient
	taxRate    decimal.Decimal
	logger     *slog.Logger
}

// NewService wires the dividend payout service. taxRate mirrors
// banka.tax.capital-gains-rate (default 0.15, shared with the tax domain).
func NewService(repo *Repository, orders *order.Repository, portfolios *portfolio.Repository,
	cl *clients.Clients, taxRate decimal.Decimal, logger *slog.Logger) *Service {
	return &Service{
		repo:       repo,
		orders:     orders,
		portfolios: portfolios,
		market:     cl.Market,
		account:    cl.Account,
		taxRate:    taxRate,
		logger:     logger,
	}
}

// Repo exposes the payout repository for the read-side handlers.
func (s *Service) Repo() *Repository { return s.repo }

// Distribute mirrors DividendDistributionService.distribute: for every STOCK
// with a positive dividendYield, pay every holder (portfolio row with
// listingType=STOCK, quantity>0). Returns the number of holders successfully
// paid. Per-holder failures are logged and skipped — one bad holder cannot
// abort the quarterly run.
func (s *Service) Distribute(ctx context.Context, asOf time.Time) int {
	stocks := s.market.FetchDividendData(ctx)
	if len(stocks) == 0 {
		s.logger.Info("dividend distribute: no dividend data — nothing to pay", "asOf", asOf.Format("2006-01-02"))
		return 0
	}

	paid := 0
	for i := range stocks {
		stock := &stocks[i]
		if stock.DividendYield == nil || stock.DividendYield.Sign() <= 0 {
			continue
		}
		holders, err := s.portfolios.FindStockHoldersByListingID(ctx, s.portfolios.Pool(), stock.ListingID)
		if err != nil {
			s.logger.Error("dividend: holder lookup failed", "listingId", stock.ListingID, "error", err)
			continue
		}
		for h := range holders {
			ok, err := s.payoutForHolder(ctx, stock, &holders[h], asOf)
			if err != nil {
				s.logger.Error("dividend: payout failed",
					"userId", holders[h].UserID, "listingId", stock.ListingID, "asOf", asOf.Format("2006-01-02"), "error", err)
				continue
			}
			if ok {
				paid++
			}
		}
	}
	s.logger.Info("dividend distribute finished", "asOf", asOf.Format("2006-01-02"), "paid", paid)
	return paid
}

// payoutForHolder mirrors DividendPayoutExecutor.payoutForHolder, in its own
// transaction. The position quantity splits into bank-held (derived from
// executed PurchaseFor.BANK BUY orders, clamped to [0, total]) and personal;
// each part pays separately — bank-held untaxed into Profit Banke, personal
// with 15% to the state account.
func (s *Service) payoutForHolder(ctx context.Context, stock *clients.DividendData, holder *portfolio.Portfolio, asOf time.Time) (bool, error) {
	anyPaid := false
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		userID := holder.UserID
		listingID := stock.ListingID
		total := holder.Quantity

		rawBank, err := s.orders.BankHeldBuyQuantity(ctx, tx, userID, listingID)
		if err != nil {
			return err
		}
		bankQty := int(rawBank)
		if bankQty > total {
			bankQty = total
		}
		if bankQty < 0 {
			bankQty = 0
		}
		personalQty := total - bankQty

		if personalQty > 0 {
			exists, err := s.repo.ExistsForDate(ctx, tx, userID, listingID, asOf, false)
			if err != nil {
				return err
			}
			if exists {
				s.logger.Debug("dividend (personal): already paid", "userId", userID, "listingId", listingID)
			} else {
				gross := computeGross(personalQty, stock.Price, stock.DividendYield)
				if gross.Sign() > 0 {
					if err := s.payPersonal(ctx, tx, stock, userID, asOf, gross, personalQty); err != nil {
						return err
					}
					anyPaid = true
				}
			}
		}

		if bankQty > 0 {
			exists, err := s.repo.ExistsForDate(ctx, tx, userID, listingID, asOf, true)
			if err != nil {
				return err
			}
			if exists {
				s.logger.Debug("dividend (bank): already paid", "userId", userID, "listingId", listingID)
			} else {
				gross := computeGross(bankQty, stock.Price, stock.DividendYield)
				if gross.Sign() > 0 {
					if err := s.payBankHeld(ctx, tx, stock, userID, asOf, gross, bankQty); err != nil {
						return err
					}
					anyPaid = true
				}
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return anyPaid, nil
}

// payBankHeld mirrors payBankHeld: no tax, full gross (converted to RSD) is
// credited to the bank's RSD account (Profit Banke). Account-resolution
// failures only skip the credit — the payout row is still recorded.
func (s *Service) payBankHeld(ctx context.Context, tx pgx.Tx, stock *clients.DividendData, userID int64, asOf time.Time, gross decimal.Decimal, quantity int) error {
	currency := stock.Currency
	grossRsd := s.convertToRsd(ctx, gross, currency)
	bankAccount := s.account.GetBankRsdOwnerAccount(ctx)

	var accountID *int64
	if bankAccount != nil {
		accountID = bankAccount.ID
	}
	ticker := stock.Ticker
	if err := s.repo.Insert(ctx, tx, &Payout{
		UserID:       userID,
		StockTicker:  &ticker,
		ListingID:    stock.ListingID,
		Quantity:     quantity,
		GrossAmount:  scale(gross),
		Currency:     currency,
		TaxAmountRsd: decimal.Zero,
		NetAmount:    scale(gross),
		AccountID:    accountID,
		PaymentDate:  asOf,
		ForBank:      true,
	}); err != nil {
		return err
	}

	if bankAccount != nil {
		if err := s.account.CreditAccount(ctx, bankAccount.Number(), grossRsd, bankAccount.OwnerIDValue()); err != nil {
			return err
		}
	} else {
		s.logger.Warn("dividend: bank RSD account unresolved — bank-held payout recorded without credit",
			"userId", userID, "listingId", stock.ListingID)
	}
	s.logger.Info("dividend (bank) paid", "userId", userID, "ticker", stock.Ticker,
		"quantity", quantity, "gross", gross, "grossRsd", grossRsd)
	return nil
}

// payPersonal mirrors payPersonal: 15% tax in RSD to the state account, net to
// the holder — in the listing currency when the holder has such an account
// (WP-14b, no FX), otherwise converted to RSD onto the holder's RSD account.
func (s *Service) payPersonal(ctx context.Context, tx pgx.Tx, stock *clients.DividendData, userID int64, asOf time.Time, gross decimal.Decimal, quantity int) error {
	currency := stock.Currency
	grossRsd := s.convertToRsd(ctx, gross, currency)
	taxRsd := grossRsd.Mul(s.taxRate).Round(monetaryScale)
	netRsd := grossRsd.Sub(taxRsd)

	taxInListingCurrency := s.convertFromRsd(ctx, taxRsd, currency)
	netListing := scale(gross.Sub(taxInListingCurrency))

	target := s.resolvePersonalTarget(ctx, userID, currency)
	stateAccount := s.account.GetStateRsdOwnerAccount(ctx)

	ticker := stock.Ticker
	if err := s.repo.Insert(ctx, tx, &Payout{
		UserID:       userID,
		StockTicker:  &ticker,
		ListingID:    stock.ListingID,
		Quantity:     quantity,
		GrossAmount:  scale(gross),
		Currency:     currency,
		TaxAmountRsd: taxRsd,
		NetAmount:    netListing,
		AccountID:    target.accountID,
		PaymentDate:  asOf,
		ForBank:      false,
	}); err != nil {
		return err
	}

	if target.accountNumber != "" {
		creditAmount := netRsd
		if target.inListingCurrency {
			creditAmount = netListing
		}
		if err := s.account.CreditAccount(ctx, target.accountNumber, creditAmount, userID); err != nil {
			return err
		}
	} else {
		s.logger.Warn("dividend: holder account unresolved (neither listing currency nor RSD) — payout recorded without credit",
			"userId", userID, "listingId", stock.ListingID)
	}
	if taxRsd.Sign() > 0 && stateAccount != nil {
		if err := s.account.CreditAccount(ctx, stateAccount.Number(), taxRsd, stateAccount.OwnerIDValue()); err != nil {
			return err
		}
	} else if taxRsd.Sign() > 0 {
		s.logger.Warn("dividend: state RSD account unresolved — tax recorded without credit",
			"taxRsd", taxRsd, "userId", userID, "listingId", stock.ListingID)
	}
	s.logger.Info("dividend (personal) paid", "userId", userID, "ticker", stock.Ticker,
		"quantity", quantity, "gross", gross, "taxRsd", taxRsd)
	return nil
}

// payoutTarget mirrors PayoutTarget.
type payoutTarget struct {
	accountID         *int64
	accountNumber     string
	inListingCurrency bool
}

// resolvePersonalTarget mirrors resolvePersonalTarget (WP-14b fallback chain):
// listing-currency account first (no FX), then the holder's RSD account, then
// the default RSD account number.
func (s *Service) resolvePersonalTarget(ctx context.Context, userID int64, currency *string) payoutTarget {
	listingIsRsd := currency == nil || strings.EqualFold(*currency, rsd)

	if !listingIsRsd {
		if listingAccount := s.account.GetAccountInCurrency(ctx, userID, *currency); listingAccount != nil {
			return payoutTarget{accountID: listingAccount.ID, accountNumber: listingAccount.Number(), inListingCurrency: true}
		}
	}
	if rsdAccount := s.account.GetAccountInCurrency(ctx, userID, rsd); rsdAccount != nil {
		return payoutTarget{accountID: rsdAccount.ID, accountNumber: rsdAccount.Number(), inListingCurrency: false}
	}
	return payoutTarget{accountNumber: s.account.GetDefaultRsdAccountNumberForOwner(ctx, userID), inListingCurrency: false}
}

// computeGross mirrors computeGross: quantity * price * (dividendYield / 4),
// rounded to 4 decimals; zero for any missing/non-positive input.
func computeGross(quantity int, price, dividendYield *decimal.Decimal) decimal.Decimal {
	if quantity <= 0 || price == nil || dividendYield == nil {
		return decimal.Zero
	}
	if price.Sign() <= 0 || dividendYield.Sign() <= 0 {
		return decimal.Zero
	}
	quarterlyYield := dividendYield.DivRound(quarters, 10)
	return decimal.NewFromInt(int64(quantity)).Mul(*price).Mul(quarterlyYield).Round(monetaryScale)
}

func (s *Service) convertToRsd(ctx context.Context, amount decimal.Decimal, currency *string) decimal.Decimal {
	if currency == nil || strings.EqualFold(*currency, rsd) {
		return scale(amount)
	}
	if converted, ok := s.market.ConvertNoCommission(ctx, amount, *currency, rsd); ok {
		return scale(converted)
	}
	return scale(amount)
}

func (s *Service) convertFromRsd(ctx context.Context, amountRsd decimal.Decimal, currency *string) decimal.Decimal {
	if currency == nil || strings.EqualFold(*currency, rsd) {
		return scale(amountRsd)
	}
	if converted, ok := s.market.ConvertNoCommission(ctx, amountRsd, rsd, *currency); ok {
		return scale(converted)
	}
	return scale(amountRsd)
}

func scale(value decimal.Decimal) decimal.Decimal {
	return value.Round(monetaryScale)
}
