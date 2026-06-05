package order

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func dt(year int, month time.Month, day, hour, min, sec int) time.Time {
	return time.Date(year, month, day, hour, min, sec, 0, time.UTC)
}

// ---- advanceCadence ----

func TestAdvanceCadence_Daily(t *testing.T) {
	from := dt(2024, 3, 15, 9, 0, 0)
	got := advanceCadence(CadenceDaily, from)
	want := dt(2024, 3, 16, 9, 0, 0)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAdvanceCadence_Weekly(t *testing.T) {
	from := dt(2024, 3, 15, 9, 0, 0)
	got := advanceCadence(CadenceWeekly, from)
	want := dt(2024, 3, 22, 9, 0, 0)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAdvanceCadence_Monthly_NormalDay(t *testing.T) {
	from := dt(2024, 1, 15, 10, 0, 0)
	got := advanceCadence(CadenceMonthly, from)
	want := dt(2024, 2, 15, 10, 0, 0)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAdvanceCadence_Monthly_Jan31_ClampsFeb(t *testing.T) {
	// Jan 31 + 1 month → Feb 29 (2024 is leap year) or Feb 28 (non-leap)
	from := dt(2024, 1, 31, 0, 0, 0)
	got := advanceCadence(CadenceMonthly, from)
	// 2024 is a leap year, Feb has 29 days
	want := dt(2024, 2, 29, 0, 0, 0)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAdvanceCadence_Monthly_Jan31_NonLeap(t *testing.T) {
	from := dt(2023, 1, 31, 0, 0, 0)
	got := advanceCadence(CadenceMonthly, from)
	want := dt(2023, 2, 28, 0, 0, 0)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAdvanceCadence_Unknown_FallsToMonthly(t *testing.T) {
	from := dt(2024, 3, 15, 0, 0, 0)
	got := advanceCadence("UNKNOWN", from)
	want := dt(2024, 4, 15, 0, 0, 0)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// ---- plusMonthsClamped ----

func TestPlusMonthsClamped_NormalDay(t *testing.T) {
	from := dt(2024, 5, 10, 8, 30, 0)
	got := plusMonthsClamped(from, 3)
	want := dt(2024, 8, 10, 8, 30, 0)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPlusMonthsClamped_ClampsToLastDay(t *testing.T) {
	// March 31 + 1 month → April 30 (April has only 30 days)
	from := dt(2024, 3, 31, 12, 0, 0)
	got := plusMonthsClamped(from, 1)
	want := dt(2024, 4, 30, 12, 0, 0)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPlusMonthsClamped_PreservesTime(t *testing.T) {
	from := dt(2024, 1, 15, 14, 30, 59)
	got := plusMonthsClamped(from, 2)
	if got.Hour() != 14 || got.Minute() != 30 || got.Second() != 59 {
		t.Errorf("time not preserved: %v", got)
	}
}

// ---- recurringDto ----

func TestRecurringDto_MapsFields(t *testing.T) {
	now := time.Now()
	r := &RecurringOrder{
		ID:        42,
		UserID:    1,
		ListingID: 5,
		Direction: DirectionBuy,
		Mode:      RecurringModeByQuantity,
		Value:     decimal.NewFromFloat(100),
		AccountID: 3,
		Cadence:   CadenceWeekly,
		NextRun:   now,
		Active:    true,
		CreatedAt: now,
	}
	dto := recurringDto(r)
	if dto.ID != 42 {
		t.Errorf("ID = %d, want 42", dto.ID)
	}
	if dto.Direction != DirectionBuy {
		t.Errorf("Direction = %q, want %q", dto.Direction, DirectionBuy)
	}
	if dto.Cadence != CadenceWeekly {
		t.Errorf("Cadence = %q, want %q", dto.Cadence, CadenceWeekly)
	}
	if !dto.Active {
		t.Error("Active should be true")
	}
}

// ---- constants ----

func TestCadenceConstants_NonEmpty(t *testing.T) {
	for _, c := range []string{CadenceDaily, CadenceWeekly, CadenceMonthly} {
		if c == "" {
			t.Error("cadence constant should not be empty")
		}
	}
}

func TestRecurringModeConstants(t *testing.T) {
	if RecurringModeByQuantity == "" || RecurringModeByAmount == "" {
		t.Error("mode constants should not be empty")
	}
	if RecurringModeByQuantity == RecurringModeByAmount {
		t.Error("mode constants should be distinct")
	}
}
