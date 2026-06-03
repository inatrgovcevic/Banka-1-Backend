package order

import (
	"context"
	"errors"
	"strconv"
	"time"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/clients"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// Recurring (standing) orders — Celina 3.6, port of order-service
// RecurringOrderServiceImpl + RecurringOrderScheduler. A recurring order is a
// template (listing, direction, BY_QUANTITY/BY_AMOUNT value, funding account,
// cadence); every 15 minutes the scheduler fires the due ones through the normal
// CreateBuyOrder/CreateSellOrder + ConfirmOrder pipeline, then advances next_run
// by the cadence regardless of the outcome (a failed run never stalls the
// schedule, mirroring the Java finally-advance).

// Recurring cadences (RecurringCadence enum).
const (
	CadenceDaily   = "DAILY"
	CadenceWeekly  = "WEEKLY"
	CadenceMonthly = "MONTHLY"
)

// Recurring modes (RecurringMode enum).
const (
	RecurringModeByQuantity = "BY_QUANTITY"
	RecurringModeByAmount   = "BY_AMOUNT"
)

// advanceCadence mirrors RecurringCadence.advance. MONTHLY reproduces Java's
// plusMonths(1) day-clamping (Jan 31 -> Feb 28), which Go's AddDate would
// normalize forward instead.
func advanceCadence(cadence string, from time.Time) time.Time {
	switch cadence {
	case CadenceDaily:
		return from.AddDate(0, 0, 1)
	case CadenceWeekly:
		return from.AddDate(0, 0, 7)
	default: // MONTHLY
		return plusMonthsClamped(from, 1)
	}
}

// plusMonthsClamped mirrors java.time plusMonths: the day-of-month is clamped to
// the target month's last day instead of overflowing into the next month.
func plusMonthsClamped(t time.Time, months int) time.Time {
	firstOfTarget := time.Date(t.Year(), t.Month(), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location()).AddDate(0, months, 0)
	lastDay := time.Date(firstOfTarget.Year(), firstOfTarget.Month(), 1, 0, 0, 0, 0, t.Location()).AddDate(0, 1, -1).Day()
	day := t.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(firstOfTarget.Year(), firstOfTarget.Month(), day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

// RecurringOrder mirrors a row of recurring_orders (migration 005 / Java
// changeset order:13).
type RecurringOrder struct {
	ID        int64
	UserID    int64
	ListingID int64
	Direction string
	Mode      string
	Value     decimal.Decimal
	AccountID int64
	Cadence   string
	NextRun   time.Time
	Active    bool
	CreatedAt time.Time
}

// --- Repository -------------------------------------------------------------

const recurringColumns = `id, user_id, listing_id, direction, mode, "value"::text,
	account_id, cadence, next_run, active, created_at`

func scanRecurring(row pgx.Row) (*RecurringOrder, error) {
	var (
		r         RecurringOrder
		valueText string
	)
	if err := row.Scan(&r.ID, &r.UserID, &r.ListingID, &r.Direction, &r.Mode, &valueText,
		&r.AccountID, &r.Cadence, &r.NextRun, &r.Active, &r.CreatedAt); err != nil {
		return nil, err
	}
	value, err := decimal.NewFromString(valueText)
	if err != nil {
		return nil, err
	}
	r.Value = value
	return &r, nil
}

func scanRecurrings(rows pgx.Rows) ([]RecurringOrder, error) {
	defer rows.Close()
	out := make([]RecurringOrder, 0)
	for rows.Next() {
		r, err := scanRecurring(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// FindRecurringByUserID mirrors findByUserIdOrderByCreatedAtDesc.
func (r *Repository) FindRecurringByUserID(ctx context.Context, q Querier, userID int64) ([]RecurringOrder, error) {
	rows, err := q.Query(ctx, `SELECT `+recurringColumns+` FROM recurring_orders WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	return scanRecurrings(rows)
}

// FindRecurringByID returns one recurring order, or (nil, nil) when absent.
func (r *Repository) FindRecurringByID(ctx context.Context, q Querier, id int64) (*RecurringOrder, error) {
	ro, err := scanRecurring(q.QueryRow(ctx, `SELECT `+recurringColumns+` FROM recurring_orders WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return ro, err
}

// FindDueRecurring mirrors findDue: active = true AND next_run <= now.
func (r *Repository) FindDueRecurring(ctx context.Context, q Querier, now time.Time) ([]RecurringOrder, error) {
	rows, err := q.Query(ctx, `SELECT `+recurringColumns+` FROM recurring_orders WHERE active = true AND next_run <= $1`, now)
	if err != nil {
		return nil, err
	}
	return scanRecurrings(rows)
}

// InsertRecurring persists a new recurring order, stamping created_at (mirrors
// @CreationTimestamp). The struct is updated in place.
func (r *Repository) InsertRecurring(ctx context.Context, q Querier, ro *RecurringOrder) error {
	return q.QueryRow(ctx, `
		INSERT INTO recurring_orders (user_id, listing_id, direction, mode, "value", account_id, cadence, next_run, active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
		RETURNING id, created_at`,
		ro.UserID, ro.ListingID, ro.Direction, ro.Mode, ro.Value.String(), ro.AccountID, ro.Cadence, ro.NextRun, ro.Active).
		Scan(&ro.ID, &ro.CreatedAt)
}

// SetRecurringActive flips the pause flag (mirrors setActive + save). Returns
// false when no row was updated.
func (r *Repository) SetRecurringActive(ctx context.Context, q Querier, id int64, active bool) (bool, error) {
	cmd, err := q.Exec(ctx, `UPDATE recurring_orders SET active = $2 WHERE id = $1`, id, active)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}

// DeleteRecurring removes a recurring order (mirrors delete in cancel).
func (r *Repository) DeleteRecurring(ctx context.Context, q Querier, id int64) error {
	_, err := q.Exec(ctx, `DELETE FROM recurring_orders WHERE id = $1`, id)
	return err
}

// UpdateRecurringNextRun writes the advanced next_run (mirrors advanceNextRun's
// save of the fresh row).
func (r *Repository) UpdateRecurringNextRun(ctx context.Context, q Querier, id int64, nextRun time.Time) error {
	_, err := q.Exec(ctx, `UPDATE recurring_orders SET next_run = $2 WHERE id = $1`, id, nextRun)
	return err
}

// --- Service ----------------------------------------------------------------

func recurringDto(r *RecurringOrder) api.RecurringOrderDto {
	return api.RecurringOrderDto{
		ID:        r.ID,
		UserID:    r.UserID,
		ListingID: r.ListingID,
		Direction: r.Direction,
		Mode:      r.Mode,
		Value:     r.Value,
		AccountID: r.AccountID,
		Cadence:   r.Cadence,
		NextRun:   api.NewLocalDateTime(r.NextRun),
		Active:    r.Active,
		CreatedAt: api.NewLocalDateTime(r.CreatedAt),
	}
}

// GetRecurringOrders mirrors getForUser.
func (s *Service) GetRecurringOrders(ctx context.Context, userID int64) ([]api.RecurringOrderDto, error) {
	rows, err := s.repo.FindRecurringByUserID(ctx, s.repo.Pool(), userID)
	if err != nil {
		return nil, err
	}
	out := make([]api.RecurringOrderDto, 0, len(rows))
	for i := range rows {
		out = append(out, recurringDto(&rows[i]))
	}
	return out, nil
}

// CreateRecurringOrder mirrors create. The handler has already validated the
// request fields (bean validation), so values arrive parsed and non-nil.
func (s *Service) CreateRecurringOrder(ctx context.Context, userID int64, listingID int64, direction, mode string, value decimal.Decimal, accountID int64, cadence string, nextRun time.Time) (api.RecurringOrderDto, error) {
	ro := &RecurringOrder{
		UserID:    userID,
		ListingID: listingID,
		Direction: direction,
		Mode:      mode,
		Value:     value,
		AccountID: accountID,
		Cadence:   cadence,
		NextRun:   nextRun,
		Active:    true,
	}
	if err := s.repo.InsertRecurring(ctx, s.repo.Pool(), ro); err != nil {
		return api.RecurringOrderDto{}, err
	}
	return recurringDto(ro), nil
}

// PauseRecurringOrder mirrors pause (ownership-checked).
func (s *Service) PauseRecurringOrder(ctx context.Context, userID, id int64) (api.RecurringOrderDto, error) {
	return s.setRecurringActive(ctx, userID, id, false)
}

// ResumeRecurringOrder mirrors resume (ownership-checked).
func (s *Service) ResumeRecurringOrder(ctx context.Context, userID, id int64) (api.RecurringOrderDto, error) {
	return s.setRecurringActive(ctx, userID, id, true)
}

// CancelRecurringOrder mirrors cancel: hard delete of an owned recurring order.
func (s *Service) CancelRecurringOrder(ctx context.Context, userID, id int64) error {
	ro, err := s.getOwnedRecurring(ctx, userID, id)
	if err != nil {
		return err
	}
	return s.repo.DeleteRecurring(ctx, s.repo.Pool(), ro.ID)
}

func (s *Service) setRecurringActive(ctx context.Context, userID, id int64, active bool) (api.RecurringOrderDto, error) {
	ro, err := s.getOwnedRecurring(ctx, userID, id)
	if err != nil {
		return api.RecurringOrderDto{}, err
	}
	if _, err := s.repo.SetRecurringActive(ctx, s.repo.Pool(), ro.ID, active); err != nil {
		return api.RecurringOrderDto{}, err
	}
	ro.Active = active
	return recurringDto(ro), nil
}

// getOwnedRecurring mirrors getOwned: absent or foreign rows surface the same
// 404 (no ownership leak), matching ResourceNotFoundException.
func (s *Service) getOwnedRecurring(ctx context.Context, userID, id int64) (*RecurringOrder, error) {
	ro, err := s.repo.FindRecurringByID(ctx, s.repo.Pool(), id)
	if err != nil {
		return nil, err
	}
	if ro == nil || ro.UserID != userID {
		return nil, api.NewOrderError(404, "Recurring order not found")
	}
	return ro, nil
}

// --- Scheduler / firing -----------------------------------------------------

// RunDueRecurringOrders mirrors RecurringOrderScheduler.runDueRecurringOrders:
// fire every active recurring order whose next_run has passed. One bad order
// cannot stall the batch (runDueRecurringOrder swallows its own failures).
func (s *Service) RunDueRecurringOrders(ctx context.Context) error {
	due, err := s.repo.FindDueRecurring(ctx, s.repo.Pool(), time.Now())
	if err != nil {
		return err
	}
	if len(due) == 0 {
		return nil
	}
	s.logger.Info("firing due recurring orders", "count", len(due))
	for i := range due {
		s.runDueRecurringOrder(ctx, due[i].ID)
	}
	return nil
}

// runDueRecurringOrder mirrors runDueOrder. Not transactional: it delegates to
// CreateBuyOrder/CreateSellOrder + ConfirmOrder (each with its own transaction),
// so a failed order rolls back only that order — and the schedule ALWAYS
// advances (Java's finally), so a permanently failing template cannot re-fire
// every 15 minutes forever.
func (s *Service) runDueRecurringOrder(ctx context.Context, id int64) {
	ro, err := s.repo.FindRecurringByID(ctx, s.repo.Pool(), id)
	if err != nil {
		s.logger.Error("recurring order load failed", "recurringOrderId", id, "error", err)
		return
	}
	if ro == nil || !ro.Active {
		return
	}
	defer s.advanceRecurringNextRun(ctx, ro.ID)

	if err := s.fireRecurringOrder(ctx, ro); err != nil {
		var de *api.DomainError
		if errors.As(err, &de) && de.Status == 409 {
			// BusinessConflictException analog (insufficient funds & co.):
			// notify the owner the run was skipped, schedule still advances.
			s.logger.Info("recurring order skipped", "recurringOrderId", ro.ID, "reason", de.Message)
			s.publishRecurringSkipped(ctx, ro, de.Message)
			return
		}
		s.logger.Warn("recurring order failed to run; advancing schedule anyway", "recurringOrderId", ro.ID, "error", err)
	}
}

// fireRecurringOrder mirrors fire: resolve the integer quantity, build the owner
// principal, then run the normal create + confirm pipeline.
func (s *Service) fireRecurringOrder(ctx context.Context, ro *RecurringOrder) error {
	listing, err := s.market.GetListing(ctx, ro.ListingID)
	if err != nil {
		return err
	}
	quantity := resolveRecurringQuantity(ro, listing)
	if quantity <= 0 {
		return api.NewOrderError(409, "Insufficient funds: amount does not cover one whole unit")
	}
	user := s.buildRecurringOwner(ctx, ro.UserID)

	margin := false
	allOrNone := false
	var created api.OrderResponse
	if ro.Direction == DirectionSell {
		created, err = s.CreateSellOrder(ctx, user, api.CreateSellOrderRequest{
			ListingID: &ro.ListingID,
			Quantity:  &quantity,
			AccountID: &ro.AccountID,
			Margin:    &margin,
			AllOrNone: &allOrNone,
		})
	} else {
		created, err = s.CreateBuyOrder(ctx, user, api.CreateBuyOrderRequest{
			ListingID: &ro.ListingID,
			Quantity:  &quantity,
			AccountID: &ro.AccountID,
			Margin:    &margin,
			AllOrNone: &allOrNone,
		})
	}
	if err != nil {
		return err
	}
	_, err = s.ConfirmOrder(ctx, user, created.ID)
	return err
}

// resolveRecurringQuantity mirrors resolveQuantity: BY_QUANTITY floors the value;
// BY_AMOUNT divides by ask*contractSize (ask missing or <= 0 -> 0 units).
func resolveRecurringQuantity(ro *RecurringOrder, listing *clients.StockListing) int {
	if ro.Mode == RecurringModeByQuantity {
		return int(ro.Value.IntPart())
	}
	unitCost := recurringUnitCost(listing)
	if unitCost.Sign() <= 0 {
		return 0
	}
	// Java: value.divide(unitCost, 0, RoundingMode.DOWN) — truncate, never round up.
	return int(ro.Value.DivRound(unitCost, 16).Truncate(0).IntPart())
}

// recurringUnitCost mirrors unitCost: ask * contractSize (contractSize default 1).
func recurringUnitCost(listing *clients.StockListing) decimal.Decimal {
	if listing == nil || listing.Ask == nil {
		return decimal.Zero
	}
	return listing.Ask.Mul(decimal.NewFromInt(int64(listing.ContractSizeOr(1))))
}

// buildRecurringOwner mirrors buildOwner: actuary rows fire as AGENT; everyone
// else as CLIENT_TRADING with the SECURITIES_TRADE permission.
func (s *Service) buildRecurringOwner(ctx context.Context, userID int64) AuthUser {
	if info, err := s.actuaries.FindByEmployeeID(ctx, userID); err == nil && info != nil {
		return AuthUser{UserID: userID, Roles: []string{"AGENT"}}
	}
	return AuthUser{UserID: userID, Roles: []string{"CLIENT_TRADING"}, Permissions: []string{"SECURITIES_TRADE"}}
}

// advanceRecurringNextRun mirrors advanceNextRun: reload the fresh row (the run
// may have taken time) and advance next_run by the cadence.
func (s *Service) advanceRecurringNextRun(ctx context.Context, id int64) {
	fresh, err := s.repo.FindRecurringByID(ctx, s.repo.Pool(), id)
	if err != nil || fresh == nil {
		if err != nil {
			s.logger.Error("recurring next_run advance reload failed", "recurringOrderId", id, "error", err)
		}
		return
	}
	next := advanceCadence(fresh.Cadence, fresh.NextRun)
	if err := s.repo.UpdateRecurringNextRun(ctx, s.repo.Pool(), id, next); err != nil {
		s.logger.Error("recurring next_run advance failed", "recurringOrderId", id, "error", err)
	}
}

// publishRecurringSkipped mirrors publishSkippedNotification: best-effort
// order.recurring_skipped publish. clientId is set only for client owners (the
// FCM push audience); actuary owners get null (no device push).
func (s *Service) publishRecurringSkipped(ctx context.Context, ro *RecurringOrder, reason string) {
	var clientID *int64
	if info, err := s.actuaries.FindByEmployeeID(ctx, ro.UserID); err == nil && info == nil {
		uid := ro.UserID
		clientID = &uid
	}
	if reason == "" {
		reason = "Nedovoljno sredstava"
	}
	s.notifier.RecurringOrderSkipped(ctx, api.RecurringOrderSkippedNotification{
		ClientID: clientID,
		TemplateVariables: map[string]string{
			"orderId":   strconv.FormatInt(ro.ID, 10),
			"reason":    reason,
			"listingId": strconv.FormatInt(ro.ListingID, 10),
		},
	})
}
