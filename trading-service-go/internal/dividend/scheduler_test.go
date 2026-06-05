package dividend

import (
	"testing"
	"time"
)

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// ---- isWeekend ----

func TestIsWeekend_Saturday(t *testing.T) {
	if !isWeekend(date(2024, 3, 2)) { // March 2, 2024 = Saturday
		t.Error("expected Saturday to be weekend")
	}
}

func TestIsWeekend_Sunday(t *testing.T) {
	if !isWeekend(date(2024, 3, 3)) { // March 3, 2024 = Sunday
		t.Error("expected Sunday to be weekend")
	}
}

func TestIsWeekend_Monday(t *testing.T) {
	if isWeekend(date(2024, 3, 4)) { // March 4, 2024 = Monday
		t.Error("expected Monday to be weekday")
	}
}

func TestIsWeekend_Friday(t *testing.T) {
	if isWeekend(date(2024, 3, 1)) { // March 1, 2024 = Friday
		t.Error("expected Friday to be weekday")
	}
}

// ---- truncateToDate ----

func TestTruncateToDate_RemovesTime(t *testing.T) {
	in := time.Date(2024, 5, 15, 14, 30, 59, 999, time.UTC)
	got := truncateToDate(in)
	want := time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// ---- lastBusinessDayOfMonth ----

func TestLastBusinessDayOfMonth_March2024(t *testing.T) {
	// March 31, 2024 = Sunday → last business day = March 29 (Friday)
	got := lastBusinessDayOfMonth(date(2024, 3, 15))
	if got.Day() != 29 || got.Month() != time.March {
		t.Errorf("got %v, want March 29", got)
	}
}

func TestLastBusinessDayOfMonth_June2024(t *testing.T) {
	// June 30, 2024 = Sunday → last business day = June 28 (Friday)
	got := lastBusinessDayOfMonth(date(2024, 6, 1))
	if got.Day() != 28 || got.Month() != time.June {
		t.Errorf("got %v, want June 28", got)
	}
}

func TestLastBusinessDayOfMonth_December2024(t *testing.T) {
	// December 31, 2024 = Tuesday → last business day = December 31
	got := lastBusinessDayOfMonth(date(2024, 12, 1))
	if got.Day() != 31 || got.Month() != time.December {
		t.Errorf("got %v, want December 31", got)
	}
}

// ---- IsLastBusinessDayOfQuarterMonth ----

func TestIsLastBusinessDayOfQuarterMonth_NonQuarterMonth(t *testing.T) {
	// January is not a quarter month
	if IsLastBusinessDayOfQuarterMonth(date(2024, 1, 31)) {
		t.Error("January should not be a quarter month")
	}
}

func TestIsLastBusinessDayOfQuarterMonth_Weekend(t *testing.T) {
	// March 30, 2024 = Saturday → not the last business day
	if IsLastBusinessDayOfQuarterMonth(date(2024, 3, 30)) {
		t.Error("Saturday should not be last business day")
	}
}

func TestIsLastBusinessDayOfQuarterMonth_NotLastDay(t *testing.T) {
	// March 28, 2024 = Thursday, but not the LAST business day
	if IsLastBusinessDayOfQuarterMonth(date(2024, 3, 28)) {
		t.Error("March 28 is not the last business day")
	}
}

func TestIsLastBusinessDayOfQuarterMonth_LastDay(t *testing.T) {
	// March 29, 2024 = Friday = last business day of March 2024
	if !IsLastBusinessDayOfQuarterMonth(date(2024, 3, 29)) {
		t.Error("March 29, 2024 should be last business day of quarter month")
	}
}

func TestIsLastBusinessDayOfQuarterMonth_June_LastDay(t *testing.T) {
	// June 28, 2024 = Friday = last business day of June 2024
	if !IsLastBusinessDayOfQuarterMonth(date(2024, 6, 28)) {
		t.Error("June 28, 2024 should be last business day of quarter month")
	}
}

func TestIsLastBusinessDayOfQuarterMonth_September_LastDay(t *testing.T) {
	// September 30, 2024 = Monday = last business day (September 30 is Monday)
	if !IsLastBusinessDayOfQuarterMonth(date(2024, 9, 30)) {
		t.Error("September 30, 2024 should be last business day of quarter month")
	}
}

func TestIsLastBusinessDayOfQuarterMonth_AllMonths(t *testing.T) {
	nonQuarter := []time.Month{
		time.January, time.February, time.April, time.May,
		time.July, time.August, time.October, time.November,
	}
	for _, m := range nonQuarter {
		d := date(2024, m, 28) // always a weekday for any reasonable month
		if IsLastBusinessDayOfQuarterMonth(d) {
			t.Errorf("month %v should not be a quarter month", m)
		}
	}
}
