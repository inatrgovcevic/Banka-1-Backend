package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

// ---------------------------------------------------------------------------
// In-memory Querier fake — exercises the store's SQL-bound methods without a
// real PostgreSQL connection. It interprets the queries by their argument
// shape rather than parsing SQL.
// ---------------------------------------------------------------------------

type row12 struct {
	id              uuid.UUID
	sagaType        string
	correlationID   string
	currentStep     int
	totalSteps      int
	state           string
	payload         []byte
	compensationLog []byte
	createdAt       time.Time
	updatedAt       time.Time
	retryCount      int
	version         int64
}

type fakeQuerier struct {
	rows []row12

	queryErr  error // forced error for Query
	execErr   error // forced error for Exec
	execAff   int64 // RowsAffected to return for Exec (UPDATE)
	execIsUpd bool  // when true, Exec returns execAff; when false, returns 1

	lastExecSQL string
}

func (f *fakeQuerier) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	// Single-row lookups: either FindByID (1 arg: uuid) or
	// FindByTypeAndCorrelation (2 args: sagaType, corrID).
	switch len(args) {
	case 1:
		id, _ := args[0].(uuid.UUID)
		for i := range f.rows {
			if f.rows[i].id == id {
				return &fakeRow{r: f.rows[i]}
			}
		}
	case 2:
		st, _ := args[0].(string)
		corr, _ := args[1].(string)
		for i := range f.rows {
			if f.rows[i].sagaType == st && f.rows[i].correlationID == corr {
				return &fakeRow{r: f.rows[i]}
			}
		}
	}
	return &fakeRow{noRows: true}
}

func (f *fakeQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	cp := make([]row12, len(f.rows))
	copy(cp, f.rows)
	return &fakeRows{rows: cp, idx: -1}, nil
}

func (f *fakeQuerier) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	f.lastExecSQL = sql
	if f.execErr != nil {
		return pgconn.CommandTag{}, f.execErr
	}
	if f.execIsUpd {
		// Simulate UPDATE ... returning execAff affected rows.
		return pgconn.NewCommandTag(commandTagUpdate(f.execAff)), nil
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func commandTagUpdate(n int64) string {
	// pgconn parses the trailing number as RowsAffected for UPDATE/DELETE.
	if n == 1 {
		return "UPDATE 1"
	}
	if n == 0 {
		return "UPDATE 0"
	}
	return "UPDATE 2"
}

// ---------------------------------------------------------------------------
// fakeRow / fakeRows implement pgx.Row / pgx.Rows
// ---------------------------------------------------------------------------

type fakeRow struct {
	r      row12
	noRows bool
}

func (fr *fakeRow) Scan(dest ...any) error {
	if fr.noRows {
		return pgx.ErrNoRows
	}
	return scanInto(fr.r, dest)
}

type fakeRows struct {
	rows []row12
	idx  int
	err  error
}

func (fr *fakeRows) Next() bool {
	fr.idx++
	return fr.idx < len(fr.rows)
}
func (fr *fakeRows) Scan(dest ...any) error { return scanInto(fr.rows[fr.idx], dest) }
func (fr *fakeRows) Close()                 {}
func (fr *fakeRows) Err() error             { return fr.err }
func (fr *fakeRows) CommandTag() pgconn.CommandTag         { return pgconn.CommandTag{} }
func (fr *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (fr *fakeRows) Values() ([]any, error)               { return nil, nil }
func (fr *fakeRows) RawValues() [][]byte                  { return nil }
func (fr *fakeRows) Conn() *pgx.Conn                      { return nil }

// scanInto copies a row12 into the 12 scan destinations used by the store.
func scanInto(r row12, dest []any) error {
	if len(dest) != 12 {
		return errors.New("fake scan: expected 12 destinations")
	}
	*(dest[0].(*uuid.UUID)) = r.id
	*(dest[1].(*string)) = r.sagaType
	*(dest[2].(*string)) = r.correlationID
	*(dest[3].(*int)) = r.currentStep
	*(dest[4].(*int)) = r.totalSteps
	*(dest[5].(*string)) = r.state
	*(dest[6].(*[]byte)) = r.payload
	*(dest[7].(*[]byte)) = r.compensationLog
	*(dest[8].(*time.Time)) = r.createdAt
	*(dest[9].(*time.Time)) = r.updatedAt
	*(dest[10].(*int)) = r.retryCount
	*(dest[11].(*int64)) = r.version
	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func sampleRow(id uuid.UUID, sagaType, corr, state string) row12 {
	now := time.Now().UTC()
	return row12{
		id:            id,
		sagaType:      sagaType,
		correlationID: corr,
		currentStep:   1,
		totalSteps:    5,
		state:         state,
		payload:       []byte(`{"x":1}`),
		createdAt:     now,
		updatedAt:     now,
		version:       3,
	}
}

func TestFake_FindByID_Found(t *testing.T) {
	id := uuid.New()
	q := &fakeQuerier{rows: []row12{sampleRow(id, "OTC_EXERCISE", "1", store.SagaStateStarted)}}
	s := store.NewSagaInstanceStoreWithQuerier(q)

	got, err := s.FindByID(context.Background(), id)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected found instance, got nil")
	}
	if got.SagaType != "OTC_EXERCISE" || got.Version != 3 {
		t.Errorf("unexpected instance: %+v", got)
	}
	if string(got.Payload) != `{"x":1}` {
		t.Errorf("payload mismatch: %s", got.Payload)
	}
}

func TestFake_FindByID_NotFound(t *testing.T) {
	q := &fakeQuerier{}
	s := store.NewSagaInstanceStoreWithQuerier(q)
	got, err := s.FindByID(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestFake_FindByTypeAndCorrelation_Found(t *testing.T) {
	id := uuid.New()
	q := &fakeQuerier{rows: []row12{sampleRow(id, "FUND_REDEEM", "tx-9", store.SagaStateInProgress)}}
	s := store.NewSagaInstanceStoreWithQuerier(q)

	got, err := s.FindByTypeAndCorrelation(context.Background(), "FUND_REDEEM", "tx-9")
	if err != nil {
		t.Fatalf("FindByTypeAndCorrelation: %v", err)
	}
	if got == nil || got.ID != id {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestFake_FindByTypeAndCorrelation_NotFound(t *testing.T) {
	q := &fakeQuerier{}
	s := store.NewSagaInstanceStoreWithQuerier(q)
	got, err := s.FindByTypeAndCorrelation(context.Background(), "X", "Y")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestFake_Insert_AssignsIDAndTimestamps(t *testing.T) {
	q := &fakeQuerier{}
	s := store.NewSagaInstanceStoreWithQuerier(q)
	inst := &store.SagaInstance{
		SagaType:      "OTC_EXERCISE",
		CorrelationID: "100",
		State:         store.SagaStateStarted,
		TotalSteps:    5,
	}
	if err := s.Insert(context.Background(), inst); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if inst.ID == uuid.Nil {
		t.Error("Insert should assign an ID")
	}
	if inst.CreatedAt.IsZero() || inst.UpdatedAt.IsZero() {
		t.Error("Insert should set timestamps")
	}
}

func TestFake_Insert_ExecError(t *testing.T) {
	q := &fakeQuerier{execErr: errors.New("boom")}
	s := store.NewSagaInstanceStoreWithQuerier(q)
	inst := &store.SagaInstance{SagaType: "X", CorrelationID: "1", State: "STARTED"}
	if err := s.Insert(context.Background(), inst); err == nil {
		t.Fatal("expected exec error, got nil")
	}
}

func TestFake_Insert_UniqueViolation(t *testing.T) {
	q := &fakeQuerier{execErr: &pgconn.PgError{Code: "23505"}}
	s := store.NewSagaInstanceStoreWithQuerier(q)
	inst := &store.SagaInstance{SagaType: "X", CorrelationID: "1", State: "STARTED"}
	err := s.Insert(context.Background(), inst)
	if !errors.Is(err, store.ErrOptimisticLockConflict) {
		t.Fatalf("expected ErrOptimisticLockConflict, got %v", err)
	}
}

func TestFake_UpdateOptimistic_Success(t *testing.T) {
	q := &fakeQuerier{execIsUpd: true, execAff: 1}
	s := store.NewSagaInstanceStoreWithQuerier(q)
	inst := &store.SagaInstance{
		ID:            uuid.New(),
		SagaType:      "OTC_EXERCISE",
		CorrelationID: "1",
		State:         store.SagaStateInProgress,
		Version:       4,
	}
	if err := s.UpdateOptimistic(context.Background(), inst); err != nil {
		t.Fatalf("UpdateOptimistic: %v", err)
	}
	if inst.Version != 5 {
		t.Errorf("version=%d, want 5", inst.Version)
	}
}

func TestFake_UpdateOptimistic_Conflict(t *testing.T) {
	q := &fakeQuerier{execIsUpd: true, execAff: 0}
	s := store.NewSagaInstanceStoreWithQuerier(q)
	inst := &store.SagaInstance{ID: uuid.New(), SagaType: "X", CorrelationID: "1", State: "STARTED", Version: 1}
	err := s.UpdateOptimistic(context.Background(), inst)
	if !errors.Is(err, store.ErrOptimisticLockConflict) {
		t.Fatalf("expected ErrOptimisticLockConflict, got %v", err)
	}
}

func TestFake_UpdateOptimistic_ExecError(t *testing.T) {
	q := &fakeQuerier{execErr: errors.New("db down")}
	s := store.NewSagaInstanceStoreWithQuerier(q)
	inst := &store.SagaInstance{ID: uuid.New(), SagaType: "X", CorrelationID: "1", State: "STARTED"}
	if err := s.UpdateOptimistic(context.Background(), inst); err == nil {
		t.Fatal("expected exec error, got nil")
	}
}

func TestFake_ListAll(t *testing.T) {
	q := &fakeQuerier{rows: []row12{
		sampleRow(uuid.New(), "OTC_EXERCISE", "1", store.SagaStateCompleted),
		sampleRow(uuid.New(), "FUND_REDEEM", "2", store.SagaStateFailed),
	}}
	s := store.NewSagaInstanceStoreWithQuerier(q)
	list, err := s.ListAll(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("got %d rows, want 2", len(list))
	}
}

func TestFake_ListAll_QueryError(t *testing.T) {
	q := &fakeQuerier{queryErr: errors.New("query fail")}
	s := store.NewSagaInstanceStoreWithQuerier(q)
	if _, err := s.ListAll(context.Background(), 10, 0); err == nil {
		t.Fatal("expected query error, got nil")
	}
}

func TestFake_ListByState(t *testing.T) {
	q := &fakeQuerier{rows: []row12{
		sampleRow(uuid.New(), "OTC_EXERCISE", "1", store.SagaStateInProgress),
	}}
	s := store.NewSagaInstanceStoreWithQuerier(q)
	list, err := s.ListByState(context.Background(), store.SagaStateInProgress, 10, 0)
	if err != nil {
		t.Fatalf("ListByState: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("got %d, want 1", len(list))
	}
}

func TestFake_FindStuck(t *testing.T) {
	q := &fakeQuerier{rows: []row12{
		sampleRow(uuid.New(), "OTC_EXERCISE", "1", store.SagaStateInProgress),
	}}
	s := store.NewSagaInstanceStoreWithQuerier(q)
	list, err := s.FindStuck(context.Background(), time.Now(), 10)
	if err != nil {
		t.Fatalf("FindStuck: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("got %d, want 1", len(list))
	}
}

func TestFake_FindStuck_QueryError(t *testing.T) {
	q := &fakeQuerier{queryErr: errors.New("fail")}
	s := store.NewSagaInstanceStoreWithQuerier(q)
	if _, err := s.FindStuck(context.Background(), time.Now(), 10); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Pure-logic tests
// ---------------------------------------------------------------------------

func TestIsTerminal(t *testing.T) {
	cases := []struct {
		state string
		want  bool
	}{
		{store.SagaStateCompleted, true},
		{store.SagaStateCompensated, true},
		{store.SagaStateFailed, true},
		{store.SagaStateStarted, false},
		{store.SagaStateInProgress, false},
		{store.SagaStateCompensating, false},
	}
	for _, c := range cases {
		inst := &store.SagaInstance{State: c.state}
		if got := inst.IsTerminal(); got != c.want {
			t.Errorf("IsTerminal(%q)=%v, want %v", c.state, got, c.want)
		}
	}
}

func TestIsUniqueViolation_True(t *testing.T) {
	if !store.IsUniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Error("expected true for 23505")
	}
}

func TestIsUniqueViolation_FalseForOtherCode(t *testing.T) {
	if store.IsUniqueViolation(&pgconn.PgError{Code: "42P01"}) {
		t.Error("expected false for 42P01")
	}
}

func TestIsUniqueViolation_Nil(t *testing.T) {
	if store.IsUniqueViolation(nil) {
		t.Error("expected false for nil")
	}
}
