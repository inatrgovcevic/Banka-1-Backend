package actuary

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
)

// ---- fake pgx plumbing ----

type fakeRow struct {
	vals    []any
	scanErr error
}

func (r *fakeRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	for i, d := range dest {
		if i >= len(r.vals) {
			break
		}
		if err := assignFake(d, r.vals[i]); err != nil {
			return err
		}
	}
	return nil
}

func assignFake(dst, src any) error {
	switch d := dst.(type) {
	case *int64:
		if src == nil {
			*d = 0
		} else {
			*d = src.(int64)
		}
	case *bool:
		if src == nil {
			*d = false
		} else {
			*d = src.(bool)
		}
	case **string:
		if src == nil {
			*d = nil
		} else {
			switch v := src.(type) {
			case string:
				*d = &v
			case *string:
				*d = v
			}
		}
	case *string:
		if src == nil {
			*d = ""
		} else {
			switch v := src.(type) {
			case string:
				*d = v
			case *string:
				if v != nil {
					*d = *v
				}
			}
		}
	default:
		// ignore unknown types in tests
	}
	return nil
}

type fakeRows struct {
	rows    [][]any
	idx     int
	scanErr error
}

func (r *fakeRows) Next() bool     { return r.idx < len(r.rows) }
func (r *fakeRows) Close()         {}
func (r *fakeRows) Err() error     { return nil }
func (r *fakeRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error) { return nil, nil }
func (r *fakeRows) RawValues() [][]byte    { return nil }
func (r *fakeRows) Conn() *pgx.Conn       { return nil }
func (r *fakeRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.rows[r.idx]
	r.idx++
	for i, d := range dest {
		if i >= len(row) {
			break
		}
		if err := assignFake(d, row[i]); err != nil {
			return err
		}
	}
	return nil
}

type fakeDB struct {
	row     *fakeRow
	rows    *fakeRows
	execErr error
	execTag pgconn.CommandTag
}

func (f *fakeDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if f.row != nil {
		return f.row
	}
	return &fakeRow{scanErr: pgx.ErrNoRows}
}
func (f *fakeDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if f.rows != nil {
		return f.rows, nil
	}
	return &fakeRows{}, nil
}
func (f *fakeDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return f.execTag, f.execErr
}

// ---- repository tests ----

func TestFindByEmployeeID_NoRows(t *testing.T) {
	r := &Repository{db: &fakeDB{row: &fakeRow{scanErr: pgx.ErrNoRows}}}
	info, err := r.FindByEmployeeID(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Error("expected nil info for no rows")
	}
}

func TestFindByEmployeeID_ScanError(t *testing.T) {
	boom := errors.New("scan boom")
	r := &Repository{db: &fakeDB{row: &fakeRow{scanErr: boom}}}
	_, err := r.FindByEmployeeID(context.Background(), 1)
	if err == nil {
		t.Error("expected scan error")
	}
}

func TestFindByEmployeeID_SuccessNoLimit(t *testing.T) {
	r := &Repository{db: &fakeDB{row: &fakeRow{vals: []any{
		int64(42), nil, "0", "0", false,
	}}}}
	info, err := r.FindByEmployeeID(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil info")
	}
	if info.EmployeeID != 42 {
		t.Errorf("EmployeeID = %d, want 42", info.EmployeeID)
	}
	if info.Limit != nil {
		t.Error("Limit should be nil")
	}
}

func TestFindByEmployeeID_SuccessWithLimit(t *testing.T) {
	lim := "500.00"
	r := &Repository{db: &fakeDB{row: &fakeRow{vals: []any{
		int64(7), &lim, "100.00", "50.00", true,
	}}}}
	info, err := r.FindByEmployeeID(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if info.Limit == nil {
		t.Fatal("expected non-nil Limit")
	}
	if !info.Limit.Equal(decimal.RequireFromString("500.00")) {
		t.Errorf("Limit = %v, want 500.00", info.Limit)
	}
	if !info.NeedApproval {
		t.Error("expected NeedApproval=true")
	}
}

func TestUpdateLimit_Success(t *testing.T) {
	r := &Repository{db: &fakeDB{}}
	if err := r.UpdateLimit(context.Background(), 1, decimal.NewFromFloat(1000)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateLimit_ExecError(t *testing.T) {
	boom := errors.New("exec boom")
	r := &Repository{db: &fakeDB{execErr: boom}}
	if err := r.UpdateLimit(context.Background(), 1, decimal.NewFromFloat(1000)); !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestRepo_ResetLimit_Success(t *testing.T) {
	r := &Repository{db: &fakeDB{}}
	if err := r.ResetLimit(context.Background(), 1); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRepo_SetNeedApproval_Success(t *testing.T) {
	r := &Repository{db: &fakeDB{}}
	if err := r.SetNeedApproval(context.Background(), 1, true); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResetAllLimits_Success(t *testing.T) {
	r := &Repository{db: &fakeDB{}}
	if err := r.ResetAllLimits(context.Background()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSumCommissionByActuary_QueryError(t *testing.T) {
	// Empty rows → no results.
	r := &Repository{db: &fakeDB{rows: &fakeRows{}}}
	rows, err := r.SumCommissionByActuary(context.Background(), time.Now(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected empty, got %d rows", len(rows))
	}
}

func TestFindAllEmployeeIDs_Empty(t *testing.T) {
	r := &Repository{db: &fakeDB{rows: &fakeRows{}}}
	ids, err := r.FindAllEmployeeIDs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty, got %d ids", len(ids))
	}
}

func TestUpdateReservedLimit_Success(t *testing.T) {
	r := &Repository{db: &fakeDB{}}
	if err := r.UpdateReservedLimit(context.Background(), r.db, 1, decimal.NewFromFloat(100)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateReservedAndUsedLimit_Success(t *testing.T) {
	r := &Repository{db: &fakeDB{}}
	if err := r.UpdateReservedAndUsedLimit(context.Background(), r.db, 1, decimal.NewFromFloat(50), decimal.NewFromFloat(200)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFindByEmployeeIDForUpdate_NoRows(t *testing.T) {
	r := &Repository{db: &fakeDB{row: &fakeRow{scanErr: pgx.ErrNoRows}}}
	info, err := r.FindByEmployeeIDForUpdate(context.Background(), r.db, 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Error("expected nil for no rows")
	}
}

func TestFindOrCreate_ExistingRow(t *testing.T) {
	lim := "200.00"
	r := &Repository{db: &fakeDB{row: &fakeRow{vals: []any{
		int64(5), &lim, "0", "0", false,
	}}}}
	info, err := r.FindOrCreate(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if info == nil || info.EmployeeID != 5 {
		t.Errorf("expected info for employee 5, got %v", info)
	}
}

func TestSumCommissionByActuary_WithRows(t *testing.T) {
	rows := &fakeRows{rows: [][]any{
		{int64(1), "500.00", int64(10)},
		{int64(2), "200.00", int64(5)},
	}}
	r := &Repository{db: &fakeDB{rows: rows}}
	out, err := r.SumCommissionByActuary(context.Background(), time.Now(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Errorf("got %d rows, want 2", len(out))
	}
	if out[0].UserID != 1 {
		t.Errorf("UserID = %d, want 1", out[0].UserID)
	}
	if out[1].TransactionCount != 5 {
		t.Errorf("TransactionCount = %d, want 5", out[1].TransactionCount)
	}
}

func TestFindAllEmployeeIDs_WithRows(t *testing.T) {
	rows := &fakeRows{rows: [][]any{
		{int64(10)},
		{int64(20)},
		{int64(30)},
	}}
	r := &Repository{db: &fakeDB{rows: rows}}
	ids, err := r.FindAllEmployeeIDs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("got %d ids, want 3", len(ids))
	}
}

func TestFindEmployeeIDsIn_WithRows(t *testing.T) {
	rows := &fakeRows{rows: [][]any{
		{int64(5)},
		{int64(10)},
	}}
	r := &Repository{db: &fakeDB{rows: rows}}
	out, err := r.FindEmployeeIDsIn(context.Background(), []int64{5, 10, 99})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out[5] || !out[10] {
		t.Error("expected 5 and 10 in result map")
	}
}

func TestFindByEmployeeIDForUpdate_WithData(t *testing.T) {
	r := &Repository{db: &fakeDB{row: &fakeRow{vals: []any{
		int64(3), nil, "0", "0", false,
	}}}}
	info, err := r.FindByEmployeeIDForUpdate(context.Background(), r.db, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil || info.EmployeeID != 3 {
		t.Errorf("got %v", info)
	}
}

func TestFindByEmployeeID_InvalidDecimal(t *testing.T) {
	// used_limit contains invalid decimal → error returned
	r := &Repository{db: &fakeDB{row: &fakeRow{vals: []any{
		int64(1), nil, "not-a-decimal", "0", false,
	}}}}
	_, err := r.FindByEmployeeID(context.Background(), 1)
	if err == nil {
		t.Error("expected decimal parse error")
	}
}
