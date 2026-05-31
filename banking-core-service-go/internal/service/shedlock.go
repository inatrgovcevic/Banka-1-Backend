package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"
)

// distributedLock is a PostgreSQL-backed distributed lock modelled after ShedLock.
// It uses the shedlock table to ensure that at most one instance of a scheduled
// job runs at any given time across multiple application instances.
type distributedLock struct {
	db             *sql.DB
	name           string
	lockAtMostFor  time.Duration
	lockAtLeastFor time.Duration
	lockedBy       string
}

func newDistributedLock(db *sql.DB, name string, lockAtMostFor, lockAtLeastFor time.Duration) *distributedLock {
	hostname, _ := os.Hostname()
	return &distributedLock{
		db:             db,
		name:           name,
		lockAtMostFor:  lockAtMostFor,
		lockAtLeastFor: lockAtLeastFor,
		lockedBy:       fmt.Sprintf("%s@%d", hostname, os.Getpid()),
	}
}

// try acquires the distributed lock and executes fn if acquisition succeeded.
// Returns (true, fn's error) when lock was acquired and fn was called.
// Returns (false, nil) when another instance already holds the lock — caller should skip work.
func (l *distributedLock) try(ctx context.Context, fn func(context.Context) error) (ran bool, err error) {
	now := time.Now().UTC()
	lockUntil := now.Add(l.lockAtMostFor)

	result, err := l.db.ExecContext(ctx, `
INSERT INTO shedlock (name, lock_until, locked_at, locked_by)
VALUES ($1, $2, $3, $4)
ON CONFLICT (name) DO UPDATE
    SET lock_until = EXCLUDED.lock_until,
        locked_at  = EXCLUDED.locked_at,
        locked_by  = EXCLUDED.locked_by
  WHERE shedlock.lock_until <= $3
`, l.name, lockUntil, now, l.lockedBy)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return false, nil
	}

	defer func() {
		// Release: keep lock for at least lockAtLeastFor so a restarted instance
		// doesn't immediately re-run the same job.
		_, _ = l.db.ExecContext(context.Background(), `
UPDATE shedlock
   SET lock_until = $1
 WHERE name = $2 AND locked_by = $3
`, time.Now().UTC().Add(l.lockAtLeastFor), l.name, l.lockedBy)
	}()

	return true, fn(ctx)
}
