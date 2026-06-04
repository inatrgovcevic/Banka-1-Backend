package service_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"Banka1Back/notification-service-go/internal/model"
	"Banka1Back/notification-service-go/internal/service"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// TickerRetryScheduler.Schedule — advisory only, does not panic
// ---------------------------------------------------------------------------

func TestTickerRetryScheduler_Schedule_DoesNotPanic(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	sched := service.NewTickerRetryScheduler(&stubStore{}, 1*time.Minute, log)

	assert.NotPanics(t, func() {
		sched.Schedule("delivery-id-1", time.Now().Add(5*time.Second))
	})
}

// ---------------------------------------------------------------------------
// TickerRetryScheduler.Start — processes due retries immediately on start
// ---------------------------------------------------------------------------

func TestTickerRetryScheduler_Start_ProcessesDueRetriesOnFirstTick(t *testing.T) {
	t.Parallel()

	pending := model.NewPendingDelivery(
		"user@bank.io", "Subject", "Body",
		model.NotificationTypeEmployeeCreated, 4,
	)

	dueRetryStore := &retryAwareStore{
		stubStore: &stubStore{findResult: pending},
		dueItems:  []*model.NotificationDelivery{pending},
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	sched := service.NewTickerRetryScheduler(dueRetryStore, 1*time.Hour, log)

	svc, sender, _ := newService(dueRetryStore, nil, nil)
	sched.SetService(svc)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	sched.Start(ctx)

	assert.Equal(t, 1, sender.calls, "due retry must be attempted immediately on start")
}

// ---------------------------------------------------------------------------
// TickerRetryScheduler.Start — stops gracefully when context cancelled
// ---------------------------------------------------------------------------

func TestTickerRetryScheduler_Start_StopsOnContextCancellation(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	sched := service.NewTickerRetryScheduler(&stubStore{}, 50*time.Millisecond, log)

	svc, _, _ := newService(&stubStore{}, nil, nil)
	sched.SetService(svc)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		sched.Start(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("TickerRetryScheduler did not stop within timeout after context cancellation")
	}
}

// ---------------------------------------------------------------------------
// TickerRetryScheduler.processDueRetries — handles store error gracefully
// ---------------------------------------------------------------------------

func TestTickerRetryScheduler_Start_StoreError_ContinuesRunning(t *testing.T) {
	t.Parallel()

	errStore := &retryAwareStore{
		stubStore: &stubStore{},
		dueErr:    assert.AnError,
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	sched := service.NewTickerRetryScheduler(errStore, 50*time.Millisecond, log)

	svc, _, _ := newService(&stubStore{}, nil, nil)
	sched.SetService(svc)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	assert.NotPanics(t, func() {
		sched.Start(ctx)
	})
}

// ---------------------------------------------------------------------------
// retryAwareStore — extends stubStore with FindDueRetries behavior
// ---------------------------------------------------------------------------

type retryAwareStore struct {
	*stubStore
	dueItems []*model.NotificationDelivery
	dueErr   error
}

func (r *retryAwareStore) FindDueRetries(_ context.Context, _ time.Time, _ int) ([]*model.NotificationDelivery, error) {
	return r.dueItems, r.dueErr
}
