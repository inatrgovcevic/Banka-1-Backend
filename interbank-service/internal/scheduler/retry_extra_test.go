package scheduler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/scheduler"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"
)

// alwaysReturnStore ignores the cutoff and always returns its configured
// messages, so we can exercise shouldAttempt()'s backoff branch (where
// LastAttemptAt != nil and the elapsed window has / has not passed).
type alwaysReturnStore struct {
	msgs       []store.Message
	findErr    error
	updateErr  error
	updateCnt  int
}

func (s *alwaysReturnStore) FindPending(_ context.Context, _ int, _ time.Time, _ int) ([]store.Message, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	return s.msgs, nil
}

func (s *alwaysReturnStore) UpdateOptimistic(_ context.Context, _ *store.Message) error {
	s.updateCnt++
	return s.updateErr
}

// scheduler_test cannot access unexported shouldAttempt directly, so we drive it
// through TickOnce + a Sender that records whether Resend was invoked.
type recordingSender struct {
	called int
	err    error
}

func (r *recordingSender) Resend(_ context.Context, msg *store.Message) error {
	r.called++
	if r.err == nil {
		msg.Status = store.MessageStatusSent
	}
	return r.err
}

func TestNewRetryScheduler_DefaultsApplied(t *testing.T) {
	// interval <= 0 and maxRetries <= 0 should fall back to defaults; nil log too.
	fms := &alwaysReturnStore{}
	s := scheduler.NewRetryScheduler(fms, &recordingSender{}, 0, 0, nil)
	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}
	// A tick with no messages should be a clean no-op.
	s.TickOnce(context.Background())
}

func TestRetryScheduler_BackoffNotElapsed_SkipsResend(t *testing.T) {
	recent := time.Now().Add(-1 * time.Second) // backoff[0]=2s → not elapsed
	st := &alwaysReturnStore{msgs: []store.Message{{
		ID:            1,
		Direction:     store.DirectionOutbound,
		Status:        store.MessageStatusPendingSend,
		RetryCount:    0,
		LastAttemptAt: &recent,
	}}}
	sender := &recordingSender{}
	s := scheduler.NewRetryScheduler(st, sender, time.Minute, 5, nil)
	s.TickOnce(context.Background())
	if sender.called != 0 {
		t.Errorf("expected Resend skipped due to backoff, got %d calls", sender.called)
	}
}

func TestRetryScheduler_BackoffElapsed_AttemptsResend(t *testing.T) {
	old := time.Now().Add(-10 * time.Second) // backoff[0]=2s → elapsed
	st := &alwaysReturnStore{msgs: []store.Message{{
		ID:            1,
		Direction:     store.DirectionOutbound,
		Status:        store.MessageStatusPendingSend,
		RetryCount:    0,
		LastAttemptAt: &old,
	}}}
	sender := &recordingSender{}
	s := scheduler.NewRetryScheduler(st, sender, time.Minute, 5, nil)
	s.TickOnce(context.Background())
	if sender.called != 1 {
		t.Errorf("expected 1 Resend (backoff elapsed), got %d", sender.called)
	}
}

func TestRetryScheduler_HighRetryCount_ClampsBackoffIndex(t *testing.T) {
	// retry_count beyond the backoff array length must clamp to the last entry.
	old := time.Now().Add(-1 * time.Hour)
	st := &alwaysReturnStore{msgs: []store.Message{{
		ID:            1,
		Direction:     store.DirectionOutbound,
		Status:        store.MessageStatusPendingSend,
		RetryCount:    99, // >= len(backoffSeconds)
		LastAttemptAt: &old,
	}}}
	sender := &recordingSender{}
	// maxRetries large so it doesn't filter; FindPending is faked to return anyway.
	s := scheduler.NewRetryScheduler(st, sender, time.Minute, 1000, nil)
	s.TickOnce(context.Background())
	if sender.called != 1 {
		t.Errorf("expected 1 Resend with clamped backoff index, got %d", sender.called)
	}
}

func TestRetryScheduler_FindPendingError_LogsAndReturns(t *testing.T) {
	st := &alwaysReturnStore{findErr: errors.New("db unavailable")}
	sender := &recordingSender{}
	s := scheduler.NewRetryScheduler(st, sender, time.Minute, 5, nil)
	s.TickOnce(context.Background())
	if sender.called != 0 {
		t.Errorf("expected no Resend on FindPending error, got %d", sender.called)
	}
}

func TestRetryScheduler_SuccessButUpdateError_LogsWarn(t *testing.T) {
	old := time.Now().Add(-1 * time.Hour)
	st := &alwaysReturnStore{
		msgs: []store.Message{{
			ID:            1,
			Direction:     store.DirectionOutbound,
			Status:        store.MessageStatusPendingSend,
			LastAttemptAt: &old,
		}},
		updateErr: errors.New("write failed"),
	}
	sender := &recordingSender{} // success
	s := scheduler.NewRetryScheduler(st, sender, time.Minute, 5, nil)
	s.TickOnce(context.Background())
	if sender.called != 1 {
		t.Errorf("expected 1 Resend, got %d", sender.called)
	}
	if st.updateCnt != 1 {
		t.Errorf("expected UpdateOptimistic called once after success, got %d", st.updateCnt)
	}
}

func TestRetryScheduler_SuccessOptimisticConflict_Swallowed(t *testing.T) {
	old := time.Now().Add(-1 * time.Hour)
	st := &alwaysReturnStore{
		msgs: []store.Message{{
			ID:            1,
			Direction:     store.DirectionOutbound,
			Status:        store.MessageStatusPendingSend,
			LastAttemptAt: &old,
		}},
		updateErr: store.ErrOptimisticLockConflict,
	}
	sender := &recordingSender{} // success
	s := scheduler.NewRetryScheduler(st, sender, time.Minute, 5, nil)
	// Should not panic; conflict is swallowed.
	s.TickOnce(context.Background())
}

func TestRetryScheduler_FailUpdateError_LogsWarn(t *testing.T) {
	old := time.Now().Add(-1 * time.Hour)
	st := &alwaysReturnStore{
		msgs: []store.Message{{
			ID:            1,
			Direction:     store.DirectionOutbound,
			Status:        store.MessageStatusPendingSend,
			LastAttemptAt: &old,
		}},
		updateErr: errors.New("write failed"),
	}
	sender := &recordingSender{err: errors.New("resend failed")}
	s := scheduler.NewRetryScheduler(st, sender, time.Minute, 5, nil)
	s.TickOnce(context.Background())
	if st.updateCnt != 1 {
		t.Errorf("expected UpdateOptimistic called once after failure, got %d", st.updateCnt)
	}
}
