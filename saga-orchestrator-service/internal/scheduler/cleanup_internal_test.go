package scheduler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// emptyStore satisfies StoreForScheduler with no stuck rows.
type emptyStore struct{}

func (emptyStore) FindStuck(context.Context, time.Time, int) ([]store.SagaInstance, error) {
	return nil, nil
}

// fakeExecer implements idempotencyExecer for cleanupIdempotencyLog tests.
type fakeExecer struct {
	tag   pgconn.CommandTag
	err   error
	calls int
}

func (f *fakeExecer) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	f.calls++
	return f.tag, f.err
}

func baseCfg() CleanupConfig {
	return CleanupConfig{
		Interval:             time.Minute,
		StuckCutoff:          time.Hour,
		IdempotencyRetention: 24 * time.Hour,
	}
}

// ---------------------------------------------------------------------------
// isRelationNotExists
// ---------------------------------------------------------------------------

func TestIsRelationNotExists_MatchesCode(t *testing.T) {
	err := &pgconn.PgError{Code: postgresRelationNotExists, Message: "relation does not exist"}
	if !isRelationNotExists(err) {
		t.Error("expected true for SQLSTATE 42P01")
	}
}

func TestIsRelationNotExists_WrappedError(t *testing.T) {
	inner := &pgconn.PgError{Code: postgresRelationNotExists}
	wrapped := fmt.Errorf("query failed: %w", inner)
	if !isRelationNotExists(wrapped) {
		t.Error("expected true for wrapped 42P01 error")
	}
}

func TestIsRelationNotExists_DifferentCode(t *testing.T) {
	err := &pgconn.PgError{Code: "23505"} // unique_violation
	if isRelationNotExists(err) {
		t.Error("expected false for non-42P01 code")
	}
}

func TestIsRelationNotExists_NonPgError(t *testing.T) {
	if isRelationNotExists(errors.New("plain error")) {
		t.Error("expected false for non-PgError")
	}
}

func TestIsRelationNotExists_Nil(t *testing.T) {
	if isRelationNotExists(nil) {
		t.Error("expected false for nil error")
	}
}

// ---------------------------------------------------------------------------
// cleanupIdempotencyLog
// ---------------------------------------------------------------------------

func TestCleanupIdempotencyLog_NilPool_Skips(t *testing.T) {
	s := NewForTest(emptyStore{}, baseCfg(), discardLogger())
	// Must not panic with nil pool.
	s.cleanupIdempotencyLog(context.Background())
}

func TestCleanupIdempotencyLog_DeletesRows(t *testing.T) {
	exec := &fakeExecer{tag: pgconn.NewCommandTag("DELETE 3")}
	s := newForTestWithExecer(emptyStore{}, exec, baseCfg(), discardLogger())

	s.cleanupIdempotencyLog(context.Background())

	if exec.calls != 1 {
		t.Fatalf("expected 1 Exec call, got %d", exec.calls)
	}
	if s.idempotencyMissing {
		t.Error("idempotencyMissing should remain false on success")
	}
}

func TestCleanupIdempotencyLog_ZeroRows(t *testing.T) {
	exec := &fakeExecer{tag: pgconn.NewCommandTag("DELETE 0")}
	s := newForTestWithExecer(emptyStore{}, exec, baseCfg(), discardLogger())

	s.cleanupIdempotencyLog(context.Background())
	if exec.calls != 1 {
		t.Fatalf("expected 1 Exec call, got %d", exec.calls)
	}
}

func TestCleanupIdempotencyLog_TableMissing_SkipsAfterward(t *testing.T) {
	exec := &fakeExecer{err: &pgconn.PgError{Code: postgresRelationNotExists}}
	s := newForTestWithExecer(emptyStore{}, exec, baseCfg(), discardLogger())

	s.cleanupIdempotencyLog(context.Background())
	if !s.idempotencyMissing {
		t.Fatal("expected idempotencyMissing to be set after 42P01 error")
	}

	// A second call must short-circuit (no further Exec).
	s.cleanupIdempotencyLog(context.Background())
	if exec.calls != 1 {
		t.Errorf("expected Exec to be called only once; got %d", exec.calls)
	}
}

func TestCleanupIdempotencyLog_OtherError_Logged(t *testing.T) {
	exec := &fakeExecer{err: errors.New("connection reset")}
	s := newForTestWithExecer(emptyStore{}, exec, baseCfg(), discardLogger())

	s.cleanupIdempotencyLog(context.Background())
	if s.idempotencyMissing {
		t.Error("idempotencyMissing must stay false for non-42P01 errors (retry next tick)")
	}

	// Non-fatal errors do not suppress future attempts.
	s.cleanupIdempotencyLog(context.Background())
	if exec.calls != 2 {
		t.Errorf("expected Exec to be retried; got %d calls", exec.calls)
	}
}

// TestTick_RunsBothTasks verifies tick invokes the idempotency cleanup path.
func TestTick_RunsBothTasks(t *testing.T) {
	exec := &fakeExecer{tag: pgconn.NewCommandTag("DELETE 1")}
	s := newForTestWithExecer(emptyStore{}, exec, baseCfg(), discardLogger())

	s.tick(context.Background())
	if exec.calls != 1 {
		t.Errorf("expected tick to call idempotency cleanup once; got %d", exec.calls)
	}
}
