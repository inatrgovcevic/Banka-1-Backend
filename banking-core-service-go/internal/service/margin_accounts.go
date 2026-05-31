package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"banka1/banking-core-service-go/internal/account"
	"banka1/banking-core-service-go/internal/decimal"
)

type MarginAccountService struct {
	db        *sql.DB
	generator account.NumberGenerator
}

type CreateUserMarginAccountRequest struct {
	EmployeeID        *int64          `json:"employeeId"`
	UserID            *int64          `json:"userId"`
	InitialMargin     decimal.Decimal `json:"initialMargin"`
	MaintenanceMargin decimal.Decimal `json:"maintenanceMargin"`
	BankParticipation decimal.Decimal `json:"bankParticipation"`
}

type CreateCompanyMarginAccountRequest struct {
	EmployeeID        *int64          `json:"employeeId"`
	CompanyID         *int64          `json:"companyId"`
	InitialMargin     decimal.Decimal `json:"initialMargin"`
	MaintenanceMargin decimal.Decimal `json:"maintenanceMargin"`
	BankParticipation decimal.Decimal `json:"bankParticipation"`
}

type MarginAccountResponse struct {
	UserID            *int64          `json:"userId,omitempty"`
	CompanyID         *int64          `json:"companyId,omitempty"`
	AccountNumber     string          `json:"accountNumber"`
	InitialMargin     decimal.Decimal `json:"initialMargin"`
	LoanValue         decimal.Decimal `json:"loanValue"`
	MaintenanceMargin decimal.Decimal `json:"maintenanceMargin"`
	BankParticipation decimal.Decimal `json:"bankParticipation"`
	Active            bool            `json:"active"`
}

type marginAccount struct {
	ID                int64
	UserID            sql.NullInt64
	CompanyID         sql.NullInt64
	AccountNumber     string
	InitialMargin     decimal.Decimal
	LoanValue         decimal.Decimal
	MaintenanceMargin decimal.Decimal
	BankParticipation decimal.Decimal
	Active            bool
	CreatedAt         time.Time
}

func NewMarginAccountService(db *sql.DB, generator account.NumberGenerator) *MarginAccountService {
	return &MarginAccountService{db: db, generator: generator}
}

func (s *MarginAccountService) CreateForUser(ctx context.Context, req CreateUserMarginAccountRequest) (MarginAccountResponse, error) {
	if req.EmployeeID == nil || req.UserID == nil {
		return MarginAccountResponse{}, BadRequest("employeeId i userId su obavezni")
	}
	if err := validateMarginCreate(req.InitialMargin, req.MaintenanceMargin, req.BankParticipation); err != nil {
		return MarginAccountResponse{}, err
	}
	userID := *req.UserID
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MarginAccountResponse{}, err
	}
	defer tx.Rollback()

	exists, err := existsByInt64(ctx, tx, "SELECT EXISTS (SELECT 1 FROM user_margin_accounts WHERE user_id = $1)", userID)
	if err != nil {
		return MarginAccountResponse{}, err
	}
	if exists {
		return MarginAccountResponse{}, Conflict("ERR_MARGIN_ACCOUNT_EXISTS", "Marzni racun vec postoji", "Korisnik %d vec ima marzni racun (spec: max jedan po klijentu).", userID)
	}

	accountNumber, err := s.generator.Generate()
	if err != nil {
		return MarginAccountResponse{}, err
	}
	active := req.InitialMargin.Cmp(req.MaintenanceMargin) >= 0
	var id int64
	if err := tx.QueryRowContext(ctx, `
INSERT INTO margin_accounts (
    initial_margin, loan_value, maintenance_margin, bank_participation,
    account_number, currency, active, owner_kind
) VALUES ($1, 0, $2, $3, $4, 'RSD', $5, 'USER')
RETURNING id
`, req.InitialMargin, req.MaintenanceMargin, req.BankParticipation, accountNumber, active).Scan(&id); err != nil {
		return MarginAccountResponse{}, err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO user_margin_accounts (id, user_id) VALUES ($1, $2)", id, userID); err != nil {
		return MarginAccountResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return MarginAccountResponse{}, err
	}
	return s.FindByUserID(ctx, userID)
}

func (s *MarginAccountService) CreateForCompany(ctx context.Context, req CreateCompanyMarginAccountRequest) (MarginAccountResponse, error) {
	if req.EmployeeID == nil || req.CompanyID == nil {
		return MarginAccountResponse{}, BadRequest("employeeId i companyId su obavezni")
	}
	if err := validateMarginCreate(req.InitialMargin, req.MaintenanceMargin, req.BankParticipation); err != nil {
		return MarginAccountResponse{}, err
	}
	companyID := *req.CompanyID
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MarginAccountResponse{}, err
	}
	defer tx.Rollback()

	exists, err := existsByInt64(ctx, tx, "SELECT EXISTS (SELECT 1 FROM company_margin_accounts WHERE company_id = $1)", companyID)
	if err != nil {
		return MarginAccountResponse{}, err
	}
	if exists {
		return MarginAccountResponse{}, Conflict("ERR_MARGIN_ACCOUNT_EXISTS", "Marzni racun vec postoji", "Kompanija %d vec ima marzni racun (spec: max jedan po kompaniji).", companyID)
	}

	accountNumber, err := s.generator.Generate()
	if err != nil {
		return MarginAccountResponse{}, err
	}
	active := req.InitialMargin.Cmp(req.MaintenanceMargin) >= 0
	var id int64
	if err := tx.QueryRowContext(ctx, `
INSERT INTO margin_accounts (
    initial_margin, loan_value, maintenance_margin, bank_participation,
    account_number, currency, active, owner_kind
) VALUES ($1, 0, $2, $3, $4, 'RSD', $5, 'COMPANY')
RETURNING id
`, req.InitialMargin, req.MaintenanceMargin, req.BankParticipation, accountNumber, active).Scan(&id); err != nil {
		return MarginAccountResponse{}, err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO company_margin_accounts (id, company_id) VALUES ($1, $2)", id, companyID); err != nil {
		return MarginAccountResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return MarginAccountResponse{}, err
	}
	return s.FindByCompanyID(ctx, companyID)
}

func (s *MarginAccountService) FindByUserID(ctx context.Context, userID int64) (MarginAccountResponse, error) {
	acc, err := s.findByUserID(ctx, s.db, userID, false)
	if err != nil {
		return MarginAccountResponse{}, err
	}
	return acc.response(), nil
}

func (s *MarginAccountService) FindByCompanyID(ctx context.Context, companyID int64) (MarginAccountResponse, error) {
	acc, err := s.findByCompanyID(ctx, s.db, companyID, false)
	if err != nil {
		return MarginAccountResponse{}, err
	}
	return acc.response(), nil
}

func (s *MarginAccountService) recalcActive(account *marginAccount) {
	account.Active = account.InitialMargin.Cmp(account.MaintenanceMargin) >= 0
}

func (s *MarginAccountService) findByUserID(ctx context.Context, runner sqlRunner, userID int64, forUpdate bool) (marginAccount, error) {
	query := marginAccountSelectSQL + " WHERE u.user_id = $1 AND m.deleted = false"
	if forUpdate {
		query += " FOR UPDATE OF m"
	}
	return scanMarginAccount(runner.QueryRowContext(ctx, query, userID))
}

func (s *MarginAccountService) findByCompanyID(ctx context.Context, runner sqlRunner, companyID int64, forUpdate bool) (marginAccount, error) {
	query := marginAccountSelectSQL + " WHERE cma.company_id = $1 AND m.deleted = false"
	if forUpdate {
		query += " FOR UPDATE OF m"
	}
	return scanMarginAccount(runner.QueryRowContext(ctx, query, companyID))
}

func (s *MarginAccountService) updateAccountTx(ctx context.Context, tx *sql.Tx, account marginAccount) error {
	_, err := tx.ExecContext(ctx, `
UPDATE margin_accounts
   SET initial_margin = $1,
       loan_value = $2,
       active = $3,
       updated_at = NOW(),
       version = version + 1
 WHERE id = $4
`, account.InitialMargin, account.LoanValue, account.Active, account.ID)
	return err
}

func validateMarginCreate(initial, maintenance, participation decimal.Decimal) error {
	if initial.Sign() <= 0 {
		return BadRequest("initialMargin mora biti veci od 0")
	}
	if maintenance.Sign() <= 0 {
		return BadRequest("maintenanceMargin mora biti veci od 0")
	}
	if participation.Sign() < 0 || participation.Cmp(decimal.One) > 0 {
		return BadRequest("bankParticipation mora biti izmedju 0 i 1")
	}
	return nil
}

func (a marginAccount) response() MarginAccountResponse {
	out := MarginAccountResponse{
		AccountNumber:     a.AccountNumber,
		InitialMargin:     a.InitialMargin,
		LoanValue:         a.LoanValue,
		MaintenanceMargin: a.MaintenanceMargin,
		BankParticipation: a.BankParticipation,
		Active:            a.Active,
	}
	if a.UserID.Valid {
		v := a.UserID.Int64
		out.UserID = &v
	}
	if a.CompanyID.Valid {
		v := a.CompanyID.Int64
		out.CompanyID = &v
	}
	return out
}

func scanMarginAccount(row *sql.Row) (marginAccount, error) {
	var out marginAccount
	if err := row.Scan(
		&out.ID,
		&out.AccountNumber,
		&out.InitialMargin,
		&out.LoanValue,
		&out.MaintenanceMargin,
		&out.BankParticipation,
		&out.Active,
		&out.UserID,
		&out.CompanyID,
		&out.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return marginAccount{}, NotFound("Marzni racun ne postoji.")
		}
		return marginAccount{}, err
	}
	return out, nil
}

func existsByInt64(ctx context.Context, runner sqlRunner, query string, value int64) (bool, error) {
	var exists bool
	err := runner.QueryRowContext(ctx, query, value).Scan(&exists)
	return exists, err
}

const marginAccountSelectSQL = `
SELECT m.id,
       m.account_number,
       m.initial_margin,
       m.loan_value,
       m.maintenance_margin,
       m.bank_participation,
       m.active,
       u.user_id,
       cma.company_id,
       m.created_at
  FROM margin_accounts m
  LEFT JOIN user_margin_accounts u ON u.id = m.id
  LEFT JOIN company_margin_accounts cma ON cma.id = m.id
`
