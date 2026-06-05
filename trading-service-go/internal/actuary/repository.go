// Package actuary serves the /actuaries supervisor endpoints over the
// actuary_info table plus the transactions/orders profit aggregation. Mirrors
// order-service ActuaryServiceImpl. The `trading` schema is owned by Java
// Liquibase; this service runs no migrations. NUMERIC columns are read as ::text
// and parsed via shopspring/decimal to preserve scale.
package actuary

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx so the FOR UPDATE
// methods can run inside the order-execution transaction.
type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// ActuaryInfo mirrors a row of actuary_info. Limit is nullable (null for
// supervisors / unconfigured agents). The column is the SQL reserved word
// "limit" and must always be quoted.
type ActuaryInfo struct {
	EmployeeID    int64
	Limit         *decimal.Decimal
	UsedLimit     decimal.Decimal
	ReservedLimit decimal.Decimal
	NeedApproval  bool
}

// ProfitRow mirrors TransactionRepository.ActuaryProfitRow.
type ProfitRow struct {
	UserID           int64
	TotalCommission  decimal.Decimal
	TransactionCount int64
}

type Repository struct {
	db Querier
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// FindByEmployeeID returns the actuary_info row for an employee, or (nil, nil)
// when none exists.
func (r *Repository) FindByEmployeeID(ctx context.Context, employeeID int64) (*ActuaryInfo, error) {
	var (
		empID        int64
		limitText    *string
		usedText     string
		reservedText string
		needApproval bool
	)
	err := r.db.QueryRow(ctx, `
		SELECT employee_id, "limit"::text, used_limit::text, reserved_limit::text, need_approval
		FROM actuary_info WHERE employee_id = $1`, employeeID).
		Scan(&empID, &limitText, &usedText, &reservedText, &needApproval)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	used, err := decimal.NewFromString(usedText)
	if err != nil {
		return nil, err
	}
	reserved, err := decimal.NewFromString(reservedText)
	if err != nil {
		return nil, err
	}
	info := &ActuaryInfo{EmployeeID: empID, UsedLimit: used, ReservedLimit: reserved, NeedApproval: needApproval}
	if limitText != nil {
		limit, err := decimal.NewFromString(*limitText)
		if err != nil {
			return nil, err
		}
		info.Limit = &limit
	}
	return info, nil
}

// FindOrCreate returns the actuary_info row, inserting a default one
// (limit=null, used=0, reserved=0, need_approval=false) when absent. Mirrors
// ActuaryServiceImpl.createDefaultActuaryInfo, which persists a default row on
// first sight of an agent — so GET /actuaries/agents has the same write-on-read.
func (r *Repository) FindOrCreate(ctx context.Context, employeeID int64) (*ActuaryInfo, error) {
	info, err := r.FindByEmployeeID(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if info != nil {
		return info, nil
	}
	if _, err := r.db.Exec(ctx, `
		INSERT INTO actuary_info (employee_id, used_limit, reserved_limit, need_approval)
		VALUES ($1, 0, 0, FALSE)
		ON CONFLICT (employee_id) DO NOTHING`, employeeID); err != nil {
		return nil, err
	}
	return r.FindByEmployeeID(ctx, employeeID)
}

// UpdateLimit sets the daily limit. The row must already exist (callers
// FindOrCreate first, matching Java's load-or-create then save).
func (r *Repository) UpdateLimit(ctx context.Context, employeeID int64, limit decimal.Decimal) error {
	_, err := r.db.Exec(ctx, `UPDATE actuary_info SET "limit" = $2 WHERE employee_id = $1`, employeeID, limit.String())
	return err
}

// ResetLimit zeroes used_limit and reserved_limit, inserting a default row when
// absent. Mirrors resetLimit (load-or-create then set used/reserved to 0).
func (r *Repository) ResetLimit(ctx context.Context, employeeID int64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO actuary_info (employee_id, used_limit, reserved_limit, need_approval)
		VALUES ($1, 0, 0, FALSE)
		ON CONFLICT (employee_id) DO UPDATE SET used_limit = 0, reserved_limit = 0`, employeeID)
	return err
}

// SetNeedApproval sets the need_approval flag, inserting a default row when absent.
func (r *Repository) SetNeedApproval(ctx context.Context, employeeID int64, value bool) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO actuary_info (employee_id, used_limit, reserved_limit, need_approval)
		VALUES ($1, 0, 0, $2)
		ON CONFLICT (employee_id) DO UPDATE SET need_approval = $2`, employeeID, value)
	return err
}

// SumCommissionByActuary mirrors TransactionRepository.sumCommissionByActuary:
// sum of executed-transaction commission grouped by the placing actuary
// (orders.user_id), ordered by total commission desc, for [from, to).
func (r *Repository) SumCommissionByActuary(ctx context.Context, from, to time.Time) ([]ProfitRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT o.user_id,
		       COALESCE(SUM(t.commission), 0)::text AS total_commission,
		       COUNT(*) AS transaction_count
		FROM transactions t
		JOIN orders o ON o.id = t.order_id
		WHERE t.timestamp >= $1 AND t.timestamp < $2
		GROUP BY o.user_id
		ORDER BY COALESCE(SUM(t.commission), 0) DESC`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ProfitRow, 0)
	for rows.Next() {
		var row ProfitRow
		var commissionText string
		if err := rows.Scan(&row.UserID, &commissionText, &row.TransactionCount); err != nil {
			return nil, err
		}
		commission, err := decimal.NewFromString(commissionText)
		if err != nil {
			return nil, err
		}
		row.TotalCommission = commission
		out = append(out, row)
	}
	return out, rows.Err()
}

// FindByEmployeeIDForUpdate loads the actuary_info row under a write lock, or
// (nil, nil) when absent. Mirrors ActuaryInfoRepository.findByEmployeeIdForUpdate.
// Takes a Querier so it locks inside the order-execution transaction.
func (r *Repository) FindByEmployeeIDForUpdate(ctx context.Context, q Querier, employeeID int64) (*ActuaryInfo, error) {
	var (
		empID        int64
		limitText    *string
		usedText     string
		reservedText string
		needApproval bool
	)
	err := q.QueryRow(ctx, `
		SELECT employee_id, "limit"::text, used_limit::text, reserved_limit::text, need_approval
		FROM actuary_info WHERE employee_id = $1 FOR UPDATE`, employeeID).
		Scan(&empID, &limitText, &usedText, &reservedText, &needApproval)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	used, err := decimal.NewFromString(usedText)
	if err != nil {
		return nil, err
	}
	reserved, err := decimal.NewFromString(reservedText)
	if err != nil {
		return nil, err
	}
	info := &ActuaryInfo{EmployeeID: empID, UsedLimit: used, ReservedLimit: reserved, NeedApproval: needApproval}
	if limitText != nil {
		limit, err := decimal.NewFromString(*limitText)
		if err != nil {
			return nil, err
		}
		info.Limit = &limit
	}
	return info, nil
}

// UpdateReservedLimit sets reserved_limit (order confirm reserve / cancel-decline
// release) within the caller's transaction.
func (r *Repository) UpdateReservedLimit(ctx context.Context, q Querier, employeeID int64, reserved decimal.Decimal) error {
	_, err := q.Exec(ctx, `UPDATE actuary_info SET reserved_limit = $2 WHERE employee_id = $1`,
		employeeID, reserved.String())
	return err
}

// UpdateReservedAndUsedLimit sets reserved_limit and used_limit together (order
// execution moves reserved → used) within the caller's transaction.
func (r *Repository) UpdateReservedAndUsedLimit(ctx context.Context, q Querier, employeeID int64, reserved, used decimal.Decimal) error {
	_, err := q.Exec(ctx, `UPDATE actuary_info SET reserved_limit = $2, used_limit = $3 WHERE employee_id = $1`,
		employeeID, reserved.String(), used.String())
	return err
}

// (InsertAuditLog was removed: agent-management audit events now go through the
// WP-2 audit sink — see Service.recordAgentAudit over internal/audit, which
// writes the reshaped audit_log schema from migration 007.)

// ResetAllLimits zeroes used_limit and reserved_limit for every actuary row.
// Mirrors ActuaryServiceImpl.resetAllLimits (daily 23:59 scheduler).
func (r *Repository) ResetAllLimits(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `UPDATE actuary_info SET used_limit = 0, reserved_limit = 0`)
	return err
}

// FindAllEmployeeIDs returns every actuary_info employee_id. Mirrors
// ActuaryInfoRepository.findAll() as used by tax tracking, which only needs the
// ids to resolve each actuary's employee record. No ORDER BY — matches the live
// service's row order over the same table.
func (r *Repository) FindAllEmployeeIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.db.Query(ctx, `SELECT employee_id FROM actuary_info`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// FindEmployeeIDsIn returns the subset of ids that have an actuary_info row, as a
// set. Mirrors ActuaryInfoRepository.findByEmployeeIdIn (used by the order
// overview to resolve agent names only for actuaries).
func (r *Repository) FindEmployeeIDsIn(ctx context.Context, ids []int64) (map[int64]bool, error) {
	out := make(map[int64]bool)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `SELECT employee_id FROM actuary_info WHERE employee_id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
