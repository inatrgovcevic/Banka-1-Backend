// Package audit implements WP-2 / Issue 9: the centralized audit log.
// trading-service is the SINK — producer services (order decisions here in
// trading-service-go, employee permissions in user-service-go) emit
// AuditEventDto messages; this package persists them into audit_log
// (migration 007 / Java changeset trading-otc/013-audit-log.sql) and serves
// the filtered, paginated GET /audit read API.
//
// Trading-service-go's OWN events (order approve/decline) are recorded by a
// direct local insert (no broker round-trip — the sink is in-process); the
// RabbitMQ audit.# consumer exists for OTHER services' events (e.g.
// audit.employee_permissions_changed from user-service-go).
package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Entry mirrors a row of audit_log (the AuditLog JPA entity).
type Entry struct {
	ID         int64
	ActorID    *int64
	ActorName  *string
	ActionType string
	TargetType *string
	TargetID   *string
	Details    *string
	CreatedAt  time.Time
}

// SearchFilter mirrors the AuditQueryService Specification filters plus the
// pagination the controller supplies. From/To are inclusive createdAt bounds.
type SearchFilter struct {
	ActionType *string
	ActorID    *int64
	From       *time.Time
	To         *time.Time
	Page       int
	Size       int
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Pool exposes the pool for callers that run standalone queries.
func (r *Repository) Pool() *pgxpool.Pool { return r.db }

const entryColumns = `id, actor_id, actor_name, action_type, target_type, target_id, details, created_at`

// Insert persists one audit row (mirrors auditLogRepository.save).
func (r *Repository) Insert(ctx context.Context, q Querier, e *Entry) error {
	return q.QueryRow(ctx, `
		INSERT INTO audit_log (actor_id, actor_name, action_type, target_type, target_id, details, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		e.ActorID, e.ActorName, e.ActionType, e.TargetType, e.TargetID, e.Details, e.CreatedAt).
		Scan(&e.ID)
}

// Search mirrors AuditQueryService.search: dynamic equals/range filters,
// createdAt DESC, LIMIT/OFFSET paging, plus the total count for the Page
// envelope.
func (r *Repository) Search(ctx context.Context, q Querier, f SearchFilter) ([]Entry, int64, error) {
	where := ""
	args := make([]any, 0, 6)
	and := func(cond string, value any) {
		args = append(args, value)
		clause := fmt.Sprintf(cond, len(args))
		if where == "" {
			where = " WHERE " + clause
		} else {
			where += " AND " + clause
		}
	}
	if f.ActionType != nil {
		and("action_type = $%d", *f.ActionType)
	}
	if f.ActorID != nil {
		and("actor_id = $%d", *f.ActorID)
	}
	if f.From != nil {
		and("created_at >= $%d", *f.From)
	}
	if f.To != nil {
		and("created_at <= $%d", *f.To)
	}

	var total int64
	if err := q.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limitArgs := append(args, f.Size, f.Page*f.Size)
	rows, err := q.Query(ctx, fmt.Sprintf(
		`SELECT `+entryColumns+` FROM audit_log`+where+` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		len(args)+1, len(args)+2), limitArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Entry, 0)
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.ActorID, &e.ActorName, &e.ActionType, &e.TargetType, &e.TargetID, &e.Details, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}
