package interbank

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---- fake pgx plumbing ----

type ibFakeRow struct {
	vals    []any
	scanErr error
}

func (r *ibFakeRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	for i, d := range dest {
		if i >= len(r.vals) {
			break
		}
		switch v := d.(type) {
		case *string:
			if s, ok := r.vals[i].(string); ok {
				*v = s
			}
		case *int64:
			if n, ok := r.vals[i].(int64); ok {
				*v = n
			}
		case *int:
			if n, ok := r.vals[i].(int); ok {
				*v = n
			}
		}
	}
	return nil
}

type ibFakeQuerier struct {
	row     *ibFakeRow
	execErr error
}

func (q *ibFakeQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, q.execErr
}
func (q *ibFakeQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (q *ibFakeQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if q.row != nil {
		return q.row
	}
	return &ibFakeRow{scanErr: pgx.ErrNoRows}
}

// ---- NewRepository / Pool ----

func TestNewRepository_Nil(t *testing.T) {
	r := NewRepository(nil)
	if r == nil {
		t.Error("NewRepository returned nil")
	}
	if r.Pool() != nil {
		t.Error("Pool() should be nil")
	}
}

// ---- InsertStockReservation ----

func TestInsertStockReservation_Success(t *testing.T) {
	r := NewRepository(nil)
	q := &ibFakeQuerier{}
	if err := r.InsertStockReservation(context.Background(), q, "res-1", 111, "local-1", 42, "AAPL", 10); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInsertStockReservation_ExecError(t *testing.T) {
	boom := errors.New("exec boom")
	r := NewRepository(nil)
	q := &ibFakeQuerier{execErr: boom}
	if err := r.InsertStockReservation(context.Background(), q, "res-1", 111, "local-1", 42, "AAPL", 10); !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

// ---- FindStockReservationByReservationID ----

func TestFindStockReservationByReservationID_NotFound(t *testing.T) {
	r := NewRepository(nil)
	q := &ibFakeQuerier{row: &ibFakeRow{scanErr: pgx.ErrNoRows}}
	res, err := r.FindStockReservationByReservationID(context.Background(), q, "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil {
		t.Error("expected nil for not found")
	}
}

func TestFindStockReservationByReservationID_ScanError(t *testing.T) {
	boom := errors.New("scan boom")
	r := NewRepository(nil)
	q := &ibFakeQuerier{row: &ibFakeRow{scanErr: boom}}
	_, err := r.FindStockReservationByReservationID(context.Background(), q, "id")
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestFindStockReservationByReservationID_Found(t *testing.T) {
	r := NewRepository(nil)
	q := &ibFakeQuerier{row: &ibFakeRow{vals: []any{"res-1", int64(5), int(10), "HELD"}}}
	res, err := r.FindStockReservationByReservationID(context.Background(), q, "res-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil reservation")
	}
	if res.ReservationID != "res-1" {
		t.Errorf("ReservationID = %q, want res-1", res.ReservationID)
	}
	if res.Status != "HELD" {
		t.Errorf("Status = %q, want HELD", res.Status)
	}
}

// ---- FinalizeStockReservation ----

func TestFinalizeStockReservation_Success(t *testing.T) {
	r := NewRepository(nil)
	q := &ibFakeQuerier{}
	if err := r.FinalizeStockReservation(context.Background(), q, "res-1", StatusCommitted); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFinalizeStockReservation_Error(t *testing.T) {
	boom := errors.New("boom")
	r := NewRepository(nil)
	q := &ibFakeQuerier{execErr: boom}
	if err := r.FinalizeStockReservation(context.Background(), q, "res-1", StatusReleased); !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

// ---- FindOptionReservationByNegotiationID ----

func TestFindOptionReservationByNegotiationID_NotFound(t *testing.T) {
	r := NewRepository(nil)
	q := &ibFakeQuerier{row: &ibFakeRow{scanErr: pgx.ErrNoRows}}
	res, err := r.FindOptionReservationByNegotiationID(context.Background(), q, "neg-1")
	if err != nil || res != nil {
		t.Errorf("expected nil,nil; got %v, %v", res, err)
	}
}

func TestFindOptionReservationByNegotiationID_Found(t *testing.T) {
	r := NewRepository(nil)
	q := &ibFakeQuerier{row: &ibFakeRow{vals: []any{"neg-1", "res-1", "RESERVED"}}}
	res, err := r.FindOptionReservationByNegotiationID(context.Background(), q, "neg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil")
	}
	if res.NegotiationID != "neg-1" || res.Status != "RESERVED" {
		t.Errorf("unexpected result: %+v", res)
	}
}

// ---- InsertOptionReservation ----

func TestInsertOptionReservation_Success(t *testing.T) {
	r := NewRepository(nil)
	q := &ibFakeQuerier{}
	if err := r.InsertOptionReservation(context.Background(), q, "neg-1", "res-1", OptionReserved, 42, "AAPL", 5); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInsertOptionReservation_Error(t *testing.T) {
	boom := errors.New("boom")
	r := NewRepository(nil)
	q := &ibFakeQuerier{execErr: boom}
	if err := r.InsertOptionReservation(context.Background(), q, "neg-1", "res-1", OptionReserved, 42, "AAPL", 5); !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

// ---- UpdateOptionReservationStatus ----

func TestUpdateOptionReservationStatus_Success(t *testing.T) {
	r := NewRepository(nil)
	q := &ibFakeQuerier{}
	if err := r.UpdateOptionReservationStatus(context.Background(), q, "neg-1", OptionExercised); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateOptionReservationStatus_Error(t *testing.T) {
	boom := errors.New("boom")
	r := NewRepository(nil)
	q := &ibFakeQuerier{execErr: boom}
	if err := r.UpdateOptionReservationStatus(context.Background(), q, "neg-1", OptionReleased); !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}
