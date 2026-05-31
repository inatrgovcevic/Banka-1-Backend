package funds

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
)

// MinMonthlySnapshots mirrors FundStatisticsService.MIN_MONTHLY_SNAPSHOTS.
// Below this threshold all four metrics return nil — the spec & Java code
// require ≥12 monthly samples (one year of history) before any annualized
// figure makes sense.
const MinMonthlySnapshots = 12

// StatisticsService mirrors FundStatisticsService. The four metrics are
// computed in float64 (matching Java's Math.pow/Math.sqrt + double averaging)
// and only the final values are coerced back to decimal.Decimal at scale 4 for
// the response DTO — parity to the cent isn't possible with these formulas, so
// the Java code commits to float64.
type StatisticsService struct {
	snapshots *SnapshotService
}

func NewStatisticsService(snapshots *SnapshotService) *StatisticsService {
	return &StatisticsService{snapshots: snapshots}
}

// Metrics is the value type returned by StatisticsService.MetricsFor. All four
// numeric fields are nil when the fund has fewer than 12 monthly snapshots
// (mirrors metricsFromSnapshots' "return nulls" branch).
type Metrics struct {
	MonthlySnapshotsUsed     int              `json:"monthlySnapshotsUsed"`
	AnnualizedReturn         *decimal.Decimal `json:"annualizedReturn"`
	RewardToVariabilityRatio *decimal.Decimal `json:"rewardToVariabilityRatio"`
	MaxDrawdown              *decimal.Decimal `json:"maxDrawdown"`
	Volatility               *decimal.Decimal `json:"volatility"`
}

// MetricsFor mirrors metricsFor(fundId): load the per-month tail, compute
// metrics. Returns zero-valued metrics (used=0, all nil) on lookup errors so
// the discovery page renders rather than 500s.
func (s *StatisticsService) MetricsFor(ctx context.Context, fundID int64) Metrics {
	snaps, err := s.snapshots.MonthlySnapshots(ctx, fundID)
	if err != nil {
		return Metrics{}
	}
	return s.fromSnapshots(snaps)
}

// fromSnapshots mirrors metricsFromSnapshots.
func (s *StatisticsService) fromSnapshots(snaps []FundValueSnapshot) Metrics {
	if len(snaps) < MinMonthlySnapshots {
		return Metrics{MonthlySnapshotsUsed: len(snaps)}
	}
	values := make([]decimal.Decimal, len(snaps))
	for i, snap := range snaps {
		values[i] = snap.TotalValue
	}
	annual := calculateAnnualizedReturn(values)
	vol := calculateVolatility(values)
	var rwd *decimal.Decimal
	if annual != nil && vol != nil && vol.Sign() != 0 {
		r := annual.Div(*vol).Round(6)
		rwd = &r
	}
	drawdown := calculateMaxDrawdown(values)
	return Metrics{
		MonthlySnapshotsUsed:     len(snaps),
		AnnualizedReturn:         scaleOrNil(annual),
		RewardToVariabilityRatio: scaleOrNil(rwd),
		MaxDrawdown:              scaleOrNil(drawdown),
		Volatility:               scaleOrNil(vol),
	}
}

// Sort mirrors FundStatisticsService.sort: composite ORDER BY <field> + name
// ASC as the tiebreaker. nullsLast in either direction, then NAME sorting is
// case-insensitive (CASE_INSENSITIVE_ORDER).
func (s *StatisticsService) Sort(items []FundView, field, direction string) []FundView {
	if field == "" {
		field = SortByName
	}
	if direction == "" {
		direction = SortAsc
	}
	out := make([]FundView, len(items))
	copy(out, items)
	desc := strings.EqualFold(direction, SortDesc)

	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch field {
		case SortByTotalValue:
			return decimalCompareNullsLast(a.TotalValue, b.TotalValue, desc) < 0
		case SortByProfit:
			return decimalCompareNullsLast(a.Profit, b.Profit, desc) < 0
		case SortByAnnualizedReturn:
			return decimalCompareNullsLast(a.AnnualizedReturn, b.AnnualizedReturn, desc) < 0
		case SortByRewardToVariabilityRat:
			return decimalCompareNullsLast(a.RewardToVariabilityRatio, b.RewardToVariabilityRatio, desc) < 0
		case SortByMaxDrawdown:
			return decimalCompareNullsLast(a.MaxDrawdown, b.MaxDrawdown, desc) < 0
		case SortByVolatility:
			return decimalCompareNullsLast(a.Volatility, b.Volatility, desc) < 0
		case SortByName:
			c := caseInsensitiveCompare(a.Naziv, b.Naziv)
			if desc {
				c = -c
			}
			return c < 0
		default:
			return caseInsensitiveCompare(a.Naziv, b.Naziv) < 0
		}
	})
	// Tiebreaker: name ASC, case-insensitive — always asc per Java sort().thenComparing.
	sort.SliceStable(out, func(i, j int) bool {
		// stable second pass only orders equal primary keys
		if !primaryKeyEqual(field, out[i], out[j]) {
			return false
		}
		return caseInsensitiveCompare(out[i].Naziv, out[j].Naziv) < 0
	})
	return out
}

// FundView is a lightweight projection used by Sort. Carries every sortable
// field, populated by the InvestmentFundService when building the discovery DTO.
type FundView struct {
	Naziv                    string
	TotalValue               *decimal.Decimal
	Profit                   *decimal.Decimal
	AnnualizedReturn         *decimal.Decimal
	RewardToVariabilityRatio *decimal.Decimal
	MaxDrawdown              *decimal.Decimal
	Volatility               *decimal.Decimal
	// Source carries the underlying domain row so the caller can re-project to
	// its DTO after sorting.
	Source any
}

func primaryKeyEqual(field string, a, b FundView) bool {
	switch field {
	case SortByTotalValue:
		return decimalEq(a.TotalValue, b.TotalValue)
	case SortByProfit:
		return decimalEq(a.Profit, b.Profit)
	case SortByAnnualizedReturn:
		return decimalEq(a.AnnualizedReturn, b.AnnualizedReturn)
	case SortByRewardToVariabilityRat:
		return decimalEq(a.RewardToVariabilityRatio, b.RewardToVariabilityRatio)
	case SortByMaxDrawdown:
		return decimalEq(a.MaxDrawdown, b.MaxDrawdown)
	case SortByVolatility:
		return decimalEq(a.Volatility, b.Volatility)
	default:
		return false
	}
}

func decimalEq(a, b *decimal.Decimal) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Cmp(*b) == 0
}

// decimalCompareNullsLast: <0 means a should come first. nulls always go last
// regardless of direction (mirrors Comparator.nullsLast wrap).
func decimalCompareNullsLast(a, b *decimal.Decimal, desc bool) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return 1
	}
	if b == nil {
		return -1
	}
	c := a.Cmp(*b)
	if desc {
		c = -c
	}
	return c
}

func caseInsensitiveCompare(a, b string) int {
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// calculateAnnualizedReturn mirrors FundStatisticsService.calculateAnnualizedReturn:
// ratio = end/start, years = (n-1)/12, return = (ratio^(1/years) - 1) * 100.
// Multiplies by 100 like Java; ROunds to scale 4 in scaleOrNil.
func calculateAnnualizedReturn(values []decimal.Decimal) *decimal.Decimal {
	start := values[0]
	end := values[len(values)-1]
	if start.Sign() <= 0 {
		return nil
	}
	ratio, _ := end.Div(start).Float64()
	years := float64(len(values)-1) / 12.0
	if years <= 0 {
		return nil
	}
	val := math.Pow(ratio, 1.0/years) - 1.0
	out := decimal.NewFromFloat(val).Mul(decimal.NewFromInt(100))
	return &out
}

// calculateVolatility mirrors calculateVolatility: stddev of monthly returns,
// in double, then * 100. Returns nil only when there are no monthly returns
// (i.e. fewer than 2 valid values).
func calculateVolatility(values []decimal.Decimal) *decimal.Decimal {
	returns := monthlyReturns(values)
	if len(returns) == 0 {
		return nil
	}
	sum := 0.0
	for _, r := range returns {
		f, _ := r.Float64()
		sum += f
	}
	mean := sum / float64(len(returns))
	varSum := 0.0
	for _, r := range returns {
		f, _ := r.Float64()
		varSum += (f - mean) * (f - mean)
	}
	variance := varSum / float64(len(returns))
	std := math.Sqrt(variance)
	out := decimal.NewFromFloat(std).Mul(decimal.NewFromInt(100))
	return &out
}

// calculateMaxDrawdown mirrors calculateMaxDrawdown: running peak; drawdown at
// each step is (peak - value) / peak * 100; track the max. Returns 0 (not nil)
// when no drawdown is observed — matches Java initializer BigDecimal.ZERO.
func calculateMaxDrawdown(values []decimal.Decimal) *decimal.Decimal {
	peak := values[0]
	maxDrawdown := decimal.Zero
	for _, v := range values {
		if v.Cmp(peak) > 0 {
			peak = v
		}
		if peak.Sign() > 0 {
			drawdown := peak.Sub(v).Div(peak).Round(8).Mul(decimal.NewFromInt(100))
			if drawdown.Cmp(maxDrawdown) > 0 {
				maxDrawdown = drawdown
			}
		}
	}
	return &maxDrawdown
}

// monthlyReturns mirrors FundStatisticsService.monthlyReturns: (current -
// previous) / previous. Skips entries when previous <= 0.
func monthlyReturns(values []decimal.Decimal) []decimal.Decimal {
	out := make([]decimal.Decimal, 0, len(values)-1)
	for i := 1; i < len(values); i++ {
		prev := values[i-1]
		cur := values[i]
		if prev.Sign() <= 0 {
			continue
		}
		out = append(out, cur.Sub(prev).Div(prev).Round(8))
	}
	return out
}

// scaleOrNil rounds the value to scale 4 (matches FundStatisticsService.scaleOrNull),
// or returns nil unchanged.
func scaleOrNil(v *decimal.Decimal) *decimal.Decimal {
	if v == nil {
		return nil
	}
	r := v.Round(4)
	return &r
}
