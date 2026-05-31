package service

import (
	"context"
	"log/slog"
	"time"
)

// TickerRetryScheduler polls the database periodically for deliveries that are
// due for a retry attempt.
type TickerRetryScheduler struct {
	store    DeliveryStore
	interval time.Duration
	log      *slog.Logger
	svc      *NotificationService
}

func NewTickerRetryScheduler(store DeliveryStore, interval time.Duration, log *slog.Logger) *TickerRetryScheduler {
	return &TickerRetryScheduler{
		store:    store,
		interval: interval,
		log:      log,
	}
}

// SetService wires in the NotificationService reference after construction
// to avoid the circular dependency at initialisation time.
func (s *TickerRetryScheduler) SetService(svc *NotificationService) {
	s.svc = svc
}

// Schedule satisfies the RetryScheduler interface. The in-memory notification
// is advisory — the ticker's DB poll is the authoritative source of truth.
func (s *TickerRetryScheduler) Schedule(id string, at time.Time) {
	s.log.Debug("retry scheduled in DB, will be picked up by poller",
		"delivery_id", id,
		"next_attempt_at", at,
	)
}

// Start runs the ticker loop in the calling goroutine. Call it in a dedicated goroutine.
func (s *TickerRetryScheduler) Start(ctx context.Context) {
	s.log.Info("starting retry scheduler", "poll_interval", s.interval)

	s.processDueRetries(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("retry scheduler stopping gracefully")
			return
		case <-ticker.C:
			s.processDueRetries(ctx)
		}
	}
}

func (s *TickerRetryScheduler) processDueRetries(ctx context.Context) {
	const batchSize = 100

	deliveries, err := s.store.FindDueRetries(ctx, time.Now().UTC(), batchSize)
	if err != nil {
		s.log.Error("failed to fetch due retries", "error", err)
		return
	}

	for _, d := range deliveries {
		s.svc.AttemptDelivery(d.DeliveryID)
	}
}
