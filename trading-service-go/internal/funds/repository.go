package funds

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx so repository methods
// run either standalone or inside a transaction (invest/redeem/saga-complete
// wrap the position + fund + transaction writes in one RunInTx).
type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// ErrNotFound is returned by lookups when no row matches. Mirrors Spring
// Optional.empty() at the service layer.
var ErrNotFound = errors.New("funds: not found")

// Repository centralizes raw-SQL access to all seven fund tables. NUMERIC
// columns are scanned via ::text into decimal.Decimal to preserve scale.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

// Pool exposes the pool for callers that need to start a RunInTx themselves.
func (r *Repository) Pool() *pgxpool.Pool { return r.db }

// =========================== investment_funds =============================

const fundColumns = `id, naziv, opis, minimum_contribution::text, manager_id,
	likvidna_sredstva::text, account_number, dividend_strategy, datum_kreiranja,
	deleted, created_at, version`

func scanFund(row pgx.Row) (*InvestmentFund, error) {
	var (
		f       InvestmentFund
		minText string
		liqText string
	)
	if err := row.Scan(&f.ID, &f.Naziv, &f.Opis, &minText, &f.ManagerID,
		&liqText, &f.AccountNumber, &f.DividendStrategy, &f.DatumKreiranja,
		&f.Deleted, &f.CreatedAt, &f.Version); err != nil {
		return nil, err
	}
	min, err := decimal.NewFromString(minText)
	if err != nil {
		return nil, err
	}
	f.MinimumContribution = min
	liq, err := decimal.NewFromString(liqText)
	if err != nil {
		return nil, err
	}
	f.LikvidnaSredstva = liq
	return &f, nil
}

func scanFunds(rows pgx.Rows) ([]InvestmentFund, error) {
	defer rows.Close()
	out := make([]InvestmentFund, 0)
	for rows.Next() {
		f, err := scanFund(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

// FindFundByID mirrors InvestmentFundRepository.findById.
func (r *Repository) FindFundByID(ctx context.Context, q Querier, id int64) (*InvestmentFund, error) {
	if q == nil {
		q = r.db
	}
	f, err := scanFund(q.QueryRow(ctx,
		`SELECT `+fundColumns+` FROM investment_funds WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return f, err
}

// FindFundByIDForUpdate mirrors @Lock(PESSIMISTIC_WRITE)
// InvestmentFundRepository.findByIdForUpdate.
func (r *Repository) FindFundByIDForUpdate(ctx context.Context, q Querier, id int64) (*InvestmentFund, error) {
	f, err := scanFund(q.QueryRow(ctx,
		`SELECT `+fundColumns+` FROM investment_funds WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return f, err
}

// FundExists mirrors JpaRepository.existsById.
func (r *Repository) FundExists(ctx context.Context, q Querier, id int64) (bool, error) {
	if q == nil {
		q = r.db
	}
	var exists bool
	err := q.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM investment_funds WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}

// FindFundsActive mirrors findByDeletedFalseOrderByNazivAsc.
func (r *Repository) FindFundsActive(ctx context.Context) ([]InvestmentFund, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+fundColumns+` FROM investment_funds WHERE deleted = false ORDER BY naziv ASC`)
	if err != nil {
		return nil, err
	}
	return scanFunds(rows)
}

// FindFundsByManager mirrors findByManagerIdAndDeletedFalse.
func (r *Repository) FindFundsByManager(ctx context.Context, managerID int64) ([]InvestmentFund, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+fundColumns+` FROM investment_funds WHERE manager_id = $1 AND deleted = false ORDER BY naziv ASC`,
		managerID)
	if err != nil {
		return nil, err
	}
	return scanFunds(rows)
}

// InsertFund mirrors the persist branch of save(InvestmentFund).
func (r *Repository) InsertFund(ctx context.Context, q Querier, f *InvestmentFund) error {
	if q == nil {
		q = r.db
	}
	if f.DatumKreiranja.IsZero() {
		f.DatumKreiranja = time.Now().UTC()
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now().UTC()
	}
	return q.QueryRow(ctx, `
		INSERT INTO investment_funds
			(naziv, opis, minimum_contribution, manager_id, likvidna_sredstva,
			 account_number, dividend_strategy, datum_kreiranja, deleted, created_at, version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,false,$9,0)
		RETURNING id, version`,
		f.Naziv, f.Opis, f.MinimumContribution, f.ManagerID, f.LikvidnaSredstva,
		f.AccountNumber, f.DividendStrategy, f.DatumKreiranja, f.CreatedAt,
	).Scan(&f.ID, &f.Version)
}

// UpdateFundLiquidity mirrors the merge branch of save() for the likvidna_sredstva
// + dividend_strategy fields the service mutates. version is bumped to mirror
// JPA @Version. Returns ErrNotFound when the row vanished.
func (r *Repository) UpdateFundLiquidity(ctx context.Context, q Querier, fundID int64, liquidity decimal.Decimal) error {
	if q == nil {
		q = r.db
	}
	tag, err := q.Exec(ctx,
		`UPDATE investment_funds SET likvidna_sredstva = $1, version = version + 1 WHERE id = $2`,
		liquidity, fundID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateFundManager mirrors the merge branch of save() for the manager_id field
// (used by /funds/admin/reassign-manager).
func (r *Repository) UpdateFundManager(ctx context.Context, q Querier, fundID, newManagerID int64) error {
	if q == nil {
		q = r.db
	}
	tag, err := q.Exec(ctx,
		`UPDATE investment_funds SET manager_id = $1, version = version + 1 WHERE id = $2`,
		newManagerID, fundID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// =========================== client_fund_positions ========================

const positionColumns = `id, client_id, fund_id, total_invested::text,
	first_invested_at, last_modified_at, version`

func scanPosition(row pgx.Row) (*ClientFundPosition, error) {
	var (
		p       ClientFundPosition
		invText string
	)
	if err := row.Scan(&p.ID, &p.ClientID, &p.FundID, &invText, &p.FirstInvestedAt,
		&p.LastModifiedAt, &p.Version); err != nil {
		return nil, err
	}
	inv, err := decimal.NewFromString(invText)
	if err != nil {
		return nil, err
	}
	p.TotalInvested = inv
	return &p, nil
}

func scanPositions(rows pgx.Rows) ([]ClientFundPosition, error) {
	defer rows.Close()
	out := make([]ClientFundPosition, 0)
	for rows.Next() {
		p, err := scanPosition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// FindPositionsByClient mirrors ClientFundPositionRepository.findByClientId.
func (r *Repository) FindPositionsByClient(ctx context.Context, clientID int64) ([]ClientFundPosition, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+positionColumns+` FROM client_fund_positions WHERE client_id = $1`, clientID)
	if err != nil {
		return nil, err
	}
	return scanPositions(rows)
}

// FindPositionsByFund mirrors ClientFundPositionRepository.findByFundId.
func (r *Repository) FindPositionsByFund(ctx context.Context, q Querier, fundID int64) ([]ClientFundPosition, error) {
	if q == nil {
		q = r.db
	}
	rows, err := q.Query(ctx,
		`SELECT `+positionColumns+` FROM client_fund_positions WHERE fund_id = $1`, fundID)
	if err != nil {
		return nil, err
	}
	return scanPositions(rows)
}

// FindPosition mirrors findByClientIdAndFundId.
func (r *Repository) FindPosition(ctx context.Context, q Querier, clientID, fundID int64) (*ClientFundPosition, error) {
	if q == nil {
		q = r.db
	}
	p, err := scanPosition(q.QueryRow(ctx,
		`SELECT `+positionColumns+` FROM client_fund_positions WHERE client_id = $1 AND fund_id = $2`,
		clientID, fundID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// FindPositionForUpdate mirrors @Lock(PESSIMISTIC_WRITE)
// findByClientIdAndFundIdForUpdate.
func (r *Repository) FindPositionForUpdate(ctx context.Context, q Querier, clientID, fundID int64) (*ClientFundPosition, error) {
	p, err := scanPosition(q.QueryRow(ctx,
		`SELECT `+positionColumns+` FROM client_fund_positions WHERE client_id = $1 AND fund_id = $2 FOR UPDATE`,
		clientID, fundID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// UpsertPosition mirrors the persist/merge branch of save(ClientFundPosition).
// New row when version is zero/id is zero; otherwise updates total_invested +
// last_modified_at + bumps version. The unique constraint on (client_id,
// fund_id) is the safety net for race conditions.
func (r *Repository) UpsertPosition(ctx context.Context, q Querier, p *ClientFundPosition) error {
	if q == nil {
		q = r.db
	}
	if p.ID == 0 {
		if p.FirstInvestedAt.IsZero() {
			p.FirstInvestedAt = time.Now().UTC()
		}
		return q.QueryRow(ctx, `
			INSERT INTO client_fund_positions
				(client_id, fund_id, total_invested, first_invested_at, last_modified_at, version)
			VALUES ($1,$2,$3,$4,$5,0)
			RETURNING id, version`,
			p.ClientID, p.FundID, p.TotalInvested, p.FirstInvestedAt, p.LastModifiedAt,
		).Scan(&p.ID, &p.Version)
	}
	tag, err := q.Exec(ctx, `
		UPDATE client_fund_positions
		SET total_invested = $1, last_modified_at = $2, version = version + 1
		WHERE id = $3`,
		p.TotalInvested, p.LastModifiedAt, p.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// =========================== client_fund_transactions =====================

const transactionColumns = `id, client_id, fund_id, amount::text, is_inflow,
	status, occurred_at, client_account_number, failure_reason`

func scanTransaction(row pgx.Row) (*ClientFundTransaction, error) {
	var (
		t       ClientFundTransaction
		amtText string
	)
	if err := row.Scan(&t.ID, &t.ClientID, &t.FundID, &amtText, &t.Inflow,
		&t.Status, &t.OccurredAt, &t.ClientAccountNumber, &t.FailureReason); err != nil {
		return nil, err
	}
	amt, err := decimal.NewFromString(amtText)
	if err != nil {
		return nil, err
	}
	t.Amount = amt
	return &t, nil
}

func scanTransactions(rows pgx.Rows) ([]ClientFundTransaction, error) {
	defer rows.Close()
	out := make([]ClientFundTransaction, 0)
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// FindTransactionByID mirrors ClientFundTransactionRepository.findById.
func (r *Repository) FindTransactionByID(ctx context.Context, q Querier, id int64) (*ClientFundTransaction, error) {
	if q == nil {
		q = r.db
	}
	t, err := scanTransaction(q.QueryRow(ctx,
		`SELECT `+transactionColumns+` FROM client_fund_transactions WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

// FindTransactionsByClient mirrors findByClientIdOrderByOccurredAtDesc.
func (r *Repository) FindTransactionsByClient(ctx context.Context, clientID int64) ([]ClientFundTransaction, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+transactionColumns+` FROM client_fund_transactions
		 WHERE client_id = $1 ORDER BY occurred_at DESC`, clientID)
	if err != nil {
		return nil, err
	}
	return scanTransactions(rows)
}

// FindTransactionsByFund mirrors findByFundIdOrderByOccurredAtDesc.
func (r *Repository) FindTransactionsByFund(ctx context.Context, fundID int64) ([]ClientFundTransaction, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+transactionColumns+` FROM client_fund_transactions
		 WHERE fund_id = $1 ORDER BY occurred_at DESC`, fundID)
	if err != nil {
		return nil, err
	}
	return scanTransactions(rows)
}

// InsertTransaction mirrors the persist branch of
// ClientFundTransactionRepository.save. Defaults occurred_at to now().
func (r *Repository) InsertTransaction(ctx context.Context, q Querier, t *ClientFundTransaction) error {
	if q == nil {
		q = r.db
	}
	if t.OccurredAt.IsZero() {
		t.OccurredAt = time.Now().UTC()
	}
	return q.QueryRow(ctx, `
		INSERT INTO client_fund_transactions
			(client_id, fund_id, amount, is_inflow, status, occurred_at, client_account_number, failure_reason)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id`,
		t.ClientID, t.FundID, t.Amount, t.Inflow, t.Status, t.OccurredAt,
		t.ClientAccountNumber, t.FailureReason,
	).Scan(&t.ID)
}

// UpdateTransactionStatus mirrors the merge branch for the
// status/failure_reason transition (PENDING -> COMPLETED/FAILED).
func (r *Repository) UpdateTransactionStatus(ctx context.Context, q Querier, id int64, status string, reason *string) error {
	if q == nil {
		q = r.db
	}
	tag, err := q.Exec(ctx,
		`UPDATE client_fund_transactions SET status = $1, failure_reason = $2 WHERE id = $3`,
		status, reason, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
