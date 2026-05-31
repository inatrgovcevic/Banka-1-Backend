package funds

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/clients"

	gpdb "banka1/go-platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// DividendService mirrors FundDividendService. recordDividend is the single
// public entry: credit gross to fund liquidity, then split per strategy
// (REINVEST → buy more shares; PAYOUT_CLIENTS → proportional last-client-
// remainder split). Idempotent via unique constraint (fund_id, stock_ticker,
// payment_date) — duplicate call → IllegalStateException → OTC 409.
type DividendService struct {
	repo      *Repository
	holdings  *HoldingService
	snapshots *SnapshotService
	market    *clients.MarketClient
	account   *clients.AccountClient
	funds     *Service
	logger    *slog.Logger
}

func NewDividendService(repo *Repository, holdings *HoldingService, snapshots *SnapshotService, market *clients.MarketClient, account *clients.AccountClient, funds *Service, logger *slog.Logger) *DividendService {
	return &DividendService{
		repo: repo, holdings: holdings, snapshots: snapshots, market: market,
		account: account, funds: funds, logger: logger,
	}
}

// RecordDividend mirrors FundDividendService.recordDividend.
func (s *DividendService) RecordDividend(ctx context.Context, fundID int64, req DividendRequest) (*DividendDistributionDto, error) {
	ticker := strings.ToUpper(req.StockTicker)
	paymentDate := req.PaymentDate
	if paymentDate.IsZero() {
		paymentDate = time.Now().UTC().Truncate(24 * time.Hour)
	}

	// Idempotency: existing distribution for (fund, ticker, date) → 409.
	if _, err := s.repo.FindDistribution(ctx, fundID, ticker, paymentDate); err == nil {
		return nil, api.NewOtcError(http.StatusConflict,
			"Dividenda za fond "+itoa(fundID)+", ticker "+ticker+
				" i datum "+paymentDate.Format("2006-01-02")+" je vec evidentirana.")
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	var (
		distribution *FundDividendDistribution
		payouts      []FundDividendPayout
	)
	err := gpdb.RunInTx(ctx, s.repo.Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		fund, err := s.repo.FindFundByIDForUpdate(ctx, tx, fundID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return api.NewOtcError(http.StatusNotFound, "Fond "+itoa(fundID)+" ne postoji.")
			}
			return err
		}
		holding, err := s.repo.FindHolding(ctx, tx, fundID, ticker)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return api.NewOtcError(http.StatusNotFound,
					"Fond "+itoa(fundID)+" ne poseduje holding za ticker "+ticker+".")
			}
			return err
		}
		if holding.Quantity <= 0 {
			return api.NewOtcError(http.StatusNotFound,
				"Holding za ticker "+ticker+" nema raspolozivu kolicinu.")
		}
		strategy := req.Strategy
		if strategy == "" {
			strategy = fund.DividendStrategy
		}
		grossSource := req.DividendPerShare.
			Mul(decimal.NewFromInt(int64(holding.Quantity))).
			Round(8)
		grossRsd, ok := s.convertToRsd(ctx, grossSource, req.Currency)
		if !ok {
			// FX outage: faithful to Java, fall back to grossSource (no FX
			// conversion). This is "tolerant" per FundDividendService.convertToRsd.
			grossRsd = grossSource
		}
		grossRsd = grossRsd.Round(2)

		// credit fund liquidity + bank account (commit before the strategy
		// split — matching FundDividendService.creditFundDividend).
		newLiq := fund.LikvidnaSredstva.Add(grossRsd).Round(2)
		if err := s.repo.UpdateFundLiquidity(ctx, tx, fundID, newLiq); err != nil {
			return err
		}
		fund.LikvidnaSredstva = newLiq
		if err := s.account.CreditAccount(ctx, fund.AccountNumber, grossRsd, -1000-fund.ID); err != nil {
			return api.NewOtcError(http.StatusConflict,
				"AccountServiceClient credit za fond "+itoa(fundID)+" nije uspeo: "+err.Error())
		}

		dist := &FundDividendDistribution{
			FundID:            fundID,
			StockTicker:       ticker,
			PaymentDate:       paymentDate,
			DividendPerShare:  req.DividendPerShare,
			SourceCurrency:    strings.ToUpper(req.Currency),
			HoldingQuantity:   holding.Quantity,
			GrossAmountSource: grossSource,
			GrossAmountRsd:    grossRsd,
			Strategy:          strategy,
			Status:            DistStatusCompleted,
		}

		switch strategy {
		case DividendReinvest:
			payouts = s.handleReinvest(ctx, tx, dist, fund, holding, grossRsd)
		case DividendPayoutClients:
			payouts = s.handleClientPayouts(ctx, tx, dist, fund, grossRsd)
		default:
			return api.NewOtcError(http.StatusNotFound, "Nepoznata strategija dividende: "+strategy)
		}

		if err := s.repo.InsertDistribution(ctx, tx, dist); err != nil {
			return err
		}
		for i := range payouts {
			payouts[i].DistributionID = dist.ID
			if err := s.repo.InsertPayout(ctx, tx, &payouts[i]); err != nil {
				return err
			}
		}
		distribution = dist
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.snapshots.RecordSilently(ctx, fundID)
	return s.toDto(distribution, payouts), nil
}

// handleReinvest mirrors FundDividendService.handleReinvestment. Best-effort:
// if the live price is missing or the gross is below one share, the dividend
// stays in liquidity (no shares purchased), status flagged
// COMPLETED_WITH_WARNINGS.
func (s *DividendService) handleReinvest(ctx context.Context, tx pgx.Tx, dist *FundDividendDistribution, fund *InvestmentFund, holding *FundHolding, grossRsd decimal.Decimal) []FundDividendPayout {
	priceUsd, ok := s.market.CurrentPrice(ctx, holding.StockTicker)
	if !ok || priceUsd.Sign() <= 0 {
		priceUsd = holding.AvgUnitPrice
	}
	if priceUsd.Sign() <= 0 {
		dist.Status = DistStatusCompletedWithWarnings
		note := "Dividenda knjizena u likvidnost; nema validne trzisne cene za reinvestiranje."
		dist.Note = &note
		zeroShares := 0
		dist.ReinvestedShares = &zeroShares
		zero := decimal.Zero.Round(2)
		dist.ReinvestedAmountRsd = &zero
		return nil
	}
	priceRsd, ok := s.market.ConvertNoCommission(ctx, priceUsd, HoldingPriceCurrency, FundBaseCurrency)
	if !ok {
		priceRsd = priceUsd
	}
	priceRsd = priceRsd.Round(8)
	sharesD := grossRsd.Div(priceRsd).Floor()
	shares := int(sharesD.IntPart())
	if shares <= 0 {
		zero := decimal.Zero.Round(2)
		zeroShares := 0
		dist.ReinvestedShares = &zeroShares
		dist.ReinvestedAmountRsd = &zero
		note := "Dividenda knjizena u likvidnost; iznos nije dovoljan za kupovinu cele akcije."
		dist.Note = &note
		return nil
	}
	reinvestAmount := priceRsd.Mul(decimal.NewFromInt(int64(shares))).Round(2)
	if _, err := s.holdings.AddOrUpdate(ctx, tx, fund.ID, holding.StockTicker, shares, priceUsd); err != nil {
		s.logger.Warn("dividend reinvest add holding failed", "fundId", fund.ID, "ticker", holding.StockTicker, "error", err)
	}
	if err := s.funds.DebitLiquidity(ctx, fund.ID, reinvestAmount, "Fund dividend reinvestment"); err != nil {
		s.logger.Warn("dividend reinvest debit liquidity failed", "fundId", fund.ID, "error", err)
	}
	if err := s.account.DebitAccount(ctx, fund.AccountNumber, reinvestAmount, -1000-fund.ID); err != nil {
		s.logger.Warn("dividend reinvest account debit failed", "fundId", fund.ID, "error", err)
	}
	dist.ReinvestedShares = &shares
	dist.ReinvestedAmountRsd = &reinvestAmount
	zero := decimal.Zero.Round(2)
	dist.DistributedAmountRsd = &zero
	note := "Dividenda reinvestirana u dodatne hartije."
	dist.Note = &note
	return nil
}

// handleClientPayouts mirrors FundDividendService.handleClientPayouts. Sort by
// clientId asc (matches Java Comparator.comparing(::getClientId)). Each
// non-last share uses HALF_UP at scale 2; the last client absorbs the rounding
// remainder (grossRsd - distributedSoFar).
func (s *DividendService) handleClientPayouts(ctx context.Context, tx pgx.Tx, dist *FundDividendDistribution, fund *InvestmentFund, grossRsd decimal.Decimal) []FundDividendPayout {
	positions, err := s.repo.FindPositionsByFund(ctx, tx, fund.ID)
	if err != nil {
		s.logger.Warn("dividend payout: load positions failed", "fundId", fund.ID, "error", err)
		positions = nil
	}
	clientPositions := make([]ClientFundPosition, 0, len(positions))
	for _, p := range positions {
		if p.ClientID == BankInvestorID {
			continue
		}
		clientPositions = append(clientPositions, p)
	}
	sort.Slice(clientPositions, func(i, j int) bool { return clientPositions[i].ClientID < clientPositions[j].ClientID })
	if len(clientPositions) == 0 {
		dist.Status = DistStatusCompletedWithWarnings
		zero := decimal.Zero.Round(2)
		dist.DistributedAmountRsd = &zero
		note := "Fond nema klijentske ucesnike; dividenda ostaje u likvidnim sredstvima fonda."
		dist.Note = &note
		return nil
	}
	holdingsValue := s.holdings.CalculateHoldingsValue(ctx, fund.ID)
	beforeDividend := fund.LikvidnaSredstva.Add(holdingsValue).Sub(grossRsd)
	if beforeDividend.Sign() < 0 {
		beforeDividend = decimal.Zero
	}
	totalInvested := decimal.Zero
	for _, p := range clientPositions {
		totalInvested = totalInvested.Add(p.TotalInvested)
	}
	denom := totalInvested
	if beforeDividend.Cmp(denom) > 0 {
		denom = beforeDividend
	}
	if denom.Sign() <= 0 {
		dist.Status = DistStatusCompletedWithWarnings
		zero := decimal.Zero.Round(2)
		dist.DistributedAmountRsd = &zero
		note := "Nema dovoljno osnova za proporcionalnu raspodelu dividende."
		dist.Note = &note
		return nil
	}
	distributed := decimal.Zero
	out := make([]FundDividendPayout, 0, len(clientPositions))
	for i, pos := range clientPositions {
		ratio := pos.TotalInvested.Div(denom).Round(8)
		var amount decimal.Decimal
		if i == len(clientPositions)-1 {
			amount = grossRsd.Sub(distributed)
		} else {
			amount = grossRsd.Mul(ratio).Round(2)
		}
		if amount.Sign() <= 0 {
			continue
		}
		p := FundDividendPayout{
			ClientID:       pos.ClientID,
			OwnershipRatio: ratio,
			AmountRsd:      amount,
		}
		clientAcc := s.account.GetDefaultRsdAccountNumberForOwner(ctx, pos.ClientID)
		if clientAcc != "" {
			p.ClientAccountNumber = &clientAcc
		}
		if clientAcc == "" {
			p.Status = PayoutStatusSkipped
			reason := "Klijent nema podrazumevani RSD racun."
			p.FailureReason = &reason
			dist.Status = DistStatusCompletedWithWarnings
		} else {
			payment := clients.Payment{
				FromAccountNumber: fund.AccountNumber,
				ToAccountNumber:   clientAcc,
				FromAmount:        amount,
				ToAmount:          amount,
				Commission:        decimal.Zero.Round(2),
				ClientID:          -1000 - fund.ID,
			}
			if err := s.account.Transaction(ctx, payment); err != nil {
				p.Status = PayoutStatusFailed
				msg := err.Error()
				p.FailureReason = &msg
				dist.Status = DistStatusCompletedWithWarnings
			} else {
				if err := s.funds.DebitLiquidity(ctx, fund.ID, amount, "Fund dividend payout"); err != nil {
					s.logger.Warn("dividend payout: debit liquidity failed", "fundId", fund.ID, "error", err)
				}
				p.Status = PayoutStatusCompleted
				distributed = distributed.Add(amount)
			}
		}
		out = append(out, p)
	}
	distRsd := distributed.Round(2)
	dist.DistributedAmountRsd = &distRsd
	if dist.Note == nil {
		note := "Dividenda raspodeljena klijentima proporcionalno udelu."
		dist.Note = &note
	}
	return out
}

// convertToRsd mirrors FundDividendService.convertToRsd. Same-currency = pass-
// through (true). FX failure → false (caller falls back to source amount).
func (s *DividendService) convertToRsd(ctx context.Context, amount decimal.Decimal, fromCurrency string) (decimal.Decimal, bool) {
	if fromCurrency == "" || strings.EqualFold(fromCurrency, FundBaseCurrency) {
		return amount, true
	}
	return s.market.ConvertNoCommission(ctx, amount, strings.ToUpper(fromCurrency), FundBaseCurrency)
}

func (s *DividendService) toDto(d *FundDividendDistribution, payouts []FundDividendPayout) *DividendDistributionDto {
	dto := &DividendDistributionDto{
		ID:                   d.ID,
		FundID:               d.FundID,
		StockTicker:          d.StockTicker,
		PaymentDate:          api.NewLocalDate(d.PaymentDate),
		DividendPerShare:     d.DividendPerShare,
		SourceCurrency:       d.SourceCurrency,
		HoldingQuantity:      d.HoldingQuantity,
		GrossAmountSource:    d.GrossAmountSource,
		GrossAmountRsd:       d.GrossAmountRsd,
		Strategy:             d.Strategy,
		Status:               d.Status,
		ReinvestedShares:     d.ReinvestedShares,
		ReinvestedAmountRsd:  d.ReinvestedAmountRsd,
		DistributedAmountRsd: d.DistributedAmountRsd,
		Note:                 d.Note,
		ProcessedAt:          api.NewLocalDateTime(d.ProcessedAt),
		Payouts:              make([]DividendPayoutDto, 0, len(payouts)),
	}
	for _, p := range payouts {
		dto.Payouts = append(dto.Payouts, DividendPayoutDto{
			ClientID:            p.ClientID,
			ClientAccountNumber: p.ClientAccountNumber,
			OwnershipRatio:      p.OwnershipRatio,
			AmountRsd:           p.AmountRsd,
			Status:              p.Status,
			FailureReason:       p.FailureReason,
		})
	}
	return dto
}

// DividendRequest mirrors funds RecordFundDividendRequest. CamelCase JSON keys
// match the Java DTO.
type DividendRequest struct {
	StockTicker      string          `json:"stockTicker"`
	DividendPerShare decimal.Decimal `json:"dividendPerShare"`
	Currency         string          `json:"currency"`
	PaymentDate      time.Time       `json:"paymentDate"`
	Strategy         string          `json:"strategy"`
}
