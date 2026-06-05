package otc

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

// ============================ fake Querier ================================

// fakeRow returns the provided values on Scan, or an error.
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

// fakeRows iterates over rowsets.
type fakeRows struct {
	data   [][]any
	idx    int
	err    error
	closed bool
}

func (r *fakeRows) Close()                                       { r.closed = true }
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
func (r *fakeRows) Scan(dest ...any) error {
	return assignScan(dest, r.data[r.idx-1])
}

// assignScan copies src values into dest pointers via reflection.
func assignScan(dest []any, src []any) error {
	if len(dest) != len(src) {
		return errors.New("scan: column count mismatch")
	}
	for i := range dest {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr || dv.IsNil() {
			return errors.New("scan: dest not a pointer")
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
				return errors.New("scan: type mismatch at col " + string(rune('0'+i)))
			}
		}
		dv.Elem().Set(sv)
	}
	return nil
}

// fakeQuerier returns prepared rows for Query/QueryRow and a tag for Exec.
type fakeQuerier struct {
	row      *fakeRow
	rows     *fakeRows
	queryErr error
	execErr  error
	execTag  pgconn.CommandTag

	lastSQL  string
	lastArgs []any
}

func (q *fakeQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.lastSQL = sql
	q.lastArgs = args
	return q.execTag, q.execErr
}
func (q *fakeQuerier) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.lastSQL = sql
	q.lastArgs = args
	if q.queryErr != nil {
		return nil, q.queryErr
	}
	return q.rows, nil
}
func (q *fakeQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.lastSQL = sql
	q.lastArgs = args
	return q.row
}

func tag(s string) pgconn.CommandTag { return pgconn.NewCommandTag(s) }

// ============================ tests =======================================

func TestRepository_PoolAndNew(t *testing.T) {
	r := NewRepository(nil)
	if r == nil || r.Pool() != nil {
		t.Error("NewRepository/Pool")
	}
}

func TestInsertOffer(t *testing.T) {
	now := time.Now()
	q := &fakeQuerier{row: &fakeRow{vals: []any{int64(42), now, now, int64(0)}}}
	r := NewRepository(nil)
	o := &OtcOffer{StockTicker: "AAPL", PricePerStock: dec("1"), Premium: dec("1")}
	if err := r.InsertOffer(context.Background(), q, o); err != nil {
		t.Fatal(err)
	}
	if o.ID != 42 {
		t.Errorf("ID = %d want 42", o.ID)
	}
}

func TestFindOfferByID(t *testing.T) {
	now := time.Now()
	mb := "Jovan"
	vals := []any{int64(1), "AAPL", int64(10), int64(20), 5, "100.00", "5.00", now, OfferAccepted, &mb, now, now, int64(0)}
	q := &fakeQuerier{row: &fakeRow{vals: vals}}
	r := NewRepository(nil)
	o, err := r.FindOfferByID(context.Background(), q, 1)
	if err != nil {
		t.Fatal(err)
	}
	if o.ID != 1 || o.StockTicker != "AAPL" || !o.PricePerStock.Equal(dec("100.00")) {
		t.Errorf("offer wrong: %+v", o)
	}
}

func TestFindOfferByID_NotFound(t *testing.T) {
	q := &fakeQuerier{row: &fakeRow{err: pgx.ErrNoRows}}
	r := NewRepository(nil)
	_, err := r.FindOfferByID(context.Background(), q, 1)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestFindOfferByIDForUpdate_NotFound(t *testing.T) {
	q := &fakeQuerier{row: &fakeRow{err: pgx.ErrNoRows}}
	r := NewRepository(nil)
	_, err := r.FindOfferByIDForUpdate(context.Background(), q, 1)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateOffer(t *testing.T) {
	now := time.Now()
	q := &fakeQuerier{row: &fakeRow{vals: []any{now, int64(2)}}}
	r := NewRepository(nil)
	o := &OtcOffer{ID: 1, PricePerStock: dec("1"), Premium: dec("1")}
	if err := r.UpdateOffer(context.Background(), q, o); err != nil {
		t.Fatal(err)
	}
	if o.Version != 2 {
		t.Errorf("version = %d want 2", o.Version)
	}
}

func TestInsertOptionContract(t *testing.T) {
	now := time.Now()
	q := &fakeQuerier{row: &fakeRow{vals: []any{int64(7), now, int64(0)}}}
	r := NewRepository(nil)
	c := &OptionContract{StockTicker: "AAPL", PricePerStock: dec("1")}
	if err := r.InsertOptionContract(context.Background(), q, c); err != nil {
		t.Fatal(err)
	}
	if c.ID != 7 {
		t.Errorf("ID = %d want 7", c.ID)
	}
}

func TestFindOptionContractByID(t *testing.T) {
	now := time.Now()
	vals := []any{int64(5), int64(1), "AAPL", int64(10), int64(20), 3, "100.00", now, ContractActive, now, (*time.Time)(nil), int64(0)}
	q := &fakeQuerier{row: &fakeRow{vals: vals}}
	r := NewRepository(nil)
	c, err := r.FindOptionContractByID(context.Background(), q, 5)
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != 5 || c.Status != ContractActive {
		t.Errorf("contract wrong: %+v", c)
	}
}

func TestFindOptionContractByIDForUpdate_NotFound(t *testing.T) {
	q := &fakeQuerier{row: &fakeRow{err: pgx.ErrNoRows}}
	r := NewRepository(nil)
	_, err := r.FindOptionContractByIDForUpdate(context.Background(), q, 5)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateOptionContractStatus(t *testing.T) {
	q := &fakeQuerier{execTag: tag("UPDATE 1")}
	r := NewRepository(nil)
	if err := r.UpdateOptionContractStatus(context.Background(), q, 5, ContractActive); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateOptionContractStatus_NotFound(t *testing.T) {
	q := &fakeQuerier{execTag: tag("UPDATE 0")}
	r := NewRepository(nil)
	if err := r.UpdateOptionContractStatus(context.Background(), q, 5, ContractActive); !errors.Is(err, ErrNotFound) {
		t.Errorf("0 rows should give ErrNotFound, got %v", err)
	}
}

func TestUpdateOptionContractStatus_ExecError(t *testing.T) {
	q := &fakeQuerier{execErr: errors.New("db")}
	r := NewRepository(nil)
	if err := r.UpdateOptionContractStatus(context.Background(), q, 5, ContractActive); err == nil {
		t.Error("expected exec error")
	}
}

func TestSetOptionContractExercisedAt(t *testing.T) {
	q := &fakeQuerier{execTag: tag("UPDATE 1")}
	r := NewRepository(nil)
	if err := r.SetOptionContractExercisedAt(context.Background(), q, 5, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestSetOptionContractExercisedAt_NotFound(t *testing.T) {
	q := &fakeQuerier{execTag: tag("UPDATE 0")}
	r := NewRepository(nil)
	if err := r.SetOptionContractExercisedAt(context.Background(), q, 5, time.Now()); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSumActiveBySellerAndTicker(t *testing.T) {
	q := &fakeQuerier{row: &fakeRow{vals: []any{int64(8)}}}
	r := NewRepository(nil)
	sum, err := r.SumActiveBySellerAndTicker(context.Background(), q, 20, "AAPL")
	if err != nil || sum != 8 {
		t.Errorf("sum = %d, err = %v", sum, err)
	}
}

func TestInsertHistory(t *testing.T) {
	now := time.Now()
	q := &fakeQuerier{row: &fakeRow{vals: []any{int64(3), now}}}
	r := NewRepository(nil)
	h := &NegotiationHistory{OfferID: 1, EventType: EventCreated}
	if err := r.InsertHistory(context.Background(), q, h); err != nil {
		t.Fatal(err)
	}
	if h.ID != 3 {
		t.Errorf("ID = %d want 3", h.ID)
	}
}

func TestInsertExpiryReminderIfAbsent(t *testing.T) {
	q := &fakeQuerier{execTag: tag("INSERT 0 1")}
	r := NewRepository(nil)
	inserted, err := r.InsertExpiryReminderIfAbsent(context.Background(), q, 5, 3)
	if err != nil || !inserted {
		t.Errorf("inserted = %v, err = %v", inserted, err)
	}
}

func TestInsertExpiryReminderIfAbsent_Conflict(t *testing.T) {
	q := &fakeQuerier{execTag: tag("INSERT 0 0")}
	r := NewRepository(nil)
	inserted, err := r.InsertExpiryReminderIfAbsent(context.Background(), q, 5, 3)
	if err != nil || inserted {
		t.Errorf("conflict should give inserted=false, got %v %v", inserted, err)
	}
}

func TestInsertExpiryReminderIfAbsent_Error(t *testing.T) {
	q := &fakeQuerier{execErr: errors.New("db")}
	r := NewRepository(nil)
	if _, err := r.InsertExpiryReminderIfAbsent(context.Background(), q, 5, 3); err == nil {
		t.Error("expected error")
	}
}

func TestScanOffer_BadDecimal(t *testing.T) {
	now := time.Now()
	vals := []any{int64(1), "AAPL", int64(10), int64(20), 5, "bad", "5.00", now, OfferAccepted, (*string)(nil), now, now, int64(0)}
	_, err := scanOffer(&fakeRow{vals: vals})
	if err == nil {
		t.Error("bad price decimal should error")
	}
}

func TestScanContract_BadDecimal(t *testing.T) {
	now := time.Now()
	vals := []any{int64(5), int64(1), "AAPL", int64(10), int64(20), 3, "bad", now, ContractActive, now, (*time.Time)(nil), int64(0)}
	_, err := scanContract(&fakeRow{vals: vals})
	if err == nil {
		t.Error("bad price decimal should error")
	}
}

func TestDecimalRoundTrip(t *testing.T) {
	// guard helper used in tests
	if !dec("12.34").Equal(decimal.RequireFromString("12.34")) {
		t.Error("dec helper")
	}
}

// --- Query-based read methods (inject fake Querier via r.q) -----------------

func offerRow(id int64, status string) []any {
	now := time.Now()
	return []any{id, "AAPL", int64(10), int64(20), 5, "100.00", "5.00", now, status, (*string)(nil), now, now, int64(0)}
}

func contractRow(id int64, status string) []any {
	now := time.Now()
	return []any{id, int64(1), "AAPL", int64(10), int64(20), 3, "100.00", now, status, now, (*time.Time)(nil), int64(0)}
}

func TestFindActiveOffersForUser(t *testing.T) {
	q := &fakeQuerier{rows: &fakeRows{data: [][]any{offerRow(1, OfferPendingSeller), offerRow(2, OfferPendingBuyer)}}}
	r := &Repository{q: q}
	out, err := r.FindActiveOffersForUser(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].ID != 1 || out[1].ID != 2 {
		t.Errorf("offers wrong: %+v", out)
	}
}

func TestFindActiveOffersForUser_QueryError(t *testing.T) {
	q := &fakeQuerier{queryErr: errors.New("db")}
	r := &Repository{q: q}
	if _, err := r.FindActiveOffersForUser(context.Background(), 10); err == nil {
		t.Error("expected query error")
	}
}

func TestFindContractsByBuyerIDAndStatus(t *testing.T) {
	q := &fakeQuerier{rows: &fakeRows{data: [][]any{contractRow(5, ContractActive)}}}
	r := &Repository{q: q}
	out, err := r.FindContractsByBuyerIDAndStatus(context.Background(), 10, ContractActive)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].ID != 5 {
		t.Errorf("contracts wrong: %+v", out)
	}
}

func TestFindContractsBySellerIDAndStatus(t *testing.T) {
	q := &fakeQuerier{rows: &fakeRows{data: [][]any{contractRow(6, ContractPendingPremium)}}}
	r := &Repository{q: q}
	out, err := r.FindContractsBySellerIDAndStatus(context.Background(), 20, ContractPendingPremium)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].ID != 6 {
		t.Errorf("contracts wrong: %+v", out)
	}
}

func TestFindContractsByStatusAndSettlementDateBefore(t *testing.T) {
	q := &fakeQuerier{rows: &fakeRows{data: [][]any{contractRow(7, ContractActive)}}}
	r := &Repository{q: q}
	out, err := r.FindContractsByStatusAndSettlementDateBefore(context.Background(), ContractActive, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Errorf("len = %d want 1", len(out))
	}
}

func TestFindContractsByStatusAndSettlementDate(t *testing.T) {
	q := &fakeQuerier{rows: &fakeRows{data: [][]any{contractRow(8, ContractActive)}}}
	r := &Repository{q: q}
	out, err := r.FindContractsByStatusAndSettlementDate(context.Background(), ContractActive, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Errorf("len = %d want 1", len(out))
	}
}

func TestFindContracts_QueryError(t *testing.T) {
	q := &fakeQuerier{queryErr: errors.New("db")}
	r := &Repository{q: q}
	if _, err := r.FindContractsByBuyerIDAndStatus(context.Background(), 10, ContractActive); err == nil {
		t.Error("expected error")
	}
}

func TestHistoryForUser_Repo(t *testing.T) {
	now := time.Now()
	// historyColumns order: id, offer_id, buyer_id, seller_id, actor_id, actor_name,
	// event_type, stock_ticker, old_amount, new_amount, old_pps, new_pps,
	// old_prem, new_prem, old_settle, new_settle, old_status, new_status, changed_at
	row := []any{int64(1), int64(2), int64(10), int64(20), (*int64)(nil), (*string)(nil),
		EventCreated, "AAPL", (*int)(nil), (*int)(nil), (*string)(nil), (*string)(nil),
		(*string)(nil), (*string)(nil), (*time.Time)(nil), (*time.Time)(nil), (*string)(nil), (*string)(nil), now}
	q := &fakeQuerier{rows: &fakeRows{data: [][]any{row}}}
	r := &Repository{q: q}
	status := OfferAccepted
	other := int64(20)
	from := now.AddDate(0, 0, -1)
	to := now
	out, err := r.HistoryForUser(context.Background(), 10, &status, &other, &from, &to)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].ID != 1 {
		t.Errorf("history wrong: %+v", out)
	}
}

func TestHistoryForUser_Repo_QueryError(t *testing.T) {
	q := &fakeQuerier{queryErr: errors.New("db")}
	r := &Repository{q: q}
	if _, err := r.HistoryForUser(context.Background(), 10, nil, nil, nil, nil); err == nil {
		t.Error("expected error")
	}
}
