package order

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"banka1/trading-service-go/internal/actuary"
	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/audit"
	"banka1/trading-service-go/internal/clients"
	"banka1/trading-service-go/internal/portfolio"

	gpdb "banka1/go-platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// FundCallback is the order-side dependency interface for the funds domain.
// Mirrors order-service TradingServiceClient (Java HTTP client to trading-
// service): records a holdings row after an INVESTMENT_FUND BUY executes and
// debits cached fund liquidity when the fund's bank account is spent. The
// real implementation lives in package funds (funds.ServiceCallback); a
// NoopFundCallback falls in when funds is not wired (kept for tests).
type FundCallback interface {
	AddHolding(ctx context.Context, fundID int64, ticker string, quantity int, unitPrice decimal.Decimal) error
	DebitLiquidity(ctx context.Context, fundID int64, amount decimal.Decimal, reason string) error
}

// NoopFundCallback discards every call. Used when no funds binding is wired
// (the BUY simply skips the cached-state side effects; the order still
// executes against the fund's bank account at banking-core).
type NoopFundCallback struct{}

func (NoopFundCallback) AddHolding(context.Context, int64, string, int, decimal.Decimal) error {
	return nil
}
func (NoopFundCallback) DebitLiquidity(context.Context, int64, decimal.Decimal, string) error {
	return nil
}

// Service implements the /orders operations (creation + lifecycle) and drives the
// execution worker. Mirrors order-service OrderCreationServiceImpl +
// OrderExecutionServiceImpl (unified here since they share the same clients and
// the worker calls back into the service).
type Service struct {
	repo       *Repository
	portfolios *portfolio.Repository
	actuaries  *actuary.Repository
	market     *clients.MarketClient
	account    *clients.AccountClient
	employees  *clients.EmployeeClient
	customers  *clients.CustomerClient
	notifier   Notifier
	funds      FundCallback
	auditor    *audit.Service
	worker     *Worker
	logger     *slog.Logger
}

// NewService wires the order service and its execution worker (pool size 4,
// matching the Java ThreadPoolTaskScheduler). Call Start before serving and Stop
// on shutdown.
func NewService(repo *Repository, portfolios *portfolio.Repository, actuaries *actuary.Repository,
	cl *clients.Clients, notifier Notifier, fundCallback FundCallback, auditor *audit.Service, logger *slog.Logger) *Service {
	if notifier == nil {
		notifier = NoopNotifier{}
	}
	if fundCallback == nil {
		fundCallback = NoopFundCallback{}
	}
	s := &Service{
		repo:       repo,
		portfolios: portfolios,
		actuaries:  actuaries,
		market:     cl.Market,
		account:    cl.Account,
		employees:  cl.Employee,
		customers:  cl.Customer,
		notifier:   notifier,
		funds:      fundCallback,
		auditor:    auditor,
		logger:     logger,
	}
	s.worker = NewWorker(s.processExecutionAttempt, logger, 4)
	return s
}

// Start launches the execution worker pool.
func (s *Service) Start() { s.worker.Start() }

// Stop drains the execution worker pool.
func (s *Service) Stop() { s.worker.Stop() }

// --- Creation -------------------------------------------------------------

// CreateBuyOrder mirrors OrderCreationServiceImpl.createBuyOrder. Supports
// standard, BANK, and INVESTMENT_FUND orders (P5 re-enabled INVESTMENT_FUND —
// fund holdings/liquidity callbacks fire from execution.go via FundCallback).
func (s *Service) CreateBuyOrder(ctx context.Context, user AuthUser, req api.CreateBuyOrderRequest) (api.OrderResponse, error) {
	if err := validateCommonRequest(req.ListingID, req.Quantity, req.LimitValue, req.StopValue); err != nil {
		return api.OrderResponse{}, err
	}
	purchaseFor, err := parsePurchaseFor(req.PurchaseFor)
	if err != nil {
		return api.OrderResponse{}, err
	}
	isFund := purchaseFor == PurchaseForInvestmentFund
	isBank := purchaseFor == PurchaseForBank
	if isFund || isBank {
		if user.IsClient() {
			return api.OrderResponse{}, api.NewOrderError(403, "Clients cannot buy securities for institutional accounts")
		}
		if isFund && req.FundID == nil {
			return api.OrderResponse{}, api.NewOrderError(400, "fundId is required when purchaseFor is INVESTMENT_FUND")
		}
		if isFund && req.AccountID == nil {
			return api.OrderResponse{}, api.NewOrderError(400, "accountId is required for fund buy orders")
		}
	}

	listing, err := s.market.GetListing(ctx, *req.ListingID)
	if err != nil {
		return api.OrderResponse{}, err
	}
	if err := s.validateTradingAccess(user, listing); err != nil {
		return api.OrderResponse{}, err
	}
	currency := listing.Currency()
	closed, afterHours, err := s.resolveExchangeWindow(ctx, listing)
	if err != nil {
		return api.OrderResponse{}, err
	}

	var accountID int64
	if isFund {
		// Fund order uses the fund's RSD account, already chosen by the
		// supervisor (validated above as non-nil).
		accountID = *req.AccountID
	} else if isBank {
		bank, err := s.account.GetBankAccount(ctx, currency)
		if err != nil {
			return api.OrderResponse{}, err
		}
		accountID = bank.ResolvedID()
	} else {
		accountID, err = s.initialBuyAccountID(ctx, user, req.AccountID, currency)
		if err != nil {
			return api.OrderResponse{}, err
		}
		if user.IsClient() {
			if err := s.validateClientAccount(ctx, user.UserID, accountID); err != nil {
				return api.OrderResponse{}, err
			}
		}
	}

	orderType := determineOrderType(req.LimitValue, req.StopValue)
	approx, err := calculateApproximatePrice(orderType, DirectionBuy, listing, *req.Quantity, req.LimitValue, req.StopValue)
	if err != nil {
		return api.OrderResponse{}, err
	}
	fee := s.commission(ctx, orderType, approx, currency)
	// Issue #255: investment-fund orders are exempt from the brokerage commission —
	// only the security price may leave the fund's account (chargeableFee = 0).
	if isFund {
		fee = decimal.Zero
	}
	// Scenario 63: reject margin orders for clients without MARGIN_TRADE permission.
	if boolValue(req.Margin) && user.IsClient() && !user.HasMarginPermission() {
		return api.OrderResponse{}, api.NewOrderError(409, "User does not have margin permission")
	}

	// At creation Java checks funds for institutional (BANK / INVESTMENT_FUND)
	// or client orders, non-margin; a plain agent's funds are verified later
	// at confirm.
	if (isFund || isBank || user.IsClient()) && !boolValue(req.Margin) {
		if err := s.checkFunds(ctx, accountID, approx.Add(fee), currency); err != nil {
			return api.OrderResponse{}, err
		}
	}

	order, err := s.buildBaseOrder(user.UserID, *req.ListingID, orderType, *req.Quantity, listing,
		req.LimitValue, req.StopValue, DirectionBuy, req.AllOrNone, req.Margin, accountID, closed, afterHours)
	if err != nil {
		return api.OrderResponse{}, err
	}
	if isFund {
		pf := PurchaseForInvestmentFund
		order.PurchaseFor = &pf
		order.FundID = req.FundID
	} else if isBank {
		pf := PurchaseForBank
		order.PurchaseFor = &pf
	}
	order.Status = StatusPendingConfirmation
	order.ApprovedBy = nil
	if err := s.repo.Insert(ctx, s.repo.Pool(), order); err != nil {
		return api.OrderResponse{}, err
	}
	return s.mapToResponse(order, approx, fee, closed), nil
}

// CreateSellOrder mirrors createSellOrder.
func (s *Service) CreateSellOrder(ctx context.Context, user AuthUser, req api.CreateSellOrderRequest) (api.OrderResponse, error) {
	if req.AccountID == nil {
		return api.OrderResponse{}, api.NewOrderError(400, "Invalid request parameters")
	}
	if err := validateCommonRequest(req.ListingID, req.Quantity, req.LimitValue, req.StopValue); err != nil {
		return api.OrderResponse{}, err
	}
	listing, err := s.market.GetListing(ctx, *req.ListingID)
	if err != nil {
		return api.OrderResponse{}, err
	}
	if err := s.validateTradingAccess(user, listing); err != nil {
		return api.OrderResponse{}, err
	}
	if err := s.ensurePortfolioOwnership(ctx, user.UserID, *req.ListingID, *req.Quantity); err != nil {
		return api.OrderResponse{}, err
	}
	currency := listing.Currency()
	closed, afterHours, err := s.resolveExchangeWindow(ctx, listing)
	if err != nil {
		return api.OrderResponse{}, err
	}
	orderType := determineOrderType(req.LimitValue, req.StopValue)
	approx, err := calculateApproximatePrice(orderType, DirectionSell, listing, *req.Quantity, req.LimitValue, req.StopValue)
	if err != nil {
		return api.OrderResponse{}, err
	}
	fee := s.commission(ctx, orderType, approx, currency)

	order, err := s.buildBaseOrder(user.UserID, *req.ListingID, orderType, *req.Quantity, listing,
		req.LimitValue, req.StopValue, DirectionSell, req.AllOrNone, req.Margin, *req.AccountID, closed, afterHours)
	if err != nil {
		return api.OrderResponse{}, err
	}
	order.Status = StatusPendingConfirmation
	order.ApprovedBy = nil
	if err := s.repo.Insert(ctx, s.repo.Pool(), order); err != nil {
		return api.OrderResponse{}, err
	}
	return s.mapToResponse(order, approx, fee, closed), nil
}

// --- Reads ----------------------------------------------------------------

// GetOrders mirrors getOrders: the supervisor portal overview. ALL excludes
// drafts (PENDING_CONFIRMATION); a status filter selects exactly that status.
func (s *Service) GetOrders(ctx context.Context, statusFilter string, page, size int) (api.Page[api.OrderOverviewResponse], error) {
	var orders []Order
	var err error
	if statusFilter == "" || statusFilter == "ALL" {
		all, e := s.repo.FindAll(ctx, s.repo.Pool())
		if e != nil {
			return api.Page[api.OrderOverviewResponse]{}, e
		}
		for _, o := range all {
			if o.Status != StatusPendingConfirmation {
				orders = append(orders, o)
			}
		}
	} else {
		orders, err = s.repo.FindByStatus(ctx, s.repo.Pool(), statusFilter)
		if err != nil {
			return api.Page[api.OrderOverviewResponse]{}, err
		}
	}

	listingIDs := map[int64]bool{}
	userIDs := map[int64]bool{}
	for _, o := range orders {
		listingIDs[o.ListingID] = true
		userIDs[o.UserID] = true
	}
	listingCache := map[int64]*clients.StockListing{}
	for id := range listingIDs {
		listing, err := s.market.GetListing(ctx, id)
		if err != nil {
			return api.Page[api.OrderOverviewResponse]{}, err
		}
		listingCache[id] = listing
	}
	ids := make([]int64, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}
	actuaryIDs, err := s.actuaries.FindEmployeeIDsIn(ctx, ids)
	if err != nil {
		return api.Page[api.OrderOverviewResponse]{}, err
	}
	employeeCache := map[int64]*clients.Employee{}

	rows := make([]api.OrderOverviewResponse, 0, len(orders))
	for i := range orders {
		rows = append(rows, s.mapToOverviewResponse(ctx, &orders[i], listingCache, employeeCache, actuaryIDs))
	}

	total := len(rows)
	slice := make([]api.OrderOverviewResponse, 0)
	if start := page * size; start < total {
		end := start + size
		if end > total {
			end = total
		}
		slice = rows[start:end]
	}
	return api.NewPage(slice, page, size, int64(total)), nil
}

// GetMyOrders mirrors getMyOrders: a client's own orders.
func (s *Service) GetMyOrders(ctx context.Context, user AuthUser) ([]api.OrderResponse, error) {
	if !user.IsClient() {
		return nil, api.NewOrderError(403, "Only clients can view their orders")
	}
	orders, err := s.repo.FindByUserID(ctx, s.repo.Pool(), user.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]api.OrderResponse, 0, len(orders))
	for i := range orders {
		resp, err := s.mapStoredOrderToResponse(ctx, &orders[i])
		if err != nil {
			return nil, err
		}
		out = append(out, resp)
	}
	return out, nil
}

// GetMyOrdersPaged mirrors getMyOrdersPaged: the filtered, paginated mobile My
// Orders view. Loads the client's orders, filters by status / listing type /
// created-at date range, sorts by createdAt desc, enriches each row with the
// listing ticker/name/type and the weighted-average execution price, then pages
// in memory (matching the Java PageImpl slicing).
func (s *Service) GetMyOrdersPaged(ctx context.Context, user AuthUser, statusFilter string, listingType *string, dateFrom, dateTo *time.Time, page, size int) (api.Page[api.OrderResponse], error) {
	if !user.IsClient() {
		return api.Page[api.OrderResponse]{}, api.NewOrderError(403, "Only clients can view their orders")
	}
	orders, err := s.repo.FindByUserID(ctx, s.repo.Pool(), user.UserID)
	if err != nil {
		return api.Page[api.OrderResponse]{}, err
	}

	listingCache := map[int64]*clients.StockListing{}
	for i := range orders {
		id := orders[i].ListingID
		if _, ok := listingCache[id]; !ok {
			listing, err := s.market.GetListing(ctx, id)
			if err != nil {
				return api.Page[api.OrderResponse]{}, err
			}
			listingCache[id] = listing
		}
	}

	filtered := make([]*Order, 0, len(orders))
	for i := range orders {
		o := &orders[i]
		if !matchesStatusFilter(o, statusFilter) ||
			!matchesListingTypeFilter(o, listingType, listingCache) ||
			!matchesDateRange(o, dateFrom, dateTo) {
			continue
		}
		filtered = append(filtered, o)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	rows := make([]api.OrderResponse, 0, len(filtered))
	for _, o := range filtered {
		resp, err := s.enrichStoredOrderResponse(ctx, o, listingCache[o.ListingID])
		if err != nil {
			return api.Page[api.OrderResponse]{}, err
		}
		rows = append(rows, resp)
	}

	total := len(rows)
	slice := make([]api.OrderResponse, 0)
	if start := page * size; start < total {
		end := start + size
		if end > total {
			end = total
		}
		slice = rows[start:end]
	}
	return api.NewPage(slice, page, size, int64(total)), nil
}

func matchesStatusFilter(o *Order, statusFilter string) bool {
	if statusFilter == "" || statusFilter == "ALL" {
		return true
	}
	return o.Status == statusFilter
}

func matchesListingTypeFilter(o *Order, listingType *string, listingCache map[int64]*clients.StockListing) bool {
	if listingType == nil {
		return true
	}
	listing := listingCache[o.ListingID]
	return listing != nil && listing.ListingType != nil && *listing.ListingType == *listingType
}

func matchesDateRange(o *Order, dateFrom, dateTo *time.Time) bool {
	if o.CreatedAt.IsZero() {
		return dateFrom == nil && dateTo == nil
	}
	created := o.CreatedAt.UTC()
	createdDate := time.Date(created.Year(), created.Month(), created.Day(), 0, 0, 0, 0, time.UTC)
	if dateFrom != nil && createdDate.Before(*dateFrom) {
		return false
	}
	return dateTo == nil || !createdDate.After(*dateTo)
}

// enrichStoredOrderResponse mirrors enrichStoredOrderResponse: base response plus
// ticker / security name / listing type from the cached listing and the realized
// execution price computed from recorded transactions.
func (s *Service) enrichStoredOrderResponse(ctx context.Context, order *Order, listing *clients.StockListing) (api.OrderResponse, error) {
	approx := order.PricePerUnit.
		Mul(decimal.NewFromInt(int64(order.ContractSize))).
		Mul(decimal.NewFromInt(int64(order.Quantity)))
	currency := ""
	if listing != nil {
		currency = listing.Currency()
	}
	fee := s.commission(ctx, orderPricingFamily(order.OrderType), approx, currency)
	resp := s.mapToResponse(order, approx, fee, order.ExchangeClosed)
	if listing != nil {
		resp.Ticker = listing.Ticker
		resp.SecurityName = listing.Name
		resp.ListingType = listing.ListingType
	}
	execPrice, err := s.calculateExecutionPrice(ctx, order.ID)
	if err != nil {
		return api.OrderResponse{}, err
	}
	resp.ExecutionPrice = execPrice
	return resp, nil
}

// calculateExecutionPrice mirrors calculateExecutionPrice: weighted-average
// execution price per unit across all recorded fills, or nil when none.
func (s *Service) calculateExecutionPrice(ctx context.Context, orderID int64) (*decimal.Decimal, error) {
	transactions, err := s.repo.FindTransactionsByOrderID(ctx, s.repo.Pool(), orderID)
	if err != nil {
		return nil, err
	}
	weightedSum := decimal.Zero
	var totalQuantity int64
	for i := range transactions {
		t := &transactions[i]
		weightedSum = weightedSum.Add(t.PricePerUnit.Mul(decimal.NewFromInt(int64(t.Quantity))))
		totalQuantity += int64(t.Quantity)
	}
	if totalQuantity == 0 {
		return nil, nil
	}
	avg := weightedSum.DivRound(decimal.NewFromInt(totalQuantity), 4)
	return &avg, nil
}

// --- Confirm / approve / decline / cancel ---------------------------------

// ConfirmOrder mirrors confirmOrder: finalize a draft, decide APPROVED vs PENDING
// (agents), reserve limit/sell quantity, transfer the fee, and (if approved)
// schedule execution after commit.
func (s *Service) ConfirmOrder(ctx context.Context, user AuthUser, orderID int64) (api.OrderResponse, error) {
	var (
		result         *Order
		approx, fee    decimal.Decimal
		exchangeClosed bool
		triggerExec    bool
		createdTicker  string
		emitCreated    bool
	)
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		order, err := s.repo.FindByID(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if order == nil {
			return api.NewOrderError(404, "Order not found")
		}
		if order.UserID != user.UserID {
			return api.NewOrderError(403, "Order does not belong to the authenticated user")
		}
		if order.Status != StatusPendingConfirmation {
			return api.NewOrderError(409, "Only draft orders can be confirmed")
		}
		listing, err := s.market.GetListing(ctx, order.ListingID)
		if err != nil {
			return err
		}
		if err := s.validateTradingAccess(user, listing); err != nil {
			return err
		}
		currency := listing.Currency()
		ap, err := calculateApproximatePrice(order.OrderType, order.Direction, listing, order.Quantity, order.LimitValue, order.StopValue)
		if err != nil {
			return err
		}
		f := s.commission(ctx, orderPricingFamily(order.OrderType), ap, currency)
		// Issue #255: investment-fund orders are exempt from the brokerage
		// commission (chargeableFee = 0 in funds checks and the response).
		isFundOrder := order.PurchaseFor != nil && *order.PurchaseFor == PurchaseForInvestmentFund
		if isFundOrder {
			f = decimal.Zero
		}
		approx, fee, exchangeClosed = ap, f, order.ExchangeClosed

		if hasPastSettlementDate(listing) {
			order.Status = StatusDeclined
			sa := systemApproval
			order.ApprovedBy = &sa
			order.RemainingPortions = 0
			order.IsDone = true
			if err := s.repo.Update(ctx, tx, order); err != nil {
				return err
			}
			result = order
			return nil
		}

		var fundingAccountID int64
		switch {
		case order.PurchaseFor != nil && *order.PurchaseFor == PurchaseForInvestmentFund:
			fundingAccountID = order.AccountID
		case user.IsClient():
			fundingAccountID = order.AccountID
		default:
			fundingAccountID, err = s.determineFundingAccountID(ctx, order.UserID, &order.AccountID, currency)
			if err != nil {
				return err
			}
			if order.Direction == DirectionBuy {
				order.AccountID = fundingAccountID
			}
		}

		if order.Margin {
			if err := s.checkMarginRequirements(ctx, user, fundingAccountID, listing, order.Quantity); err != nil {
				return err
			}
		} else if order.Direction == DirectionBuy {
			if err := s.checkFunds(ctx, fundingAccountID, ap.Add(f), currency); err != nil {
				return err
			}
		}

		var status string
		var reservedExposure decimal.Decimal
		if user.IsClient() {
			status, reservedExposure = StatusApproved, decimal.Zero
		} else {
			status, reservedExposure, err = s.determineOrderStatusAndReserveExposure(ctx, tx, order.UserID, ap, currency)
			if err != nil {
				return err
			}
		}
		if err := s.reserveSellQuantityIfNeeded(ctx, tx, order); err != nil {
			return err
		}
		if status == StatusApproved {
			// Issue #255: investment-fund orders are commission-exempt — the
			// fund's account is debited the security price only, so the fee leg
			// (and the old cached-liquidity fee mirror) is skipped entirely.
			if order.Direction == DirectionBuy && !isFundOrder {
				if _, err := s.transferFee(ctx, fundingAccountID, f, currency, user.IsClient()); err != nil {
					return err
				}
			}
			s.market.RefreshListing(ctx, order.ListingID)
		}

		order.Status = status
		if status == StatusApproved {
			v := noApprovalRequired
			order.ApprovedBy = &v
		} else {
			order.ApprovedBy = nil
		}
		order.ReservedLimitExposure = reservedExposure
		if err := s.repo.Update(ctx, tx, order); err != nil {
			return err
		}
		result = order
		triggerExec = status == StatusApproved
		if listing.Ticker != nil {
			createdTicker = *listing.Ticker
		}
		emitCreated = true
		return nil
	})
	if err != nil {
		return api.OrderResponse{}, err
	}
	// Lifecycle CREATED notification (mirrors OrderCreationServiceImpl.confirmOrder
	// → notifyOrderEvent(CREATED) after save). Published after commit, before the
	// execution is scheduled — matching OrderEventNotifier's afterCommit ordering.
	if emitCreated && result != nil {
		s.notifier.OrderCreated(ctx, *s.buildLifecyclePayload(ctx, result, eventCreated, createdTicker, result.PricePerUnit))
	}
	if triggerExec {
		s.ExecuteOrderAsync(result.ID)
	}
	return s.mapToResponse(result, approx, fee, exchangeClosed), nil
}

// ApproveOrder mirrors approveOrder (supervisor). Publishes the approval and
// schedules execution after commit.
func (s *Service) ApproveOrder(ctx context.Context, supervisorID, orderID int64) (api.OrderResponse, error) {
	var (
		result      *Order
		approx, fee decimal.Decimal
		notify      *api.OrderNotificationPayload
	)
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		order, err := s.repo.FindByID(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if order == nil {
			return api.NewOrderError(404, "Order not found")
		}
		if order.Status != StatusPending {
			return api.NewOrderError(409, "Only pending orders can be approved")
		}
		listing, err := s.market.GetListing(ctx, order.ListingID)
		if err != nil {
			return err
		}
		if hasPastSettlementDate(listing) {
			return api.NewOrderError(409, "Orders with past settlement date can only be declined")
		}
		currency := listing.Currency()
		ap, err := calculateApproximatePrice(order.OrderType, order.Direction, listing, order.Quantity, order.LimitValue, order.StopValue)
		if err != nil {
			return err
		}
		f := s.commission(ctx, orderPricingFamily(order.OrderType), ap, currency)
		// Issue #255: investment-fund orders are exempt from the brokerage commission.
		isFundOrder := order.PurchaseFor != nil && *order.PurchaseFor == PurchaseForInvestmentFund
		if isFundOrder {
			f = decimal.Zero
		}
		approx, fee = ap, f

		fundingAccountID, err := s.determineFundingAccountID(ctx, order.UserID, &order.AccountID, currency)
		if err != nil {
			return err
		}
		applyConversionFee := !s.isEmployeeUser(ctx, order.UserID)
		if !isFundOrder {
			if _, err := s.transferFee(ctx, fundingAccountID, f, currency, applyConversionFee); err != nil {
				return err
			}
		}
		if order.Direction == DirectionBuy && !order.Margin {
			if err := s.checkFunds(ctx, fundingAccountID, ap, currency); err != nil {
				return err
			}
		}
		s.market.RefreshListing(ctx, order.ListingID)

		order.Status = StatusApproved
		order.ApprovedBy = &supervisorID
		if err := s.repo.Update(ctx, tx, order); err != nil {
			return err
		}
		result = order
		notify = s.buildDecisionPayload(ctx, order, supervisorID, StatusApproved)
		return nil
	})
	if err != nil {
		return api.OrderResponse{}, err
	}
	if notify != nil {
		s.notifier.OrderApproved(ctx, *notify)
	}
	s.publishOrderAuditEvent(ctx, result, supervisorID, true)
	s.ExecuteOrderAsync(result.ID)
	return s.mapToResponse(result, approx, fee, result.ExchangeClosed), nil
}

// DeclineOrder mirrors declineOrder (supervisor): release reservations, mark
// DECLINED, publish after commit.
func (s *Service) DeclineOrder(ctx context.Context, supervisorID, orderID int64) (api.OrderResponse, error) {
	var (
		result      *Order
		approx, fee decimal.Decimal
		notify      *api.OrderNotificationPayload
	)
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		order, err := s.repo.FindByID(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if order == nil {
			return api.NewOrderError(404, "Order not found")
		}
		if order.Status != StatusPending {
			return api.NewOrderError(409, "Only pending orders can be declined")
		}
		if err := s.declinePendingOrder(ctx, tx, order, supervisorID); err != nil {
			return err
		}
		notify = s.buildDecisionPayload(ctx, order, supervisorID, StatusDeclined)

		listing, err := s.market.GetListing(ctx, order.ListingID)
		if err != nil {
			return err
		}
		ap, err := calculateApproximatePrice(order.OrderType, order.Direction, listing, order.Quantity, order.LimitValue, order.StopValue)
		if err != nil {
			return err
		}
		approx = ap
		fee = s.commission(ctx, orderPricingFamily(order.OrderType), ap, listing.Currency())
		result = order
		return nil
	})
	if err != nil {
		return api.OrderResponse{}, err
	}
	if notify != nil {
		s.notifier.OrderDeclined(ctx, *notify)
	}
	s.publishOrderAuditEvent(ctx, result, supervisorID, false)
	return s.mapToResponse(result, approx, fee, result.ExchangeClosed), nil
}

// CancelOrder mirrors the client/agent POST /{id}/cancel: cancel the whole
// remaining quantity of an owned order.
func (s *Service) CancelOrder(ctx context.Context, user AuthUser, orderID int64) (api.OrderResponse, error) {
	return s.cancelOrder(ctx, orderID, nil, &user.UserID)
}

// CancelOrderSupervisor mirrors the supervisor PUT /{id}/cancel: cancel all, or a
// given quantity, of the remainder (no ownership check).
func (s *Service) CancelOrderSupervisor(ctx context.Context, orderID int64, quantity *int) (api.OrderResponse, error) {
	return s.cancelOrder(ctx, orderID, quantity, nil)
}

func (s *Service) cancelOrder(ctx context.Context, orderID int64, quantityToCancel *int, ownerID *int64) (api.OrderResponse, error) {
	var (
		result      *Order
		approx, fee decimal.Decimal
	)
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		order, err := s.repo.FindByIDForUpdate(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if order == nil {
			return api.NewOrderError(404, "Order not found")
		}
		if ownerID != nil && order.UserID != *ownerID {
			return api.NewOrderError(403, "Order does not belong to the authenticated user")
		}
		if order.Status == StatusDone || order.Status == StatusCancelled || order.Status == StatusDeclined {
			return api.NewOrderError(409, "Order can no longer be cancelled")
		}
		listing, err := s.market.GetListing(ctx, order.ListingID)
		if err != nil {
			return err
		}
		if order.Status == StatusPending && hasPastSettlementDate(listing) {
			return api.NewOrderError(409, "Expired pending orders can only be declined")
		}
		cancelQty := order.RemainingPortions
		if quantityToCancel != nil {
			cancelQty = *quantityToCancel
		}
		if cancelQty <= 0 || cancelQty > order.RemainingPortions {
			return api.NewOrderError(400, "Invalid cancellation quantity")
		}
		if err := s.releaseReservedState(ctx, tx, order, cancelQty); err != nil {
			return err
		}
		order.RemainingPortions -= cancelQty
		if order.RemainingPortions == 0 {
			order.Status = StatusCancelled
			order.IsDone = true
		}
		if err := s.repo.Update(ctx, tx, order); err != nil {
			return err
		}
		ap, err := calculateApproximatePrice(order.OrderType, order.Direction, listing, order.Quantity, order.LimitValue, order.StopValue)
		if err != nil {
			return err
		}
		approx = ap
		fee = s.commission(ctx, orderPricingFamily(order.OrderType), ap, listing.Currency())
		result = order
		return nil
	})
	if err != nil {
		return api.OrderResponse{}, err
	}
	return s.mapToResponse(result, approx, fee, result.ExchangeClosed), nil
}

// AutoDeclineExpiredPendingOrders mirrors autoDeclineExpiredPendingOrders: decline
// every PENDING order whose settlement date has passed. Each order is handled in
// its own transaction; the decline notification is published after that commit.
func (s *Service) AutoDeclineExpiredPendingOrders(ctx context.Context) error {
	pending, err := s.repo.FindByStatus(ctx, s.repo.Pool(), StatusPending)
	if err != nil {
		return err
	}
	for i := range pending {
		id := pending[i].ID
		var (
			cancelled *Order
			ticker    string
			price     decimal.Decimal
		)
		err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
			locked, err := s.repo.FindByIDForUpdate(ctx, tx, id)
			if err != nil {
				return err
			}
			if locked == nil || locked.Status != StatusPending {
				return nil
			}
			listing, err := s.market.GetListing(ctx, locked.ListingID)
			if err != nil {
				return err
			}
			if !hasPastSettlementDate(listing) {
				return nil
			}
			if err := s.declinePendingOrder(ctx, tx, locked, systemApproval); err != nil {
				return err
			}
			cancelled = locked
			if listing.Ticker != nil {
				ticker = *listing.Ticker
			}
			price = locked.PricePerUnit
			return nil
		})
		if err != nil {
			s.logger.Error("auto-decline failed for order", "orderId", id, "error", err)
			continue
		}
		// Mirror OrderCreationServiceImpl.autoDeclineExpiredPendingOrders: the
		// expiry path emits AUTO_CANCELLED (not a supervisor DECLINED), so the
		// mobile client sees the order was auto-cancelled on expiry. The audit
		// trail still records ORDER_DECLINED with the SYSTEM actor (Java's
		// declinePendingOrder publishes the audit event on both paths).
		if cancelled != nil {
			s.notifier.OrderAutoCancelled(ctx, *s.buildLifecyclePayload(ctx, cancelled, eventAutoCancelled, ticker, price))
			s.publishOrderAuditEvent(ctx, cancelled, systemApproval, false)
		}
	}
	return nil
}

// declinePendingOrder releases reservations and marks the order DECLINED (no
// publish — the caller publishes after commit). Mirrors declinePendingOrder.
func (s *Service) declinePendingOrder(ctx context.Context, tx pgx.Tx, order *Order, approverID int64) error {
	if err := s.releaseReservedState(ctx, tx, order, order.RemainingPortions); err != nil {
		return err
	}
	order.Status = StatusDeclined
	order.ApprovedBy = &approverID
	order.RemainingPortions = 0
	order.IsDone = true
	return s.repo.Update(ctx, tx, order)
}

// --- Reservation helpers --------------------------------------------------

func (s *Service) reserveSellQuantityIfNeeded(ctx context.Context, tx pgx.Tx, order *Order) error {
	if order.Direction != DirectionSell {
		return nil
	}
	p, err := s.portfolios.FindByUserIDAndListingIDForUpdate(ctx, tx, order.UserID, order.ListingID)
	if err != nil {
		return err
	}
	if p == nil {
		return api.NewOrderError(404, "Portfolio position not found")
	}
	available := p.Quantity - p.ReservedQuantity
	if available < order.RemainingPortions {
		return api.NewOrderError(409, "Insufficient portfolio quantity")
	}
	return s.portfolios.UpdateReservedQuantity(ctx, tx, p.ID, p.ReservedQuantity+order.RemainingPortions)
}

func (s *Service) releaseReservedState(ctx context.Context, tx pgx.Tx, order *Order, quantityToRelease int) error {
	if err := s.releaseSellReservation(ctx, tx, order, quantityToRelease); err != nil {
		return err
	}
	return s.releaseAgentExposure(ctx, tx, order, quantityToRelease)
}

func (s *Service) releaseSellReservation(ctx context.Context, tx pgx.Tx, order *Order, quantityToRelease int) error {
	if order.Direction != DirectionSell || quantityToRelease <= 0 {
		return nil
	}
	p, err := s.portfolios.FindByUserIDAndListingIDForUpdate(ctx, tx, order.UserID, order.ListingID)
	if err != nil {
		return err
	}
	if p == nil {
		return nil
	}
	newReserved := p.ReservedQuantity - quantityToRelease
	if newReserved < 0 {
		newReserved = 0
	}
	return s.portfolios.UpdateReservedQuantity(ctx, tx, p.ID, newReserved)
}

// releaseAgentExposure mirrors releaseAgentExposure: proportionally release the
// order's reserved RSD exposure back to the actuary's reserved_limit.
func (s *Service) releaseAgentExposure(ctx context.Context, tx pgx.Tx, order *Order, quantityToRelease int) error {
	if quantityToRelease <= 0 || order.ReservedLimitExposure.Sign() <= 0 {
		return nil
	}
	info, err := s.actuaries.FindByEmployeeIDForUpdate(ctx, tx, order.UserID)
	if err != nil {
		return err
	}
	if info == nil || order.Quantity <= 0 {
		order.ReservedLimitExposure = decimal.Zero
		return nil
	}

	if order.RemainingPortions <= 0 {
		releasable := decimalMin(order.ReservedLimitExposure, info.ReservedLimit)
		newReserved := decimalMaxZero(info.ReservedLimit.Sub(releasable))
		if err := s.actuaries.UpdateReservedLimit(ctx, tx, order.UserID, newReserved); err != nil {
			return err
		}
		order.ReservedLimitExposure = decimalMaxZero(order.ReservedLimitExposure.Sub(releasable))
		return nil
	}

	releasable := decimalMin(
		order.ReservedLimitExposure.Mul(decimal.NewFromInt(int64(quantityToRelease))).
			DivRound(decimal.NewFromInt(int64(order.RemainingPortions)), 4),
		order.ReservedLimitExposure)
	newReserved := decimalMaxZero(info.ReservedLimit.Sub(releasable))
	if err := s.actuaries.UpdateReservedLimit(ctx, tx, order.UserID, newReserved); err != nil {
		return err
	}
	order.ReservedLimitExposure = decimalMaxZero(order.ReservedLimitExposure.Sub(releasable))
	return nil
}

// determineOrderStatusAndReserveExposure mirrors the agent approval gate: locks
// actuary_info, reserves the order's RSD amount, and returns PENDING when
// need-approval is set or the (used+reserved) limit is exhausted/exceeded.
func (s *Service) determineOrderStatusAndReserveExposure(ctx context.Context, tx pgx.Tx, userID int64, approx decimal.Decimal, currency string) (string, decimal.Decimal, error) {
	info, err := s.actuaries.FindByEmployeeIDForUpdate(ctx, tx, userID)
	if err != nil {
		return "", decimal.Zero, err
	}
	if info == nil {
		return StatusApproved, decimal.Zero, nil
	}
	orderAmount, err := s.convertAmountNoComm(ctx, currency, limitCurrency, approx)
	if err != nil {
		return "", decimal.Zero, err
	}
	usedPlusReserved := info.UsedLimit.Add(info.ReservedLimit)
	exhausted := info.Limit != nil && usedPlusReserved.Cmp(*info.Limit) >= 0
	exceeds := info.Limit != nil && usedPlusReserved.Add(orderAmount).Cmp(*info.Limit) > 0
	if info.Limit != nil {
		if err := s.actuaries.UpdateReservedLimit(ctx, tx, userID, info.ReservedLimit.Add(orderAmount)); err != nil {
			return "", decimal.Zero, err
		}
	}
	status := StatusApproved
	if info.NeedApproval || exhausted || exceeds {
		status = StatusPending
	}
	reservedExposure := decimal.Zero
	if info.Limit != nil {
		reservedExposure = orderAmount
	}
	return status, reservedExposure, nil
}

// --- Funding / fees / validation ------------------------------------------

func (s *Service) initialBuyAccountID(ctx context.Context, user AuthUser, selected *int64, currency string) (int64, error) {
	if user.IsClient() {
		if selected == nil {
			return 0, api.NewOrderError(400, "Account is required for client buy orders")
		}
		return *selected, nil
	}
	return s.determineFundingAccountID(ctx, user.UserID, selected, currency)
}

// determineFundingAccountID mirrors determineFundingAccountId: agents (actuary
// rows) and employees fall back to the bank account for the currency; an employee
// who selected an account keeps it.
func (s *Service) determineFundingAccountID(ctx context.Context, userID int64, selected *int64, currency string) (int64, error) {
	if info, err := s.actuaries.FindByEmployeeID(ctx, userID); err == nil && info != nil {
		bank, err := s.account.GetBankAccount(ctx, currency)
		if err != nil {
			return 0, err
		}
		return bank.ResolvedID(), nil
	}
	if emp, err := s.employees.GetEmployee(ctx, userID); err == nil && emp != nil {
		if selected != nil {
			return *selected, nil
		}
		bank, err := s.account.GetBankAccount(ctx, currency)
		if err != nil {
			return 0, err
		}
		return bank.ResolvedID(), nil
	}
	if selected != nil {
		return *selected, nil
	}
	return 0, nil
}

func (s *Service) isEmployeeUser(ctx context.Context, userID int64) bool {
	if info, err := s.actuaries.FindByEmployeeID(ctx, userID); err == nil && info != nil {
		return true
	}
	emp, err := s.employees.GetEmployee(ctx, userID)
	return err == nil && emp != nil
}

func (s *Service) checkFunds(ctx context.Context, accountID int64, totalAmount decimal.Decimal, amountCurrency string) error {
	account, err := s.account.GetAccountDetailsByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, clients.ErrNotFound) {
			return api.NewOrderError(400, fmt.Sprintf("Account not found: %d", accountID))
		}
		return err
	}
	amountInAccountCurrency, err := s.convertAmountNoComm(ctx, amountCurrency, account.CurrencyOrEmpty(), totalAmount)
	if err != nil {
		return err
	}
	if account.BalanceOrZero().Cmp(amountInAccountCurrency) < 0 {
		return api.NewOrderError(409, "Insufficient funds")
	}
	return nil
}

func (s *Service) checkMarginRequirements(ctx context.Context, user AuthUser, fundingAccountID int64, listing *clients.StockListing, quantity int) error {
	if !user.HasMarginPermission() {
		return api.NewOrderError(409, "User does not have margin permission")
	}
	initialMarginCost, err := calculateInitialMarginCost(listing, quantity)
	if err != nil {
		return err
	}
	account, err := s.account.GetAccountDetailsByID(ctx, fundingAccountID)
	if err != nil {
		return err
	}
	hasCredit := account.AvailableCreditOrZero().Cmp(initialMarginCost) > 0
	hasFunds := account.BalanceOrZero().Cmp(initialMarginCost) > 0
	if !hasCredit && !hasFunds {
		return api.NewOrderError(409, "Margin requirements are not satisfied")
	}
	return nil
}

func (s *Service) validateClientAccount(ctx context.Context, userID, accountID int64) error {
	account, err := s.account.GetAccountDetailsByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, clients.ErrNotFound) {
			return api.NewOrderError(400, fmt.Sprintf("Account not found: %d", accountID))
		}
		return err
	}
	if account.OwnerID != nil && *account.OwnerID != userID {
		return api.NewOrderError(403, "Account does not belong to this user")
	}
	return nil
}

// transferFee mirrors transferFee: bills the fee from the funding account to the
// bank account (no-op when funding IS the bank account). Returns the amount
// debited from the sender (in the sender's currency).
func (s *Service) transferFee(ctx context.Context, fundingAccountID int64, fee decimal.Decimal, currency string, applyConversionFee bool) (decimal.Decimal, error) {
	bank, err := s.account.GetBankAccount(ctx, currency)
	if err != nil {
		return decimal.Zero, err
	}
	if fundingAccountID != 0 && fundingAccountID == bank.ResolvedID() {
		return decimal.Zero, nil
	}
	return s.transferWithConversionIfNeeded(ctx, fundingAccountID, bank.ResolvedID(), fee, currency, applyConversionFee)
}

func (s *Service) transferWithConversionIfNeeded(ctx context.Context, fromAccountID, toAccountID int64, targetAmount decimal.Decimal, targetCurrency string, applyConversionFee bool) (decimal.Decimal, error) {
	from, err := s.account.GetAccountDetailsByID(ctx, fromAccountID)
	if err != nil {
		return decimal.Zero, err
	}
	to, err := s.account.GetAccountDetailsByID(ctx, toAccountID)
	if err != nil {
		return decimal.Zero, err
	}
	if from.CurrencyOrEmpty() == "" || strings.EqualFold(from.CurrencyOrEmpty(), targetCurrency) {
		payment := clients.Payment{
			FromAccountNumber: from.Number(),
			ToAccountNumber:   to.Number(),
			FromAmount:        targetAmount,
			ToAmount:          targetAmount,
			Commission:        decimal.Zero,
			ClientID:          from.OwnerIDValue(),
		}
		if err := s.executePaymentByOwnership(ctx, from, to, payment); err != nil {
			return decimal.Zero, err
		}
		return targetAmount, nil
	}

	var rate *clients.ExchangeRate
	if applyConversionFee {
		rate, err = s.market.Calculate(ctx, targetCurrency, from.CurrencyOrEmpty(), targetAmount)
	} else {
		rate, err = s.market.CalculateWithoutCommission(ctx, targetCurrency, from.CurrencyOrEmpty(), targetAmount)
	}
	if err != nil {
		return decimal.Zero, err
	}
	fromAmount := targetAmount
	if rate != nil && rate.Converted() != nil {
		fromAmount = *rate.Converted()
	}
	commission := decimal.Zero
	if applyConversionFee && rate != nil && rate.Commission != nil {
		commission = *rate.Commission
	}
	payment := clients.Payment{
		FromAccountNumber: from.Number(),
		ToAccountNumber:   to.Number(),
		FromAmount:        fromAmount,
		ToAmount:          targetAmount,
		Commission:        commission,
		ClientID:          from.OwnerIDValue(),
	}
	if err := s.executePaymentByOwnership(ctx, from, to, payment); err != nil {
		return decimal.Zero, err
	}
	return fromAmount, nil
}

// executePaymentByOwnership routes same-owner legs to /transfer and cross-owner
// legs to /transaction (mirrors executePaymentByOwnership).
func (s *Service) executePaymentByOwnership(ctx context.Context, from, to *clients.AccountDetails, payment clients.Payment) error {
	if from.OwnerID != nil && to.OwnerID != nil && *from.OwnerID == *to.OwnerID {
		return s.account.Transfer(ctx, payment)
	}
	return s.account.Transaction(ctx, payment)
}

func (s *Service) validateTradingAccess(user AuthUser, listing *clients.StockListing) error {
	if !user.IsClient() {
		return nil
	}
	if !user.HasTradingPermission() {
		return api.NewOrderError(403, "Client does not have trading permission")
	}
	listingType := listing.ListingTypeOr("STOCK")
	if listingType != "STOCK" && listingType != "FUTURES" {
		return api.NewOrderError(403, "Clients can trade only stocks and futures")
	}
	return nil
}

func (s *Service) resolveExchangeWindow(ctx context.Context, listing *clients.StockListing) (bool, bool, error) {
	if listing.ExchangeID == nil {
		return false, false, fmt.Errorf("order: listing %d has no exchange id", listing.ID)
	}
	status, err := s.market.GetExchangeStatus(ctx, *listing.ExchangeID)
	if err != nil {
		return false, false, err
	}
	return status.IsClosed(), status.IsAfterHours(), nil
}

func (s *Service) ensurePortfolioOwnership(ctx context.Context, userID, listingID int64, requestedQuantity int) error {
	p, err := s.portfolios.FindByUserIDAndListingID(ctx, s.portfolios.Pool(), userID, listingID)
	if err != nil {
		return err
	}
	if p == nil {
		return api.NewOrderError(404, "Portfolio position not found")
	}
	if p.Quantity-p.ReservedQuantity < requestedQuantity {
		return api.NewOrderError(409, "Insufficient portfolio quantity")
	}
	return nil
}

// --- Builders / mappers ---------------------------------------------------

func (s *Service) buildBaseOrder(userID, listingID int64, orderType string, quantity int, listing *clients.StockListing,
	limit, stop *decimal.Decimal, direction string, allOrNone, margin *bool, accountID int64, exchangeClosed, afterHours bool) (*Order, error) {
	if listing.ContractSize == nil {
		return nil, fmt.Errorf("order: listing %d has no contract size", listing.ID)
	}
	ppu, err := referencePricePerUnit(orderType, direction, listing, limit, stop)
	if err != nil {
		return nil, err
	}
	return &Order{
		UserID:                userID,
		ListingID:             listingID,
		OrderType:             orderType,
		Quantity:              quantity,
		ContractSize:          *listing.ContractSize,
		PricePerUnit:          ppu,
		LimitValue:            limit,
		StopValue:             stop,
		Direction:             direction,
		IsDone:                false,
		RemainingPortions:     quantity,
		AfterHours:            afterHours,
		ExchangeClosed:        exchangeClosed,
		AllOrNone:             boolValue(allOrNone),
		Margin:                boolValue(margin),
		AccountID:             accountID,
		ReservedLimitExposure: decimal.Zero,
	}, nil
}

func (s *Service) mapToResponse(order *Order, approx, fee decimal.Decimal, exchangeClosed bool) api.OrderResponse {
	// Celina 3: timestamps serialize as UTC Instants (trailing Z), mirroring
	// OrderCreationServiceImpl.toUtcInstant. createdAt is NOT NULL after
	// migration 004; the zero-check guards rows mapped before persistence.
	createdAt := api.UTCInstant{}
	if !order.CreatedAt.IsZero() {
		createdAt = api.NewUTCInstant(order.CreatedAt)
	}
	return api.OrderResponse{
		ID:                order.ID,
		UserID:            order.UserID,
		ListingID:         order.ListingID,
		OrderType:         order.OrderType,
		Quantity:          order.Quantity,
		ContractSize:      order.ContractSize,
		PricePerUnit:      order.PricePerUnit,
		LimitValue:        order.LimitValue,
		StopValue:         order.StopValue,
		Direction:         order.Direction,
		Status:            order.Status,
		ApprovedBy:        order.ApprovedBy,
		IsDone:            order.IsDone,
		LastModification:  api.NewUTCInstant(order.LastModification),
		RemainingPortions: order.RemainingPortions,
		AfterHours:        order.AfterHours,
		ExchangeClosed:    exchangeClosed,
		AllOrNone:         order.AllOrNone,
		Margin:            order.Margin,
		AccountID:         order.AccountID,
		ApproximatePrice:  approx,
		Fee:               fee,
		CreatedAt:         createdAt,
		ExecutedAt:        api.UTCInstantFromPtr(order.ExecutedAt),
	}
}

func (s *Service) mapStoredOrderToResponse(ctx context.Context, order *Order) (api.OrderResponse, error) {
	listing, err := s.market.GetListing(ctx, order.ListingID)
	if err != nil {
		return api.OrderResponse{}, err
	}
	approx := order.PricePerUnit.
		Mul(decimal.NewFromInt(int64(order.ContractSize))).
		Mul(decimal.NewFromInt(int64(order.Quantity)))
	fee := s.commission(ctx, orderPricingFamily(order.OrderType), approx, listing.Currency())
	return s.mapToResponse(order, approx, fee, order.ExchangeClosed), nil
}

func (s *Service) mapToOverviewResponse(ctx context.Context, order *Order, listingCache map[int64]*clients.StockListing,
	employeeCache map[int64]*clients.Employee, actuaryIDs map[int64]bool) api.OrderOverviewResponse {
	var listingType *string
	if l := listingCache[order.ListingID]; l != nil {
		listingType = l.ListingType
	}
	return api.OrderOverviewResponse{
		OrderID:           order.ID,
		AgentName:         s.resolveAgentName(ctx, order.UserID, employeeCache, actuaryIDs),
		OrderType:         order.OrderType,
		ListingType:       listingType,
		Quantity:          order.Quantity,
		ContractSize:      order.ContractSize,
		PricePerUnit:      order.PricePerUnit,
		Direction:         order.Direction,
		RemainingPortions: order.RemainingPortions,
		Status:            order.Status,
	}
}

func (s *Service) resolveAgentName(ctx context.Context, userID int64, cache map[int64]*clients.Employee, actuaryIDs map[int64]bool) *string {
	if !actuaryIDs[userID] {
		return nil
	}
	emp, ok := cache[userID]
	if !ok {
		fetched, err := s.employees.GetEmployee(ctx, userID)
		if err != nil {
			s.logger.Warn("failed to resolve employee name for order owner", "userId", userID, "error", err)
			cache[userID] = nil
			return nil
		}
		cache[userID] = fetched
		emp = fetched
	}
	if emp == nil {
		return nil
	}
	return formatEmployeeName(emp)
}

func (s *Service) buildDecisionPayload(ctx context.Context, order *Order, supervisorID int64, status string) *api.OrderNotificationPayload {
	var username, email *string
	if emp, err := s.employees.GetEmployee(ctx, order.UserID); err == nil && emp != nil {
		username = formatEmployeeName(emp)
		email = emp.Email
	}
	return &api.OrderNotificationPayload{
		OrderID:      order.ID,
		Status:       status,
		UserID:       order.UserID,
		ClientID:     order.UserID,
		SupervisorID: supervisorID,
		ListingID:    order.ListingID,
		OrderType:    order.OrderType,
		Direction:    order.Direction,
		Username:     username,
		UserEmail:    email,
		TemplateVariables: map[string]string{
			"orderId":      fmt.Sprintf("%d", order.ID),
			"status":       status,
			"userId":       fmt.Sprintf("%d", order.UserID),
			"supervisorId": fmt.Sprintf("%d", supervisorID),
			"listingId":    fmt.Sprintf("%d", order.ListingID),
			"orderType":    order.OrderType,
			"direction":    order.Direction,
		},
	}
}

// buildLifecyclePayload mirrors OrderEventNotifier.buildPayload: resolves the
// order owner client-first (so the FCM push, keyed on clientId == userId, reaches
// the mobile device) with an employee fallback, and populates the template
// variables the lifecycle notification templates render. ClientID is always the
// order owner.
func (s *Service) buildLifecyclePayload(ctx context.Context, order *Order, event, ticker string, price decimal.Decimal) *api.OrderNotificationPayload {
	name, email := s.resolveRecipient(ctx, order.UserID)
	vars := map[string]string{
		"orderId":           fmt.Sprintf("%d", order.ID),
		"status":            order.Status,
		"ticker":            ticker,
		"listingId":         fmt.Sprintf("%d", order.ListingID),
		"quantity":          fmt.Sprintf("%d", order.Quantity),
		"remainingPortions": fmt.Sprintf("%d", order.RemainingPortions),
		"price":             price.String(),
		"orderType":         order.OrderType,
		"direction":         order.Direction,
		"event":             event,
	}
	if name != nil {
		vars["name"] = *name
	}
	return &api.OrderNotificationPayload{
		OrderID:           order.ID,
		Status:            order.Status,
		UserID:            order.UserID,
		ClientID:          order.UserID,
		ListingID:         order.ListingID,
		OrderType:         order.OrderType,
		Direction:         order.Direction,
		Username:          name,
		UserEmail:         email,
		TemplateVariables: vars,
	}
}

// resolveRecipient mirrors OrderEventNotifier.resolveRecipient: clients are the
// push audience, so resolve them first (name + email); fall back to employee for
// agent/actuary-placed orders. Best-effort — returns (nil, nil) when neither
// lookup yields a usable recipient.
func (s *Service) resolveRecipient(ctx context.Context, userID int64) (*string, *string) {
	if s.customers != nil {
		if cust, err := s.customers.GetCustomer(ctx, userID); err == nil && cust != nil &&
			cust.Email != nil && strings.TrimSpace(*cust.Email) != "" {
			return customerDisplayName(cust), cust.Email
		}
	}
	if emp, err := s.employees.GetEmployee(ctx, userID); err == nil && emp != nil {
		return formatEmployeeName(emp), emp.Email
	}
	return nil, nil
}

// publishOrderAuditEvent mirrors OrderCreationServiceImpl.publishOrderAuditEvent
// (WP-2 / Issue 9): ORDER_APPROVED / ORDER_DECLINED with a SYSTEM actor for the
// auto-decline path. Recorded by a direct local insert (the audit sink is
// in-process), called AFTER the decision transaction commits — best-effort,
// audit must never break the business flow.
func (s *Service) publishOrderAuditEvent(ctx context.Context, order *Order, actorID int64, approved bool) {
	if s.auditor == nil {
		return
	}
	var auditActorID *int64
	actorName := "SYSTEM"
	if actorID != systemApproval {
		id := actorID
		auditActorID = &id
		actorName = s.resolveActorName(ctx, actorID)
	}
	actionType := audit.ActionOrderDeclined
	details := "Order odbijen (PENDING -> DECLINED)"
	if approved {
		actionType = audit.ActionOrderApproved
		details = "Order odobren (PENDING -> APPROVED)"
	}
	targetType := "ORDER"
	targetID := fmt.Sprintf("%d", order.ID)
	ts := time.Now().UnixMilli()
	s.auditor.RecordBestEffort(ctx, audit.Event{
		ActorID:    auditActorID,
		ActorName:  &actorName,
		ActionType: actionType,
		TargetType: &targetType,
		TargetID:   &targetID,
		Details:    &details,
		Timestamp:  &ts,
	})
}

// resolveActorName mirrors resolveActorName: the employee's display name, or
// the raw actor id string when the lookup fails.
func (s *Service) resolveActorName(ctx context.Context, actorID int64) string {
	if emp, err := s.employees.GetEmployee(ctx, actorID); err == nil && emp != nil {
		if name := formatEmployeeName(emp); name != nil {
			return *name
		}
	}
	return fmt.Sprintf("%d", actorID)
}

// customerDisplayName mirrors OrderEventNotifier.formatCustomerName: trimmed
// "first last", or nil when empty.
func customerDisplayName(c *clients.Customer) *string {
	var first, last string
	if f := c.First(); f != nil {
		first = strings.TrimSpace(*f)
	}
	if l := c.Last(); l != nil {
		last = strings.TrimSpace(*l)
	}
	full := strings.TrimSpace(first + " " + last)
	if full == "" {
		return nil
	}
	return &full
}

// --- Small helpers --------------------------------------------------------

func parsePurchaseFor(value *string) (string, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "", nil
	}
	upper := strings.ToUpper(strings.TrimSpace(*value))
	if upper == PurchaseForBank || upper == PurchaseForInvestmentFund {
		return upper, nil
	}
	return "", api.NewOrderError(400, "Unsupported purchaseFor: "+*value)
}

func validateCommonRequest(listingID *int64, quantity *int, limit, stop *decimal.Decimal) error {
	if listingID == nil || quantity == nil || *quantity <= 0 {
		return api.NewOrderError(400, "Invalid request parameters")
	}
	if limit != nil && limit.Sign() <= 0 {
		return api.NewOrderError(400, "Limit value must be positive")
	}
	if stop != nil && stop.Sign() <= 0 {
		return api.NewOrderError(400, "Stop value must be positive")
	}
	return nil
}

func formatEmployeeName(emp *clients.Employee) *string {
	first := ""
	if emp.Ime != nil {
		first = strings.TrimSpace(*emp.Ime)
	}
	last := ""
	if emp.Prezime != nil {
		last = strings.TrimSpace(*emp.Prezime)
	}
	full := strings.TrimSpace(first + " " + last)
	if full == "" {
		return emp.Username
	}
	return &full
}

func hasPastSettlementDate(listing *clients.StockListing) bool {
	if listing.SettlementDate == nil {
		return false
	}
	settle, err := time.Parse("2006-01-02", *listing.SettlementDate)
	if err != nil {
		return false
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	settleDate := time.Date(settle.Year(), settle.Month(), settle.Day(), 0, 0, 0, 0, now.Location())
	return settleDate.Before(today)
}

func boolValue(b *bool) bool {
	return b != nil && *b
}
