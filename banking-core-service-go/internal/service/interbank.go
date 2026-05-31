package service

import (
	"context"
	"database/sql"
	"errors"

	"banka1/banking-core-service-go/internal/decimal"
	"banka1/banking-core-service-go/internal/uuid"
)

const (
	InterbankHeld      = "HELD"
	InterbankCommitted = "COMMITTED"
	InterbankReleased  = "RELEASED"
)

type InterbankService struct {
	db       *sql.DB
	accounts *AccountService
}

type ReserveMonasRequest struct {
	AccountNum           string          `json:"accountNum"`
	Currency             string          `json:"currency"`
	Amount               decimal.Decimal `json:"amount"`
	TransactionIDRouting int             `json:"transactionIdRouting"`
	TransactionIDLocal   string          `json:"transactionIdLocal"`
}

type ReserveMonasResponse struct {
	ReservationID string `json:"reservationId"`
}

type AccountResolveResponse struct {
	OwnerType        string          `json:"ownerType"`
	OwnerID          int64           `json:"ownerId"`
	Currency         string          `json:"currency"`
	AvailableBalance decimal.Decimal `json:"availableBalance"`
}

type AccountByOwnerResponse struct {
	AccountNumber string `json:"accountNumber"`
}

func NewInterbankService(db *sql.DB, accounts *AccountService) *InterbankService {
	return &InterbankService{db: db, accounts: accounts}
}

func (s *InterbankService) ReserveMonas(ctx context.Context, req ReserveMonasRequest) (string, error) {
	if req.AccountNum == "" || req.Currency == "" || req.TransactionIDLocal == "" {
		return "", BadRequest("accountNum, currency i transactionIdLocal su obavezni")
	}
	if req.Amount.Sign() <= 0 {
		return "", BadRequest("Amount must be positive")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	account, err := s.accounts.getByNumber(ctx, tx, req.AccountNum, true)
	if err != nil {
		return "", err
	}
	if account.Currency != req.Currency {
		return "", BadRequest("Currency mismatch: account=%s requested=%s", account.Currency, req.Currency)
	}
	if account.AvailableBalance.Cmp(req.Amount) < 0 {
		return "", Conflict("ERR_INSUFFICIENT_ASSET", "Nedovoljno raspolozivo stanje", "Insufficient available balance: have=%s need=%s", account.AvailableBalance.String(), req.Amount.String())
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE account_table
   SET raspolozivo_stanje = raspolozivo_stanje - $1,
       version = COALESCE(version, 0) + 1,
       updated_at = now()
 WHERE id = $2
`, req.Amount, account.ID); err != nil {
		return "", err
	}

	reservationID, err := uuid.New()
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO interbank_reservations (
    reservation_id, transaction_id_routing, transaction_id_local,
    account_number, currency, amount, status
) VALUES ($1::uuid, $2, $3, $4, $5, $6, 'HELD')
`, reservationID, req.TransactionIDRouting, req.TransactionIDLocal, req.AccountNum, req.Currency, req.Amount); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return reservationID, nil
}

func (s *InterbankService) CommitReservation(ctx context.Context, reservationID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := s.loadReservationForUpdate(ctx, tx, reservationID)
	if err != nil {
		return err
	}
	if res.Status == InterbankCommitted {
		return nil
	}
	if res.Status != InterbankHeld {
		return Conflict("ERR_INVALID_RESERVATION_STATE", "Neispravno stanje rezervacije", "Cannot commit reservation %s in state %s", reservationID, res.Status)
	}
	account, err := s.accounts.getByNumber(ctx, tx, res.AccountNumber, true)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE account_table
   SET stanje = stanje - $1,
       version = COALESCE(version, 0) + 1,
       updated_at = now()
 WHERE id = $2
`, res.Amount, account.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE interbank_reservations
   SET status = 'COMMITTED', finalized_at = NOW()
 WHERE reservation_id = $1::uuid
`, reservationID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *InterbankService) ReleaseReservation(ctx context.Context, reservationID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := s.loadReservationForUpdate(ctx, tx, reservationID)
	if err != nil {
		return err
	}
	if res.Status == InterbankReleased {
		return nil
	}
	if res.Status == InterbankCommitted {
		return Conflict("ERR_INVALID_RESERVATION_STATE", "Neispravno stanje rezervacije", "Cannot release reservation %s - already COMMITTED", reservationID)
	}
	account, err := s.accounts.getByNumber(ctx, tx, res.AccountNumber, true)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE account_table
   SET raspolozivo_stanje = raspolozivo_stanje + $1,
       version = COALESCE(version, 0) + 1,
       updated_at = now()
 WHERE id = $2
`, res.Amount, account.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE interbank_reservations
   SET status = 'RELEASED', finalized_at = NOW()
 WHERE reservation_id = $1::uuid
`, reservationID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *InterbankService) AccountByOwner(ctx context.Context, ownerID int64, currency string) (AccountByOwnerResponse, error) {
	acc, err := s.accounts.getByOwnerAndCurrency(ctx, s.db, ownerID, currency, false)
	if err != nil {
		return AccountByOwnerResponse{}, err
	}
	return AccountByOwnerResponse{AccountNumber: acc.AccountNumber}, nil
}

func (s *InterbankService) ResolveAccount(ctx context.Context, accountNumber string) (AccountResolveResponse, error) {
	acc, err := s.accounts.getByNumber(ctx, s.db, accountNumber, false)
	if err != nil {
		return AccountResolveResponse{}, err
	}
	return AccountResolveResponse{
		OwnerType:        resolveOwnerType(acc.OwnerID),
		OwnerID:          acc.OwnerID,
		Currency:         acc.Currency,
		AvailableBalance: acc.AvailableBalance,
	}, nil
}

type interbankReservationRow struct {
	AccountNumber string
	Amount        decimal.Decimal
	Status        string
}

func (s *InterbankService) loadReservationForUpdate(ctx context.Context, tx *sql.Tx, reservationID string) (interbankReservationRow, error) {
	var out interbankReservationRow
	err := tx.QueryRowContext(ctx, `
SELECT account_number, amount, status
  FROM interbank_reservations
 WHERE reservation_id = $1::uuid
 FOR UPDATE
`, reservationID).Scan(&out.AccountNumber, &out.Amount, &out.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return interbankReservationRow{}, NotFound("Reservation not found: %s", reservationID)
		}
		return interbankReservationRow{}, err
	}
	return out, nil
}

func resolveOwnerType(ownerID int64) string {
	switch ownerID {
	case -1:
		return "BANK"
	case -2:
		return "STATE"
	case -3:
		return "EXCHANGE"
	default:
		return "CLIENT"
	}
}
