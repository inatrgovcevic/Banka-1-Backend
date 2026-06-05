package otc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// =========================== otc_negotiation_history ======================

const historyColumns = `id, offer_id, buyer_id, seller_id, actor_id, actor_name,
	event_type, stock_ticker, old_amount, new_amount,
	old_price_per_stock::text, new_price_per_stock::text,
	old_premium::text, new_premium::text,
	old_settlement_date, new_settlement_date, old_status, new_status, changed_at`

func scanHistory(row pgx.Row) (*NegotiationHistory, error) {
	var (
		h        NegotiationHistory
		oldPPS   *string
		newPPS   *string
		oldPrem  *string
		newPremS *string
	)
	if err := row.Scan(&h.ID, &h.OfferID, &h.BuyerID, &h.SellerID, &h.ActorID, &h.ActorName,
		&h.EventType, &h.StockTicker, &h.OldAmount, &h.NewAmount,
		&oldPPS, &newPPS, &oldPrem, &newPremS,
		&h.OldSettlementDate, &h.NewSettlementDate, &h.OldStatus, &h.NewStatus, &h.ChangedAt); err != nil {
		return nil, err
	}
	var err error
	if h.OldPricePerStock, err = decPtr(oldPPS); err != nil {
		return nil, err
	}
	if h.NewPricePerStock, err = decPtr(newPPS); err != nil {
		return nil, err
	}
	if h.OldPremium, err = decPtr(oldPrem); err != nil {
		return nil, err
	}
	if h.NewPremium, err = decPtr(newPremS); err != nil {
		return nil, err
	}
	return &h, nil
}

// decPtr converts a nullable NUMERIC::text into a *decimal.Decimal (nil → nil).
func decPtr(s *string) (*decimal.Decimal, error) {
	if s == nil {
		return nil, nil
	}
	d, err := decimal.NewFromString(*s)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// InsertHistory mirrors OtcNegotiationHistoryService.record — one audit row per
// offer transition. The caller supplies the old/new snapshot fields.
func (r *Repository) InsertHistory(ctx context.Context, q Querier, h *NegotiationHistory) error {
	if q == nil {
		q = r.db
	}
	return q.QueryRow(ctx, `
		INSERT INTO otc_negotiation_history
			(offer_id, buyer_id, seller_id, actor_id, actor_name, event_type, stock_ticker,
			 old_amount, new_amount, old_price_per_stock, new_price_per_stock,
			 old_premium, new_premium, old_settlement_date, new_settlement_date,
			 old_status, new_status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		RETURNING id, changed_at`,
		h.OfferID, h.BuyerID, h.SellerID, h.ActorID, h.ActorName, h.EventType, h.StockTicker,
		h.OldAmount, h.NewAmount, h.OldPricePerStock, h.NewPricePerStock,
		h.OldPremium, h.NewPremium, h.OldSettlementDate, h.NewSettlementDate,
		h.OldStatus, h.NewStatus,
	).Scan(&h.ID, &h.ChangedAt)
}

// HistoryForUser mirrors OtcNegotiationHistoryService.historyForUser: the
// JpaSpecification (buyer_id = user OR seller_id = user) plus the optional status
// / otherParty / dateFrom / dateTo predicates, sorted changed_at DESC. The date
// bounds are already resolved by the service (dateFrom → start-of-day, dateTo →
// start of the following day), mirroring Java's atStartOfDay() / plusDays(1).
func (r *Repository) HistoryForUser(ctx context.Context, userID int64, status *string, otherPartyID *int64, dateFrom, dateTo *time.Time) ([]NegotiationHistory, error) {
	conds := []string{"(buyer_id = $1 OR seller_id = $1)"}
	args := []any{userID}
	n := 1
	if status != nil {
		n++
		conds = append(conds, fmt.Sprintf("new_status = $%d", n))
		args = append(args, *status)
	}
	if otherPartyID != nil {
		n++
		conds = append(conds, fmt.Sprintf(
			"((buyer_id = $1 AND seller_id = $%d) OR (seller_id = $1 AND buyer_id = $%d))", n, n))
		args = append(args, *otherPartyID)
	}
	if dateFrom != nil {
		n++
		conds = append(conds, fmt.Sprintf("changed_at >= $%d", n))
		args = append(args, *dateFrom)
	}
	if dateTo != nil {
		n++
		conds = append(conds, fmt.Sprintf("changed_at < $%d", n))
		args = append(args, *dateTo)
	}
	query := `SELECT ` + historyColumns + ` FROM otc_negotiation_history WHERE ` +
		strings.Join(conds, " AND ") + ` ORDER BY changed_at DESC`
	rows, err := r.querier().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]NegotiationHistory, 0)
	for rows.Next() {
		h, err := scanHistory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *h)
	}
	return out, rows.Err()
}

// ======================= otc_contract_expiry_reminders ====================

// InsertExpiryReminderIfAbsent inserts the (contract_id, reminder_days) idempotency
// marker, returning true when this call won the race and inserted the row (so the
// caller should send the reminder), false when a marker already existed. Uses
// INSERT ... ON CONFLICT DO NOTHING on uk_otc_contract_expiry_reminder — the
// race-safe insert-then-skip the platform checklist (§B) prescribes, replacing the
// Java existsBy-then-save check-then-insert.
func (r *Repository) InsertExpiryReminderIfAbsent(ctx context.Context, q Querier, contractID int64, reminderDays int) (bool, error) {
	if q == nil {
		q = r.db
	}
	tag, err := q.Exec(ctx, `
		INSERT INTO otc_contract_expiry_reminders (contract_id, reminder_days)
		VALUES ($1, $2)
		ON CONFLICT ON CONSTRAINT uk_otc_contract_expiry_reminder DO NOTHING`,
		contractID, reminderDays)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
