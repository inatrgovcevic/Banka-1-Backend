package order

import (
	"context"
	"fmt"
	mrand "math/rand/v2"
	"time"

	"banka1/trading-service-go/internal/clients"

	gpdb "banka1/go-platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

const (
	initialExecutionDelay   = 60 * time.Second // first attempt (INITIAL_EXECUTION_DELAY_MILLIS)
	retryDelayOnError       = 5 * time.Second  // RETRY_DELAY_ON_ERROR_MILLIS
	missingQuoteRetryDelay  = 1 * time.Second  // MISSING_QUOTE_RETRY_DELAY_MILLIS
	afterHoursExtraDelay    = 30 * time.Minute // +30 min per portion when after-hours
	executionAttemptTimeout = 60 * time.Second // per-attempt context budget (tx + outbound calls)
)

// ExecuteOrderAsync starts asynchronous execution of an approved order after the
// initial 60s delay (mirrors executeOrderAsync). Called post-commit by confirm /
// approve.
func (s *Service) ExecuteOrderAsync(orderID int64) {
	s.worker.Schedule(orderID, initialExecutionDelay)
}

// processExecutionAttempt is one worker tick (mirrors processExecutionAttempt):
// load → execute one portion in a transaction → reschedule the next portion, a
// retry on error, or stop.
func (s *Service) processExecutionAttempt(orderID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), executionAttemptTimeout)
	defer cancel()

	order, err := s.repo.FindByID(ctx, s.repo.Pool(), orderID)
	if err != nil {
		s.logger.Error("order execution preload failed", "orderId", orderID, "error", err)
		return
	}
	if !executable(order) {
		return
	}

	var fill *portionFill
	if err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		f, e := s.executeOrderPortion(ctx, tx, orderID)
		fill = f
		return e
	}); err != nil {
		s.logger.Error("order execution attempt failed, retrying", "orderId", orderID, "delay", retryDelayOnError, "error", err)
		s.worker.Schedule(orderID, retryDelayOnError)
		return
	}

	updated, err := s.repo.FindByID(ctx, s.repo.Pool(), orderID)
	if err != nil {
		s.logger.Error("order execution reload failed", "orderId", orderID, "error", err)
		return
	}
	// Lifecycle DONE / PARTIAL_FILL notification after the portion commits (mirrors
	// OrderExecutionServiceImpl: event = remainingPortions==0 ? DONE : PARTIAL_FILL).
	// Published only when a portion actually filled this round, and after commit so
	// a rolled-back/retried attempt never double-pushes (OrderEventNotifier's
	// afterCommit guarantee).
	if fill != nil && fill.executed && updated != nil {
		if fill.done {
			s.notifier.OrderDone(ctx, *s.buildLifecyclePayload(ctx, updated, eventDone, fill.ticker, fill.price))
		} else {
			s.notifier.OrderPartialFill(ctx, *s.buildLifecyclePayload(ctx, updated, eventPartialFill, fill.ticker, fill.price))
		}
	}
	if !executable(updated) {
		return
	}
	s.worker.Schedule(orderID, s.calculateExecutionDelay(ctx, updated))
}

// portionFill captures the outcome of one executeOrderPortion call so
// processExecutionAttempt can publish the DONE / PARTIAL_FILL lifecycle
// notification after the transaction commits. executed is false for no-op rounds
// (not eligible, no capacity, missing quote) — those publish nothing.
type portionFill struct {
	executed bool
	done     bool
	ticker   string
	price    decimal.Decimal
}

// executable mirrors the guard repeated across processExecutionAttempt /
// executeOrderPortion: APPROVED, has remaining portions, not done.
func executable(o *Order) bool {
	return o != nil && o.Status == StatusApproved && o.RemainingPortions > 0 && !o.IsDone
}

// executeOrderPortion executes a single portion under row locks
// (order → portfolio → actuary), mirroring OrderExecutionServiceImpl
// .executeOrderPortion. Runs inside RunInTx.
func (s *Service) executeOrderPortion(ctx context.Context, tx pgx.Tx, orderID int64) (*portionFill, error) {
	managed, err := s.repo.FindByIDForUpdate(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	if !executable(managed) {
		return nil, nil
	}

	listing, err := s.market.GetListing(ctx, managed.ListingID)
	if err != nil {
		return nil, err
	}
	if !hasRequiredQuoteData(managed, listing) {
		s.logger.Warn("skipping execution attempt: missing quote data", "orderId", managed.ID)
		return nil, nil
	}

	eligible, err := s.activateIfEligible(ctx, tx, managed, listing)
	if err != nil {
		return nil, err
	}
	if !eligible {
		return nil, nil
	}

	capacity := currentExecutableCapacity(managed, listing)
	if capacity <= 0 {
		return nil, nil
	}
	if managed.AllOrNone && capacity < managed.RemainingPortions {
		return nil, nil
	}

	quantityToExecute := determineExecutionQuantity(managed, capacity)
	executionPrice, ok := calculateExecutionPricePerUnit(managed, listing)
	if !ok {
		return nil, nil
	}

	grossChunk := executionPrice.
		Mul(decimal.NewFromInt(int64(managed.ContractSize))).
		Mul(decimal.NewFromInt(int64(quantityToExecute)))
	commission := s.commission(ctx, orderPricingFamily(managed.OrderType), grossChunk, listing.Currency())

	if err := s.createTransaction(ctx, tx, managed, quantityToExecute, executionPrice, grossChunk, commission); err != nil {
		return nil, err
	}
	accountDebit, err := s.transferFunds(ctx, managed, listing.Currency(), grossChunk)
	if err != nil {
		return nil, err
	}
	// INVESTMENT_FUND BUY: shares go to the fund's holdings table, not the
	// user's portfolio. Mirror Java OrderExecutionServiceImpl.executeOrderPortion
	// (notifyFundLiquidityDebit + notifyFundHolding) via the in-process
	// FundCallback. SELL of a fund-purchased holding still updates the
	// portfolio table (only BUY is the fund path).
	if managed.PurchaseFor != nil && *managed.PurchaseFor == PurchaseForInvestmentFund &&
		managed.Direction == DirectionBuy && managed.FundID != nil {
		if err := s.funds.DebitLiquidity(ctx, *managed.FundID, accountDebit, "Order execution trade leg"); err != nil {
			s.logger.Warn("fund liquidity debit failed", "orderId", managed.ID, "fundId", *managed.FundID, "amount", accountDebit, "error", err)
		}
		ticker := ""
		if listing.Ticker != nil {
			ticker = *listing.Ticker
		}
		if ticker == "" {
			s.logger.Warn("fund order missing ticker — skip notifyFundHolding", "orderId", managed.ID, "listingId", managed.ListingID)
		} else if err := s.funds.AddHolding(ctx, *managed.FundID, ticker, quantityToExecute, executionPrice); err != nil {
			s.logger.Warn("fund add holding failed", "orderId", managed.ID, "fundId", *managed.FundID, "ticker", ticker, "qty", quantityToExecute, "error", err)
		}
	} else {
		if err := s.updatePortfolio(ctx, tx, managed, listing, quantityToExecute, executionPrice); err != nil {
			return nil, err
		}
	}
	if managed.Direction == DirectionSell {
		if err := s.transferSellCommission(ctx, managed, listing.Currency(), commission); err != nil {
			return nil, err
		}
	}
	if err := s.finalizeActuaryExposure(ctx, tx, managed, listing.Currency(), grossChunk); err != nil {
		return nil, err
	}

	managed.RemainingPortions -= quantityToExecute
	done := false
	if managed.RemainingPortions == 0 {
		managed.IsDone = true
		managed.Status = StatusDone
		// Mirror OrderExecutionServiceImpl: stamp executedAt when the final
		// portion fills (migration 004 / Java changeset order:12).
		now := time.Now().UTC()
		managed.ExecutedAt = &now
		done = true
	}
	if err := s.repo.Update(ctx, tx, managed); err != nil {
		return nil, err
	}
	fillTicker := ""
	if listing.Ticker != nil {
		fillTicker = *listing.Ticker
	}
	return &portionFill{executed: true, done: done, ticker: fillTicker, price: executionPrice}, nil
}

// activateIfEligible mirrors activateIfEligible. STOP→MARKET / STOP_LIMIT→LIMIT on
// trigger (persisted within the tx, even if no fill follows this round), else the
// current-market eligibility check for MARKET/LIMIT.
func (s *Service) activateIfEligible(ctx context.Context, tx pgx.Tx, order *Order, listing *clients.StockListing) (bool, error) {
	switch order.OrderType {
	case TypeStop, TypeStopLimit:
		quote := listing.Ask
		if order.Direction == DirectionSell {
			quote = listing.Bid
		}
		if quote == nil { // canEvaluateStop == false
			s.logger.Warn("skipping STOP activation: quote unavailable", "orderId", order.ID, "direction", order.Direction)
			return false, nil
		}
		activated := isStopActivated(order, *quote)
		if !activated {
			return false, nil
		}
		if order.OrderType == TypeStop {
			order.OrderType = TypeMarket
		} else {
			order.OrderType = TypeLimit
		}
		if err := s.repo.Update(ctx, tx, order); err != nil {
			return false, err
		}
		return true, nil
	default:
		return isExecutableAtCurrentMarket(order, listing), nil
	}
}

// isStopActivated mirrors isStopActivated: BUY activates when ask ≥ stop; SELL
// activates when bid < stop.
func isStopActivated(order *Order, quote decimal.Decimal) bool {
	stop := decimal.Zero
	if order.StopValue != nil {
		stop = *order.StopValue
	}
	if order.Direction == DirectionBuy {
		return quote.Cmp(stop) >= 0
	}
	return quote.Cmp(stop) < 0
}

// isExecutableAtCurrentMarket mirrors isExecutableAtCurrentMarket for MARKET/LIMIT.
func isExecutableAtCurrentMarket(order *Order, listing *clients.StockListing) bool {
	if order.OrderType == TypeMarket {
		return true
	}
	if order.OrderType != TypeLimit || order.LimitValue == nil {
		return false
	}
	if order.Direction == DirectionBuy {
		if listing.Ask == nil {
			return false
		}
		return listing.Ask.Cmp(*order.LimitValue) <= 0
	}
	if listing.Bid == nil {
		return false
	}
	return listing.Bid.Cmp(*order.LimitValue) >= 0
}

// currentExecutableCapacity mirrors currentExecutableCapacity: min(volume,
// remaining), volume defaulting to remaining when the listing omits it.
func currentExecutableCapacity(order *Order, listing *clients.StockListing) int {
	volume := int64(order.RemainingPortions)
	if listing.Volume != nil {
		volume = *listing.Volume
	}
	remaining := int64(order.RemainingPortions)
	capacity := volume
	if remaining < capacity {
		capacity = remaining
	}
	if capacity < 0 {
		capacity = 0
	}
	return int(capacity)
}

// determineExecutionQuantity mirrors determineExecutionQuantity: AON fills the
// whole remainder; otherwise a random 1..capacity.
func determineExecutionQuantity(order *Order, capacity int) int {
	if order.AllOrNone {
		return order.RemainingPortions
	}
	return mrand.IntN(capacity) + 1
}

// calculateExecutionPricePerUnit mirrors calculateExecutionPricePerUnit. STOP /
// STOP_LIMIT must already be activated to MARKET / LIMIT by this point.
func calculateExecutionPricePerUnit(order *Order, listing *clients.StockListing) (decimal.Decimal, bool) {
	switch order.OrderType {
	case TypeMarket:
		quote := listing.Ask
		if order.Direction == DirectionSell {
			quote = listing.Bid
		}
		if quote == nil {
			return decimal.Zero, false
		}
		return *quote, true
	case TypeLimit:
		if order.LimitValue == nil {
			return decimal.Zero, false
		}
		if order.Direction == DirectionBuy {
			if listing.Ask == nil {
				return decimal.Zero, false
			}
			return decimalMin(*order.LimitValue, *listing.Ask), true
		}
		if listing.Bid == nil {
			return decimal.Zero, false
		}
		return decimalMax(*order.LimitValue, *listing.Bid), true
	default:
		// STOP / STOP_LIMIT should never reach execution unactivated.
		return decimal.Zero, false
	}
}

func (s *Service) createTransaction(ctx context.Context, tx pgx.Tx, order *Order, quantity int, price, gross, commission decimal.Decimal) error {
	return s.repo.InsertTransaction(ctx, tx, &Transaction{
		OrderID:      order.ID,
		Quantity:     quantity,
		PricePerUnit: price,
		TotalPrice:   gross,
		Commission:   commission,
	})
}

// updatePortfolio mirrors updatePortfolio: BUY merges with weighted-average cost
// (scale 4 HALF_UP); SELL decrements and deletes on zero. Locks the position FOR
// UPDATE within the execution transaction.
func (s *Service) updatePortfolio(ctx context.Context, tx pgx.Tx, order *Order, listing *clients.StockListing, quantity int, price decimal.Decimal) error {
	p, err := s.portfolios.FindByUserIDAndListingIDForUpdate(ctx, tx, order.UserID, order.ListingID)
	if err != nil {
		return err
	}

	if order.Direction == DirectionBuy {
		if p == nil {
			return s.portfolios.Insert(ctx, tx, order.UserID, order.ListingID, listing.ListingTypeOr("STOCK"), quantity, price)
		}
		totalValue := p.AveragePurchasePrice.Mul(decimal.NewFromInt(int64(p.Quantity))).
			Add(price.Mul(decimal.NewFromInt(int64(quantity))))
		newQuantity := p.Quantity + quantity
		newAvg := totalValue.DivRound(decimal.NewFromInt(int64(newQuantity)), 4)
		return s.portfolios.UpdateQuantityAndAvg(ctx, tx, p.ID, newQuantity, newAvg)
	}

	// SELL
	if p == nil || p.Quantity < quantity {
		return fmt.Errorf("order: cannot execute sell order %d without owned quantity", order.ID)
	}
	newReserved := p.ReservedQuantity - quantity
	if newReserved < 0 {
		newReserved = 0
	}
	newQuantity := p.Quantity - quantity
	newPublic := p.PublicQuantity
	if newPublic > newQuantity {
		newPublic = newQuantity
	}
	if newQuantity == 0 && newReserved == 0 {
		return s.portfolios.Delete(ctx, tx, p.ID)
	}
	return s.portfolios.UpdateSellPosition(ctx, tx, p.ID, newQuantity, newReserved, newPublic)
}

// transferFunds mirrors transferFunds: the one-sided trade leg (GHI #199). BANK
// orders convert without commission; others (clients + INVESTMENT_FUND) with
// commission. BUY debits, SELL credits the user/fund account. Returns the
// amount actually debited in the account currency — needed by the
// INVESTMENT_FUND callback so the funds cached-liquidity mirror matches the
// banking-core debit to the cent.
func (s *Service) transferFunds(ctx context.Context, order *Order, currency string, amount decimal.Decimal) (decimal.Decimal, error) {
	userAccount, err := s.account.GetAccountDetailsByID(ctx, order.AccountID)
	if err != nil {
		return decimal.Zero, err
	}
	var accountAmount decimal.Decimal
	if order.PurchaseFor != nil && *order.PurchaseFor == PurchaseForBank {
		accountAmount, err = s.convertTradeAmountToAccountCurrencyNoComm(ctx, userAccount, currency, amount)
	} else {
		accountAmount, err = s.convertTradeAmountToAccountCurrency(ctx, userAccount, currency, amount)
	}
	if err != nil {
		return decimal.Zero, err
	}
	if order.Margin {
		// Margin orders: route through banking-core margin transaction instead of
		// the regular one-sided exchange debit/credit. Banking-core handles the
		// split between client's initialMargin and the bank-loaned portion.
		if order.Direction == DirectionBuy {
			if err := s.account.StockBuyMarginTransaction(ctx, order.UserID, accountAmount); err != nil {
				return decimal.Zero, err
			}
		} else {
			if err := s.account.StockSellMarginTransaction(ctx, order.UserID, accountAmount); err != nil {
				return decimal.Zero, err
			}
		}
		return accountAmount, nil
	}

	req := clients.OneSidedTransaction{
		AccountNumber: userAccount.Number(),
		AccountID:     order.AccountID,
		Amount:        accountAmount,
		ClientID:      userAccount.OwnerIDValue(),
		Description:   "Order execution trade leg (one-sided, GHI #199)",
	}
	if order.Direction == DirectionBuy {
		if err := s.account.ExchangeBuy(ctx, req); err != nil {
			return decimal.Zero, err
		}
	} else {
		if err := s.account.ExchangeSell(ctx, req); err != nil {
			return decimal.Zero, err
		}
	}
	return accountAmount, nil
}

// transferSellCommission mirrors transferSellCommission: bills the user account
// the commission to the bank account (skipped when the order is funded by the
// bank account itself).
func (s *Service) transferSellCommission(ctx context.Context, order *Order, currency string, commission decimal.Decimal) error {
	if commission.Sign() <= 0 {
		return nil
	}
	bankAccount, err := s.account.GetBankAccount(ctx, currency)
	if err != nil {
		return err
	}
	if bankAccount.ResolvedID() != 0 && bankAccount.ResolvedID() == order.AccountID {
		return nil
	}
	userAccount, err := s.account.GetAccountDetailsByID(ctx, order.AccountID)
	if err != nil {
		return err
	}
	bankDetails, err := s.account.GetAccountDetailsByID(ctx, bankAccount.ResolvedID())
	if err != nil {
		return err
	}
	fromAmount, err := s.convertTradeAmountToAccountCurrency(ctx, userAccount, currency, commission)
	if err != nil {
		return err
	}
	return s.account.Transaction(ctx, clients.Payment{
		FromAccountNumber: userAccount.Number(),
		ToAccountNumber:   bankDetails.Number(),
		FromAmount:        fromAmount,
		ToAmount:          commission,
		Commission:        decimal.Zero,
		ClientID:          userAccount.OwnerIDValue(),
	})
}

// finalizeActuaryExposure mirrors finalizeActuaryExposure: when the order is
// funded by the bank account (agent buying with bank funds), move the RSD
// exposure from reserved_limit to used_limit. Locks actuary_info FOR UPDATE.
func (s *Service) finalizeActuaryExposure(ctx context.Context, tx pgx.Tx, order *Order, currency string, amount decimal.Decimal) error {
	bankAccount, err := s.account.GetBankAccount(ctx, currency)
	if err != nil {
		return err
	}
	if bankAccount.ResolvedID() == 0 || bankAccount.ResolvedID() != order.AccountID {
		return nil
	}
	info, err := s.actuaries.FindByEmployeeIDForUpdate(ctx, tx, order.UserID)
	if err != nil {
		return err
	}
	if info == nil {
		return nil
	}
	converted, err := s.convertAmountNoComm(ctx, currency, limitCurrency, amount)
	if err != nil {
		return err
	}
	newReserved := decimalMaxZero(info.ReservedLimit.Sub(converted))
	newUsed := info.UsedLimit.Add(converted)
	if err := s.actuaries.UpdateReservedAndUsedLimit(ctx, tx, order.UserID, newReserved, newUsed); err != nil {
		return err
	}
	order.ReservedLimitExposure = decimalMaxZero(order.ReservedLimitExposure.Sub(converted))
	return nil
}

// calculateExecutionDelay mirrors calculateExecutionDelay: a random interval up
// to (24*60)/(volume/remaining) seconds, plus 30 min when after-hours; 1s backoff
// when the quote is missing.
func (s *Service) calculateExecutionDelay(ctx context.Context, order *Order) time.Duration {
	listing, err := s.market.GetListing(ctx, order.ListingID)
	if err != nil || listing == nil || !hasRequiredQuoteData(order, listing) {
		return missingQuoteRetryDelay
	}
	volume := int64(1)
	if listing.Volume != nil && *listing.Volume > 0 {
		volume = *listing.Volume
	}
	remaining := order.RemainingPortions
	if remaining < 1 {
		remaining = 1
	}
	maxSeconds := (24.0 * 60.0) / (float64(volume) / float64(remaining))
	delaySeconds := mrand.Float64() * maxSeconds
	delay := time.Duration(delaySeconds * float64(time.Second))
	if order.AfterHours {
		delay += afterHoursExtraDelay
	}
	return delay
}

// hasRequiredQuoteData mirrors hasRequiredQuoteData: a price plus the directional
// quote (ask for BUY, bid for SELL).
func hasRequiredQuoteData(order *Order, listing *clients.StockListing) bool {
	if listing == nil || listing.Price == nil {
		return false
	}
	if order.Direction == DirectionBuy {
		return listing.Ask != nil
	}
	return listing.Bid != nil
}

func (s *Service) convertTradeAmountToAccountCurrency(ctx context.Context, userAccount *clients.AccountDetails, tradeCurrency string, tradeAmount decimal.Decimal) (decimal.Decimal, error) {
	return s.convertAmount(ctx, tradeCurrency, userAccount.CurrencyOrEmpty(), tradeAmount)
}

func (s *Service) convertTradeAmountToAccountCurrencyNoComm(ctx context.Context, userAccount *clients.AccountDetails, tradeCurrency string, tradeAmount decimal.Decimal) (decimal.Decimal, error) {
	return s.convertAmountNoComm(ctx, tradeCurrency, userAccount.CurrencyOrEmpty(), tradeAmount)
}

func decimalMin(a, b decimal.Decimal) decimal.Decimal {
	if a.Cmp(b) <= 0 {
		return a
	}
	return b
}

func decimalMax(a, b decimal.Decimal) decimal.Decimal {
	if a.Cmp(b) >= 0 {
		return a
	}
	return b
}

func decimalMaxZero(a decimal.Decimal) decimal.Decimal {
	if a.Sign() < 0 {
		return decimal.Zero
	}
	return a
}
