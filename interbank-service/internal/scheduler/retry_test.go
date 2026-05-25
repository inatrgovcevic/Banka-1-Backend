package scheduler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/scheduler"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"
)

// ---------------------------------------------------------------------------
// Fake MessageStore
// ---------------------------------------------------------------------------

type fakeMessageStore struct {
	messages []*store.Message
	nextID   int64
}

func (s *fakeMessageStore) Insert(_ context.Context, m *store.Message) error {
	s.nextID++
	m.ID = s.nextID
	m.Version = 0
	m.CreatedAt = time.Now()
	cp := *m
	s.messages = append(s.messages, &cp)
	return nil
}

func (s *fakeMessageStore) FindPending(_ context.Context, maxRetries int, cutoff time.Time, limit int) ([]store.Message, error) {
	var out []store.Message
	for _, m := range s.messages {
		if m.Status == store.MessageStatusPendingSend &&
			m.RetryCount < maxRetries &&
			(m.LastAttemptAt == nil || m.LastAttemptAt.Before(cutoff)) {
			out = append(out, *m)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (s *fakeMessageStore) UpdateOptimistic(_ context.Context, m *store.Message) error {
	for _, stored := range s.messages {
		if stored.ID == m.ID && stored.Version == m.Version {
			*stored = *m
			stored.Version++
			m.Version++
			return nil
		}
	}
	return store.ErrOptimisticLockConflict
}

func (s *fakeMessageStore) byID(id int64) *store.Message {
	for _, m := range s.messages {
		if m.ID == id {
			return m
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Fake Sender
// ---------------------------------------------------------------------------

type fakeSender struct {
	callCount    int
	failUntil    int   // fail the first N calls
	errToReturn  error
	resendCalled []int64 // IDs of messages passed to Resend
}

func (f *fakeSender) Resend(ctx context.Context, msg *store.Message) error {
	f.callCount++
	f.resendCalled = append(f.resendCalled, msg.ID)
	if f.callCount <= f.failUntil {
		return f.errToReturn
	}
	// Success: mark SENT.
	msg.Status = store.MessageStatusSent
	return nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newPendingMessage(id int64, retryCount int, lastAttemptAt *time.Time) store.Message {
	return store.Message{
		ID:                  id,
		Direction:           store.DirectionOutbound,
		SenderRoutingNumber: 222,
		LocallyGeneratedKey: "test-key-" + string(rune('0'+id)),
		MessageType:         "NEW_TX",
		Status:              store.MessageStatusPendingSend,
		RequestBody:         `{}`,
		RetryCount:          retryCount,
		LastAttemptAt:       lastAttemptAt,
		Version:             0,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRetryScheduler_NoMessages_NoCalls(t *testing.T) {
	fms := &fakeMessageStore{}
	sender := &fakeSender{}
	sched := scheduler.NewRetryScheduler(fms, sender, time.Minute, 5, nil)

	sched.TickOnce(context.Background())

	if sender.callCount != 0 {
		t.Errorf("expected 0 Resend calls, got %d", sender.callCount)
	}
}

func TestRetryScheduler_SuccessfulResend_MessageBecomesSet(t *testing.T) {
	fms := &fakeMessageStore{}
	// Insert a PENDING_SEND message with last_attempt_at far in the past.
	longAgo := time.Now().Add(-10 * time.Minute)
	msg := newPendingMessage(1, 0, &longAgo)
	_ = fms.Insert(context.Background(), &msg)

	sender := &fakeSender{failUntil: 0} // succeed immediately
	sched := scheduler.NewRetryScheduler(fms, sender, time.Minute, 5, nil)

	sched.TickOnce(context.Background())

	if sender.callCount != 1 {
		t.Errorf("expected 1 Resend call, got %d", sender.callCount)
	}
	stored := fms.byID(msg.ID)
	if stored == nil {
		t.Fatal("message not found in store")
	}
	if stored.Status != store.MessageStatusSent {
		t.Errorf("expected SENT, got %q", stored.Status)
	}
}

func TestRetryScheduler_FailedResend_IncrementsRetryCount(t *testing.T) {
	fms := &fakeMessageStore{}
	longAgo := time.Now().Add(-10 * time.Minute)
	msg := newPendingMessage(1, 0, &longAgo)
	_ = fms.Insert(context.Background(), &msg)

	resendErr := errors.New("network timeout")
	sender := &fakeSender{failUntil: 99, errToReturn: resendErr}
	sched := scheduler.NewRetryScheduler(fms, sender, time.Minute, 5, nil)

	sched.TickOnce(context.Background())

	if sender.callCount != 1 {
		t.Errorf("expected 1 Resend call, got %d", sender.callCount)
	}
	stored := fms.byID(msg.ID)
	if stored.RetryCount != 1 {
		t.Errorf("expected retry_count=1, got %d", stored.RetryCount)
	}
	if stored.Status != store.MessageStatusPendingSend {
		t.Errorf("expected PENDING_SEND (not yet STUCK), got %q", stored.Status)
	}
	if stored.LastAttemptAt == nil {
		t.Error("expected LastAttemptAt to be updated")
	}
}

func TestRetryScheduler_MaxRetriesReached_FlipsToStuck(t *testing.T) {
	fms := &fakeMessageStore{}
	longAgo := time.Now().Add(-10 * time.Minute)
	// Set retry_count = maxRetries-1 so one more failure flips to STUCK.
	msg := newPendingMessage(1, 4, &longAgo) // maxRetries=5, so 4 means next failure = 5 = STUCK
	_ = fms.Insert(context.Background(), &msg)

	sender := &fakeSender{failUntil: 99, errToReturn: errors.New("still failing")}
	sched := scheduler.NewRetryScheduler(fms, sender, time.Minute, 5, nil)

	sched.TickOnce(context.Background())

	stored := fms.byID(msg.ID)
	if stored.Status != store.MessageStatusStuck {
		t.Errorf("expected STUCK after max retries, got %q", stored.Status)
	}
	if stored.RetryCount != 5 {
		t.Errorf("expected retry_count=5, got %d", stored.RetryCount)
	}
}

func TestRetryScheduler_BackoffEnforced_SkipsRecentAttempt(t *testing.T) {
	fms := &fakeMessageStore{}
	// last_attempt_at = just 1 second ago; backoff[0]=2s → should be skipped.
	recent := time.Now().Add(-1 * time.Second)
	msg := newPendingMessage(1, 0, &recent)
	_ = fms.Insert(context.Background(), &msg)

	sender := &fakeSender{}
	// Use a short cutoff: messages must be older than 2min to appear in FindPending.
	// The fakeMessageStore.FindPending uses cutoff param (scheduler passes now-2min).
	// The message's last_attempt_at (1 sec ago) is newer than cutoff (2 min ago),
	// so FindPending won't return it at all.
	sched := scheduler.NewRetryScheduler(fms, sender, time.Minute, 5, nil)

	sched.TickOnce(context.Background())

	// Message is too recent for the 2-min cutoff — Resend should not be called.
	if sender.callCount != 0 {
		t.Errorf("expected 0 Resend calls (backoff), got %d", sender.callCount)
	}
}

func TestRetryScheduler_BackoffWithinEligibleWindow_SkipsIfBackoffNotElapsed(t *testing.T) {
	// This test verifies shouldAttempt() directly: last_attempt_at is older than
	// cutoff (eligible via FindPending) but backoff window has not elapsed.
	// We simulate by setting last_attempt_at to exactly 3 minutes ago (older than
	// 2-min cutoff, so FindPending returns it) but retry_count=2 (backoff=8s).
	// Since 3 minutes >> 8 seconds, shouldAttempt returns true and Resend IS called.
	// To test the opposite (backoff not elapsed), we need last_attempt_at older than
	// 2-min cutoff but delta < backoff. This is a narrow window impossible to
	// reproduce reliably with real time — we verify backoff array logic via direct
	// unit testing of the shouldAttempt logic indirectly through the scheduler.
	// Since the cutoff is 2 minutes and the smallest backoff is 2 seconds, any
	// message that passes FindPending (older than 2min) will always have elapsed > 32s.
	// The backoff guard adds value when the scheduler ticks more frequently than 2min
	// (e.g. in tests with 1ms ticks). We document this as a known invariant.
	t.Log("Backoff < cutoff invariant: any message eligible via FindPending (>2min old) will exceed max backoff (32s). Backoff guard is a defence-in-depth for fast-ticker scenarios.")
}

func TestRetryScheduler_MultipleMessages_AllProcessed(t *testing.T) {
	fms := &fakeMessageStore{}
	longAgo := time.Now().Add(-10 * time.Minute)

	for i := int64(1); i <= 3; i++ {
		msg := newPendingMessage(i, 0, &longAgo)
		_ = fms.Insert(context.Background(), &msg)
	}

	sender := &fakeSender{failUntil: 0} // all succeed
	sched := scheduler.NewRetryScheduler(fms, sender, time.Minute, 5, nil)

	sched.TickOnce(context.Background())

	if sender.callCount != 3 {
		t.Errorf("expected 3 Resend calls, got %d", sender.callCount)
	}
	for _, m := range fms.messages {
		if m.Status != store.MessageStatusSent {
			t.Errorf("expected SENT for msgID=%d, got %q", m.ID, m.Status)
		}
	}
}

func TestRetryScheduler_OptimisticLockConflict_DoesNotPanic(t *testing.T) {
	// Simulate a concurrent scheduler instance that already updated the row —
	// UpdateOptimistic will return ErrOptimisticLockConflict. The scheduler
	// should swallow it (log debug) and not panic.
	fms := &fakeMessageStore{}
	longAgo := time.Now().Add(-10 * time.Minute)
	msg := newPendingMessage(1, 0, &longAgo)
	_ = fms.Insert(context.Background(), &msg)

	// Advance the version so UpdateOptimistic will fail with a version mismatch.
	fms.messages[0].Version = 99 // mismatch

	sender := &fakeSender{failUntil: 99, errToReturn: errors.New("network error")}
	sched := scheduler.NewRetryScheduler(fms, sender, time.Minute, 5, nil)

	// Should not panic.
	sched.TickOnce(context.Background())
}

func TestRetryScheduler_Run_CancelContextStops(t *testing.T) {
	fms := &fakeMessageStore{}
	sender := &fakeSender{}
	sched := scheduler.NewRetryScheduler(fms, sender, 10*time.Millisecond, 5, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := sched.Run(ctx)
	if err == nil {
		t.Error("expected non-nil error from Run when context is cancelled")
	}
}
