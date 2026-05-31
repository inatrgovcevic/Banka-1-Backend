package tax

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"banka1/trading-service-go/internal/actuary"
	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/clients"
	"banka1/trading-service-go/internal/order"
	"banka1/trading-service-go/internal/portfolio"

	"github.com/shopspring/decimal"
)

// clockNow is the wall clock for period math. UTC is used so the calendar
// boundaries align with the `timestamp` (without time zone) columns pgx reads back
// in UTC — matching Java's LocalDateTime semantics in the common TZ=UTC container.
// Overridable in tests.
var clockNow = func() time.Time { return time.Now().UTC() }

const trackingPageSize = 100

// historyStart mirrors TaxServiceImpl.HISTORY_START (1970-01-01T00:00) — the lower
// bound for "all history" debt/tracking aggregations.
var historyStart = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

// Service implements the /tax capital-gains operations. Faithful port of
// order-service TaxServiceImpl: a FIFO cost-basis engine + an OTC capital-gains
// pass + a tax_charges ledger, with strict/tolerant RSD conversion. It also feeds
// the /portfolio summary (yearlyTaxPaid/monthlyTaxDue) via the portfolio.TaxReporter
// interface — see CurrentYearPaidTax / CurrentMonthUnpaidTax.
type Service struct {
	taxRepo       *Repository
	orderRepo     *order.Repository
	portfolioRepo *portfolio.Repository
	actuaryRepo   *actuary.Repository
	market        *clients.MarketClient
	account       *clients.AccountClient
	employee      *clients.EmployeeClient
	customer      *clients.CustomerClient
	notifier      Notifier
	taxRate       decimal.Decimal
	logger        *slog.Logger
}

// NewService wires the tax service. taxRate mirrors banka.tax.capital-gains-rate
// (default 0.15). A nil notifier falls back to Noop.
func NewService(
	taxRepo *Repository,
	orderRepo *order.Repository,
	portfolioRepo *portfolio.Repository,
	actuaryRepo *actuary.Repository,
	cl *clients.Clients,
	notifier Notifier,
	taxRate decimal.Decimal,
	logger *slog.Logger,
) *Service {
	if notifier == nil {
		notifier = NoopNotifier{}
	}
	return &Service{
		taxRepo:       taxRepo,
		orderRepo:     orderRepo,
		portfolioRepo: portfolioRepo,
		actuaryRepo:   actuaryRepo,
		market:        cl.Market,
		account:       cl.Account,
		employee:      cl.Employee,
		customer:      cl.Customer,
		notifier:      notifier,
		taxRate:       taxRate,
		logger:        logger,
	}
}

// --- Public API (mirrors TaxService) --------------------------------------

// CollectMonthlyTax collects capital-gains tax for the previous calendar month.
func (s *Service) CollectMonthlyTax(ctx context.Context) error {
	now := clockNow()
	firstThisMonth := firstOfMonthUTC(now)
	firstPrevMonth := firstThisMonth.AddDate(0, -1, 0)
	return s.collectTaxForPeriod(ctx, firstPrevMonth, firstThisMonth)
}

// CollectMonthlyTaxManually is the supervisor-triggered equivalent of the monthly job.
func (s *Service) CollectMonthlyTaxManually(ctx context.Context) error {
	s.logger.Info("manually triggering monthly tax collection")
	return s.CollectMonthlyTax(ctx)
}

// CollectCurrentMonthTax collects tax from the 1st of the current month up to now.
func (s *Service) CollectCurrentMonthTax(ctx context.Context) error {
	now := clockNow()
	firstThisMonth := firstOfMonthUTC(now)
	tomorrowStart := startOfDayUTC(now).AddDate(0, 0, 1)
	return s.collectTaxForPeriod(ctx, firstThisMonth, tomorrowStart)
}

// GetAllDebts aggregates per-user tax (raw taxAmount, NO FX, NO OTC — matching
// Java) and returns a Spring-style page. NOTE: Java iterates a HashMap (arbitrary
// order); the Go port orders by userId ascending for determinism — values match,
// but multi-user page ordering may differ from the live service (documented
// parity caveat; confirm during the sweep).
func (s *Service) GetAllDebts(ctx context.Context, page, size int) (api.Page[api.TaxDebtResponse], error) {
	entries, err := s.buildTaxChargeEntries(ctx, historyStart, clockNow(), nil)
	if err != nil {
		return api.Page[api.TaxDebtResponse]{}, err
	}
	debtMap := map[int64]decimal.Decimal{}
	for _, e := range entries {
		debtMap[e.userID] = debtMap[e.userID].Add(e.taxAmount)
	}
	all := make([]api.TaxDebtResponse, 0, len(debtMap))
	for _, uid := range sortedInt64Keys(debtMap) {
		all = append(all, api.TaxDebtResponse{UserID: uid, DebtRsd: debtMap[uid]})
	}
	return api.NewPage(paginate(all, page, size), page, size, int64(len(all))), nil
}

// GetUserDebt returns the total tax for a user (raw taxAmount, NO FX, NO OTC).
func (s *Service) GetUserDebt(ctx context.Context, userID int64) (api.TaxDebtResponse, error) {
	entries, err := s.buildTaxChargeEntries(ctx, historyStart, clockNow(), &userID)
	if err != nil {
		return api.TaxDebtResponse{}, err
	}
	total := decimal.Zero
	for _, e := range entries {
		total = total.Add(e.taxAmount)
	}
	return api.TaxDebtResponse{UserID: userID, DebtRsd: total}, nil
}

// CurrentYearPaidTax returns CHARGED tax (RSD) for the user in the current calendar
// year. Tolerant (no FX). Feeds PortfolioSummary.yearlyTaxPaid.
func (s *Service) CurrentYearPaidTax(ctx context.Context, userID int64) (decimal.Decimal, error) {
	now := clockNow()
	yearStart := firstOfYearUTC(now)
	tomorrowStart := startOfDayUTC(now).AddDate(0, 0, 1)
	charges, err := s.taxRepo.FindByUserIDAndStatus(ctx, userID, StatusCharged)
	if err != nil {
		return decimal.Zero, err
	}
	total := decimal.Zero
	for _, c := range charges {
		if c.ChargedAt == nil || c.ChargedAt.Before(yearStart) || !c.ChargedAt.Before(tomorrowStart) {
			continue
		}
		amt := c.TaxAmount
		if c.TaxAmountRsd != nil {
			amt = *c.TaxAmountRsd
		}
		total = total.Add(amt)
	}
	return total, nil
}

// CurrentMonthUnpaidTax returns the not-yet-charged tax (RSD, strict FX) accrued in
// the current month. Feeds PortfolioSummary.monthlyTaxDue — a strict-FX failure
// surfaces as a 409, exactly as the Java /portfolio endpoint does.
func (s *Service) CurrentMonthUnpaidTax(ctx context.Context, userID int64) (decimal.Decimal, error) {
	now := clockNow()
	monthStart := firstOfMonthUTC(now)
	tomorrowStart := startOfDayUTC(now).AddDate(0, 0, 1)
	uid := userID
	return s.calculateUnchargedTaxForRangeInRsd(ctx, &uid, monthStart, tomorrowStart)
}

// GetTaxTracking returns supervisor tracking rows (clients then actuaries),
// optionally filtered, as a Spring-style page.
func (s *Service) GetTaxTracking(ctx context.Context, userType, firstName, lastName *string, page, size int) (api.Page[api.TaxTrackingRowResponse], error) {
	metricsByUser, err := s.calculateTrackingMetrics(ctx)
	if err != nil {
		return api.Page[api.TaxTrackingRowResponse]{}, err
	}
	all := make([]api.TaxTrackingRowResponse, 0)
	if userType == nil || strings.EqualFold(*userType, "CLIENT") {
		rows, err := s.loadClientTrackingRows(ctx, firstName, lastName, metricsByUser)
		if err != nil {
			return api.Page[api.TaxTrackingRowResponse]{}, err
		}
		all = append(all, rows...)
	}
	if userType == nil || strings.EqualFold(*userType, "ACTUARY") {
		rows, err := s.loadActuaryTrackingRows(ctx, firstName, lastName, metricsByUser)
		if err != nil {
			return api.Page[api.TaxTrackingRowResponse]{}, err
		}
		all = append(all, rows...)
	}
	return api.NewPage(paginate(all, page, size), page, size, int64(len(all))), nil
}

// --- Collection -----------------------------------------------------------

func (s *Service) collectTaxForPeriod(ctx context.Context, start, end time.Time) error {
	s.logger.Info("collecting taxes for period", "start", start, "end", end)

	entries, err := s.buildTaxChargeEntries(ctx, start, end, nil)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		reservation, skip, err := s.reserveTaxCharge(ctx, entry, start, end)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		s.chargeStockEntry(ctx, entry, reservation)
	}

	return s.collectOtcTaxForPeriod(ctx, start, end)
}

// chargeStockEntry runs the per-entry settlement (the Java try/catch body): a
// failure here is logged and rolled back via handleFailedChargeAttempt — it never
// aborts the run.
func (s *Service) chargeStockEntry(ctx context.Context, entry taxChargeEntry, reservation *TaxCharge) {
	debitSucceeded := false
	err := func() error {
		sourceAccount, err := s.account.GetAccountDetailsByID(ctx, entry.sourceAccountID)
		if err != nil {
			return err
		}
		governmentAccount, err := s.account.GetGovernmentBankAccountRsd(ctx)
		if err != nil {
			return err
		}
		taxInRsd := s.convertTaxToRsd(ctx, entry.currency, entry.taxAmount)
		payment := clients.Payment{
			FromAccountNumber: sourceAccount.Number(),
			ToAccountNumber:   governmentAccount.Number(),
			FromAmount:        taxInRsd,
			ToAmount:          taxInRsd,
			Commission:        decimal.Zero,
			ClientID:          entry.userID,
		}
		if err := s.account.Transaction(ctx, payment); err != nil {
			return err
		}
		debitSucceeded = true
		if err := s.taxRepo.UpdateCharged(ctx, reservation.ID, taxInRsd, clockNow()); err != nil {
			return err
		}
		s.notifier.TaxCollected(ctx, s.createTaxNotificationPayload(ctx, entry, taxInRsd))
		s.logger.Info("collected stock tax", "tax", entry.taxAmount.String(), "rsd", taxInRsd.String(), "transactionId", entry.transactionID)
		return nil
	}()
	if err != nil {
		s.handleFailedChargeAttempt(ctx, reservation, debitSucceeded)
		s.logger.Error("failed to collect tax for transaction", "transactionId", entry.transactionID, "error", err)
	}
}

func (s *Service) collectOtcTaxForPeriod(ctx context.Context, start, end time.Time) error {
	governmentAccount, err := s.account.GetGovernmentBankAccountRsd(ctx)
	if err != nil {
		return err
	}

	for _, entry := range s.loadExercisedOtcTaxEntries(ctx, end) {
		if entry.ExercisedAt.IsZero() || entry.ExercisedAt.Before(start) {
			continue
		}
		exists, err := s.taxRepo.ExistsByOtcContractID(ctx, entry.ContractID)
		if err != nil {
			return err
		}
		if exists {
			s.logger.Info("skipping already processed OTC tax for contract", "contractId", entry.ContractID)
			continue
		}

		taxInRsd, err := s.calculateOtcTaxInRsd(ctx, entry)
		if err != nil {
			return err
		}
		if taxInRsd.Sign() <= 0 {
			continue
		}

		sellerAccountNumber := s.account.GetDefaultRsdAccountNumberForOwner(ctx, entry.SellerID)
		if sellerAccountNumber == "" {
			s.logger.Warn("cannot collect OTC tax: no RSD account for seller", "contractId", entry.ContractID, "sellerId", entry.SellerID)
			continue
		}

		contractID := entry.ContractID
		amountRsd := taxInRsd
		charge := &TaxCharge{
			SellTransactionID: contractID,
			BuyTransactionID:  contractID,
			OtcContractID:     &contractID,
			UserID:            entry.SellerID,
			ListingID:         entry.ListingID,
			SourceAccountID:   0,
			TaxPeriodStart:    start,
			TaxPeriodEnd:      end,
			TaxAmount:         taxInRsd,
			TaxAmountRsd:      &amountRsd,
			Status:            StatusReserved,
		}
		if err := s.taxRepo.Insert(ctx, charge); err != nil {
			if errors.Is(err, ErrDuplicate) {
				s.logger.Info("skipping duplicate OTC tax charge for contract", "contractId", entry.ContractID)
				continue
			}
			return err
		}
		s.chargeOtcEntry(ctx, charge, governmentAccount.Number(), sellerAccountNumber, entry.SellerID, contractID, taxInRsd)
	}
	return nil
}

func (s *Service) chargeOtcEntry(ctx context.Context, charge *TaxCharge, govAccountNumber, sellerAccountNumber string, sellerID, contractID int64, taxInRsd decimal.Decimal) {
	debitSucceeded := false
	err := func() error {
		payment := clients.Payment{
			FromAccountNumber: sellerAccountNumber,
			ToAccountNumber:   govAccountNumber,
			FromAmount:        taxInRsd,
			ToAmount:          taxInRsd,
			Commission:        decimal.Zero,
			ClientID:          sellerID,
		}
		if err := s.account.Transaction(ctx, payment); err != nil {
			return err
		}
		debitSucceeded = true
		if err := s.taxRepo.MarkCharged(ctx, charge.ID, clockNow()); err != nil {
			return err
		}
		s.logger.Info("collected OTC tax", "rsd", taxInRsd.String(), "contractId", contractID)
		return nil
	}()
	if err != nil {
		s.handleFailedChargeAttempt(ctx, charge, debitSucceeded)
		s.logger.Error("failed to collect OTC tax for contract", "contractId", contractID, "error", err)
	}
}

// reserveTaxCharge inserts a RESERVED stock charge. Returns skip=true when the
// (sell,buy) pair already exists or the insert hits the unique constraint
// (idempotent re-run); a real DB error propagates.
func (s *Service) reserveTaxCharge(ctx context.Context, entry taxChargeEntry, start, end time.Time) (charge *TaxCharge, skip bool, err error) {
	exists, err := s.taxRepo.ExistsBySellAndBuy(ctx, entry.transactionID, entry.buyTransactionID)
	if err != nil {
		return nil, false, err
	}
	if exists {
		s.logger.Info("skipping already processed tax charge", "sellTransactionId", entry.transactionID, "buyTransactionId", entry.buyTransactionID)
		return nil, true, nil
	}
	charge = &TaxCharge{
		SellTransactionID: entry.transactionID,
		BuyTransactionID:  entry.buyTransactionID,
		UserID:            entry.userID,
		ListingID:         entry.listingID,
		SourceAccountID:   entry.sourceAccountID,
		TaxPeriodStart:    start,
		TaxPeriodEnd:      end,
		TaxAmount:         entry.taxAmount,
		Status:            StatusReserved,
	}
	if err := s.taxRepo.Insert(ctx, charge); err != nil {
		if errors.Is(err, ErrDuplicate) {
			s.logger.Info("skipping duplicate tax charge reservation", "sellTransactionId", entry.transactionID, "buyTransactionId", entry.buyTransactionID)
			return nil, true, nil
		}
		return nil, false, err
	}
	return charge, false, nil
}

// handleFailedChargeAttempt mirrors the Java rollback: if the debit never
// succeeded the reservation is DELETED (so a later run retries it, NOT marked
// FAILED); if the debit succeeded but a later step failed, the row is forced to
// CHARGED.
func (s *Service) handleFailedChargeAttempt(ctx context.Context, reservation *TaxCharge, debitSucceeded bool) {
	if reservation == nil {
		return
	}
	if !debitSucceeded {
		if err := s.taxRepo.Delete(ctx, reservation.ID); err != nil {
			s.logger.Error("failed to delete tax reservation after failed debit", "id", reservation.ID, "error", err)
		}
		return
	}
	if err := s.taxRepo.MarkCharged(ctx, reservation.ID, clockNow()); err != nil {
		s.logger.Error("failed to persist charged tax reservation after successful debit", "id", reservation.ID, "error", err)
	}
}

// --- FIFO cost-basis engine -----------------------------------------------

func (s *Service) buildTaxChargeEntries(ctx context.Context, start, end time.Time, userIDFilter *int64) ([]taxChargeEntry, error) {
	orderCache := map[int64]*order.Order{}
	stockListingCache := map[int64]bool{}
	listingCurrencyCache := map[int64]string{}

	relevantSell, err := s.loadRelevantSellTransactions(ctx, start, end, userIDFilter, orderCache, stockListingCache, listingCurrencyCache)
	if err != nil {
		return nil, err
	}
	if len(relevantSell) == 0 {
		return nil, nil
	}

	transactions, err := s.loadHistoricalTransactionsForRelevantKeys(ctx, end, relevantSell, orderCache, stockListingCache, listingCurrencyCache)
	if err != nil {
		return nil, err
	}
	if len(transactions) == 0 {
		return nil, nil
	}

	// Pre-resolve every order so the sort comparator stays pure (cache-only).
	for _, tx := range transactions {
		s.resolveOrder(ctx, orderCache, tx.OrderID)
	}
	sort.SliceStable(transactions, func(i, j int) bool {
		ti, tj := transactions[i], transactions[j]
		if !ti.Timestamp.Equal(tj.Timestamp) {
			return ti.Timestamp.Before(tj.Timestamp)
		}
		ri := orderDirectionRank(orderCache[ti.OrderID])
		rj := orderDirectionRank(orderCache[tj.OrderID])
		if ri != rj {
			return ri < rj
		}
		return ti.ID < tj.ID
	})

	buyLots := map[userListingKey]*[]buyLot{}
	charges := make([]taxChargeEntry, 0)

	for _, tx := range transactions {
		o := s.resolveOrder(ctx, orderCache, tx.OrderID)
		if o == nil || o.UserID == 0 {
			continue
		}
		if userIDFilter != nil && *userIDFilter != o.UserID {
			continue
		}
		if !s.isStockOrder(ctx, o, stockListingCache, listingCurrencyCache) {
			continue
		}
		if tx.Quantity <= 0 {
			continue
		}

		listingCurrency := listingCurrencyCache[o.ListingID]
		if listingCurrency == "" {
			listingCurrency = "USD"
		}
		key := userListingKey{userID: o.UserID, listingID: o.ListingID}
		lots := buyLots[key]
		if lots == nil {
			fresh := make([]buyLot, 0)
			lots = &fresh
			buyLots[key] = lots
		}

		if o.Direction == order.DirectionBuy {
			*lots = append(*lots, buyLot{
				buyTransactionID:     tx.ID,
				remainingQuantity:    tx.Quantity,
				purchasePricePerUnit: tx.PricePerUnit,
				sourceAccountID:      o.AccountID,
			})
			continue
		}
		if o.Direction != order.DirectionSell {
			continue
		}

		var portfolioAvg *decimal.Decimal
		p, err := s.portfolioRepo.FindByUserIDAndListingID(ctx, s.portfolioRepo.Pool(), o.UserID, o.ListingID)
		if err != nil {
			// Mirrors the Java per-transaction try/catch: log and skip this sell.
			s.logger.Error("error building tax charge entries for transaction", "transactionId", tx.ID, "error", err)
			continue
		}
		if p != nil {
			avg := p.AveragePurchasePrice
			portfolioAvg = &avg
		}
		s.allocateSellTaxLots(&charges, lots, o, tx, start, end, portfolioAvg, listingCurrency)
	}

	return charges, nil
}

func (s *Service) allocateSellTaxLots(charges *[]taxChargeEntry, lots *[]buyLot, sellOrder *order.Order, sellTx order.Transaction, start, end time.Time, portfolioAvgFallback *decimal.Decimal, listingCurrency string) {
	quantityToMatch := sellTx.Quantity

	for quantityToMatch > 0 && len(*lots) > 0 {
		lot := &(*lots)[0]
		matched := quantityToMatch
		if lot.remainingQuantity < matched {
			matched = lot.remainingQuantity
		}
		gainPerShare := sellTx.PricePerUnit.Sub(lot.purchasePricePerUnit)

		if inTaxWindow(sellTx.Timestamp, start, end) && gainPerShare.Sign() > 0 {
			taxableGain := gainPerShare.Mul(decimal.NewFromInt(int64(matched)))
			*charges = append(*charges, taxChargeEntry{
				userID:               sellOrder.UserID,
				listingID:            sellOrder.ListingID,
				transactionID:        sellTx.ID,
				buyTransactionID:     lot.buyTransactionID,
				sourceAccountID:      lot.sourceAccountID,
				taxAmount:            taxableGain.Mul(s.taxRate).Round(2),
				transactionTimestamp: sellTx.Timestamp,
				currency:             listingCurrency,
			})
		}

		quantityToMatch -= matched
		lot.remainingQuantity -= matched
		if lot.remainingQuantity == 0 {
			*lots = (*lots)[1:]
		}
	}

	if quantityToMatch > 0 && portfolioAvgFallback != nil && portfolioAvgFallback.Sign() > 0 {
		// No buy-transaction history — use the portfolio average_purchase_price as
		// cost basis (stocks seeded directly, via OTC exercise, or interbank transfer).
		gainPerShare := sellTx.PricePerUnit.Sub(*portfolioAvgFallback)
		if inTaxWindow(sellTx.Timestamp, start, end) && gainPerShare.Sign() > 0 {
			taxableGain := gainPerShare.Mul(decimal.NewFromInt(int64(quantityToMatch)))
			*charges = append(*charges, taxChargeEntry{
				userID:               sellOrder.UserID,
				listingID:            sellOrder.ListingID,
				transactionID:        sellTx.ID,
				buyTransactionID:     -1,
				sourceAccountID:      sellOrder.AccountID,
				taxAmount:            taxableGain.Mul(s.taxRate).Round(2),
				transactionTimestamp: sellTx.Timestamp,
				currency:             listingCurrency,
			})
		}
	} else if quantityToMatch > 0 {
		s.logger.Warn("unable to fully match sold stock quantity", "transactionId", sellTx.ID, "unmatchedQuantity", quantityToMatch)
	}
}

func (s *Service) loadRelevantSellTransactions(ctx context.Context, start, end time.Time, userIDFilter *int64, orderCache map[int64]*order.Order, stockCache map[int64]bool, currencyCache map[int64]string) ([]order.Transaction, error) {
	var sellOrders []order.Order
	var err error
	if userIDFilter == nil {
		sellOrders, err = s.orderRepo.FindByDirection(ctx, s.orderRepo.Pool(), order.DirectionSell)
	} else {
		sellOrders, err = s.orderRepo.FindByUserIDAndDirection(ctx, s.orderRepo.Pool(), *userIDFilter, order.DirectionSell)
	}
	if err != nil {
		return nil, err
	}
	if len(sellOrders) == 0 {
		return nil, nil
	}

	sellOrderIDs := make([]int64, 0, len(sellOrders))
	for i := range sellOrders {
		o := sellOrders[i]
		orderCache[o.ID] = &o
		sellOrderIDs = append(sellOrderIDs, o.ID)
	}

	txs, err := s.orderRepo.FindTransactionsByOrderIDsAndTimestampBetween(ctx, s.orderRepo.Pool(), sellOrderIDs, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]order.Transaction, 0, len(txs))
	for _, tx := range txs {
		o := s.resolveOrder(ctx, orderCache, tx.OrderID)
		if o != nil && s.isStockOrder(ctx, o, stockCache, currencyCache) {
			out = append(out, tx)
		}
	}
	return out, nil
}

func (s *Service) loadHistoricalTransactionsForRelevantKeys(ctx context.Context, end time.Time, relevantSell []order.Transaction, orderCache map[int64]*order.Order, stockCache map[int64]bool, currencyCache map[int64]string) ([]order.Transaction, error) {
	listingsByUser := map[int64]map[int64]bool{}
	relevantUserIDs := map[int64]bool{}

	for _, sellTx := range relevantSell {
		sellOrder := s.resolveOrder(ctx, orderCache, sellTx.OrderID)
		if sellOrder == nil || sellOrder.UserID == 0 || sellOrder.ListingID == 0 {
			continue
		}
		relevantUserIDs[sellOrder.UserID] = true
		if listingsByUser[sellOrder.UserID] == nil {
			listingsByUser[sellOrder.UserID] = map[int64]bool{}
		}
		listingsByUser[sellOrder.UserID][sellOrder.ListingID] = true
	}
	if len(relevantUserIDs) == 0 {
		return nil, nil
	}

	userIDs := setKeys(relevantUserIDs)
	var candidateOrders []order.Order
	var err error
	if len(userIDs) == 1 {
		candidateOrders, err = s.orderRepo.FindByUserID(ctx, s.orderRepo.Pool(), userIDs[0])
	} else {
		candidateOrders, err = s.orderRepo.FindByUserIDIn(ctx, s.orderRepo.Pool(), userIDs)
	}
	if err != nil {
		return nil, err
	}
	for i := range candidateOrders {
		o := candidateOrders[i]
		orderCache[o.ID] = &o
	}

	orderIDs := make([]int64, 0)
	for i := range candidateOrders {
		o := &candidateOrders[i]
		if belongsToRelevantTaxScope(o, listingsByUser) && s.isStockOrder(ctx, o, stockCache, currencyCache) {
			orderIDs = append(orderIDs, o.ID)
		}
	}
	if len(orderIDs) == 0 {
		return nil, nil
	}

	return s.orderRepo.FindTransactionsByOrderIDsAndTimestampBefore(ctx, s.orderRepo.Pool(), orderIDs, end)
}

func belongsToRelevantTaxScope(o *order.Order, listingsByUser map[int64]map[int64]bool) bool {
	if o == nil || o.UserID == 0 || o.ListingID == 0 {
		return false
	}
	listings := listingsByUser[o.UserID]
	return listings != nil && listings[o.ListingID]
}

// resolveOrder is cache-first with a DB fallback (findById.orElse(null)); found
// orders are cached, misses are not (mirrors Map.computeIfAbsent not storing null).
func (s *Service) resolveOrder(ctx context.Context, cache map[int64]*order.Order, orderID int64) *order.Order {
	if orderID == 0 {
		return nil
	}
	if o, ok := cache[orderID]; ok {
		return o
	}
	o, err := s.orderRepo.FindByID(ctx, s.orderRepo.Pool(), orderID)
	if err != nil {
		s.logger.Debug("tax: order lookup failed", "orderId", orderID, "error", err)
		return nil
	}
	if o != nil {
		cache[orderID] = o
	}
	return o
}

// isStockOrder resolves (and caches) whether a listing is a STOCK and caches its
// currency. Any market-service failure caches "not a stock" + USD and returns
// false — never propagates (matching the Java try/catch).
func (s *Service) isStockOrder(ctx context.Context, o *order.Order, stockCache map[int64]bool, currencyCache map[int64]string) bool {
	if o == nil || o.ListingID == 0 {
		return false
	}
	if v, ok := stockCache[o.ListingID]; ok {
		return v
	}
	listing, err := s.market.GetListing(ctx, o.ListingID)
	if err != nil {
		s.logger.Warn("unable to resolve listing type for listing during tax calculation", "listingId", o.ListingID, "error", err)
		if _, ok := currencyCache[o.ListingID]; !ok {
			currencyCache[o.ListingID] = "USD"
		}
		stockCache[o.ListingID] = false
		return false
	}
	if cur := listing.Currency(); cur != "" {
		currencyCache[o.ListingID] = cur
	} else {
		currencyCache[o.ListingID] = "USD"
	}
	isStock := listing.ListingType != nil && *listing.ListingType == "STOCK"
	stockCache[o.ListingID] = isStock
	return isStock
}

// --- OTC capital-gains ----------------------------------------------------

func (s *Service) loadExercisedOtcTaxEntries(ctx context.Context, endExclusive time.Time) []OtcTaxEntry {
	entries, err := s.taxRepo.LoadExercisedOtcTaxEntries(ctx, endExclusive)
	if err != nil {
		s.logger.Warn("unable to load exercised OTC contracts for tax tracking", "error", err)
		return nil
	}
	return entries
}

func (s *Service) calculateOtcTaxInRsd(ctx context.Context, e OtcTaxEntry) (decimal.Decimal, error) {
	profit := e.SellPricePerStock.Sub(e.AveragePurchasePrice).Mul(decimal.NewFromInt(int64(e.Amount)))
	if profit.Sign() <= 0 {
		return decimal.Zero, nil
	}
	tax := profit.Mul(s.taxRate).Round(2)
	return s.convertOtcTaxToRsd(ctx, tax, e.ContractID)
}

func (s *Service) convertOtcTaxToRsd(ctx context.Context, taxAmount decimal.Decimal, contractID int64) (decimal.Decimal, error) {
	conversion, err := s.market.CalculateWithoutCommission(ctx, "USD", "RSD", taxAmount)
	if err != nil || conversion == nil || conversion.Converted() == nil {
		return decimal.Zero, api.NewOtcError(409, "Failed to convert OTC tax tracking debt to RSD for contract "+strconv.FormatInt(contractID, 10))
	}
	return *conversion.Converted(), nil
}

// --- RSD conversion (tolerant vs strict) ----------------------------------

// convertTaxToRsd is the tolerant variant used during collection: an FX failure or
// missing rate falls back to the original amount (the tax still settles).
func (s *Service) convertTaxToRsd(ctx context.Context, fromCurrency string, taxAmount decimal.Decimal) decimal.Decimal {
	if strings.TrimSpace(fromCurrency) == "" || strings.EqualFold(fromCurrency, "RSD") {
		return taxAmount
	}
	conversion, err := s.market.CalculateWithoutCommission(ctx, fromCurrency, "RSD", taxAmount)
	if err != nil {
		s.logger.Warn("exchange failed while converting tax to RSD; falling back to original amount", "fromCurrency", fromCurrency, "error", err)
		return taxAmount
	}
	if conversion == nil || conversion.Converted() == nil {
		return taxAmount
	}
	return *conversion.Converted()
}

// convertTaxToRsdStrict is the read-path variant: it FAILS (409, IllegalState ->
// OtcExceptionHandler) on a missing currency or any FX failure, so tracking/debt
// reads surface a 409 exactly as the live JVM does. The wrapped message matches
// the outermost IllegalStateException Java rethrows.
func (s *Service) convertTaxToRsdStrict(ctx context.Context, fromCurrency string, taxAmount decimal.Decimal, transactionID int64) (decimal.Decimal, error) {
	if strings.TrimSpace(fromCurrency) == "" {
		return decimal.Zero, api.NewOtcError(409, "Missing account currency for tax tracking entry "+strconv.FormatInt(transactionID, 10))
	}
	if strings.EqualFold(fromCurrency, "RSD") {
		return taxAmount, nil
	}
	conversion, err := s.market.CalculateWithoutCommission(ctx, fromCurrency, "RSD", taxAmount)
	if err != nil || conversion == nil || conversion.Converted() == nil {
		return decimal.Zero, api.NewOtcError(409, "Failed to convert tax tracking debt to RSD for transaction "+strconv.FormatInt(transactionID, 10))
	}
	return *conversion.Converted(), nil
}

// resolveChargeAmountRsd returns the row's RSD amount (the converted column when
// set, else the original taxAmount — the strict RSD passthrough never converts).
func resolveChargeAmountRsd(charge TaxCharge) decimal.Decimal {
	if charge.TaxAmountRsd != nil {
		return *charge.TaxAmountRsd
	}
	return charge.TaxAmount
}

// --- Tracking aggregation -------------------------------------------------

func (s *Service) calculateUnchargedTaxForRangeInRsd(ctx context.Context, userID *int64, start, end time.Time) (decimal.Decimal, error) {
	chargedKeys := map[taxChargeKey]bool{}
	chargedOtcIDs := map[int64]bool{}

	all, err := s.taxRepo.FindAll(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	for _, c := range all {
		if c.Status != StatusCharged {
			continue
		}
		if userID != nil && *userID != c.UserID {
			continue
		}
		chargedKeys[taxChargeKey{sellTransactionID: c.SellTransactionID, buyTransactionID: c.BuyTransactionID}] = true
		if c.OtcContractID != nil {
			chargedOtcIDs[*c.OtcContractID] = true
		}
	}

	entries, err := s.buildTaxChargeEntries(ctx, start, end, userID)
	if err != nil {
		return decimal.Zero, err
	}
	exchangeTax := decimal.Zero
	for _, entry := range entries {
		if chargedKeys[taxChargeKey{sellTransactionID: entry.transactionID, buyTransactionID: entry.buyTransactionID}] {
			continue
		}
		rsd, err := s.convertTaxToRsdStrict(ctx, entry.currency, entry.taxAmount, entry.transactionID)
		if err != nil {
			return decimal.Zero, err
		}
		exchangeTax = exchangeTax.Add(rsd)
	}

	otcTax := decimal.Zero
	for _, e := range s.loadExercisedOtcTaxEntries(ctx, end) {
		if userID != nil && *userID != e.SellerID {
			continue
		}
		if e.ExercisedAt.IsZero() || e.ExercisedAt.Before(start) {
			continue
		}
		if chargedOtcIDs[e.ContractID] {
			continue
		}
		t, err := s.calculateOtcTaxInRsd(ctx, e)
		if err != nil {
			return decimal.Zero, err
		}
		otcTax = otcTax.Add(t)
	}

	return exchangeTax.Add(otcTax), nil
}

func (s *Service) calculateTrackingMetrics(ctx context.Context) (map[int64]*taxTrackingMetrics, error) {
	now := clockNow()
	currentMonthStart := firstOfMonthUTC(now)
	tomorrowStart := startOfDayUTC(now).AddDate(0, 0, 1)
	metricsByUser := map[int64]*taxTrackingMetrics{}
	chargedKeys := map[taxChargeKey]bool{}
	chargedOtcIDs := map[int64]bool{}

	all, err := s.taxRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range all {
		m := ensureMetrics(metricsByUser, c.UserID)
		m.recordCalculation(c.CreatedAt)
		if c.Status == StatusCharged {
			chargedKeys[taxChargeKey{sellTransactionID: c.SellTransactionID, buyTransactionID: c.BuyTransactionID}] = true
			m.addPaid(resolveChargeAmountRsd(c))
			if c.OtcContractID != nil {
				chargedOtcIDs[*c.OtcContractID] = true
			}
		} else if c.Status == StatusFailed {
			m.markFailed()
		}
	}

	entries, err := s.buildTaxChargeEntries(ctx, historyStart, tomorrowStart, nil)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		rsd, err := s.convertTaxToRsdStrict(ctx, entry.currency, entry.taxAmount, entry.transactionID)
		if err != nil {
			return nil, err
		}
		m := ensureMetrics(metricsByUser, entry.userID)
		if !entry.transactionTimestamp.Before(currentMonthStart) {
			m.addCurrentMonthTax(rsd)
		}
		if !chargedKeys[taxChargeKey{sellTransactionID: entry.transactionID, buyTransactionID: entry.buyTransactionID}] {
			m.addDebt(rsd)
		}
	}

	if err := s.addOtcTrackingMetrics(ctx, metricsByUser, currentMonthStart, tomorrowStart, chargedOtcIDs); err != nil {
		return nil, err
	}
	return metricsByUser, nil
}

func (s *Service) addOtcTrackingMetrics(ctx context.Context, metricsByUser map[int64]*taxTrackingMetrics, currentMonthStart, endExclusive time.Time, chargedOtcIDs map[int64]bool) error {
	for _, entry := range s.loadExercisedOtcTaxEntries(ctx, endExclusive) {
		taxInRsd, err := s.calculateOtcTaxInRsd(ctx, entry)
		if err != nil {
			return err
		}
		if taxInRsd.Sign() <= 0 {
			continue
		}
		m := ensureMetrics(metricsByUser, entry.SellerID)
		if !entry.ExercisedAt.IsZero() && !entry.ExercisedAt.Before(currentMonthStart) {
			m.addCurrentMonthTax(taxInRsd)
		}
		if !chargedOtcIDs[entry.ContractID] {
			m.addDebt(taxInRsd)
		}
	}
	return nil
}

func (s *Service) loadClientTrackingRows(ctx context.Context, firstName, lastName *string, metricsByUser map[int64]*taxTrackingMetrics) ([]api.TaxTrackingRowResponse, error) {
	rows := make([]api.TaxTrackingRowResponse, 0)
	page := 0
	for {
		customers, err := s.customer.SearchCustomers(ctx, firstName, lastName, page, trackingPageSize)
		if err != nil {
			return nil, err
		}
		if customers == nil || len(customers.Content) == 0 {
			break
		}
		for _, c := range customers.Content {
			m := metricsOf(metricsByUser, c.ID)
			rows = append(rows, api.TaxTrackingRowResponse{
				FirstName:              c.First(),
				LastName:               c.Last(),
				UserType:               "CLIENT",
				TaxDebtRsd:             m.debt,
				LastTaxCalculationDate: api.LocalDateTimeFromPtr(m.lastCalculationDate),
				CurrentMonthTaxRsd:     m.currentMonthTax,
				TotalPaidTaxRsd:        m.paidTax,
				Status:                 m.status(),
			})
		}
		page++
		if page >= customers.TotalPages {
			break
		}
	}
	return rows, nil
}

func (s *Service) loadActuaryTrackingRows(ctx context.Context, firstName, lastName *string, metricsByUser map[int64]*taxTrackingMetrics) ([]api.TaxTrackingRowResponse, error) {
	rows := make([]api.TaxTrackingRowResponse, 0)
	ids, err := s.actuaryRepo.FindAllEmployeeIDs(ctx)
	if err != nil {
		return nil, err
	}
	for _, employeeID := range ids {
		emp, err := s.employee.GetEmployee(ctx, employeeID)
		if err != nil {
			s.logger.Warn("failed to fetch employee for actuary tax tracking", "employeeId", employeeID, "error", err)
			continue
		}
		if emp == nil {
			continue
		}
		if firstName != nil && strings.TrimSpace(*firstName) != "" {
			if emp.Ime == nil || !strings.Contains(strings.ToLower(*emp.Ime), strings.ToLower(*firstName)) {
				continue
			}
		}
		if lastName != nil && strings.TrimSpace(*lastName) != "" {
			if emp.Prezime == nil || !strings.Contains(strings.ToLower(*emp.Prezime), strings.ToLower(*lastName)) {
				continue
			}
		}
		m := metricsOf(metricsByUser, employeeID)
		rows = append(rows, api.TaxTrackingRowResponse{
			FirstName:              emp.Ime,
			LastName:               emp.Prezime,
			UserType:               "ACTUARY",
			TaxDebtRsd:             m.debt,
			LastTaxCalculationDate: api.LocalDateTimeFromPtr(m.lastCalculationDate),
			CurrentMonthTaxRsd:     m.currentMonthTax,
			TotalPaidTaxRsd:        m.paidTax,
			Status:                 m.status(),
		})
	}
	return rows, nil
}

// --- Notifications --------------------------------------------------------

func (s *Service) createTaxNotificationPayload(ctx context.Context, entry taxChargeEntry, taxInRsd decimal.Decimal) api.TaxCollectedPayload {
	// senderBalance/receiverBalance are omitted: the Go account client discards the
	// transaction response body (as the order client does), and Java only adds those
	// template vars when present. Notifications are side-effects (not parity-gated).
	tv := map[string]string{
		"listingId":     strconv.FormatInt(entry.listingID, 10),
		"transactionId": strconv.FormatInt(entry.transactionID, 10),
		"tax":           entry.taxAmount.String(),
		"taxRsd":        taxInRsd.String(),
	}
	payload := api.TaxCollectedPayload{TemplateVariables: tv}
	s.enrichTaxNotificationPayload(ctx, &payload, entry.userID)
	return payload
}

func (s *Service) enrichTaxNotificationPayload(ctx context.Context, payload *api.TaxCollectedPayload, userID int64) {
	if c, err := s.customer.GetCustomer(ctx, userID); err == nil && c != nil {
		name := buildFullName(c.First(), c.Last())
		payload.Username = &name
		payload.UserEmail = c.Email
		return
	}
	s.logger.Debug("user not resolved via client-service for tax notification", "userId", userID)

	if e, err := s.employee.GetEmployee(ctx, userID); err == nil && e != nil {
		name := buildFullName(e.Ime, e.Prezime)
		payload.Username = &name
		payload.UserEmail = e.Email
		return
	}
	s.logger.Debug("user not resolved via employee-service for tax notification", "userId", userID)
}

func buildFullName(first, last *string) string {
	f := ""
	if first != nil {
		f = strings.TrimSpace(*first)
	}
	l := ""
	if last != nil {
		l = strings.TrimSpace(*last)
	}
	return strings.TrimSpace(f + " " + l)
}

// --- Small helpers + internal types ---------------------------------------

func orderDirectionRank(o *order.Order) int {
	if o == nil || o.Direction == "" {
		return 2
	}
	if o.Direction == order.DirectionBuy {
		return 0
	}
	return 1
}

// inTaxWindow is the half-open [start, end) gain window (start inclusive, end
// exclusive) — mirrors !ts.isBefore(start) && ts.isBefore(end).
func inTaxWindow(ts, start, end time.Time) bool {
	return !ts.Before(start) && ts.Before(end)
}

func startOfDayUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func firstOfMonthUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func firstOfYearUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
}

func sortedInt64Keys(m map[int64]decimal.Decimal) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func setKeys(m map[int64]bool) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// paginate returns the page slice (Java PageImpl(subList(start,end), ...)).
func paginate[T any](all []T, page, size int) []T {
	start := page * size
	if start < 0 || start >= len(all) {
		return []T{}
	}
	end := start + size
	if size <= 0 || end > len(all) {
		end = len(all)
	}
	return all[start:end]
}

type taxChargeEntry struct {
	userID               int64
	listingID            int64
	transactionID        int64
	buyTransactionID     int64
	sourceAccountID      int64
	taxAmount            decimal.Decimal
	transactionTimestamp time.Time
	currency             string
}

type buyLot struct {
	buyTransactionID     int64
	remainingQuantity    int
	purchasePricePerUnit decimal.Decimal
	sourceAccountID      int64
}

type userListingKey struct {
	userID    int64
	listingID int64
}

type taxChargeKey struct {
	sellTransactionID int64
	buyTransactionID  int64
}

type taxTrackingMetrics struct {
	debt                decimal.Decimal
	currentMonthTax     decimal.Decimal
	paidTax             decimal.Decimal
	lastCalculationDate *time.Time
	failed              bool
}

func newMetrics() *taxTrackingMetrics {
	return &taxTrackingMetrics{debt: decimal.Zero, currentMonthTax: decimal.Zero, paidTax: decimal.Zero}
}

func ensureMetrics(m map[int64]*taxTrackingMetrics, id int64) *taxTrackingMetrics {
	got := m[id]
	if got == nil {
		got = newMetrics()
		m[id] = got
	}
	return got
}

// metricsOf returns the user's metrics, or a fresh empty (not stored) one —
// mirrors metricsFor's getOrDefault(EMPTY).
func metricsOf(m map[int64]*taxTrackingMetrics, id int64) *taxTrackingMetrics {
	if got := m[id]; got != nil {
		return got
	}
	return newMetrics()
}

func (m *taxTrackingMetrics) addDebt(amount decimal.Decimal) { m.debt = m.debt.Add(amount) }
func (m *taxTrackingMetrics) addCurrentMonthTax(amount decimal.Decimal) {
	m.currentMonthTax = m.currentMonthTax.Add(amount)
}
func (m *taxTrackingMetrics) addPaid(amount decimal.Decimal) { m.paidTax = m.paidTax.Add(amount) }
func (m *taxTrackingMetrics) markFailed()                    { m.failed = true }

func (m *taxTrackingMetrics) recordCalculation(t time.Time) {
	if m.lastCalculationDate == nil || t.After(*m.lastCalculationDate) {
		tt := t
		m.lastCalculationDate = &tt
	}
}

func (m *taxTrackingMetrics) status() string {
	if m.failed {
		return "FAILED"
	}
	debtPositive := m.debt.Sign() > 0
	paidPositive := m.paidTax.Sign() > 0
	if debtPositive && paidPositive {
		return "PARTIALLY_PAID"
	}
	if debtPositive {
		return "PENDING"
	}
	if paidPositive {
		return "PAID"
	}
	return "ACTIVE"
}
