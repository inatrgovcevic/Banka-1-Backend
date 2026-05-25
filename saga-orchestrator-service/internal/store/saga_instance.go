// Package store implements the saga_instance persistence layer using pgx/v5.
// All public methods accept a context.Context; callers are responsible for
// deadline management.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// State constants — mirror Java SagaState enum.
const (
	SagaStateStarted      = "STARTED"
	SagaStateInProgress   = "IN_PROGRESS"
	SagaStateCompensating = "COMPENSATING"
	SagaStateCompleted    = "COMPLETED"
	SagaStateFailed       = "FAILED"
)

// SagaInstance is the Go representation of the saga_instance table row.
// Payload and CompensationLog are stored as raw JSONB bytes; callers encode
// and decode them using encoding/json as needed.
type SagaInstance struct {
	ID              uuid.UUID
	SagaType        string
	CorrelationID   string
	CurrentStep     int
	TotalSteps      int
	State           string
	Payload         []byte // JSONB — may be nil
	CompensationLog []byte // JSONB — may be nil
	CreatedAt       time.Time
	UpdatedAt       time.Time
	RetryCount      int
	Version         int64
}

// IsTerminal returns true when the saga is in a final state (COMPLETED or FAILED).
func (s *SagaInstance) IsTerminal() bool {
	return s.State == SagaStateCompleted || s.State == SagaStateFailed
}

// SagaInstanceStore provides CRUD operations for saga_instance rows.
type SagaInstanceStore struct {
	pool *pgxpool.Pool
}

// NewSagaInstanceStore constructs a store backed by the provided connection pool.
func NewSagaInstanceStore(pool *pgxpool.Pool) *SagaInstanceStore {
	return &SagaInstanceStore{pool: pool}
}

// FindByTypeAndCorrelation returns the instance matching (sagaType, correlationID).
// Returns (nil, nil) if no such row exists.
func (s *SagaInstanceStore) FindByTypeAndCorrelation(
	ctx context.Context, sagaType, correlationID string,
) (*SagaInstance, error) {
	const q = `
		SELECT id, saga_type, correlation_id, current_step, total_steps, state,
		       payload, compensation_log, created_at, updated_at, retry_count, version
		FROM saga_instance
		WHERE saga_type = $1 AND correlation_id = $2`
	row := s.pool.QueryRow(ctx, q, sagaType, correlationID)
	inst, err := scanRow(row)
	if err != nil {
		return nil, err
	}
	return inst, nil
}

// Insert persists a new SagaInstance. If ID is zero it is auto-assigned.
// CreatedAt and UpdatedAt are set to now if they are zero.
// Returns ErrOptimisticLockConflict if the UNIQUE(saga_type, correlation_id)
// constraint fires (i.e. a duplicate was already inserted by a concurrent goroutine).
func (s *SagaInstanceStore) Insert(ctx context.Context, inst *SagaInstance) error {
	if inst.ID == uuid.Nil {
		inst.ID = uuid.New()
	}
	now := time.Now().UTC()
	if inst.CreatedAt.IsZero() {
		inst.CreatedAt = now
	}
	inst.UpdatedAt = now
	if inst.Version == 0 {
		inst.Version = 0 // explicit
	}

	const q = `
		INSERT INTO saga_instance
		    (id, saga_type, correlation_id, current_step, total_steps, state,
		     payload, compensation_log, created_at, updated_at, retry_count, version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`
	_, err := s.pool.Exec(ctx, q,
		inst.ID, inst.SagaType, inst.CorrelationID,
		inst.CurrentStep, inst.TotalSteps, inst.State,
		safeJSON(inst.Payload), safeJSON(inst.CompensationLog),
		inst.CreatedAt, inst.UpdatedAt, inst.RetryCount, inst.Version,
	)
	if IsUniqueViolation(err) {
		return fmt.Errorf("%w: saga_type=%s correlation_id=%s", ErrOptimisticLockConflict, inst.SagaType, inst.CorrelationID)
	}
	return err
}

// UpdateOptimistic updates a saga instance using optimistic locking.
// The WHERE clause includes version = inst.Version; if 0 rows are affected
// ErrOptimisticLockConflict is returned. On success, inst.Version is incremented.
func (s *SagaInstanceStore) UpdateOptimistic(ctx context.Context, inst *SagaInstance) error {
	inst.UpdatedAt = time.Now().UTC()
	const q = `
		UPDATE saga_instance
		SET current_step     = $1,
		    total_steps      = $2,
		    state            = $3,
		    payload          = $4,
		    compensation_log = $5,
		    updated_at       = $6,
		    retry_count      = $7,
		    version          = version + 1
		WHERE id = $8 AND version = $9`
	tag, err := s.pool.Exec(ctx, q,
		inst.CurrentStep, inst.TotalSteps, inst.State,
		safeJSON(inst.Payload), safeJSON(inst.CompensationLog),
		inst.UpdatedAt, inst.RetryCount,
		inst.ID, inst.Version,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLockConflict
	}
	inst.Version++
	return nil
}

// FindByID looks up a saga instance by its primary key.
// Returns (nil, nil) if not found.
func (s *SagaInstanceStore) FindByID(ctx context.Context, id uuid.UUID) (*SagaInstance, error) {
	const q = `
		SELECT id, saga_type, correlation_id, current_step, total_steps, state,
		       payload, compensation_log, created_at, updated_at, retry_count, version
		FROM saga_instance
		WHERE id = $1`
	row := s.pool.QueryRow(ctx, q, id)
	return scanRow(row)
}

// ListByState returns saga instances in a specific state, with pagination.
func (s *SagaInstanceStore) ListByState(
	ctx context.Context, state string, limit, offset int,
) ([]SagaInstance, error) {
	const q = `
		SELECT id, saga_type, correlation_id, current_step, total_steps, state,
		       payload, compensation_log, created_at, updated_at, retry_count, version
		FROM saga_instance
		WHERE state = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	return queryRows(ctx, s.pool, q, state, limit, offset)
}

// ListAll returns all saga instances, newest first, with pagination.
func (s *SagaInstanceStore) ListAll(ctx context.Context, limit, offset int) ([]SagaInstance, error) {
	const q = `
		SELECT id, saga_type, correlation_id, current_step, total_steps, state,
		       payload, compensation_log, created_at, updated_at, retry_count, version
		FROM saga_instance
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`
	return queryRows(ctx, s.pool, q, limit, offset)
}

// FindStuck returns sagas still in IN_PROGRESS or COMPENSATING state whose
// created_at is older than cutoff. Used by the stuck-saga sweeper goroutine.
func (s *SagaInstanceStore) FindStuck(
	ctx context.Context, cutoff time.Time, limit int,
) ([]SagaInstance, error) {
	const q = `
		SELECT id, saga_type, correlation_id, current_step, total_steps, state,
		       payload, compensation_log, created_at, updated_at, retry_count, version
		FROM saga_instance
		WHERE state IN ('IN_PROGRESS', 'COMPENSATING') AND created_at < $1
		ORDER BY created_at ASC
		LIMIT $2`
	return queryRows(ctx, s.pool, q, cutoff, limit)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// scanRow reads one saga_instance row. Returns (nil, nil) on pgx.ErrNoRows.
func scanRow(row pgx.Row) (*SagaInstance, error) {
	var inst SagaInstance
	var payload, compensationLog []byte
	err := row.Scan(
		&inst.ID, &inst.SagaType, &inst.CorrelationID,
		&inst.CurrentStep, &inst.TotalSteps, &inst.State,
		&payload, &compensationLog,
		&inst.CreatedAt, &inst.UpdatedAt, &inst.RetryCount, &inst.Version,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("store: scan saga_instance: %w", err)
	}
	inst.Payload = payload
	inst.CompensationLog = compensationLog
	return &inst, nil
}

// queryRows executes a multi-row query and returns all scanned instances.
func queryRows(ctx context.Context, pool *pgxpool.Pool, q string, args ...any) ([]SagaInstance, error) {
	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query: %w", err)
	}
	defer rows.Close()
	var result []SagaInstance
	for rows.Next() {
		var inst SagaInstance
		var payload, compensationLog []byte
		if err := rows.Scan(
			&inst.ID, &inst.SagaType, &inst.CorrelationID,
			&inst.CurrentStep, &inst.TotalSteps, &inst.State,
			&payload, &compensationLog,
			&inst.CreatedAt, &inst.UpdatedAt, &inst.RetryCount, &inst.Version,
		); err != nil {
			return nil, fmt.Errorf("store: scan row: %w", err)
		}
		inst.Payload = payload
		inst.CompensationLog = compensationLog
		result = append(result, inst)
	}
	return result, rows.Err()
}

// safeJSON returns nil for a zero-length slice (stored as SQL NULL) rather than
// an empty JSONB value, which would violate column constraints in some configs.
func safeJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
