package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"banka1/banking-core-service-go/internal/config"
	"banka1/banking-core-service-go/internal/decimal"
)

type TransferService struct {
	db           *sql.DB
	cfg          config.Config
	accounts     *AccountService
	transactions *TransactionService
	verification *VerificationService
	rabbit       *RabbitPublisher
}

func NewTransferService(db *sql.DB, cfg config.Config, accounts *AccountService, transactions *TransactionService, verification *VerificationService, rabbit *RabbitPublisher) *TransferService {
	return &TransferService{db: db, cfg: cfg, accounts: accounts, transactions: transactions, verification: verification, rabbit: rabbit}
}

type TransferRequest struct {
	FromAccountNumber     string          `json:"fromAccountNumber"`
	ToAccountNumber       string          `json:"toAccountNumber"`
	Amount                decimal.Decimal `json:"amount"`
	VerificationSessionID int64           `json:"verificationSessionId"`
}

type PublicTransferResponse struct {
	OrderNumber       string           `json:"orderNumber"`
	FromAccountNumber string           `json:"fromAccountNumber"`
	ToAccountNumber   string           `json:"toAccountNumber"`
	InitialAmount     decimal.Decimal  `json:"initialAmount"`
	FinalAmount       decimal.Decimal  `json:"finalAmount"`
	ExchangeRate      *decimal.Decimal `json:"exchangeRate,omitempty"`
	Commission        decimal.Decimal  `json:"commission"`
	Timestamp         time.Time        `json:"timestamp"`
}

type transferEmailPayload struct {
	Ime       string `json:"ime,omitempty"`
	Email     string `json:"email,omitempty"`
	EmailType string `json:"emailType,omitempty"`
	Message   string `json:"message,omitempty"`
}

func (s *TransferService) Execute(ctx context.Context, principal Principal, req TransferRequest) (PublicTransferResponse, error) {
	if err := req.validate(); err != nil {
		return PublicTransferResponse{}, err
	}
	if req.FromAccountNumber == req.ToAccountNumber {
		return PublicTransferResponse{}, Conflict("ERR_SAME_ACCOUNT_TRANSFER", "Transfer na isti racun nije dozvoljen", "Ne mozete prebaciti novac na isti racun sa kog saljete.")
	}
	if exists, err := s.transferByVerificationExists(ctx, req.VerificationSessionID); err != nil {
		return PublicTransferResponse{}, err
	} else if exists {
		return PublicTransferResponse{}, Conflict("ERR_TRANSFER_ALREADY_PROCESSED", "Transfer je vec realizovan", "Ovaj transfer je vec realizovan.")
	}
	if !s.cfg.SkipVerification {
		ok, err := s.transactions.verificationVerified(ctx, req.VerificationSessionID)
		if err != nil || !ok {
			return PublicTransferResponse{}, Conflict("ERR_INVALID_VERIFICATION", "Verifikacija nije uspela", "Verifikacija nije uspela")
		}
	}

	from, err := s.accounts.GetByNumber(ctx, req.FromAccountNumber)
	if err != nil {
		return PublicTransferResponse{}, err
	}
	to, err := s.accounts.GetByNumber(ctx, req.ToAccountNumber)
	if err != nil {
		return PublicTransferResponse{}, err
	}
	if principal.ID != 0 && !principal.IsPrivileged() && from.OwnerID != principal.ID {
		return PublicTransferResponse{}, Conflict("ERR_ACCOUNT_OWNERSHIP_MISMATCH", "Neispravno vlasnistvo racuna", "Ne mozete inicirati transfer sa tudjeg racuna.")
	}
	if from.OwnerID != to.OwnerID {
		return PublicTransferResponse{}, Conflict("ERR_ACCOUNT_OWNERSHIP_MISMATCH", "Neispravno vlasnistvo racuna", "Transfer je dozvoljen samo izmedju racuna istog vlasnika.")
	}

	finalAmount := req.Amount
	commission := decimal.Zero
	var exchangeRate *decimal.Decimal
	if !strings.EqualFold(from.Currency, to.Currency) {
		conversion, err := s.transactions.convert(ctx, req.Amount, from.Currency, to.Currency)
		if err != nil {
			return PublicTransferResponse{}, err
		}
		finalAmount = conversion.ToAmount
		commission = conversion.Commission
		exchangeRate = &conversion.Rate
	}

	payment := PaymentRequest{
		FromAccountNumber: req.FromAccountNumber,
		ToAccountNumber:   req.ToAccountNumber,
		FromAmount:        req.Amount,
		ToAmount:          finalAmount,
		Commission:        commission,
		ClientID:          from.OwnerID,
	}
	if _, err := s.accounts.ApplyPaymentWithoutRecord(ctx, payment, true); err != nil {
		return PublicTransferResponse{}, err
	}

	orderNumber := fmt.Sprintf("TRF-%s-%d", orderSuffix(), time.Now().UnixMilli())
	resp, err := s.insertTransfer(ctx, req, orderNumber, from.OwnerID, finalAmount, exchangeRate, commission)
	if err != nil {
		return PublicTransferResponse{}, err
	}
	s.publishTransferCompleted(ctx, from, orderNumber)
	return resp, nil
}

func (s *TransferService) ListByClient(ctx context.Context, principal Principal, clientID int64, page, size int) (Page[PublicTransferResponse], error) {
	if principal.ID != 0 && principal.ID != clientID && !principal.IsPrivileged() {
		return Page[PublicTransferResponse]{}, &Error{Status: 403, Code: "ERR_FORBIDDEN", Title: "Pristup odbijen", Message: "Klijent ne sme da gleda tudje transfere"}
	}
	return s.queryTransfers(ctx, "client_id = $1 AND deleted = false", []any{clientID}, page, size)
}

func (s *TransferService) GetDetails(ctx context.Context, principal Principal, orderNumber string) (PublicTransferResponse, error) {
	resp, clientID, err := s.transferByOrder(ctx, orderNumber)
	if err != nil {
		return PublicTransferResponse{}, err
	}
	if principal.ID != 0 && clientID != principal.ID && !principal.IsPrivileged() {
		return PublicTransferResponse{}, Conflict("ERR_ACCOUNT_OWNERSHIP_MISMATCH", "Neispravno vlasnistvo racuna", "Nemate prava da pregledate ovaj transfer.")
	}
	return resp, nil
}

func (s *TransferService) ListByAccount(ctx context.Context, principal Principal, accountNumber string, page, size int) (Page[PublicTransferResponse], error) {
	account, err := s.accounts.GetByNumber(ctx, accountNumber)
	if err != nil {
		return Page[PublicTransferResponse]{}, err
	}
	if principal.ID != 0 && account.OwnerID != principal.ID && !principal.IsPrivileged() {
		return Page[PublicTransferResponse]{}, Conflict("ERR_ACCOUNT_OWNERSHIP_MISMATCH", "Neispravno vlasnistvo racuna", "Nemate prava da listate transfere tudjeg racuna.")
	}
	return s.queryTransfers(ctx, "(from_account_number = $1 OR to_account_number = $1) AND deleted = false", []any{accountNumber}, page, size)
}

func (s *TransferService) transferByVerificationExists(ctx context.Context, verificationSessionID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM transfers WHERE verification_session_id = $1 AND deleted = false)", fmt.Sprintf("%d", verificationSessionID)).Scan(&exists)
	return exists, err
}

func (s *TransferService) insertTransfer(ctx context.Context, req TransferRequest, orderNumber string, clientID int64, finalAmount decimal.Decimal, exchangeRate *decimal.Decimal, commission decimal.Decimal) (PublicTransferResponse, error) {
	var exchange any
	if exchangeRate != nil {
		exchange = *exchangeRate
	}
	var id int64
	var timestamp time.Time
	err := s.db.QueryRowContext(ctx, `
INSERT INTO transfers (
    order_number, client_id, from_account_number, to_account_number,
    initial_amount, final_amount, exchange_rate, commission,
    timestamp, verification_session_id, version, deleted, created_at, updated_at
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8,
    NOW(), $9, 0, false, NOW(), NOW()
)
RETURNING id, timestamp
`, orderNumber, clientID, req.FromAccountNumber, req.ToAccountNumber,
		req.Amount, finalAmount, exchange, commission, fmt.Sprintf("%d", req.VerificationSessionID)).Scan(&id, &timestamp)
	if err != nil {
		if looksUniqueViolation(err) {
			return PublicTransferResponse{}, Conflict("ERR_TRANSFER_ALREADY_PROCESSED", "Transfer je vec realizovan", "Ovaj transfer je vec realizovan.")
		}
		return PublicTransferResponse{}, err
	}
	_ = id
	return PublicTransferResponse{
		OrderNumber:       orderNumber,
		FromAccountNumber: req.FromAccountNumber,
		ToAccountNumber:   req.ToAccountNumber,
		InitialAmount:     req.Amount,
		FinalAmount:       finalAmount,
		ExchangeRate:      exchangeRate,
		Commission:        commission,
		Timestamp:         timestamp,
	}, nil
}

func (s *TransferService) transferByOrder(ctx context.Context, orderNumber string) (PublicTransferResponse, int64, error) {
	row, clientID, err := s.scanTransfer(s.db.QueryRowContext(ctx, transferSelectSQL+" WHERE order_number = $1 AND deleted = false", orderNumber))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PublicTransferResponse{}, 0, NotFound("Transfer sa brojem %s ne postoji.", orderNumber)
		}
		return PublicTransferResponse{}, 0, err
	}
	return row, clientID, nil
}

func (s *TransferService) queryTransfers(ctx context.Context, where string, args []any, page, size int) (Page[PublicTransferResponse], error) {
	if size <= 0 {
		size = 20
	}
	if page < 0 {
		page = 0
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM transfers WHERE "+where, args...).Scan(&total); err != nil {
		return Page[PublicTransferResponse]{}, err
	}
	args = append(args, size, page*size)
	rows, err := s.db.QueryContext(ctx, transferSelectSQL+" WHERE "+where+fmt.Sprintf(" ORDER BY timestamp DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return Page[PublicTransferResponse]{}, err
	}
	defer rows.Close()
	items := []PublicTransferResponse{}
	for rows.Next() {
		item, _, err := s.scanTransfer(rows)
		if err != nil {
			return Page[PublicTransferResponse]{}, err
		}
		items = append(items, item)
	}
	return NewPage(items, page, size, total), rows.Err()
}

func (s *TransferService) scanTransfer(row rowScanner) (PublicTransferResponse, int64, error) {
	var out PublicTransferResponse
	var clientID int64
	var exchange decimal.Decimal
	var hasExchange bool
	err := row.Scan(
		&out.OrderNumber,
		&clientID,
		&out.FromAccountNumber,
		&out.ToAccountNumber,
		&out.InitialAmount,
		&out.FinalAmount,
		&exchange,
		&hasExchange,
		&out.Commission,
		&out.Timestamp,
	)
	if err != nil {
		return PublicTransferResponse{}, 0, err
	}
	if hasExchange {
		out.ExchangeRate = &exchange
	}
	return out, clientID, nil
}

func (s *TransferService) publishTransferCompleted(ctx context.Context, from AccountDetails, orderNumber string) {
	name := strings.TrimSpace(from.Username)
	if name == "" {
		name = fmt.Sprintf("client_%d", from.OwnerID)
	}
	s.rabbit.PublishJSONBestEffort(ctx, s.cfg.NotificationExchange, "transfer.completed", transferEmailPayload{
		Ime:       name,
		Email:     strings.TrimSpace(from.Email),
		EmailType: "TRANSFER_COMPLETED",
		Message:   "Uspesno ste izvrsili prenos sredstava. Broj naloga: " + orderNumber,
	})
}

func (r TransferRequest) validate() error {
	if strings.TrimSpace(r.FromAccountNumber) == "" {
		return BadRequest("fromAccountNumber je obavezan")
	}
	if strings.TrimSpace(r.ToAccountNumber) == "" {
		return BadRequest("toAccountNumber je obavezan")
	}
	if r.Amount.Sign() <= 0 {
		return BadRequest("Amount must be strictly positive")
	}
	if r.VerificationSessionID == 0 {
		return BadRequest("verificationSessionId je obavezan")
	}
	return nil
}

const transferSelectSQL = `
SELECT order_number,
       client_id,
       from_account_number,
       to_account_number,
       initial_amount,
       final_amount,
       COALESCE(exchange_rate, 0),
       exchange_rate IS NOT NULL,
       commission,
       timestamp
  FROM transfers
`
