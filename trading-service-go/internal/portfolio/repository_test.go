package portfolio

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
)

// ---- fake Querier ----------------------------------------------------------

type fakeRow struct {
	vals []any
	err  error
}

func (r *fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignScan(dest, r.vals)
}

type fakeRows struct {
	data []([]any)
	idx  int
	err  error
}

func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return r.err }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }
func (r *fakeRows) Next() bool {
	if r.idx >= len(r.data) {
		return false
	}
	r.idx++
	return true
}
func (r *fakeRows) Scan(dest ...any) error { return assignScan(dest, r.data[r.idx-1]) }

func assignScan(dest, src []any) error {
	if len(dest) != len(src) {
		return errors.New("scan: column count mismatch")
	}
	for i := range dest {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr || dv.IsNil() {
			return errors.New("scan: dest not pointer")
		}
		if src[i] == nil {
			dv.Elem().Set(reflect.Zero(dv.Elem().Type()))
			continue
		}
		sv := reflect.ValueOf(src[i])
		if !sv.Type().AssignableTo(dv.Elem().Type()) {
			if sv.Type().ConvertibleTo(dv.Elem().Type()) {
				sv = sv.Convert(dv.Elem().Type())
			} else {
				return errors.New("scan: type mismatch")
			}
		}
		dv.Elem().Set(sv)
	}
	return nil
}

type fakeQuerier struct {
	row      *fakeRow
	rows     *fakeRows
	queryErr error
	execErr  error
	execTag  pgconn.CommandTag
	lastSQL  string
}

func (q *fakeQuerier) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	q.lastSQL = sql
	return q.execTag, q.execErr
}
func (q *fakeQuerier) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	q.lastSQL = sql
	if q.queryErr != nil {
		return nil, q.queryErr
	}
	return q.rows, nil
}
func (q *fakeQuerier) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	q.lastSQL = sql
	return q.row
}

func portfolioRow(id, userID, listingID int64) []any {
	return []any{id, userID, listingID, "STOCK", 10, 2, "100.00", true, 5, time.Now()}
}

// ---- tests -----------------------------------------------------------------

func TestNewRepository_Pool(t *testing.T) {
	r := NewRepository(nil)
	if r == nil || r.Pool() != nil {
		t.Error("NewRepository/Pool")
	}
}

func TestFindByUserID(t *testing.T) {
	q := &fakeQuerier{rows: &fakeRows{data: [][]any{portfolioRow(1, 10, 100), portfolioRow(2, 10, 200)}}}
	out, err := NewRepository(nil).FindByUserID(context.Background(), q, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].ID != 1 {
		t.Errorf("out wrong: %+v", out)
	}
}

func TestFindByUserID_QueryError(t *testing.T) {
	q := &fakeQuerier{queryErr: errors.New("db")}
	if _, err := NewRepository(nil).FindByUserID(context.Background(), q, 10); err == nil {
		t.Error("expected error")
	}
}

func TestFindStockHoldersByListingID(t *testing.T) {
	q := &fakeQuerier{rows: &fakeRows{data: [][]any{portfolioRow(1, 10, 100)}}}
	out, err := NewRepository(nil).FindStockHoldersByListingID(context.Background(), q, 100)
	if err != nil || len(out) != 1 {
		t.Fatalf("err=%v out=%+v", err, out)
	}
}

func TestFindByID(t *testing.T) {
	q := &fakeQuerier{row: &fakeRow{vals: portfolioRow(5, 10, 100)}}
	p, err := NewRepository(nil).FindByID(context.Background(), q, 5)
	if err != nil || p == nil || p.ID != 5 {
		t.Fatalf("err=%v p=%+v", err, p)
	}
}

func TestFindByID_NoRows(t *testing.T) {
	q := &fakeQuerier{row: &fakeRow{err: pgx.ErrNoRows}}
	p, err := NewRepository(nil).FindByID(context.Background(), q, 5)
	if err != nil || p != nil {
		t.Errorf("no rows -> (nil,nil), got %v %v", p, err)
	}
}

func TestFindByIDForUpdate(t *testing.T) {
	q := &fakeQuerier{row: &fakeRow{vals: portfolioRow(5, 10, 100)}}
	p, err := NewRepository(nil).FindByIDForUpdate(context.Background(), q, 5)
	if err != nil || p == nil {
		t.Fatalf("err=%v p=%+v", err, p)
	}
}

func TestFindByUserIDAndListingID(t *testing.T) {
	q := &fakeQuerier{row: &fakeRow{vals: portfolioRow(5, 10, 100)}}
	p, err := NewRepository(nil).FindByUserIDAndListingID(context.Background(), q, 10, 100)
	if err != nil || p == nil {
		t.Fatalf("err=%v p=%+v", err, p)
	}
}

func TestFindByUserIDAndListingID_NoRows(t *testing.T) {
	q := &fakeQuerier{row: &fakeRow{err: pgx.ErrNoRows}}
	p, err := NewRepository(nil).FindByUserIDAndListingID(context.Background(), q, 10, 100)
	if err != nil || p != nil {
		t.Errorf("no rows -> (nil,nil)")
	}
}

func TestFindByUserIDAndListingIDForUpdate(t *testing.T) {
	q := &fakeQuerier{row: &fakeRow{vals: portfolioRow(5, 10, 100)}}
	p, err := NewRepository(nil).FindByUserIDAndListingIDForUpdate(context.Background(), q, 10, 100)
	if err != nil || p == nil {
		t.Fatalf("err=%v p=%+v", err, p)
	}
}

func TestExecMethods(t *testing.T) {
	q := &fakeQuerier{execTag: tag("UPDATE 1")}
	r := NewRepository(nil)
	ctx := context.Background()
	if err := r.UpdateReservedQuantity(ctx, q, 1, 5); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateSellPosition(ctx, q, 1, 5, 2, 1); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdatePublic(ctx, q, 1, 5, true); err != nil {
		t.Fatal(err)
	}
	if err := r.Insert(ctx, q, 10, 100, "STOCK", 5, dec("1")); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateQuantityAndAvg(ctx, q, 1, 5, dec("1")); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateQuantity(ctx, q, 1, 5); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, q, 1); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateReservedAndPublic(ctx, q, 1, 2, 3); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateQuantityAndReserved(ctx, q, 1, 5, 2); err != nil {
		t.Fatal(err)
	}
}

func TestExecMethods_Error(t *testing.T) {
	q := &fakeQuerier{execErr: errors.New("db")}
	if err := NewRepository(nil).UpdateQuantity(context.Background(), q, 1, 5); err == nil {
		t.Error("expected exec error")
	}
}

func TestFindAllPublicStocks(t *testing.T) {
	q := &fakeQuerier{rows: &fakeRows{data: [][]any{portfolioRow(1, 10, 100)}}}
	out, err := NewRepository(nil).FindAllPublicStocks(context.Background(), q)
	if err != nil || len(out) != 1 {
		t.Fatalf("err=%v out=%+v", err, out)
	}
}

func TestScanPortfolio_BadDecimal(t *testing.T) {
	vals := []any{int64(1), int64(10), int64(100), "STOCK", 10, 2, "bad", true, 5, time.Now()}
	q := &fakeQuerier{row: &fakeRow{vals: vals}}
	if _, err := NewRepository(nil).FindByID(context.Background(), q, 1); err == nil {
		t.Error("bad decimal should error")
	}
}

func tag(s string) pgconn.CommandTag { return pgconn.NewCommandTag(s) }
func dec(s string) decimal.Decimal   { return decimal.RequireFromString(s) }
