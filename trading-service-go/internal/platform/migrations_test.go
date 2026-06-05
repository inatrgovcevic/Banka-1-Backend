package platform

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---- fake pgx row ----

type migFakeRow struct {
	boolVal bool
	intVal  int64
	scanErr error
}

func (r *migFakeRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	for _, d := range dest {
		switch v := d.(type) {
		case *bool:
			*v = r.boolVal
		case *int64:
			*v = r.intVal
		}
	}
	return nil
}

// ---- fake pgx.Tx ----

type migFakeTx struct {
	execErr    error
	commitErr  error
	rollbackErr error
}

func (t *migFakeTx) Begin(_ context.Context) (pgx.Tx, error)                          { return t, nil }
func (t *migFakeTx) BeginFunc(_ context.Context, _ func(pgx.Tx) error) error          { return nil }
func (t *migFakeTx) Commit(_ context.Context) error                                    { return t.commitErr }
func (t *migFakeTx) Rollback(_ context.Context) error                                  { return t.rollbackErr }
func (t *migFakeTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *migFakeTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (t *migFakeTx) LargeObjects() pgx.LargeObjects                              { return pgx.LargeObjects{} }
func (t *migFakeTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *migFakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, t.execErr
}
func (t *migFakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) { return nil, nil }
func (t *migFakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &migFakeRow{}
}
func (t *migFakeTx) Conn() *pgx.Conn { return nil }

// ---- fake migrationDB ----

// migFakeDB is configurable: rows is a queue of responses for QueryRow calls.
type migFakeDB struct {
	execErr   error
	rows      []*migFakeRow // queued responses for successive QueryRow calls
	rowIdx    int
	beginErr  error
	tx        *migFakeTx
}

func (d *migFakeDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, d.execErr
}
func (d *migFakeDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if d.rowIdx < len(d.rows) {
		r := d.rows[d.rowIdx]
		d.rowIdx++
		return r
	}
	return &migFakeRow{} // zero-value (bool=false, int=0)
}
func (d *migFakeDB) Begin(_ context.Context) (pgx.Tx, error) {
	if d.beginErr != nil {
		return nil, d.beginErr
	}
	if d.tx != nil {
		return d.tx, nil
	}
	return &migFakeTx{}, nil
}

// ---- helpers ----

func tmpDirWithSQL(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write temp sql: %v", err)
		}
	}
	return dir
}

// ---- RunMigrations tests ----

func TestRunMigrations_ExecFails_ReturnsError(t *testing.T) {
	boom := errors.New("exec boom")
	db := &migFakeDB{execErr: boom}
	dir := t.TempDir()
	err := runMigrations(context.Background(), db, dir)
	if err == nil {
		t.Error("expected error from Exec")
	}
}

func TestRunMigrations_EmptyDir_Success(t *testing.T) {
	// CREATE TABLE succeeds, baseline returns nil (tracked=0, no portfolio/orders),
	// ReadDir returns 0 .sql files → done.
	db := &migFakeDB{rows: []*migFakeRow{
		{intVal: 0},   // SELECT COUNT(*) → 0 (baseline check)
		{boolVal: false}, // portfolio
		{boolVal: false}, // orders
	}}
	dir := t.TempDir()
	if err := runMigrations(context.Background(), db, dir); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunMigrations_BadDir_ReturnsError(t *testing.T) {
	db := &migFakeDB{rows: []*migFakeRow{
		{intVal: 0},
		{boolVal: false},
		{boolVal: false},
	}}
	err := runMigrations(context.Background(), db, "/nonexistent/dir/xyz")
	if err == nil {
		t.Error("expected error for nonexistent dir")
	}
}

func TestRunMigrations_SqlFileAlreadyApplied_Skips(t *testing.T) {
	dir := tmpDirWithSQL(t, map[string]string{
		"001_init.sql": "CREATE TABLE foo (id INT);",
	})
	db := &migFakeDB{rows: []*migFakeRow{
		{intVal: 0},      // COUNT(*) baseline
		{boolVal: false}, // portfolio
		{boolVal: false}, // orders
		{boolVal: true},  // EXISTS → already applied → skip
	}}
	if err := runMigrations(context.Background(), db, dir); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunMigrations_SqlFileNotApplied_Runs(t *testing.T) {
	dir := tmpDirWithSQL(t, map[string]string{
		"001_init.sql": "CREATE TABLE foo (id INT);",
	})
	db := &migFakeDB{rows: []*migFakeRow{
		{intVal: 0},      // COUNT(*) baseline
		{boolVal: false}, // portfolio
		{boolVal: false}, // orders
		{boolVal: false}, // EXISTS → not applied → run it
	}}
	if err := runMigrations(context.Background(), db, dir); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunMigrations_SqlFileNotApplied_ExecFails(t *testing.T) {
	dir := tmpDirWithSQL(t, map[string]string{
		"001_init.sql": "CREATE TABLE foo (id INT);",
	})
	db := &migFakeDB{
		rows: []*migFakeRow{
			{intVal: 0},      // COUNT(*)
			{boolVal: false}, // portfolio
			{boolVal: false}, // orders
			{boolVal: false}, // EXISTS
		},
		tx: &migFakeTx{execErr: errors.New("sql exec fail")},
	}
	err := runMigrations(context.Background(), db, dir)
	if err == nil {
		t.Error("expected error when sql exec fails")
	}
}

func TestRunMigrations_BeginFails_ReturnsError(t *testing.T) {
	dir := tmpDirWithSQL(t, map[string]string{
		"001_init.sql": "SELECT 1;",
	})
	db := &migFakeDB{
		rows: []*migFakeRow{
			{intVal: 0},
			{boolVal: false},
			{boolVal: false},
			{boolVal: false},
		},
		beginErr: errors.New("begin failed"),
	}
	if err := runMigrations(context.Background(), db, dir); err == nil {
		t.Error("expected error when Begin fails")
	}
}

func TestRunMigrations_DevseedSkipped_WhenNotDevContext(t *testing.T) {
	t.Setenv("LIQUIBASE_CONTEXTS", "prod")
	dir := tmpDirWithSQL(t, map[string]string{
		"002_devseed_trading.sql": "INSERT INTO foo VALUES (1);",
	})
	db := &migFakeDB{rows: []*migFakeRow{
		{intVal: 0},
		{boolVal: false},
		{boolVal: false},
		// No EXISTS call expected — devseed file is skipped before querying
	}}
	if err := runMigrations(context.Background(), db, dir); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunMigrations_CommitFails_ReturnsError(t *testing.T) {
	dir := tmpDirWithSQL(t, map[string]string{
		"001_init.sql": "SELECT 1;",
	})
	db := &migFakeDB{
		rows: []*migFakeRow{
			{intVal: 0},
			{boolVal: false},
			{boolVal: false},
			{boolVal: false},
		},
		tx: &migFakeTx{commitErr: errors.New("commit failed")},
	}
	if err := runMigrations(context.Background(), db, dir); err == nil {
		t.Error("expected error when commit fails")
	}
}

// ---- baselineExistingJavaSchema tests ----

func TestBaselineExistingJavaSchema_AlreadyTracked_NoOp(t *testing.T) {
	db := &migFakeDB{rows: []*migFakeRow{
		{intVal: 5}, // COUNT(*) = 5 → already tracked → return nil
	}}
	dir := t.TempDir()
	if err := baselineExistingJavaSchema(context.Background(), db, dir); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBaselineExistingJavaSchema_NoSentinelTables_NoOp(t *testing.T) {
	db := &migFakeDB{rows: []*migFakeRow{
		{intVal: 0},      // COUNT(*) = 0
		{boolVal: false}, // portfolio → missing
		{boolVal: false}, // orders → missing
	}}
	dir := t.TempDir()
	if err := baselineExistingJavaSchema(context.Background(), db, dir); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBaselineExistingJavaSchema_CountFails_ReturnsError(t *testing.T) {
	db := &migFakeDB{rows: []*migFakeRow{
		{scanErr: errors.New("count failed")},
	}}
	if err := baselineExistingJavaSchema(context.Background(), db, t.TempDir()); err == nil {
		t.Error("expected error when COUNT fails")
	}
}

func TestBaselineExistingJavaSchema_PortfolioCheckFails_ReturnsError(t *testing.T) {
	db := &migFakeDB{rows: []*migFakeRow{
		{intVal: 0},
		{scanErr: errors.New("portfolio check failed")},
	}}
	if err := baselineExistingJavaSchema(context.Background(), db, t.TempDir()); err == nil {
		t.Error("expected error when portfolio check fails")
	}
}

func TestBaselineExistingJavaSchema_BothSentinelsThere_BaselinesSQLFiles(t *testing.T) {
	dir := tmpDirWithSQL(t, map[string]string{
		"001_init.sql": "SELECT 1;",
		"002_data.sql": "SELECT 2;",
	})
	db := &migFakeDB{rows: []*migFakeRow{
		{intVal: 0},     // COUNT(*) = 0
		{boolVal: true}, // portfolio exists
		{boolVal: true}, // orders exists
	}}
	if err := baselineExistingJavaSchema(context.Background(), db, dir); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBaselineExistingJavaSchema_BothSentinels_BeginFails(t *testing.T) {
	dir := tmpDirWithSQL(t, map[string]string{"001.sql": "SELECT 1;"})
	db := &migFakeDB{
		rows: []*migFakeRow{
			{intVal: 0},
			{boolVal: true},
			{boolVal: true},
		},
		beginErr: errors.New("begin failed"),
	}
	if err := baselineExistingJavaSchema(context.Background(), db, dir); err == nil {
		t.Error("expected error when Begin fails")
	}
}

func TestBaselineExistingJavaSchema_BadDir_ReturnsError(t *testing.T) {
	db := &migFakeDB{rows: []*migFakeRow{
		{intVal: 0},
		{boolVal: true},
		{boolVal: true},
	}}
	if err := baselineExistingJavaSchema(context.Background(), db, "/nonexistent/xyz"); err == nil {
		t.Error("expected error for bad dir")
	}
}

func TestBaselineExistingJavaSchema_OrdersCheckFails(t *testing.T) {
	db := &migFakeDB{rows: []*migFakeRow{
		{intVal: 0},
		{boolVal: true},
		{scanErr: errors.New("orders check failed")},
	}}
	if err := baselineExistingJavaSchema(context.Background(), db, t.TempDir()); err == nil {
		t.Error("expected error when orders check fails")
	}
}
