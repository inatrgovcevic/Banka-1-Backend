package scheduler_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/scheduler"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

// ---------------------------------------------------------------------------
// Fake SagaInstanceStore for scheduler tests
// ---------------------------------------------------------------------------

// fakeStoreForScheduler is an in-memory SagaInstanceStore stand-in.
// It only implements FindStuck (the scheduler's store dependency).
type fakeStoreForScheduler struct {
	stuckRows []store.SagaInstance
	// Track how many times FindStuck was called.
	findStuckCalls int
}

func (f *fakeStoreForScheduler) FindStuck(_ context.Context, cutoff time.Time, limit int) ([]store.SagaInstance, error) {
	f.findStuckCalls++
	var out []store.SagaInstance
	for _, r := range f.stuckRows {
		if r.CreatedAt.Before(cutoff) {
			out = append(out, r)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Tests — no DB required; scheduler.New accepts the real store type so we
// bypass it and test the internal sweepStuck/cleanupIdempotencyLog logic
// via the exported helpers.
// ---------------------------------------------------------------------------

// newStuckInstance creates a SagaInstance that is "stuck" (age = 2h).
func newStuckInstance(state string) store.SagaInstance {
	old := time.Now().UTC().Add(-2 * time.Hour)
	return store.SagaInstance{
		ID:            uuid.New(),
		SagaType:      "OTC_EXERCISE",
		CorrelationID: "42",
		State:         state,
		CurrentStep:   2,
		TotalSteps:    5,
		CreatedAt:     old,
		UpdatedAt:     old,
	}
}

// TestSweepStuck_IncreasesCounter verifies that observing stuck sagas
// increments the StuckTotal counter.
func TestSweepStuck_IncreasesCounter(t *testing.T) {
	scheduler.ResetStuckTotal()

	fake := &fakeStoreForScheduler{
		stuckRows: []store.SagaInstance{
			newStuckInstance(store.SagaStateInProgress),
			newStuckInstance(store.SagaStateCompensating),
		},
	}

	log := slog.Default()

	cfg := scheduler.CleanupConfig{
		Interval:             time.Minute,
		StuckCutoff:          1 * time.Hour,
		IdempotencyRetention: 336 * time.Hour,
	}

	// We can't call internal sweepStuck directly, but we can verify the behaviour
	// via a short-lived scheduler run:
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Override interval to something very short so we get one tick quickly.
	cfg.Interval = 10 * time.Millisecond

	// Build a minimal scheduler by re-wiring through the exported New.
	// We pass nil pool because cleanupIdempotencyLog will get a nil pool error,
	// which we trap as a test-only acceptable failure.
	s := scheduler.NewForTest(fake, cfg, log)
	// Run in background — it will tick once before ctx expires.
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	<-done

	if scheduler.StuckTotal() < 2 {
		t.Errorf("StuckTotal: got %d, want >= 2", scheduler.StuckTotal())
	}
}

// TestSweepStuck_NoStuckSagas — counter stays at 0 when no sagas are old enough.
func TestSweepStuck_NoStuckSagas(t *testing.T) {
	scheduler.ResetStuckTotal()

	recent := time.Now().UTC()
	fake := &fakeStoreForScheduler{
		stuckRows: []store.SagaInstance{
			{
				ID:            uuid.New(),
				SagaType:      "OTC_EXERCISE",
				CorrelationID: "1",
				State:         store.SagaStateInProgress,
				CreatedAt:     recent, // NOT old enough
			},
		},
	}

	log := slog.Default()
	cfg := scheduler.CleanupConfig{
		Interval:    10 * time.Millisecond,
		StuckCutoff: 1 * time.Hour,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	s := scheduler.NewForTest(fake, cfg, log)
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	<-done

	if scheduler.StuckTotal() != 0 {
		t.Errorf("StuckTotal: got %d, want 0", scheduler.StuckTotal())
	}
}

// TestRun_StopsOnContextCancel ensures the scheduler goroutine exits cleanly.
func TestRun_StopsOnContextCancel(t *testing.T) {
	fake := &fakeStoreForScheduler{}
	log := slog.Default()
	cfg := scheduler.CleanupConfig{
		Interval:    500 * time.Millisecond,
		StuckCutoff: 1 * time.Hour,
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := scheduler.NewForTest(fake, cfg, log)

	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	// Cancel before any tick fires.
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}
