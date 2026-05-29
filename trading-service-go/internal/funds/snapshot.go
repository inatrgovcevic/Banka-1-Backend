package funds

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// SnapshotService mirrors FundValueSnapshotService. recordSnapshot is the
// invariant write — every fund-mutating path (invest, redeem, create, debit
// liquidity, holding add, liquidation, dividend) calls it. monthlySnapshots
// projects the daily history into the per-month tail used by the statistics
// engine; averagePerformance produces the "fund vs peers" comparison curve.
type SnapshotService struct {
	repo    *Repository
	holding *HoldingService
	logger  *slog.Logger
}

// NewSnapshotService wires the snapshot service. The holding service is
// required so the snapshot can capture the current (live-priced) holdings
// value alongside cash liquidity.
func NewSnapshotService(repo *Repository, holding *HoldingService, logger *slog.Logger) *SnapshotService {
	return &SnapshotService{repo: repo, holding: holding, logger: logger}
}

// Record persists today's snapshot for fundId (mirrors recordSnapshot). Idempotent
// per (fund_id, snapshot_date) — re-running on the same day overwrites the
// captured values. Holdings value is priced live via market-service with
// avgUnitPrice fallback (see HoldingService.CalculateHoldingsValue).
func (s *SnapshotService) Record(ctx context.Context, fundID int64, day time.Time) (*FundValueSnapshot, error) {
	fund, err := s.repo.FindFundByID(ctx, nil, fundID)
	if err != nil {
		return nil, err
	}
	holdings := s.holding.CalculateHoldingsValue(ctx, fundID)
	liquidity := fund.LikvidnaSredstva.RoundBank(2)
	holdings = holdings.RoundBank(2)
	total := liquidity.Add(holdings).RoundBank(2)
	snap := &FundValueSnapshot{
		FundID:         fundID,
		SnapshotDate:   day.UTC().Truncate(24 * time.Hour),
		LiquidityValue: liquidity,
		HoldingsValue:  holdings,
		TotalValue:     total,
	}
	if err := s.repo.UpsertSnapshot(ctx, nil, snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// RecordSilently logs but swallows record errors. Mirrors how the Java service
// calls recordSnapshot inside non-snapshot transactions — a snapshot failure
// must not roll back the host write (invest, dividend, etc.).
func (s *SnapshotService) RecordSilently(ctx context.Context, fundID int64) {
	if _, err := s.Record(ctx, fundID, time.Now().UTC()); err != nil {
		s.logger.Warn("fund snapshot record failed", "fundId", fundID, "error", err)
	}
}

// History mirrors FundValueSnapshotService.history.
func (s *SnapshotService) History(ctx context.Context, fundID int64) ([]FundValueSnapshot, error) {
	return s.repo.FindSnapshots(ctx, fundID)
}

// MonthlySnapshots mirrors FundValueSnapshotService.monthlySnapshots: keep the
// last snapshot in each calendar month (java.time.YearMonth bucket).
func (s *SnapshotService) MonthlySnapshots(ctx context.Context, fundID int64) ([]FundValueSnapshot, error) {
	all, err := s.History(ctx, fundID)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, nil
	}
	// last-per-month wins (history is already ASC by date)
	lastByMonth := make(map[string]FundValueSnapshot, len(all))
	for _, snap := range all {
		key := snap.SnapshotDate.UTC().Format("2006-01")
		lastByMonth[key] = snap
	}
	out := make([]FundValueSnapshot, 0, len(lastByMonth))
	for _, snap := range lastByMonth {
		out = append(out, snap)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].SnapshotDate.Before(out[j].SnapshotDate)
	})
	return out, nil
}

// AveragePerformance mirrors FundValueSnapshotService.averagePerformance: for
// each date present in fundId's history, compute (totalValue / firstSnapshot
// totalValue) * 100 for this fund AND the average across all active funds.
// Returns an empty slice when the fund has no history.
func (s *SnapshotService) AveragePerformance(ctx context.Context, fundID int64) ([]PerformanceComparisonPoint, error) {
	base, err := s.History(ctx, fundID)
	if err != nil {
		return nil, err
	}
	if len(base) == 0 {
		return nil, nil
	}
	if base[0].TotalValue.Sign() <= 0 {
		return nil, nil
	}

	funds, err := s.repo.FindFundsActive(ctx)
	if err != nil {
		return nil, err
	}
	all := make(map[int64][]FundValueSnapshot, len(funds))
	for _, f := range funds {
		snaps, err := s.repo.FindSnapshots(ctx, f.ID)
		if err != nil {
			return nil, err
		}
		all[f.ID] = snaps
	}

	dates := make([]time.Time, 0, len(base))
	for _, snap := range base {
		dates = append(dates, snap.SnapshotDate)
	}
	out := make([]PerformanceComparisonPoint, 0, len(dates))
	for _, date := range dates {
		fundIdx := indexForDate(base, date)
		sum := decimal.Zero
		count := 0
		for _, snaps := range all {
			idx := indexForDate(snaps, date)
			if idx != nil {
				sum = sum.Add(*idx)
				count++
			}
		}
		var avg *decimal.Decimal
		if count > 0 {
			a := sum.Div(decimal.NewFromInt(int64(count))).Round(4)
			avg = &a
		}
		out = append(out, PerformanceComparisonPoint{
			SnapshotDate:            date,
			FundPerformanceIndex:    fundIdx,
			AveragePerformanceIndex: avg,
		})
	}
	return out, nil
}

// CaptureDailySnapshots mirrors FundValueSnapshotService.captureDailySnapshots
// (the @Scheduled hook). Used by the cron in NewApp when
// FUND_SNAPSHOT_SCHEDULER_ENABLED is true.
func (s *SnapshotService) CaptureDailySnapshots(ctx context.Context) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	funds, err := s.repo.FindFundsActive(ctx)
	if err != nil {
		s.logger.Error("snapshot scheduler: load funds failed", "error", err)
		return
	}
	for _, f := range funds {
		if _, err := s.Record(ctx, f.ID, today); err != nil {
			s.logger.Error("snapshot scheduler: record failed", "fundId", f.ID, "error", err)
			continue
		}
	}
	s.logger.Debug("captured daily fund snapshots", "date", today.Format("2006-01-02"), "count", len(funds))
}

// indexForDate mirrors FundValueSnapshotService.indexForDate. snapshots is the
// full history of ONE fund (ASC). Returns (totalValue / firstSnapshot.totalValue)
// * 100 for the snapshot on `date`, or nil if no exact match.
func indexForDate(snapshots []FundValueSnapshot, date time.Time) *decimal.Decimal {
	if len(snapshots) == 0 {
		return nil
	}
	base := snapshots[0].TotalValue
	if base.Sign() <= 0 {
		return nil
	}
	d := date.UTC()
	for _, snap := range snapshots {
		if snap.SnapshotDate.UTC().Equal(d) {
			idx := snap.TotalValue.Div(base).Round(4).Mul(decimal.NewFromInt(100))
			return &idx
		}
	}
	return nil
}

// PerformanceComparisonPoint is the output row of AveragePerformance. The HTTP
// layer projects it into FundPerformanceComparisonPointDto.
type PerformanceComparisonPoint struct {
	SnapshotDate            time.Time
	FundPerformanceIndex    *decimal.Decimal
	AveragePerformanceIndex *decimal.Decimal
}

// pool is a small helper kept here so files that need a pool reference (for
// RunInTx) can grab it from the service without exporting Repository.Pool().
// Currently unused, but keeps the package surface symmetric across files.
func (s *SnapshotService) pool() *pgxpool.Pool { return s.repo.Pool() }

// errSnapshotNoHistory is returned by callers that depend on history. Not
// surfaced over HTTP — handlers degrade gracefully (return empty array) when
// a fund has no snapshots yet (e.g. just-created).
var errSnapshotNoHistory = errors.New("funds: snapshot history empty")
