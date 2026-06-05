package analytics

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func TestAssignDecimal_Valid(t *testing.T) {
	var d decimal.Decimal
	var err error
	assignDecimal("123.45", &d, &err)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Equal(decimal.RequireFromString("123.45")) {
		t.Errorf("got %v, want 123.45", d)
	}
}

func TestAssignDecimal_Zero(t *testing.T) {
	var d decimal.Decimal
	var err error
	assignDecimal("0", &d, &err)
	if err != nil {
		t.Fatal(err)
	}
	if !d.IsZero() {
		t.Errorf("expected zero, got %v", d)
	}
}

func TestAssignDecimal_InvalidString(t *testing.T) {
	var d decimal.Decimal
	var err error
	assignDecimal("not-a-number", &d, &err)
	if err == nil {
		t.Error("expected error for invalid decimal string")
	}
}

func TestAssignDecimal_SkipsWhenAlreadyError(t *testing.T) {
	var d decimal.Decimal
	existing := errors.New("prior error")
	err := existing
	assignDecimal("not-a-number", &d, &err)
	// Should keep the original error, not overwrite.
	if err != existing {
		t.Errorf("err changed from %v to %v", existing, err)
	}
}

func TestAssignDecimal_Negative(t *testing.T) {
	var d decimal.Decimal
	var err error
	assignDecimal("-99.99", &d, &err)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Equal(decimal.RequireFromString("-99.99")) {
		t.Errorf("got %v, want -99.99", d)
	}
}

func TestNewRepository_NilPool(t *testing.T) {
	r := NewRepository(nil)
	if r == nil {
		t.Error("NewRepository(nil) returned nil")
	}
}

func TestNewService_NilRepo(t *testing.T) {
	svc := NewService(nil)
	if svc == nil {
		t.Error("NewService(nil) returned nil")
	}
}
