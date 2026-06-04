package service

import (
	"context"
	"database/sql"
	"errors"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"banka1/banking-core-service-go/internal/config"
	"banka1/banking-core-service-go/internal/decimal"
)

type sqlRunner interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type AccountService struct {
	db             *sql.DB
	cfg            config.Config
	random         *rand.Rand
	http           *http.Client
	tokenCache     *ServiceTokenCache
	rabbit         *RabbitPublisher
	automaticCards AutomaticCardCreator
}

type AutomaticCardCreator interface {
	CreateAutomaticCard(context.Context, AutoCardCreationRequest) (CardCreationResponse, error)
}

type AccountDetails struct {
	ID               int64           `json:"id"`
	AccountNumber    string          `json:"accountNumber"`
	OwnerID          int64           `json:"ownerId"`
	Currency         string          `json:"currency"`
	AvailableBalance decimal.Decimal `json:"availableBalance"`
	Status           string          `json:"status"`
	AccountType      string          `json:"accountType,omitempty"`
	Email            string          `json:"email,omitempty"`
	Username         string          `json:"username,omitempty"`
}

type InternalAccountByOwnerCurrencyResponse struct {
	ID            int64  `json:"id"`
	AccountNumber string `json:"accountNumber"`
	Currency      string `json:"currency"`
}

type accountBalanceRow struct {
	ID                     int64
	AccountNumber          string
	OwnerID                int64
	Currency               string
	AvailableBalance       decimal.Decimal
	BookedBalance          decimal.Decimal
	Status                 string
	AccountType            string
	Email                  string
	Username               string
	DailyLimit             decimal.Decimal
	MonthlyLimit           decimal.Decimal
	DailySpending          decimal.Decimal
	MonthlySpending        decimal.Decimal
	HasDailyLimit          bool
	HasMonthlyLimit        bool
	ExpiresAt              sql.NullTime
	DailyLimitRemaining    decimal.Decimal
	HasDailyLimitRemaining bool
}

func NewAccountService(db *sql.DB, cfg config.Config, rabbit *RabbitPublisher) *AccountService {
	return &AccountService{
		db:         db,
		cfg:        cfg,
		random:     rand.New(rand.NewSource(time.Now().UnixNano())),
		http:       &http.Client{Timeout: 10 * time.Second},
		tokenCache: NewServiceTokenCache(cfg),
		rabbit:     rabbit,
	}
}

func (s *AccountService) SetAutomaticCardCreator(creator AutomaticCardCreator) {
	s.automaticCards = creator
}

func (s *AccountService) serviceToken() (string, error) {
	if s.tokenCache != nil {
		return s.tokenCache.Token()
	}
	return serviceJWT(s.cfg)
}

func (s *AccountService) GetByNumber(ctx context.Context, accountNumber string) (AccountDetails, error) {
	row, err := s.getByNumber(ctx, s.db, accountNumber, false)
	if err != nil {
		return AccountDetails{}, err
	}
	return row.details(), nil
}

func (s *AccountService) GetByID(ctx context.Context, id int64) (AccountDetails, error) {
	row, err := s.getByID(ctx, s.db, id)
	if err != nil {
		return AccountDetails{}, err
	}
	return row.details(), nil
}

func (s *AccountService) GetBankAccount(ctx context.Context, currency string) (AccountDetails, error) {
	row, err := s.getByOwnerAndCurrency(ctx, s.db, -1, currency, false)
	if err != nil {
		return AccountDetails{}, err
	}
	return row.details(), nil
}

func (s *AccountService) GetStateAccount(ctx context.Context, currency string) (AccountDetails, error) {
	row, err := s.getByOwnerAndCurrency(ctx, s.db, -2, currency, false)
	if err != nil {
		return AccountDetails{}, err
	}
	return row.details(), nil
}

func (s *AccountService) FindDefaultRSDByOwner(ctx context.Context, ownerID int64) (AccountDetails, error) {
	row, err := s.getByOwnerAndCurrency(ctx, s.db, ownerID, "RSD", false)
	if err != nil {
		return AccountDetails{}, err
	}
	return row.details(), nil
}

func (s *AccountService) FindByOwnerAndCurrency(ctx context.Context, ownerID int64, currency string) (InternalAccountByOwnerCurrencyResponse, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		return InternalAccountByOwnerCurrencyResponse{}, BadRequest("currencyCode je obavezan")
	}
	row, err := s.getByOwnerAndCurrency(ctx, s.db, ownerID, currency, false)
	if err != nil {
		return InternalAccountByOwnerCurrencyResponse{}, err
	}
	return InternalAccountByOwnerCurrencyResponse{
		ID:            row.ID,
		AccountNumber: row.AccountNumber,
		Currency:      row.Currency,
	}, nil
}

func (s *AccountService) FindClientAccounts(ctx context.Context, ownerID int64) ([]AccountDetails, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT a.id, a.broj_racuna, a.vlasnik, COALESCE(c.oznaka, ''), a.raspolozivo_stanje,
       a.status, COALESCE(a.account_ownership_type, a.account_concrete, a.account_type, ''),
       COALESCE(a.email, ''), COALESCE(a.username, '')
  FROM account_table a
 LEFT JOIN currency_table c ON c.id = a.currency_id
 WHERE a.vlasnik = $1
   AND a.deleted = false
 ORDER BY a.id
`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AccountDetails
	for rows.Next() {
		var item AccountDetails
		if err := rows.Scan(&item.ID, &item.AccountNumber, &item.OwnerID, &item.Currency,
			&item.AvailableBalance, &item.Status, &item.AccountType, &item.Email, &item.Username); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *AccountService) Debit(ctx context.Context, accountNumber string, amount decimal.Decimal, clientID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.DebitTx(ctx, tx, accountNumber, amount, clientID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *AccountService) Credit(ctx context.Context, accountNumber string, amount decimal.Decimal, clientID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.CreditTx(ctx, tx, accountNumber, amount, clientID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *AccountService) DebitTx(ctx context.Context, tx *sql.Tx, accountNumber string, amount decimal.Decimal, clientID int64) error {
	if amount.Sign() <= 0 {
		return BadRequest("Iznos mora biti veci od 0")
	}
	account, err := s.getByNumber(ctx, tx, accountNumber, true)
	if err != nil {
		return err
	}
	if err := account.validateMutable(accountNumber, clientID); err != nil {
		return err
	}
	if account.AvailableBalance.Cmp(amount) < 0 {
		return Conflict("ERR_INSUFFICIENT_FUNDS", "Nedovoljno sredstava", "Nedovoljno sredstava na racunu %s", accountNumber)
	}
	if account.HasDailyLimit && account.DailySpending.Add(amount).Cmp(account.DailyLimit) > 0 {
		return Conflict("ERR_DAILY_LIMIT_EXCEEDED", "Dnevni limit je prekoracen", "Dnevni limit je prekoracen")
	}
	if account.HasDailyLimitRemaining && account.DailyLimitRemaining.Cmp(amount) < 0 {
		return Conflict("ERR_DAILY_LIMIT_EXCEEDED", "Dnevni limit je prekoracen", "Dnevni limit je prekoracen")
	}
	if account.HasMonthlyLimit && account.MonthlySpending.Add(amount).Cmp(account.MonthlyLimit) > 0 {
		return Conflict("ERR_MONTHLY_LIMIT_EXCEEDED", "Mesecni limit je prekoracen", "Mesecni limit je prekoracen")
	}

	_, err = tx.ExecContext(ctx, `
UPDATE account_table
   SET stanje = stanje - $1,
       raspolozivo_stanje = raspolozivo_stanje - $1,
       dnevna_potrosnja = dnevna_potrosnja + $1,
       mesecna_potrosnja = mesecna_potrosnja + $1,
       daily_limit_remaining = CASE
           WHEN daily_limit_remaining IS NOT NULL THEN daily_limit_remaining - $1
           ELSE NULL
       END,
       version = COALESCE(version, 0) + 1,
       updated_at = now()
 WHERE id = $2
`, amount, account.ID)
	return err
}

func (s *AccountService) CreditTx(ctx context.Context, tx *sql.Tx, accountNumber string, amount decimal.Decimal, clientID int64) error {
	if amount.Sign() <= 0 {
		return BadRequest("Iznos mora biti veci od 0")
	}
	account, err := s.getByNumber(ctx, tx, accountNumber, true)
	if err != nil {
		return err
	}
	if err := account.validateMutable(accountNumber, clientID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
UPDATE account_table
   SET stanje = stanje + $1,
       raspolozivo_stanje = raspolozivo_stanje + $1,
       version = COALESCE(version, 0) + 1,
       updated_at = now()
 WHERE id = $2
`, amount, account.ID)
	return err
}

func (s *AccountService) getByID(ctx context.Context, runner sqlRunner, id int64) (accountBalanceRow, error) {
	return scanAccount(runner.QueryRowContext(ctx, accountSelectSQL+" WHERE a.id = $1 AND a.deleted = false", id))
}

func (s *AccountService) getByNumber(ctx context.Context, runner sqlRunner, accountNumber string, forUpdate bool) (accountBalanceRow, error) {
	query := accountSelectSQL + " WHERE a.broj_racuna = $1 AND a.deleted = false"
	if forUpdate {
		query += " FOR UPDATE OF a"
	}
	return scanAccount(runner.QueryRowContext(ctx, query, accountNumber))
}

func (s *AccountService) getByOwnerAndCurrency(ctx context.Context, runner sqlRunner, ownerID int64, currency string, forUpdate bool) (accountBalanceRow, error) {
	query := accountSelectSQL + " WHERE a.vlasnik = $1 AND c.oznaka = $2 AND a.deleted = false ORDER BY a.id LIMIT 1"
	if forUpdate {
		query += " FOR UPDATE OF a"
	}
	return scanAccount(runner.QueryRowContext(ctx, query, ownerID, currency))
}

func (r accountBalanceRow) validateMutable(accountNumber string, clientID int64) error {
	if r.Status == "INACTIVE" {
		return BadRequest("Racun je neaktivan:%s", accountNumber)
	}
	if r.ExpiresAt.Valid && r.ExpiresAt.Time.Before(time.Now()) {
		return BadRequest("Racun je istekao:%s", accountNumber)
	}
	if r.OwnerID != -1 && r.OwnerID != clientID {
		return BadRequest("Nisi vlasnik racuna")
	}
	return nil
}

func (r accountBalanceRow) details() AccountDetails {
	return AccountDetails{
		ID:               r.ID,
		AccountNumber:    r.AccountNumber,
		OwnerID:          r.OwnerID,
		Currency:         r.Currency,
		AvailableBalance: r.AvailableBalance,
		Status:           r.Status,
		AccountType:      r.AccountType,
		Email:            r.Email,
		Username:         r.Username,
	}
}

func scanAccount(row *sql.Row) (accountBalanceRow, error) {
	var out accountBalanceRow
	if err := row.Scan(
		&out.ID,
		&out.AccountNumber,
		&out.OwnerID,
		&out.Currency,
		&out.AvailableBalance,
		&out.BookedBalance,
		&out.Status,
		&out.AccountType,
		&out.Email,
		&out.Username,
		&out.DailyLimit,
		&out.MonthlyLimit,
		&out.DailySpending,
		&out.MonthlySpending,
		&out.HasDailyLimit,
		&out.HasMonthlyLimit,
		&out.ExpiresAt,
		&out.DailyLimitRemaining,
		&out.HasDailyLimitRemaining,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return accountBalanceRow{}, NotFound("Ne postoji racun")
		}
		return accountBalanceRow{}, err
	}
	return out, nil
}

const accountSelectSQL = `
SELECT a.id,
       a.broj_racuna,
       a.vlasnik,
       COALESCE(c.oznaka, ''),
       a.raspolozivo_stanje,
       a.stanje,
       a.status,
       COALESCE(a.account_ownership_type, a.account_concrete, a.account_type, ''),
       COALESCE(a.email, ''),
       COALESCE(a.username, ''),
       COALESCE(a.dnevni_limit, 0),
       COALESCE(a.mesecni_limit, 0),
       COALESCE(a.dnevna_potrosnja, 0),
       COALESCE(a.mesecna_potrosnja, 0),
       a.dnevni_limit IS NOT NULL,
       a.mesecni_limit IS NOT NULL,
       a.datum_isteka,
       COALESCE(a.daily_limit_remaining, 0),
       a.daily_limit_remaining IS NOT NULL
  FROM account_table a
  LEFT JOIN currency_table c ON c.id = a.currency_id
`
