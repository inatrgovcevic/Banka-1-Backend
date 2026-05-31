package service

import (
	"testing"
	"time"
)

func TestNextDailyMidnightReturnsTomorrow(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 5, 29, 14, 30, 0, 0, loc)
	next := nextDailyMidnight(now)

	want := time.Date(2026, 5, 30, 0, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("nextDailyMidnight(%v) = %v, want %v", now, next, want)
	}
}

func TestNextDailyMidnightAtExactMidnightReturnsTomorrow(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 5, 29, 0, 0, 0, 0, loc)
	next := nextDailyMidnight(now)

	want := time.Date(2026, 5, 30, 0, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("nextDailyMidnight at midnight = %v, want %v", next, want)
	}
}

func TestNextDailyMidnightMonthBoundary(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 5, 31, 23, 59, 59, 0, loc)
	next := nextDailyMidnight(now)

	want := time.Date(2026, 6, 1, 0, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("nextDailyMidnight at month end = %v, want %v", next, want)
	}
}

func TestNextDailyMidnightYearBoundary(t *testing.T) {
	loc := time.UTC
	now := time.Date(2025, 12, 31, 12, 0, 0, 0, loc)
	next := nextDailyMidnight(now)

	want := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("nextDailyMidnight at year end = %v, want %v", next, want)
	}
}

func TestNextMonthFirstMidnightReturnNextMonth(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, loc)
	next := nextMonthFirstMidnight(now)

	want := time.Date(2026, 6, 1, 0, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("nextMonthFirstMidnight(%v) = %v, want %v", now, next, want)
	}
}

func TestNextMonthFirstMidnightOnFirstButNotYetMidnight(t *testing.T) {
	loc := time.UTC
	// It's the 1st but past midnight — next run is 1st of next month.
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, loc)
	next := nextMonthFirstMidnight(now)

	want := time.Date(2026, 6, 1, 0, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("nextMonthFirstMidnight on the 1st at midnight = %v, want %v", next, want)
	}
}

func TestNextMonthFirstMidnightDecemberToJanuary(t *testing.T) {
	loc := time.UTC
	now := time.Date(2025, 12, 10, 8, 0, 0, 0, loc)
	next := nextMonthFirstMidnight(now)

	want := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("nextMonthFirstMidnight December = %v, want %v", next, want)
	}
}

func TestNextFnsReturnFutureTime(t *testing.T) {
	now := time.Now()
	if d := nextDailyMidnight(now); !d.After(now) {
		t.Fatalf("nextDailyMidnight should be in the future, got %v", d)
	}
	if m := nextMonthFirstMidnight(now); !m.After(now) {
		t.Fatalf("nextMonthFirstMidnight should be in the future, got %v", m)
	}
}
