package dividend

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

type dvFakeRow struct {
	vals    []any
	scanErr error
}

func (r *dvFakeRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	for i, d := range dest {
		if i >= len(r.vals) {
			break
		}
		if err := dvAssign(d, r.vals[i]); err != nil {
			return err
		}
	}
	return nil
}

func dvAssign(dst, src any) error {
	switch d := dst.(type) {
	case *int64:
		if src != nil {
			*d = src.(int64)
		}
	case **int64:
		if src == nil {
			*d = nil
		} else {
			v := src.(int64)
			*d = &v
		}
	case *int:
		if src != nil {
			*d = src.(int)
		}
	case *bool:
		if src != nil {
			*d = src.(bool)
		}
	case *string:
		if src != nil {
			*d = src.(string)
		}
	case **string:
		if src == nil {
			*d = nil
		} else {
			v := src.(string)
			*d = &v
		}
	case *time.Time:
		if src != nil {
			*d = src.(time.Time)
		}
	}
	return nil
}

type dvFakeRows struct {
	rows [][]any
	idx  int
}

func (r *dvFakeRows) Next() bool { return r.idx < len(r.rows) }
func (r *dvFakeRows) Close()     {}
func (r *dvFakeRows) Err() error { return nil }
func (r *dvFakeRows) CommandTag() pgconn.CommandTag               { return pgconn.CommandTag{} }
func (r *dvFakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *dvFakeRows) Values() ([]any, error)                        { return nil, nil }
func (r *dvFakeRows) RawValues() [][]byte                           { return nil }
func (r *dvFakeRows) Conn() *pgx.Conn                              { return nil }
func (r *dvFakeRows) Scan(dest ...any) error {
	row := r.rows[r.idx]
	r.idx++
	for i, d := range dest {
		if i >= len(row) {
			break
		}
		dvAssign(d, row[i])
	}
	return nil
}

type dvFakeQuerier struct {
	row    *dvFakeRow
	rows   *dvFakeRows
	rowErr error
}

func (q *dvFakeQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (q *dvFakeQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if q.rowErr != nil {
		return nil, q.rowErr
	}
	if q.rows != nil {
		return q.rows, nil
	}
	return &dvFakeRows{}, nil
}
func (q *dvFakeQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if q.row != nil {
		return q.row
	}
	return &dvFakeRow{scanErr: pgx.ErrNoRows}
}

// ---- NewRepository / Pool ----

func TestNewRepository_NilPool(t *testing.T) {
	r := NewRepository(nil)
	if r == nil {
		t.Error("NewRepository returned nil")
	}
	if r.Pool() != nil {
		t.Error("Pool should be nil")
	}
}

// ---- Insert ----

func TestInsert_Success(t *testing.T) {
	q := &dvFakeQuerier{row: &dvFakeRow{vals: []any{int64(1)}}}
	r := NewRepository(nil)
	p := &Payout{
		UserID:       1,
		ListingID:    5,
		Quantity:     10,
		GrossAmount:  decimal.NewFromFloat(100),
		TaxAmountRsd: decimal.Zero,
		NetAmount:    decimal.NewFromFloat(100),
		PaymentDate:  time.Now(),
	}
	if err := r.Insert(context.Background(), q, p); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if p.ID != 1 {
		t.Errorf("ID = %d, want 1", p.ID)
	}
}

func TestInsert_ScanError(t *testing.T) {
	boom := errors.New("insert failed")
	q := &dvFakeQuerier{row: &dvFakeRow{scanErr: boom}}
	r := NewRepository(nil)
	p := &Payout{GrossAmount: decimal.Zero, TaxAmountRsd: decimal.Zero, NetAmount: decimal.Zero}
	if err := r.Insert(context.Background(), q, p); !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

// ---- ExistsForDate ----

func TestExistsForDate_True(t *testing.T) {
	q := &dvFakeQuerier{row: &dvFakeRow{vals: []any{true}}}
	r := NewRepository(nil)
	exists, err := r.ExistsForDate(context.Background(), q, 1, 5, time.Now(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected exists=true")
	}
}

func TestExistsForDate_False(t *testing.T) {
	q := &dvFakeQuerier{row: &dvFakeRow{vals: []any{false}}}
	r := NewRepository(nil)
	exists, err := r.ExistsForDate(context.Background(), q, 1, 5, time.Now(), false)
	if err != nil || exists {
		t.Errorf("expected false,nil; got %v,%v", exists, err)
	}
}

func TestExistsForDate_Error(t *testing.T) {
	boom := errors.New("boom")
	q := &dvFakeQuerier{row: &dvFakeRow{scanErr: boom}}
	r := NewRepository(nil)
	_, err := r.ExistsForDate(context.Background(), q, 1, 5, time.Now(), false)
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

// ---- FindByUserID ----

func TestFindByUserID_Empty(t *testing.T) {
	q := &dvFakeQuerier{rows: &dvFakeRows{}}
	r := NewRepository(nil)
	out, err := r.FindByUserID(context.Background(), q, 1)
	if err != nil || len(out) != 0 {
		t.Errorf("expected empty, got %v, %v", out, err)
	}
}

func TestFindByUserID_QueryError(t *testing.T) {
	boom := errors.New("query boom")
	q := &dvFakeQuerier{rowErr: boom}
	r := NewRepository(nil)
	_, err := r.FindByUserID(context.Background(), q, 1)
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestFindByUserID_WithRows(t *testing.T) {
	now := time.Now()
	// scanPayout scans: id, user_id, stock_ticker, listing_id, quantity, gross_amount::text,
	//   currency, tax_amount_rsd::text, net_amount::text, account_id, payment_date, for_bank
	rows := &dvFakeRows{rows: [][]any{
		{int64(1), int64(42), "AAPL", int64(5), int(10), "100.00", "USD", "15.00", "85.00", nil, now, false},
	}}
	q := &dvFakeQuerier{rows: rows}
	r := NewRepository(nil)
	out, err := r.FindByUserID(context.Background(), q, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("got %d rows, want 1", len(out))
	}
	if out[0].UserID != 42 {
		t.Errorf("UserID = %d, want 42", out[0].UserID)
	}
	if !out[0].GrossAmount.Equal(decimal.RequireFromString("100.00")) {
		t.Errorf("GrossAmount = %v, want 100.00", out[0].GrossAmount)
	}
}

// ---- FindByUserIDAndListingID ----

func TestFindByUserIDAndListingID_Empty(t *testing.T) {
	q := &dvFakeQuerier{rows: &dvFakeRows{}}
	r := NewRepository(nil)
	out, err := r.FindByUserIDAndListingID(context.Background(), q, 1, 5)
	if err != nil || len(out) != 0 {
		t.Errorf("expected empty: got %v, %v", out, err)
	}
}

// ---- scanPayouts composite ----

func TestScanPayouts_InvalidDecimal(t *testing.T) {
	rows := &dvFakeRows{rows: [][]any{
		{int64(1), int64(1), "AAPL", int64(5), int(10), "not-a-decimal", "USD", "0", "0", nil, time.Now(), false},
	}}
	q := &dvFakeQuerier{rows: rows}
	r := NewRepository(nil)
	_, err := r.FindByUserID(context.Background(), q, 1)
	if err == nil {
		t.Error("expected decimal parse error")
	}
}
