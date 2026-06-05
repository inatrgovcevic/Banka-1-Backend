package actuary

import (
	"context"
	"testing"
)

func TestNewRepository_NilPool(t *testing.T) {
	r := NewRepository(nil)
	if r == nil {
		t.Error("NewRepository(nil) returned nil")
	}
}

func TestFindEmployeeIDsIn_EmptySlice(t *testing.T) {
	r := &Repository{db: nil}
	// Empty input returns empty map without touching DB.
	out, err := r.FindEmployeeIDsIn(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty map, got %v", out)
	}
}

func TestFindEmployeeIDsIn_EmptySliceExplicit(t *testing.T) {
	r := &Repository{db: nil}
	out, err := r.FindEmployeeIDsIn(context.Background(), []int64{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty map, got %v", out)
	}
}

func TestActuaryInfo_Fields(t *testing.T) {
	info := &ActuaryInfo{EmployeeID: 42, NeedApproval: true}
	if info.EmployeeID != 42 {
		t.Errorf("EmployeeID = %d, want 42", info.EmployeeID)
	}
	if !info.NeedApproval {
		t.Error("NeedApproval should be true")
	}
	if info.Limit != nil {
		t.Error("Limit should be nil by default")
	}
}

func TestProfitRow_Fields(t *testing.T) {
	row := ProfitRow{UserID: 7, TransactionCount: 100}
	if row.UserID != 7 {
		t.Errorf("UserID = %d, want 7", row.UserID)
	}
}
