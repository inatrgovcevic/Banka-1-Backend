package dividend

import (
	"context"
	"time"
)

// Quarter-end months — dividends pay on the LAST BUSINESS DAY of March, June,
// September, December (mirrors DividendScheduler.QUARTER_END_MONTHS).
var quarterEndMonths = map[time.Month]bool{
	time.March: true, time.June: true, time.September: true, time.December: true,
}

// RunQuarterlyPayout mirrors DividendScheduler.runQuarterlyDividendPayout: the
// cron fires DAILY (no cron expression can say "last business day of these
// months") and the method self-gates — when today is not the last business day
// of a quarter-end month it returns immediately.
func (s *Service) RunQuarterlyPayout(ctx context.Context) {
	today := time.Now()
	if !IsLastBusinessDayOfQuarterMonth(today) {
		s.logger.Debug("dividend scheduler: not the last business day of a quarter month — skipping",
			"today", today.Format("2006-01-02"))
		return
	}
	s.logger.Info("dividend scheduler: last business day of the quarter — running payout",
		"today", today.Format("2006-01-02"))
	paid := s.Distribute(ctx, truncateToDate(today))
	s.logger.Info("dividend scheduler: payout finished", "today", today.Format("2006-01-02"), "paid", paid)
}

// IsLastBusinessDayOfQuarterMonth mirrors isLastBusinessDayOfQuarterMonth:
// the date is a weekday AND equals the last business day (Mon-Fri) of March /
// June / September / December (a month-end weekend shifts back to Friday).
func IsLastBusinessDayOfQuarterMonth(date time.Time) bool {
	if !quarterEndMonths[date.Month()] {
		return false
	}
	if isWeekend(date) {
		return false
	}
	last := lastBusinessDayOfMonth(date)
	return date.Year() == last.Year() && date.Month() == last.Month() && date.Day() == last.Day()
}

// lastBusinessDayOfMonth mirrors lastBusinessDayOfMonth: the month's last
// calendar day, walked back over Saturday/Sunday.
func lastBusinessDayOfMonth(date time.Time) time.Time {
	candidate := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location()).AddDate(0, 1, -1)
	for isWeekend(candidate) {
		candidate = candidate.AddDate(0, 0, -1)
	}
	return candidate
}

func isWeekend(date time.Time) bool {
	day := date.Weekday()
	return day == time.Saturday || day == time.Sunday
}

func truncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
