package service

import (
	"context"
	"database/sql"
	"errors"

	"banka1/banking-core-service-go/internal/decimal"
	"banka1/banking-core-service-go/internal/uuid"
)

type InternalService struct {
	db       *sql.DB
	accounts *AccountService
	market   *MarketClient
}

type ReservationResponse struct {
	ReservationID string `json:"reservationId"`
	Status        string `json:"status"`
}

type TransferResponse struct {
	TransferID string `json:"transferId"`
	Status     string `json:"status"`
}

type ReserveFundsRequest struct {
	OwnerID int64           `json:"ownerId"`
	Amount  decimal.Decimal `json:"amount"`
}

type InternalTransferRequest struct {
	FromAccountNumber string          `json:"fromAccountNumber"`
	ToAccountNumber   string          `json:"toAccountNumber"`
	Amount            decimal.Decimal `json:"amount"`
}

func NewInternalService(db *sql.DB, accounts *AccountService, market *MarketClient) *InternalService {
	return &InternalService{db: db, accounts: accounts, market: market}
}

func (s *InternalService) ReserveFunds(ctx context.Context, ownerID int64, amount decimal.Decimal, correlationID string) (ReservationResponse, error) {
	if amount.Sign() <= 0 {
		return ReservationResponse{}, BadRequest("amount mora biti veci od 0")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReservationResponse{}, err
	}
	defer tx.Rollback()

	account, err := s.accounts.getByOwnerAndCurrency(ctx, tx, ownerID, "RSD", true)
	if err != nil {
		return ReservationResponse{}, BadRequest("Klijent %d nema RSD tekuci racun - rezervacija odbijena.", ownerID)
	}
	if err := s.accounts.DebitTx(ctx, tx, account.AccountNumber, amount, ownerID); err != nil {
		return ReservationResponse{}, err
	}
	reservationID, err := uuid.New()
	if err != nil {
		return ReservationResponse{}, err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO fund_reservations (reservation_id, correlation_id, owner_id, account_number, amount, currency, status)
VALUES ($1::uuid, $2, $3, $4, $5, 'RSD', 'HELD')
`, reservationID, correlationID, ownerID, account.AccountNumber, amount); err != nil {
		return ReservationResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReservationResponse{}, err
	}
	return ReservationResponse{ReservationID: reservationID, Status: "HELD"}, nil
}

func (s *InternalService) ReleaseFunds(ctx context.Context, reservationID, correlationID string) (ReservationResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReservationResponse{}, err
	}
	defer tx.Rollback()

	row, err := s.loadFundReservation(ctx, tx, reservationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReservationResponse{ReservationID: reservationID, Status: "UNKNOWN"}, nil
		}
		return ReservationResponse{}, err
	}
	if row.Status != "HELD" {
		return ReservationResponse{ReservationID: reservationID, Status: row.Status}, nil
	}
	if err := s.accounts.CreditTx(ctx, tx, row.AccountNumber, row.Amount, row.OwnerID); err != nil {
		return ReservationResponse{}, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE fund_reservations
   SET status = 'RELEASED', released_at = NOW()
 WHERE reservation_id = $1::uuid AND status = 'HELD'
`, reservationID); err != nil {
		return ReservationResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReservationResponse{}, err
	}
	return ReservationResponse{ReservationID: reservationID, Status: "RELEASED"}, nil
}

func (s *InternalService) CommitFunds(ctx context.Context, reservationID, correlationID string) (ReservationResponse, error) {
	result, err := s.db.ExecContext(ctx, `
UPDATE fund_reservations
   SET status = 'COMMITTED', committed_at = NOW()
 WHERE reservation_id = $1::uuid AND status = 'HELD'
`, reservationID)
	if err != nil {
		return ReservationResponse{}, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		row, err := s.loadFundReservation(ctx, s.db, reservationID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ReservationResponse{ReservationID: reservationID, Status: "UNKNOWN"}, nil
			}
			return ReservationResponse{}, err
		}
		return ReservationResponse{ReservationID: reservationID, Status: row.Status}, nil
	}
	return ReservationResponse{ReservationID: reservationID, Status: "COMMITTED"}, nil
}

func (s *InternalService) Transfer(ctx context.Context, req InternalTransferRequest, correlationID string) (TransferResponse, error) {
	if req.FromAccountNumber == "" || req.ToAccountNumber == "" {
		return TransferResponse{}, BadRequest("fromAccountNumber i toAccountNumber su obavezni")
	}
	if req.Amount.Sign() <= 0 {
		return TransferResponse{}, BadRequest("amount mora biti veci od 0")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TransferResponse{}, err
	}
	defer tx.Rollback()

	source, err := s.accounts.getByNumber(ctx, tx, req.FromAccountNumber, true)
	if err != nil {
		return TransferResponse{}, err
	}
	target, err := s.accounts.getByNumber(ctx, tx, req.ToAccountNumber, true)
	if err != nil {
		return TransferResponse{}, err
	}

	creditedAmount := req.Amount
	commission := decimal.Zero

	if err := s.accounts.DebitTx(ctx, tx, req.FromAccountNumber, req.Amount, source.OwnerID); err != nil {
		return TransferResponse{}, err
	}
	if source.Currency == target.Currency {
		if err := s.accounts.CreditTx(ctx, tx, req.ToAccountNumber, req.Amount, target.OwnerID); err != nil {
			return TransferResponse{}, err
		}
	} else {
		gross, err := s.market.Convert(ctx, req.Amount, source.Currency, target.Currency)
		if err != nil {
			return TransferResponse{}, err
		}
		commission = gross.Commission
		netSourceAmount := req.Amount.Sub(commission)
		if netSourceAmount.Sign() <= 0 {
			return TransferResponse{}, BadRequest("Iznos transfera ne pokriva FX proviziju.")
		}
		net, err := s.market.ConvertNoCommission(ctx, netSourceAmount, source.Currency, target.Currency)
		if err != nil {
			return TransferResponse{}, err
		}
		creditedAmount = net.ToAmount
		if err := s.accounts.CreditTx(ctx, tx, req.ToAccountNumber, creditedAmount, target.OwnerID); err != nil {
			return TransferResponse{}, err
		}
		bank, err := s.accounts.getByOwnerAndCurrency(ctx, tx, -1, source.Currency, true)
		if err != nil {
			return TransferResponse{}, err
		}
		if err := s.accounts.CreditTx(ctx, tx, bank.AccountNumber, commission, bank.OwnerID); err != nil {
			return TransferResponse{}, err
		}
	}

	transferID, err := uuid.New()
	if err != nil {
		return TransferResponse{}, err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO internal_transfer_log (transfer_id, correlation_id, from_account, to_account, amount, currency, status)
VALUES ($1::uuid, $2, $3, $4, $5, $6, 'COMPLETED')
`, transferID, correlationID, req.FromAccountNumber, req.ToAccountNumber, req.Amount, source.Currency); err != nil {
		return TransferResponse{}, err
	}
	_, _ = tx.ExecContext(ctx, `
INSERT INTO payment_table (
    from_account_number, to_account_number, initial_amount, final_amount, commission,
    sender_client_id, recipient_client_id, recipient_name,
    payment_code, reference_number, payment_purpose, status,
    from_currency, to_currency, order_number, created_at, updated_at, version
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, '289', $9, $10, 'COMPLETED',
          $11, $12, $13, NOW(), NOW(), 0)
ON CONFLICT (order_number) DO NOTHING
`, req.FromAccountNumber, req.ToAccountNumber, req.Amount, creditedAmount, commission,
		source.OwnerID, target.OwnerID, "Account "+req.ToAccountNumber,
		correlationID, "OTC transfer", source.Currency, target.Currency, correlationID)

	if err := tx.Commit(); err != nil {
		return TransferResponse{}, err
	}
	return TransferResponse{TransferID: transferID, Status: "COMPLETED"}, nil
}

func (s *InternalService) ReverseTransfer(ctx context.Context, transferID, correlationID string) (TransferResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TransferResponse{}, err
	}
	defer tx.Rollback()

	var fromAccount, toAccount, status string
	var amount decimal.Decimal
	err = tx.QueryRowContext(ctx, `
SELECT from_account, to_account, amount, status
  FROM internal_transfer_log
 WHERE transfer_id = $1::uuid
 FOR UPDATE
`, transferID).Scan(&fromAccount, &toAccount, &amount, &status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TransferResponse{TransferID: transferID, Status: "UNKNOWN"}, nil
		}
		return TransferResponse{}, err
	}
	if status != "COMPLETED" {
		return TransferResponse{TransferID: transferID, Status: status}, nil
	}
	source, err := s.accounts.getByNumber(ctx, tx, toAccount, true)
	if err != nil {
		return TransferResponse{}, err
	}
	target, err := s.accounts.getByNumber(ctx, tx, fromAccount, true)
	if err != nil {
		return TransferResponse{}, err
	}
	if err := s.accounts.DebitTx(ctx, tx, toAccount, amount, source.OwnerID); err != nil {
		return TransferResponse{}, err
	}
	if err := s.accounts.CreditTx(ctx, tx, fromAccount, amount, target.OwnerID); err != nil {
		return TransferResponse{}, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE internal_transfer_log
   SET status = 'REVERSED', reversed_at = NOW()
 WHERE transfer_id = $1::uuid AND status = 'COMPLETED'
`, transferID); err != nil {
		return TransferResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return TransferResponse{}, err
	}
	return TransferResponse{TransferID: transferID, Status: "REVERSED"}, nil
}

type fundReservationRow struct {
	OwnerID       int64
	AccountNumber string
	Amount        decimal.Decimal
	Status        string
}

func (s *InternalService) loadFundReservation(ctx context.Context, runner sqlRunner, reservationID string) (fundReservationRow, error) {
	var row fundReservationRow
	err := runner.QueryRowContext(ctx, `
SELECT owner_id, account_number, amount, status
  FROM fund_reservations
 WHERE reservation_id = $1::uuid
`, reservationID).Scan(&row.OwnerID, &row.AccountNumber, &row.Amount, &row.Status)
	return row, err
}
