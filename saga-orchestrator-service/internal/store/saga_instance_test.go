package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	sharedpgx "github.com/raf-si-2025/banka-1-go/shared/pgxpool"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

// getPool opens a real pool when SAGA_DB_URL is set, otherwise returns nil.
// Tests that require a real pool skip themselves when the pool is nil.
func getPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("SAGA_DB_URL")
	if dsn == "" {
		return nil
	}
	pool, err := sharedpgx.New(context.Background(), sharedpgx.Config{URL: dsn})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestSagaInstanceStore_InsertAndFind(t *testing.T) {
	pool := getPool(t)
	if pool == nil {
		t.Skip("SAGA_DB_URL not set — skipping DB integration test")
	}

	s := store.NewSagaInstanceStore(pool)
	ctx := context.Background()

	inst := &store.SagaInstance{
		SagaType:      "OTC_EXERCISE",
		CorrelationID: "test-corr-" + uuid.New().String(),
		CurrentStep:   0,
		TotalSteps:    5,
		State:         store.SagaStateStarted,
		Payload:       []byte(`{"contractId":42}`),
	}
	if err := s.Insert(ctx, inst); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if inst.ID == uuid.Nil {
		t.Fatal("Insert should auto-assign ID")
	}

	got, err := s.FindByID(ctx, inst.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got == nil {
		t.Fatal("FindByID returned nil")
	}
	if got.SagaType != "OTC_EXERCISE" {
		t.Errorf("SagaType mismatch: %q", got.SagaType)
	}
	if got.CorrelationID != inst.CorrelationID {
		t.Errorf("CorrelationID mismatch: %q", got.CorrelationID)
	}
}

func TestSagaInstanceStore_FindByTypeAndCorrelation_NotFound(t *testing.T) {
	pool := getPool(t)
	if pool == nil {
		t.Skip("SAGA_DB_URL not set — skipping DB integration test")
	}

	s := store.NewSagaInstanceStore(pool)
	ctx := context.Background()

	got, err := s.FindByTypeAndCorrelation(ctx, "OTC_EXERCISE", "no-such-corr-"+uuid.New().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestSagaInstanceStore_UpdateOptimistic(t *testing.T) {
	pool := getPool(t)
	if pool == nil {
		t.Skip("SAGA_DB_URL not set — skipping DB integration test")
	}

	s := store.NewSagaInstanceStore(pool)
	ctx := context.Background()

	inst := &store.SagaInstance{
		SagaType:      "FUND_SUBSCRIBE",
		CorrelationID: "upd-corr-" + uuid.New().String(),
		CurrentStep:   0,
		TotalSteps:    1,
		State:         store.SagaStateStarted,
	}
	if err := s.Insert(ctx, inst); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	origVersion := inst.Version

	inst.State = store.SagaStateInProgress
	inst.CurrentStep = 1
	if err := s.UpdateOptimistic(ctx, inst); err != nil {
		t.Fatalf("UpdateOptimistic: %v", err)
	}
	if inst.Version != origVersion+1 {
		t.Errorf("version should increment: got %d want %d", inst.Version, origVersion+1)
	}
}

func TestSagaInstanceStore_UpdateOptimistic_ConflictOnStaleVersion(t *testing.T) {
	pool := getPool(t)
	if pool == nil {
		t.Skip("SAGA_DB_URL not set — skipping DB integration test")
	}

	s := store.NewSagaInstanceStore(pool)
	ctx := context.Background()

	inst := &store.SagaInstance{
		SagaType:      "FUND_REDEEM",
		CorrelationID: "conflict-corr-" + uuid.New().String(),
		CurrentStep:   0,
		TotalSteps:    1,
		State:         store.SagaStateStarted,
	}
	if err := s.Insert(ctx, inst); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// First update succeeds and increments version.
	inst.State = store.SagaStateInProgress
	if err := s.UpdateOptimistic(ctx, inst); err != nil {
		t.Fatalf("first UpdateOptimistic: %v", err)
	}

	// Second update with stale version (inst.Version was NOT decremented) —
	// actually since UpdateOptimistic already incremented it, decrement manually
	// to simulate stale version.
	inst.Version-- // simulate stale reader
	inst.State = store.SagaStateCompleted
	err := s.UpdateOptimistic(ctx, inst)
	if err != store.ErrOptimisticLockConflict {
		t.Fatalf("expected ErrOptimisticLockConflict, got: %v", err)
	}
}

func TestSagaInstanceStore_FindStuck(t *testing.T) {
	pool := getPool(t)
	if pool == nil {
		t.Skip("SAGA_DB_URL not set — skipping DB integration test")
	}

	s := store.NewSagaInstanceStore(pool)
	ctx := context.Background()

	// Insert an "old" IN_PROGRESS saga.
	inst := &store.SagaInstance{
		SagaType:      "OTC_EXERCISE",
		CorrelationID: "stuck-corr-" + uuid.New().String(),
		CurrentStep:   2,
		TotalSteps:    5,
		State:         store.SagaStateInProgress,
	}
	if err := s.Insert(ctx, inst); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// cutoff in the future → should find our just-inserted row.
	stuck, err := s.FindStuck(ctx, time.Now().Add(1*time.Minute), 10)
	if err != nil {
		t.Fatalf("FindStuck: %v", err)
	}
	found := false
	for _, si := range stuck {
		if si.ID == inst.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("FindStuck should have returned the stuck saga")
	}
}

func TestSagaInstanceStore_ListAll(t *testing.T) {
	pool := getPool(t)
	if pool == nil {
		t.Skip("SAGA_DB_URL not set — skipping DB integration test")
	}

	s := store.NewSagaInstanceStore(pool)
	ctx := context.Background()

	// Just verify the query runs without error and returns a slice.
	_, err := s.ListAll(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
}

func TestIsUniqueViolation_WithNonPgError(t *testing.T) {
	// Non-pgx error should return false.
	if store.IsUniqueViolation(context.DeadlineExceeded) {
		t.Error("context.DeadlineExceeded is not a unique violation")
	}
}
