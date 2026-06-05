// Package scheduler provides background goroutines for saga maintenance.
//
// # Cleanup scheduler
//
// The Scheduler runs two tasks on each tick:
//  1. Stuck-saga sweep: finds IN_PROGRESS and COMPENSATING sagas older than
//     StuckCutoff. Emits a WARN log and increments the saga_stuck_total
//     counter for each. No automatic recovery — an operator must inspect and
//     remediate manually via the admin HTTP endpoint.
//
//  2. Idempotency-log pruning: deletes rows from saga_idempotency_log older
//     than IdempotencyRetention. If the table does not exist (Postgres error
//     42P01 — "relation does not exist"), a one-time INFO log is emitted and
//     the task is silently skipped on subsequent ticks. This is safe because
//     saga_idempotency_log is an orphan table from the Java version; the Go
//     port does not create it, and its absence must not crash the process.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

// postgresRelationNotExists is the SQLSTATE for "relation does not exist".
const postgresRelationNotExists = "42P01"

// StoreForScheduler is the subset of *store.SagaInstanceStore used by the
// scheduler. It exists so that tests can inject an in-memory fake without
// a real PostgreSQL pool.
type StoreForScheduler interface {
	FindStuck(ctx context.Context, cutoff time.Time, limit int) ([]store.SagaInstance, error)
}

// idempotencyExecer is the subset of *pgxpool.Pool used for the idempotency-log
// DELETE. Extracted as an interface so cleanupIdempotencyLog can be unit-tested
// with a fake. *pgxpool.Pool satisfies this interface.
type idempotencyExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// CleanupConfig controls the cleanup scheduler behaviour.
// Fields map to Config.Saga.Cleanup in config.go.
type CleanupConfig struct {
	// Interval is how often the cleanup goroutine wakes.
	Interval time.Duration

	// StuckCutoff is the age beyond which a saga in IN_PROGRESS or COMPENSATING
	// is flagged as stuck. A WARN log + counter is emitted; no auto-recovery.
	StuckCutoff time.Duration

	// IdempotencyRetention is how long to keep saga_idempotency_log rows.
	// Rows older than this are deleted each tick.
	IdempotencyRetention time.Duration
}

// stuckTotal accumulates the number of stuck sagas observed since startup.
// A real production service would expose this as a Prometheus counter; we
// keep a simple int64 here so tests can verify it increments.
var stuckTotal int64

// Scheduler runs periodic cleanup tasks.
type Scheduler struct {
	store              StoreForScheduler
	pool               idempotencyExecer // may be nil in test mode
	cfg                CleanupConfig
	log                *slog.Logger
	idempotencyMissing bool // set to true once the "table missing" warning fires
}

// New constructs a Scheduler for production use.
// s is used for the stuck-saga query.
// pool is used for the raw DELETE FROM saga_idempotency_log query.
func New(
	s *store.SagaInstanceStore,
	pool *pgxpool.Pool,
	cfg CleanupConfig,
	log *slog.Logger,
) *Scheduler {
	return &Scheduler{
		store: s,
		pool:  pool,
		cfg:   cfg,
		log:   log,
	}
}

// NewForTest constructs a Scheduler for unit tests.
// pool is nil; cleanupIdempotencyLog will be skipped (the nil check is handled
// inside cleanupIdempotencyLog). Any StoreForScheduler implementation can be
// injected (e.g. an in-memory fake).
func NewForTest(
	s StoreForScheduler,
	cfg CleanupConfig,
	log *slog.Logger,
) *Scheduler {
	return &Scheduler{
		store:              s,
		pool:               nil,
		cfg:                cfg,
		log:                log,
		idempotencyMissing: true, // skip idempotency cleanup in tests (no pool)
	}
}

// newForTestWithExecer constructs a Scheduler that runs the idempotency-log
// cleanup against an injected execer. Test use only.
func newForTestWithExecer(
	s StoreForScheduler,
	exec idempotencyExecer,
	cfg CleanupConfig,
	log *slog.Logger,
) *Scheduler {
	return &Scheduler{
		store: s,
		pool:  exec,
		cfg:   cfg,
		log:   log,
	}
}

// Run blocks until ctx is cancelled, executing a cleanup tick at every
// cfg.Interval. Returns nil when the context is done; returns a non-nil
// error only on catastrophic setup failures (currently unused — all per-tick
// errors are logged, not propagated).
func (s *Scheduler) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	s.log.Info("cleanup scheduler started",
		"interval", s.cfg.Interval,
		"stuckCutoff", s.cfg.StuckCutoff,
		"idempotencyRetention", s.cfg.IdempotencyRetention,
	)

	for {
		select {
		case <-ctx.Done():
			s.log.Info("cleanup scheduler stopping")
			return nil
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick executes one cleanup iteration.
func (s *Scheduler) tick(ctx context.Context) {
	s.sweepStuck(ctx)
	s.cleanupIdempotencyLog(ctx)
}

// sweepStuck queries for sagas that have been IN_PROGRESS or COMPENSATING
// longer than StuckCutoff. Each is logged at WARN level and counted.
func (s *Scheduler) sweepStuck(ctx context.Context) {
	cutoff := time.Now().UTC().Add(-s.cfg.StuckCutoff)
	const limit = 100

	stuck, err := s.store.FindStuck(ctx, cutoff, limit)
	if err != nil {
		s.log.Error("stuck-saga sweep: query failed", "error", err)
		return
	}

	for _, inst := range stuck {
		stuckTotal++
		s.log.Warn("stuck saga detected",
			"sagaId", inst.ID.String(),
			"sagaType", inst.SagaType,
			"correlationId", inst.CorrelationID,
			"state", inst.State,
			"currentStep", inst.CurrentStep,
			"retryCount", inst.RetryCount,
			"age", time.Since(inst.CreatedAt).Round(time.Second).String(),
			"stuckTotalSinceStartup", stuckTotal,
		)
	}

	if len(stuck) > 0 {
		s.log.Warn("stuck-saga sweep complete", "count", len(stuck))
	}
}

// cleanupIdempotencyLog deletes old rows from saga_idempotency_log.
// If the table does not exist (SQLSTATE 42P01), a one-time INFO log is emitted
// and subsequent calls are silently skipped.
// If pool is nil (test mode), the method returns immediately.
func (s *Scheduler) cleanupIdempotencyLog(ctx context.Context) {
	if s.pool == nil || s.idempotencyMissing {
		// Already determined the table is absent (or no pool in test mode); skip silently.
		return
	}

	cutoff := time.Now().UTC().Add(-s.cfg.IdempotencyRetention)

	tag, err := s.pool.Exec(ctx,
		`DELETE FROM saga_idempotency_log WHERE processed_at < $1`,
		cutoff,
	)
	if err != nil {
		if isRelationNotExists(err) {
			// Table missing — this is expected when running without the Java
			// schema. Log once and suppress future attempts.
			s.idempotencyMissing = true
			s.log.Info("saga_idempotency_log table missing; idempotency-log cleanup skipped")
			return
		}
		s.log.Error("idempotency-log cleanup failed", "error", err)
		return
	}

	deleted := tag.RowsAffected()
	if deleted > 0 {
		s.log.Info("idempotency-log cleanup complete", "deleted", deleted)
	}
}

// isRelationNotExists reports whether err is a Postgres "relation does not
// exist" error (SQLSTATE 42P01). It walks the error chain.
func isRelationNotExists(err error) bool {
	var pgErr *pgconn.PgError
	// errors.As walk through the chain
	for err != nil {
		if e, ok := err.(*pgconn.PgError); ok { //nolint:errorlint
			pgErr = e
			break
		}
		// Try unwrap
		type unwrapper interface{ Unwrap() error }
		if u, ok := err.(unwrapper); ok {
			err = u.Unwrap()
		} else {
			break
		}
	}
	return pgErr != nil && pgErr.Code == postgresRelationNotExists
}

// StuckTotal returns the number of stuck sagas observed since process start.
// Exposed for tests; in production a Prometheus counter would be used instead.
func StuckTotal() int64 { return stuckTotal }

// ResetStuckTotal resets the counter to zero. Test use only.
func ResetStuckTotal() { stuckTotal = 0 }
