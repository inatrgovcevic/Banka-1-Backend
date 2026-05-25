// Package scheduler provides background goroutines for the interbank-service.
// The retry scheduler re-sends OUTBOUND messages that failed on the first attempt,
// using exponential backoff and transitioning messages to STUCK after max retries.
package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/store"
)

// DefaultMaxRetries matches the Java InterbankRetryScheduler.MAX_RETRIES.
const DefaultMaxRetries = 5

// backoffSeconds defines the wait time before each retry attempt (0-indexed by
// retry_count). Mirrors Java BACKOFFS_SECONDS = {2, 4, 8, 16, 32}.
var backoffSeconds = [DefaultMaxRetries]int64{2, 4, 8, 16, 32}

// Sender is implemented by service.InterbankClient. The scheduler calls
// Resend for each eligible PENDING_SEND message.
type Sender interface {
	Resend(ctx context.Context, msg *store.Message) error
}

// MessageStore is the subset of *store.MessageStore needed by RetryScheduler.
type MessageStore interface {
	FindPending(ctx context.Context, maxRetries int, cutoff time.Time, limit int) ([]store.Message, error)
	UpdateOptimistic(ctx context.Context, m *store.Message) error
}

// RetryScheduler ticks on interval and retries stale PENDING_SEND messages.
//
// Algorithm (per Tim 2 §2.2, §6.5):
//  1. Every interval (default: 2 minutes) query store for OUTBOUND PENDING_SEND
//     messages with retry_count < maxRetries and last_attempt_at older than cutoff.
//  2. For each message, verify that the elapsed time since last_attempt_at exceeds
//     the backoff window for the current retry count (2/4/8/16/32 seconds).
//  3. Call Sender.Resend. On success the message transitions to SENT (inside Resend).
//     On failure: increment retry_count, update last_attempt_at; if retry_count
//     reaches maxRetries flip status to STUCK for operator review.
type RetryScheduler struct {
	store      MessageStore
	sender     Sender
	log        *slog.Logger
	interval   time.Duration
	maxRetries int
}

// NewRetryScheduler constructs the scheduler. interval defaults to 2 minutes if zero.
func NewRetryScheduler(store MessageStore, sender Sender, interval time.Duration, maxRetries int, log *slog.Logger) *RetryScheduler {
	if interval <= 0 {
		interval = 2 * time.Minute
	}
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetries
	}
	if log == nil {
		log = slog.Default()
	}
	return &RetryScheduler{
		store:      store,
		sender:     sender,
		log:        log,
		interval:   interval,
		maxRetries: maxRetries,
	}
}

// Run blocks until ctx is canceled, ticking every r.interval. It returns
// ctx.Err() when the context is cancelled.
func (r *RetryScheduler) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			r.tickOnce(ctx)
		}
	}
}

// tickOnce executes one retry sweep. Exported for testability.
func (r *RetryScheduler) TickOnce(ctx context.Context) {
	r.tickOnce(ctx)
}

func (r *RetryScheduler) tickOnce(ctx context.Context) {
	// Cutoff: messages newer than 2 minutes ago are skipped (they are still
	// within their initial attempt window). Matches Java cutoff logic.
	cutoff := time.Now().Add(-2 * time.Minute)
	msgs, err := r.store.FindPending(ctx, r.maxRetries, cutoff, 50)
	if err != nil {
		r.log.WarnContext(ctx, "retry scheduler: FindPending error", "err", err)
		return
	}
	if len(msgs) == 0 {
		return
	}

	r.log.InfoContext(ctx, "retry scheduler: processing pending messages", "count", len(msgs))

	for i := range msgs {
		msg := &msgs[i]
		if !r.shouldAttempt(msg) {
			continue
		}
		r.tryOnce(ctx, msg)
	}
}

// shouldAttempt returns true if enough time has elapsed since the last attempt
// to justify the next retry (per the backoff array).
func (r *RetryScheduler) shouldAttempt(msg *store.Message) bool {
	if msg.LastAttemptAt == nil {
		return true
	}
	elapsedSec := int64(time.Since(*msg.LastAttemptAt).Seconds())
	idx := msg.RetryCount
	if idx >= len(backoffSeconds) {
		idx = len(backoffSeconds) - 1
	}
	return elapsedSec >= backoffSeconds[idx]
}

// tryOnce attempts a single re-send. On failure it increments retry_count and
// sets last_attempt_at. When retry_count reaches maxRetries the message is
// marked STUCK (operator must intervene).
// On success, the scheduler calls UpdateOptimistic to persist the SENT status
// (msg.Status will have been set to SENT by the Sender).
func (r *RetryScheduler) tryOnce(ctx context.Context, msg *store.Message) {
	sendErr := r.sender.Resend(ctx, msg)
	if sendErr == nil {
		// Sender has set msg.Status = SENT; persist via UpdateOptimistic.
		now := time.Now()
		msg.LastAttemptAt = &now
		if updateErr := r.store.UpdateOptimistic(ctx, msg); updateErr != nil {
			if errors.Is(updateErr, store.ErrOptimisticLockConflict) {
				r.log.DebugContext(ctx, "retry scheduler: optimistic lock on success (concurrent resend) — skipping",
					"msgID", msg.ID)
			} else {
				r.log.WarnContext(ctx, "retry scheduler: UpdateOptimistic after resend failed",
					"msgID", msg.ID, "err", updateErr)
			}
		} else {
			r.log.InfoContext(ctx, "retry scheduler: message resent successfully",
				"msgID", msg.ID, "key", msg.LocallyGeneratedKey, "attempt", msg.RetryCount+1)
		}
		return
	}

	// Failure: increment retry count.
	msg.RetryCount++
	now := time.Now()
	msg.LastAttemptAt = &now

	if msg.RetryCount >= r.maxRetries {
		msg.Status = store.MessageStatusStuck
		r.log.ErrorContext(ctx, "retry scheduler: message STUCK — operator review required",
			"msgID", msg.ID, "key", msg.LocallyGeneratedKey, "retries", msg.RetryCount, "err", sendErr)
	} else {
		r.log.WarnContext(ctx, "retry scheduler: resend failed",
			"msgID", msg.ID, "key", msg.LocallyGeneratedKey, "attempt", msg.RetryCount, "err", sendErr)
	}

	if updateErr := r.store.UpdateOptimistic(ctx, msg); updateErr != nil {
		if errors.Is(updateErr, store.ErrOptimisticLockConflict) {
			// Tim 2 IMPORTANT-5: another scheduler instance picked this up concurrently. Skip.
			r.log.DebugContext(ctx, "retry scheduler: optimistic lock conflict — skipping",
				"msgID", msg.ID)
		} else {
			r.log.WarnContext(ctx, "retry scheduler: UpdateOptimistic failed",
				"msgID", msg.ID, "err", updateErr)
		}
	}
}
