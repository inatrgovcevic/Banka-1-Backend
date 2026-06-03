package funds

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/clients"

	gpdb "banka1/go-platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// Service mirrors InvestmentFundService — the orchestrator that owns fund
// creation, the invest/redeem saga publish, the saga-result handlers
// (CompleteInvest / CompleteRedeem / FailTransaction), liquidity adjustments,
// and the read paths (discovery, details, positions, transactions,
// performance). It pulls the snapshot/statistics/holding services as
// collaborators.
type Service struct {
	repo      *Repository
	snapshots *SnapshotService
	stats     *StatisticsService
	holdings  *HoldingService
	market    *clients.MarketClient
	account   *clients.AccountClient
	employee  *clients.EmployeeClient
	publisher SagaPublisher
	logger    *slog.Logger
}

// NewService wires the service. Publisher is the saga-events publisher; a
// noop wrapper (NewNoopSagaPublisher) is fine when the broker is unreachable
// (the local tx still commits, the transaction stays PENDING — supervisor sees
// it and can act).
func NewService(repo *Repository, snapshots *SnapshotService, stats *StatisticsService, holdings *HoldingService, market *clients.MarketClient, account *clients.AccountClient, employee *clients.EmployeeClient, publisher SagaPublisher, logger *slog.Logger) *Service {
	return &Service{
		repo: repo, snapshots: snapshots, stats: stats, holdings: holdings,
		market: market, account: account, employee: employee,
		publisher: publisher, logger: logger,
	}
}

// ---------------------- create / management -------------------------------

// CreateFund mirrors InvestmentFundService.createFund. Persists the fund,
// creates its RSD system account via banking-core, records the initial
// snapshot. Account-creation failure rolls the create back via OTC 409.
func (s *Service) CreateFund(ctx context.Context, naziv string, opis *string, minContribution decimal.Decimal, dividendStrategy string, managerID int64) (*FundDto, error) {
	accountNumber, err := GenerateAccountNumber()
	if err != nil {
		return nil, api.NewOtcError(http.StatusConflict, "Generisanje racuna fonda nije uspelo: "+err.Error())
	}
	strategy := dividendStrategy
	if strategy == "" {
		strategy = DividendReinvest
	}
	fund := &InvestmentFund{
		Naziv:               naziv,
		Opis:                opis,
		MinimumContribution: minContribution,
		ManagerID:           managerID,
		LikvidnaSredstva:    decimal.Zero,
		AccountNumber:       accountNumber,
		DividendStrategy:    strategy,
		DatumKreiranja:      time.Now().UTC().Truncate(24 * time.Hour),
	}
	if err := s.repo.InsertFund(ctx, nil, fund); err != nil {
		return nil, err
	}
	if _, err := s.account.CreateSystemAccount(ctx, accountNumber, -1000-fund.ID, FundBaseCurrency,
		"Investicioni fond: "+fund.Naziv, decimal.Zero); err != nil {
		// Java throws IllegalStateException → OTC 409. The fund row is left in
		// place (matches Java: throw happens after save() commits in the same
		// outer @Transactional).
		s.logger.Error("fund account creation failed", "fundId", fund.ID, "error", err)
		return nil, api.NewOtcError(http.StatusConflict, "Account fonda nije kreiran.")
	}
	s.snapshots.RecordSilently(ctx, fund.ID)
	return s.toDto(ctx, fund), nil
}

// ReassignManager mirrors reassignManager. Moves every active fund from
// oldManagerId to newManagerId in one pass.
func (s *Service) ReassignManager(ctx context.Context, oldManagerID, newManagerID int64) error {
	funds, err := s.repo.FindFundsByManager(ctx, oldManagerID)
	if err != nil {
		return err
	}
	for _, f := range funds {
		if err := s.repo.UpdateFundManager(ctx, nil, f.ID, newManagerID); err != nil {
			return err
		}
	}
	s.logger.Info("reassigned fund managers", "count", len(funds), "from", oldManagerID, "to", newManagerID)
	return nil
}

// DebitLiquidity mirrors InvestmentFundService.debitLiquidity. Called when an
// order or dividend reinvestment subtracts cash from the fund. FOR UPDATE
// lock on the fund row to serialize concurrent debits.
func (s *Service) DebitLiquidity(ctx context.Context, fundID int64, amount decimal.Decimal, reason string) error {
	if amount.Sign() <= 0 {
		return nil
	}
	return gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		fund, err := s.repo.FindFundByIDForUpdate(ctx, tx, fundID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return api.NewOtcError(http.StatusNotFound, "Fond "+itoa(fundID)+" ne postoji.")
			}
			return err
		}
		newLiq := fund.LikvidnaSredstva.Sub(amount).Round(2)
		if err := s.repo.UpdateFundLiquidity(ctx, tx, fundID, newLiq); err != nil {
			return err
		}
		s.logger.Info("fund liquidity debited",
			"fundId", fundID, "amount", amount, "newLiquidity", newLiq, "reason", reason)
		return nil
	})
}

// ---------------------- read paths ----------------------------------------

// Discovery mirrors InvestmentFundService.discovery + statisticsService.sort.
func (s *Service) Discovery(ctx context.Context, sortField, sortDir string) ([]FundDto, error) {
	funds, err := s.repo.FindFundsActive(ctx)
	if err != nil {
		return nil, err
	}
	dtos := make([]FundDto, 0, len(funds))
	views := make([]FundView, 0, len(funds))
	for i := range funds {
		dto := s.toDto(ctx, &funds[i])
		dtos = append(dtos, *dto)
		views = append(views, FundView{
			Naziv:                    dto.Naziv,
			TotalValue:               ptrDecimal(dto.TotalValue),
			Profit:                   ptrDecimal(dto.Profit),
			AnnualizedReturn:         dto.AnnualizedReturn,
			RewardToVariabilityRatio: dto.RewardToVariabilityRatio,
			MaxDrawdown:              dto.MaxDrawdown,
			Volatility:               dto.Volatility,
			// Store the FundDto value (not the *FundDto): the re-projection after
			// sorting type-asserts v.Source.(FundDto), which silently failed on the
			// pointer and returned an empty discovery list.
			Source: *dto,
		})
	}
	views = s.stats.Sort(views, sortField, sortDir)
	out := make([]FundDto, 0, len(views))
	for _, v := range views {
		if d, ok := v.Source.(FundDto); ok {
			out = append(out, d)
		}
	}
	return out, nil
}

// Details mirrors InvestmentFundService.details.
func (s *Service) Details(ctx context.Context, fundID int64) (*FundDto, error) {
	fund, err := s.repo.FindFundByID(ctx, nil, fundID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, api.NewOtcError(http.StatusNotFound, "Fond "+itoa(fundID)+" ne postoji.")
		}
		return nil, err
	}
	return s.toDto(ctx, fund), nil
}

// Analytics mirrors InvestmentFundService.analytics: detail + metrics +
// snapshot history + average-performance comparison.
func (s *Service) Analytics(ctx context.Context, fundID int64) (*FundAnalytics, error) {
	dto, err := s.Details(ctx, fundID)
	if err != nil {
		return nil, err
	}
	metrics := s.stats.MetricsFor(ctx, fundID)
	history, err := s.snapshots.History(ctx, fundID)
	if err != nil {
		return nil, err
	}
	avg, err := s.snapshots.AveragePerformance(ctx, fundID)
	if err != nil {
		return nil, err
	}
	// Java renders empty collections as []; a nil Go slice would marshal to null.
	if history == nil {
		history = []FundValueSnapshot{}
	}
	if avg == nil {
		avg = []PerformanceComparisonPoint{}
	}
	return &FundAnalytics{
		Fund:                         *dto,
		Metrics:                      metrics,
		HistoricalValuePoints:        history,
		AverageFundPerformancePoints: avg,
	}, nil
}

// SupervisedBy mirrors supervisedBy(managerId).
func (s *Service) SupervisedBy(ctx context.Context, managerID int64) ([]FundDto, error) {
	funds, err := s.repo.FindFundsByManager(ctx, managerID)
	if err != nil {
		return nil, err
	}
	out := make([]FundDto, 0, len(funds))
	for i := range funds {
		out = append(out, *s.toDto(ctx, &funds[i]))
	}
	return out, nil
}

// MyPositions / BankPositions / FundPositions mirror the three position
// projections. MyPositions and BankPositions filter by client; FundPositions
// returns every (client, fund) row for one fund.
func (s *Service) MyPositions(ctx context.Context, clientID int64) ([]PositionDto, error) {
	positions, err := s.repo.FindPositionsByClient(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return s.positionsToDtos(ctx, positions), nil
}

func (s *Service) BankPositions(ctx context.Context) ([]PositionDto, error) {
	return s.MyPositions(ctx, BankInvestorID)
}

func (s *Service) FundPositions(ctx context.Context, fundID int64) ([]PositionDto, error) {
	exists, err := s.repo.FundExists(ctx, nil, fundID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, api.NewOtcError(http.StatusNotFound, "Fond "+itoa(fundID)+" ne postoji.")
	}
	positions, err := s.repo.FindPositionsByFund(ctx, nil, fundID)
	if err != nil {
		return nil, err
	}
	return s.positionsToDtos(ctx, positions), nil
}

// MyTransactions / FundTransactions mirror the audit-log reads. Both map the raw
// ClientFundTransaction rows to the camelCase DTO (the raw struct's Go field names
// would otherwise leak as PascalCase JSON keys, breaking the frontend contract).
func (s *Service) MyTransactions(ctx context.Context, clientID int64) ([]ClientFundTransactionDto, error) {
	txs, err := s.repo.FindTransactionsByClient(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return transactionsToDtos(txs), nil
}

func (s *Service) FundTransactions(ctx context.Context, fundID int64) ([]ClientFundTransactionDto, error) {
	exists, err := s.repo.FundExists(ctx, nil, fundID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, api.NewOtcError(http.StatusNotFound, "Fond "+itoa(fundID)+" ne postoji.")
	}
	txs, err := s.repo.FindTransactionsByFund(ctx, fundID)
	if err != nil {
		return nil, err
	}
	return transactionsToDtos(txs), nil
}

// transactionsToDtos maps raw transaction rows to the public DTO. The make with a
// zero length (not nil) ensures an empty result marshals to [] like Java, not null.
func transactionsToDtos(txs []ClientFundTransaction) []ClientFundTransactionDto {
	out := make([]ClientFundTransactionDto, 0, len(txs))
	for _, t := range txs {
		out = append(out, ClientFundTransactionDto{
			ID:                  t.ID,
			ClientID:            t.ClientID,
			FundID:              t.FundID,
			Amount:              t.Amount,
			Inflow:              t.Inflow,
			Status:              t.Status,
			OccurredAt:          api.NewLocalDateTime(t.OccurredAt),
			ClientAccountNumber: t.ClientAccountNumber,
			FailureReason:       t.FailureReason,
		})
	}
	return out
}

// FundPerformance mirrors InvestmentFundService.fundPerformance.
func (s *Service) FundPerformance(ctx context.Context, fundID int64) ([]FundPerformancePoint, error) {
	fund, err := s.repo.FindFundByID(ctx, nil, fundID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, api.NewOtcError(http.StatusNotFound, "Fond "+itoa(fundID)+" ne postoji.")
		}
		return nil, err
	}
	totalValue := s.computeFundValue(ctx, fund)
	positions, err := s.repo.FindPositionsByFund(ctx, nil, fundID)
	if err != nil {
		return nil, err
	}
	investedSum := decimal.Zero
	for _, p := range positions {
		investedSum = investedSum.Add(p.TotalInvested)
	}
	profit := totalValue.Sub(investedSum)
	txs, err := s.repo.FindTransactionsByFund(ctx, fundID)
	if err != nil {
		return nil, err
	}
	if len(txs) == 0 {
		return []FundPerformancePoint{{
			Timestamp:  fund.DatumKreiranja,
			TotalValue: totalValue,
			Profit:     profit,
		}}, nil
	}
	out := make([]FundPerformancePoint, 0, len(txs))
	for _, t := range txs {
		amt := t.Amount
		inflow := t.Inflow
		status := t.Status
		out = append(out, FundPerformancePoint{
			Timestamp:     t.OccurredAt,
			TransactionID: &t.ID,
			Amount:        &amt,
			Inflow:        &inflow,
			Status:        &status,
			TotalValue:    totalValue,
			Profit:        profit,
		})
	}
	return out, nil
}

// GetEnrichedHoldings exposes /funds/{id}/securities (delegates).
func (s *Service) GetEnrichedHoldings(ctx context.Context, fundID int64) ([]EnrichedView, error) {
	exists, err := s.repo.FundExists(ctx, nil, fundID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, api.NewOtcError(http.StatusNotFound, "Fond "+itoa(fundID)+" ne postoji.")
	}
	return s.holdings.EnrichedHoldings(ctx, fundID)
}

// ---------------------- invest / redeem -----------------------------------

// Invest mirrors InvestmentFundService.invest. Reserves the PENDING
// transaction in DB, publishes fund.subscribe.requested AFTER commit. Result-
// listener completes via CompleteInvest. Bad amount → OTC 404; missing fund
// or position → OTC 404.
func (s *Service) Invest(ctx context.Context, fundID, clientID int64, amount decimal.Decimal, fromAccount string) (*ClientFundTransaction, error) {
	return s.investImpl(ctx, fundID, clientID, amount, fromAccount, false)
}

// BankInvest mirrors bankInvest: the bank itself invests, using
// BankInvestorID as the position owner.
func (s *Service) BankInvest(ctx context.Context, fundID int64, amount decimal.Decimal, fromAccount string) (*ClientFundTransaction, error) {
	return s.investImpl(ctx, fundID, BankInvestorID, amount, fromAccount, false)
}

func (s *Service) investImpl(ctx context.Context, fundID, clientID int64, amount decimal.Decimal, fromAccount string, _ bool) (*ClientFundTransaction, error) {
	var saved *ClientFundTransaction
	var fundAccount string
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		fund, err := s.repo.FindFundByIDForUpdate(ctx, tx, fundID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return api.NewOtcError(http.StatusNotFound, "Fond "+itoa(fundID)+" ne postoji.")
			}
			return err
		}
		if err := s.ensureFundAccountExists(ctx, fund); err != nil {
			return err
		}
		if amount.Cmp(fund.MinimumContribution) < 0 {
			return api.NewOtcError(http.StatusNotFound,
				"Iznos manji od minimumContribution ("+fund.MinimumContribution.String()+").")
		}
		fundAccount = fund.AccountNumber
		t := &ClientFundTransaction{
			ClientID:            clientID,
			FundID:              fundID,
			Amount:              amount,
			Inflow:              true,
			Status:              TxStatusPending,
			ClientAccountNumber: fromAccount,
		}
		if err := s.repo.InsertTransaction(ctx, tx, t); err != nil {
			return err
		}
		saved = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	// publish-after-commit (best-effort) — mirrors Java
	// TransactionSynchronizationManager.afterCommit.
	if err := s.publisher.PublishSubscribeRequested(ctx, FundSubscribeRequestedEvent{
		TransactionID:     strconv.FormatInt(saved.ID, 10),
		ClientID:          saved.ClientID,
		FundID:            saved.FundID,
		Amount:            saved.Amount,
		FromAccountNumber: saved.ClientAccountNumber,
		FundAccountNumber: fundAccount,
	}); err != nil {
		s.logger.Warn("fund.subscribe.requested publish failed (tx stays PENDING)",
			"transactionId", saved.ID, "error", err)
	}
	return saved, nil
}

// Redeem mirrors InvestmentFundService.redeem. Validates the position
// covers the requested amount, then publishes fund.redeem.requested OR
// fund.redeem.with-liquidation.requested depending on fund liquidity.
func (s *Service) Redeem(ctx context.Context, fundID, clientID int64, amount decimal.Decimal, toAccount string) (*ClientFundTransaction, error) {
	return s.redeemImpl(ctx, fundID, clientID, amount, toAccount)
}

// BankRedeem mirrors bankRedeem.
func (s *Service) BankRedeem(ctx context.Context, fundID int64, amount decimal.Decimal, toAccount string) (*ClientFundTransaction, error) {
	return s.redeemImpl(ctx, fundID, BankInvestorID, amount, toAccount)
}

func (s *Service) redeemImpl(ctx context.Context, fundID, clientID int64, amount decimal.Decimal, toAccount string) (*ClientFundTransaction, error) {
	var saved *ClientFundTransaction
	var fundAccount string
	var liquidEnough bool
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		fund, err := s.repo.FindFundByIDForUpdate(ctx, tx, fundID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return api.NewOtcError(http.StatusNotFound, "Fond "+itoa(fundID)+" ne postoji.")
			}
			return err
		}
		if err := s.ensureFundAccountExists(ctx, fund); err != nil {
			return err
		}
		pos, err := s.repo.FindPositionForUpdate(ctx, tx, clientID, fundID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				if clientID == BankInvestorID {
					return api.NewOtcError(http.StatusNotFound, "Banka nema poziciju u fondu "+itoa(fundID))
				}
				return api.NewOtcError(http.StatusNotFound,
					"Klijent "+itoa(clientID)+" nema poziciju u fondu "+itoa(fundID))
			}
			return err
		}
		current := s.computeCurrentPositionValue(ctx, pos, fund)
		if amount.Cmp(current) > 0 {
			msg := "Trazena isplata (" + amount.String() + ") veca od trenutne vrednosti pozicije (" + current.String() + ")."
			if clientID == BankInvestorID {
				msg = "Trazena isplata (" + amount.String() + ") veca od trenutne vrednosti bankine pozicije (" + current.String() + ")."
			}
			return api.NewOtcError(http.StatusNotFound, msg)
		}
		fundAccount = fund.AccountNumber
		liquidEnough = fund.LikvidnaSredstva.Cmp(amount) >= 0
		t := &ClientFundTransaction{
			ClientID:            clientID,
			FundID:              fundID,
			Amount:              amount,
			Inflow:              false,
			Status:              TxStatusPending,
			ClientAccountNumber: toAccount,
		}
		if err := s.repo.InsertTransaction(ctx, tx, t); err != nil {
			return err
		}
		saved = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.publisher.PublishRedeemRequested(ctx, FundRedeemRequestedEvent{
		TransactionID:     strconv.FormatInt(saved.ID, 10),
		ClientID:          saved.ClientID,
		FundID:            saved.FundID,
		Amount:            saved.Amount,
		ToAccountNumber:   saved.ClientAccountNumber,
		FundAccountNumber: fundAccount,
		LiquidEnough:      liquidEnough,
	}); err != nil {
		s.logger.Warn("fund.redeem.requested publish failed (tx stays PENDING)",
			"transactionId", saved.ID, "liquidEnough", liquidEnough, "error", err)
	}
	return saved, nil
}

// ---------------------- saga callbacks ------------------------------------

// CompleteInvest mirrors InvestmentFundService.completeInvest. Idempotent —
// no-op when the transaction has already left PENDING.
func (s *Service) CompleteInvest(ctx context.Context, txID, clientID, fundID int64, amount decimal.Decimal) error {
	return gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		t, err := s.repo.FindTransactionByID(ctx, tx, txID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				s.logger.Warn("completeInvest: tx not found — skip", "txId", txID)
				return nil
			}
			return err
		}
		if t.Status != TxStatusPending {
			s.logger.Info("completeInvest: tx already terminal — skip", "txId", txID, "status", t.Status)
			return nil
		}
		if err := s.repo.UpdateTransactionStatus(ctx, tx, txID, TxStatusCompleted, nil); err != nil {
			return err
		}
		pos, err := s.repo.FindPosition(ctx, tx, clientID, fundID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		now := time.Now().UTC()
		if pos == nil {
			pos = &ClientFundPosition{ClientID: clientID, FundID: fundID, TotalInvested: decimal.Zero}
		}
		pos.TotalInvested = pos.TotalInvested.Add(amount)
		pos.LastModifiedAt = &now
		if err := s.repo.UpsertPosition(ctx, tx, pos); err != nil {
			return err
		}
		fund, err := s.repo.FindFundByIDForUpdate(ctx, tx, fundID)
		if err != nil {
			return err
		}
		newLiq := fund.LikvidnaSredstva.Add(amount)
		if err := s.repo.UpdateFundLiquidity(ctx, tx, fundID, newLiq); err != nil {
			return err
		}
		return nil
	})
}

// CompleteRedeem mirrors completeRedeem (both fast and liquidation paths).
// On liquidation, FundLiquidationService already added to liquidity in step 1;
// this step subtracts what banking-core actually moved in step 2 (mirrors
// Java's two-step net-zero balance).
func (s *Service) CompleteRedeem(ctx context.Context, txID, clientID, fundID int64, amount decimal.Decimal) error {
	return gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		t, err := s.repo.FindTransactionByID(ctx, tx, txID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				s.logger.Warn("completeRedeem: tx not found — skip", "txId", txID)
				return nil
			}
			return err
		}
		if t.Status != TxStatusPending {
			s.logger.Info("completeRedeem: tx already terminal — skip", "txId", txID, "status", t.Status)
			return nil
		}
		if err := s.repo.UpdateTransactionStatus(ctx, tx, txID, TxStatusCompleted, nil); err != nil {
			return err
		}
		pos, err := s.repo.FindPosition(ctx, tx, clientID, fundID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		if pos != nil {
			now := time.Now().UTC()
			newInvested := pos.TotalInvested.Sub(amount)
			if newInvested.Sign() < 0 {
				newInvested = decimal.Zero
			}
			pos.TotalInvested = newInvested
			pos.LastModifiedAt = &now
			if err := s.repo.UpsertPosition(ctx, tx, pos); err != nil {
				return err
			}
		}
		fund, err := s.repo.FindFundByIDForUpdate(ctx, tx, fundID)
		if err != nil {
			return err
		}
		newLiq := fund.LikvidnaSredstva.Sub(amount)
		if newLiq.Sign() < 0 {
			newLiq = decimal.Zero
		}
		if err := s.repo.UpdateFundLiquidity(ctx, tx, fundID, newLiq); err != nil {
			return err
		}
		return nil
	})
}

// FailTransaction mirrors InvestmentFundService.failTransaction. Idempotent —
// no-op when already terminal.
func (s *Service) FailTransaction(ctx context.Context, txID int64, reason string) error {
	return gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		t, err := s.repo.FindTransactionByID(ctx, tx, txID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				s.logger.Warn("failTransaction: tx not found", "txId", txID)
				return nil
			}
			return err
		}
		if t.Status != TxStatusPending {
			s.logger.Info("failTransaction: tx already terminal — skip", "txId", txID, "status", t.Status)
			return nil
		}
		r := reason
		return s.repo.UpdateTransactionStatus(ctx, tx, txID, TxStatusFailed, &r)
	})
}

// ---------------------- internals ----------------------------------------

func (s *Service) computeFundValue(ctx context.Context, fund *InvestmentFund) decimal.Decimal {
	holdings := s.holdings.CalculateHoldingsValue(ctx, fund.ID)
	return fund.LikvidnaSredstva.Add(holdings)
}

// computeCurrentPositionValue mirrors computeCurrentPositionValue. Uses
// max(totalFundInvested, fundValue) as denominator (the "orphaned assets"
// fix in Java). Scale 10 in the percentage division, scale 2 in the final
// RSD output — both HALF_UP via Round.
func (s *Service) computeCurrentPositionValue(ctx context.Context, pos *ClientFundPosition, fund *InvestmentFund) decimal.Decimal {
	fundValue := s.computeFundValue(ctx, fund)
	positions, err := s.repo.FindPositionsByFund(ctx, nil, fund.ID)
	if err != nil {
		return decimal.Zero
	}
	totalInvested := decimal.Zero
	for _, p := range positions {
		totalInvested = totalInvested.Add(p.TotalInvested)
	}
	if totalInvested.Sign() <= 0 || fundValue.Sign() <= 0 {
		return decimal.Zero
	}
	denom := totalInvested
	if fundValue.Cmp(denom) > 0 {
		denom = fundValue
	}
	pct := pos.TotalInvested.Div(denom).Round(10)
	return pct.Mul(fundValue).Round(2)
}

func (s *Service) ensureFundAccountExists(ctx context.Context, fund *InvestmentFund) error {
	if _, err := s.account.CreateSystemAccount(ctx, fund.AccountNumber, -1000-fund.ID, FundBaseCurrency,
		"Investicioni fond: "+fund.Naziv, fund.LikvidnaSredstva); err != nil {
		return api.NewOtcError(http.StatusConflict,
			"Racun fonda "+fund.AccountNumber+" nije dostupan: "+err.Error())
	}
	return nil
}

func (s *Service) positionsToDtos(ctx context.Context, positions []ClientFundPosition) []PositionDto {
	out := make([]PositionDto, 0, len(positions))
	for _, p := range positions {
		out = append(out, s.positionToDto(ctx, p))
	}
	return out
}

func (s *Service) positionToDto(ctx context.Context, pos ClientFundPosition) PositionDto {
	fund, _ := s.repo.FindFundByID(ctx, nil, pos.FundID)
	naziv := "?"
	var opis *string
	fundValue := decimal.Zero
	if fund != nil {
		naziv = fund.Naziv
		opis = fund.Opis
		fundValue = s.computeFundValue(ctx, fund)
	}
	positions, _ := s.repo.FindPositionsByFund(ctx, nil, pos.FundID)
	totalInvested := decimal.Zero
	for _, p := range positions {
		totalInvested = totalInvested.Add(p.TotalInvested)
	}
	var pct, current decimal.Decimal
	if totalInvested.Sign() <= 0 || fundValue.Sign() <= 0 {
		pct = decimal.Zero
		current = decimal.Zero
	} else {
		denom := totalInvested
		if fundValue.Cmp(denom) > 0 {
			denom = fundValue
		}
		pct = pos.TotalInvested.Div(denom).Round(6)
		current = pct.Mul(fundValue).Round(2)
	}
	profit := current.Sub(pos.TotalInvested)
	return PositionDto{
		ID:                   pos.ID,
		ClientID:             pos.ClientID,
		FundID:               pos.FundID,
		FundNaziv:            naziv,
		FundOpis:             opis,
		FundTotalValue:       fundValue,
		TotalInvested:        pos.TotalInvested,
		PercentageOfFund:     pct,
		CurrentPositionValue: current,
		ClientProfit:         profit,
		FirstInvestedAt:      api.NewLocalDateTime(pos.FirstInvestedAt),
		LastModifiedAt:       api.LocalDateTimeFromPtr(pos.LastModifiedAt),
	}
}

func (s *Service) toDto(ctx context.Context, fund *InvestmentFund) *FundDto {
	holdings := s.holdings.CalculateHoldingsValue(ctx, fund.ID)
	totalValue := fund.LikvidnaSredstva.Add(holdings)
	positions, _ := s.repo.FindPositionsByFund(ctx, nil, fund.ID)
	investedSum := decimal.Zero
	for _, p := range positions {
		investedSum = investedSum.Add(p.TotalInvested)
	}
	profit := totalValue.Sub(investedSum)
	var managerIme, managerPrezime *string
	if fund.ManagerID != 0 {
		if emp, err := s.employee.GetEmployee(ctx, fund.ManagerID); err == nil && emp != nil {
			managerIme = emp.Ime
			managerPrezime = emp.Prezime
		}
	}
	var accountID *int64
	if fund.AccountNumber != "" {
		if det, err := s.account.GetAccountDetailsByNumber(ctx, fund.AccountNumber); err == nil && det != nil {
			if det.AccountNumber != nil || det.BrojRacuna != nil {
				// no id in the model; reuse the inferred id from `accountId`
				// alias (banking-core often returns the PK as `id`). For parity
				// with Java which calls account.id(), keep the AccountDetails
				// shape via OwnerID? No — Java returns account.id(). We omit if
				// not exposed by AccountDetails — accountId stays nil. This
				// matches the tolerant Java fallback (returns null when not
				// available).
				_ = det
			}
		}
	}
	metrics := s.stats.MetricsFor(ctx, fund.ID)
	return &FundDto{
		ID:                       fund.ID,
		Naziv:                    fund.Naziv,
		Opis:                     fund.Opis,
		MinimumContribution:      fund.MinimumContribution,
		ManagerID:                fund.ManagerID,
		ManagerIme:               managerIme,
		ManagerPrezime:           managerPrezime,
		LikvidnaSredstva:         fund.LikvidnaSredstva,
		AccountID:                accountID,
		AccountNumber:            fund.AccountNumber,
		DividendStrategy:         fund.DividendStrategy,
		DatumKreiranja:           api.NewLocalDate(fund.DatumKreiranja),
		TotalValue:               totalValue,
		Profit:                   profit,
		AnnualizedReturn:         metrics.AnnualizedReturn,
		RewardToVariabilityRatio: metrics.RewardToVariabilityRatio,
		MaxDrawdown:              metrics.MaxDrawdown,
		Volatility:               metrics.Volatility,
	}
}

func ptrDecimal(v decimal.Decimal) *decimal.Decimal { return &v }
