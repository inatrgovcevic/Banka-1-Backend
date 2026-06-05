package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---- fake pgx plumbing ----

type auFakeRow struct {
	vals    []any
	scanErr error
}

func (r *auFakeRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	for i, d := range dest {
		if i >= len(r.vals) {
			break
		}
		if p, ok := d.(*int64); ok {
			*p = r.vals[i].(int64)
		}
	}
	return nil
}

type auFakeRows struct {
	rows [][]any
	idx  int
}

func (r *auFakeRows) Next() bool { return r.idx < len(r.rows) }
func (r *auFakeRows) Close()     {}
func (r *auFakeRows) Err() error { return nil }
func (r *auFakeRows) CommandTag() pgconn.CommandTag               { return pgconn.CommandTag{} }
func (r *auFakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *auFakeRows) Values() ([]any, error)                        { return nil, nil }
func (r *auFakeRows) RawValues() [][]byte                           { return nil }
func (r *auFakeRows) Conn() *pgx.Conn                              { return nil }
func (r *auFakeRows) Scan(dest ...any) error {
	row := r.rows[r.idx]
	r.idx++
	id := int64(99)
	actorID := int64(1)
	name := "actor"
	action := "ORDER_APPROVED"
	tt := "USER"
	tid := "1"
	det := "detail"
	now := time.Now()
	vals := []any{&id, &actorID, &name, &action, &tt, &tid, &det, &now}
	_ = row
	for i, d := range dest {
		if i >= len(vals) {
			break
		}
		switch v := d.(type) {
		case *int64:
			if src, ok := vals[i].(*int64); ok {
				*v = *src
			}
		case **int64:
			if src, ok := vals[i].(*int64); ok {
				*v = src
			}
		case **string:
			if src, ok := vals[i].(*string); ok {
				*v = src
			}
		case *string:
			if src, ok := vals[i].(*string); ok {
				*v = *src
			}
		case *time.Time:
			if src, ok := vals[i].(*time.Time); ok {
				*v = *src
			}
		}
	}
	return nil
}

type auFakeQuerier struct {
	row     *auFakeRow
	rows    *auFakeRows
	execErr error
	count   int64
}

func (q *auFakeQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, q.execErr
}
func (q *auFakeQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if q.rows != nil {
		return q.rows, nil
	}
	return &auFakeRows{}, nil
}
func (q *auFakeQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if q.row != nil {
		return q.row
	}
	return &auFakeRow{scanErr: pgx.ErrNoRows}
}

// ---- repository tests ----

func TestNewRepository_NilPool(t *testing.T) {
	r := NewRepository(nil)
	if r == nil {
		t.Error("NewRepository returned nil")
	}
}

func TestPool_ReturnsDB(t *testing.T) {
	r := NewRepository(nil)
	if r.Pool() != nil {
		t.Error("expected nil pool")
	}
}

func TestInsert_Success(t *testing.T) {
	q := &auFakeQuerier{row: &auFakeRow{vals: []any{int64(42)}}}
	r := NewRepository(nil)
	e := &Entry{ActionType: ActionOrderApproved, CreatedAt: time.Now()}
	if err := r.Insert(context.Background(), q, e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.ID != 42 {
		t.Errorf("ID = %d, want 42", e.ID)
	}
}

func TestInsert_ScanError(t *testing.T) {
	boom := errors.New("scan boom")
	q := &auFakeQuerier{row: &auFakeRow{scanErr: boom}}
	r := NewRepository(nil)
	e := &Entry{ActionType: ActionOrderApproved, CreatedAt: time.Now()}
	if err := r.Insert(context.Background(), q, e); !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestSearch_CountError(t *testing.T) {
	boom := errors.New("count error")
	q := &auFakeQuerier{row: &auFakeRow{scanErr: boom}}
	r := NewRepository(nil)
	_, _, err := r.Search(context.Background(), q, SearchFilter{Page: 0, Size: 10})
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestSearch_EmptyResult(t *testing.T) {
	q := &auFakeQuerier{
		row:  &auFakeRow{vals: []any{int64(0)}},
		rows: &auFakeRows{},
	}
	r := NewRepository(nil)
	entries, total, err := r.Search(context.Background(), q, SearchFilter{Page: 0, Size: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(entries) != 0 {
		t.Errorf("entries = %d, want 0", len(entries))
	}
}

func TestSearch_WithRows(t *testing.T) {
	// Return 1 row in the result set.
	q := &auFakeQuerier{
		row:  &auFakeRow{vals: []any{int64(1)}},
		rows: &auFakeRows{rows: [][]any{{}}}, // one row, Scan fills from hardcoded vals
	}
	r := NewRepository(nil)
	entries, total, err := r.Search(context.Background(), q, SearchFilter{Page: 0, Size: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(entries) != 1 {
		t.Errorf("entries = %d, want 1", len(entries))
	}
}

func TestSearch_WithFilter(t *testing.T) {
	action := ActionOrderApproved
	actorID := int64(5)
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()
	q := &auFakeQuerier{
		row:  &auFakeRow{vals: []any{int64(3)}},
		rows: &auFakeRows{},
	}
	r := NewRepository(nil)
	_, total, err := r.Search(context.Background(), q, SearchFilter{
		ActionType: &action,
		ActorID:    &actorID,
		From:       &from,
		To:         &to,
		Page:       0,
		Size:       5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
}
