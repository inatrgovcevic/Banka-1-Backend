package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"banka1/banking-core-service-go/internal/config"
	"banka1/banking-core-service-go/internal/decimal"
)

type MarginTransactionService struct {
	db             *sql.DB
	cfg            config.Config
	accounts       *AccountService
	marginAccounts *MarginAccountService
	counter        int64
}

type StockMarginTransactionRequest struct {
	UserID    *int64          `json:"userId"`
	CompanyID *int64          `json:"companyId"`
	Amount    decimal.Decimal `json:"amount"`
}

type MarginTransferRequest struct {
	Amount            decimal.Decimal `json:"amount"`
	FromAccountNumber string          `json:"fromAccountNumber"`
}

type MarginTransactionHistoryItem struct {
	ID              int64           `json:"id"`
	AccountNumber   string          `json:"accountNumber"`
	Amount          decimal.Decimal `json:"amount"`
	TransactionType string          `json:"transactionType"`
	OccurredAt      string          `json:"occurredAt"`
	Description     string          `json:"description,omitempty"`
}

func NewMarginTransactionService(db *sql.DB, cfg config.Config, accounts *AccountService, marginAccounts *MarginAccountService) *MarginTransactionService {
	return &MarginTransactionService{db: db, cfg: cfg, accounts: accounts, marginAccounts: marginAccounts}
}

func (s *MarginTransactionService) BuyOnMargin(ctx context.Context, req StockMarginTransactionRequest) error {
	if err := validateStockMarginRequest(req); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	acc, err := s.lockMarginAccountForOwner(ctx, tx, req.UserID, req.CompanyID)
	if err != nil {
		return err
	}
	if !acc.Active {
		return Conflict("ERR_MARGIN_BLOCKED", "Marzni racun je blokiran", "Marzni racun %s je blokiran (initialMargin ispod maintenanceMargin).", acc.AccountNumber)
	}

	bankPart := req.Amount.Mul(acc.BankParticipation).Round(2)
	clientPart := req.Amount.Sub(bankPart)
	if acc.InitialMargin.Cmp(clientPart) < 0 {
		return Conflict("ERR_INSUFFICIENT_FUNDS", "Nedovoljno sredstava", "Nedovoljno sredstava na initialMargin (potrebno: %s, raspolozivo: %s).", clientPart.String(), acc.InitialMargin.String())
	}

	acc.InitialMargin = acc.InitialMargin.Sub(clientPart)
	acc.LoanValue = acc.LoanValue.Add(bankPart)
	s.marginAccounts.recalcActive(&acc)

	txID, err := s.sendToExchange(ctx, tx, bankPart)
	if err != nil {
		return err
	}
	if err := s.marginAccounts.updateAccountTx(ctx, tx, acc); err != nil {
		return err
	}
	if err := s.recordMarginTx(ctx, tx, acc, req.Amount.Neg(), "STOCK_BUY",
		fmt.Sprintf("Buy on margin: bank=%s client=%s bankTxId=%d", bankPart.Fixed(2), clientPart.Fixed(2), txID)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MarginTransactionService) SellOnMargin(ctx context.Context, req StockMarginTransactionRequest) error {
	if err := validateStockMarginRequest(req); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	acc, err := s.lockMarginAccountForOwner(ctx, tx, req.UserID, req.CompanyID)
	if err != nil {
		return err
	}
	if !acc.Active {
		return Conflict("ERR_MARGIN_BLOCKED", "Marzni racun je blokiran", "Marzni racun %s je blokiran.", acc.AccountNumber)
	}

	bankPart := req.Amount.Mul(acc.BankParticipation).Round(2)
	clientPart := req.Amount.Sub(bankPart)
	loanReduction := acc.LoanValue.Min(bankPart)
	clientCredit := clientPart.Add(bankPart.Sub(loanReduction))

	acc.LoanValue = acc.LoanValue.Sub(loanReduction)
	acc.InitialMargin = acc.InitialMargin.Add(clientCredit)
	s.marginAccounts.recalcActive(&acc)

	txIDText := "null"
	if loanReduction.Sign() > 0 {
		txID, err := s.receiveFromExchange(ctx, tx, loanReduction)
		if err != nil {
			return err
		}
		txIDText = fmt.Sprintf("%d", txID)
	}
	if err := s.marginAccounts.updateAccountTx(ctx, tx, acc); err != nil {
		return err
	}
	if err := s.recordMarginTx(ctx, tx, acc, req.Amount, "STOCK_SELL",
		fmt.Sprintf("Sell on margin: loan-reduction=%s client-credit=%s bankTxId=%s", loanReduction.Fixed(2), clientCredit.Fixed(2), txIDText)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MarginTransactionService) AddToMarginForUser(ctx context.Context, userID int64, req MarginTransferRequest) error {
	if req.Amount.Sign() <= 0 {
		return BadRequest("amount mora biti veci od 0")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	acc, err := s.marginAccounts.findByUserID(ctx, tx, userID, true)
	if err != nil {
		return err
	}
	checking, err := s.resolveCheckingAccount(ctx, tx, userID, req.FromAccountNumber)
	if err != nil {
		return err
	}
	if err := s.accounts.DebitTx(ctx, tx, checking, req.Amount, userID); err != nil {
		return err
	}
	acc.InitialMargin = acc.InitialMargin.Add(req.Amount)
	s.marginAccounts.recalcActive(&acc)
	if err := s.marginAccounts.updateAccountTx(ctx, tx, acc); err != nil {
		return err
	}
	if err := s.recordMarginTx(ctx, tx, acc, req.Amount, "ADD_TO_MARGIN", "Transfer from checking "+checking+" to margin"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MarginTransactionService) WithdrawFromMarginForUser(ctx context.Context, userID int64, req MarginTransferRequest) error {
	if req.Amount.Sign() <= 0 {
		return BadRequest("amount mora biti veci od 0")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	acc, err := s.marginAccounts.findByUserID(ctx, tx, userID, true)
	if err != nil {
		return err
	}
	if !acc.Active {
		return Conflict("ERR_MARGIN_BLOCKED", "Racun je blokiran", "Racun je blokiran; isplate nisu moguce.")
	}
	newInitial := acc.InitialMargin.Sub(req.Amount)
	if newInitial.Cmp(acc.MaintenanceMargin) < 0 {
		return Conflict("ERR_MARGIN_MAINTENANCE", "Maintenance margin", "Isplata bi spustila initialMargin ispod maintenanceMargin (novo=%s, min=%s).", newInitial.String(), acc.MaintenanceMargin.String())
	}

	checking, err := s.resolveCheckingAccount(ctx, tx, userID, req.FromAccountNumber)
	if err != nil {
		return err
	}
	acc.InitialMargin = newInitial
	s.marginAccounts.recalcActive(&acc)
	if err := s.marginAccounts.updateAccountTx(ctx, tx, acc); err != nil {
		return err
	}
	if err := s.accounts.CreditTx(ctx, tx, checking, req.Amount, userID); err != nil {
		return err
	}
	if err := s.recordMarginTx(ctx, tx, acc, req.Amount.Neg(), "WITHDRAW_FROM_MARGIN", "Transfer from margin to checking "+checking); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MarginTransactionService) History(ctx context.Context, accountNumber string) ([]MarginTransactionHistoryItem, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, account_number, amount, transaction_type, occurred_at, COALESCE(description, '')
  FROM margin_transactions
 WHERE account_number = $1
 ORDER BY occurred_at DESC
`, accountNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MarginTransactionHistoryItem
	for rows.Next() {
		var item MarginTransactionHistoryItem
		var occurredAt time.Time
		if err := rows.Scan(&item.ID, &item.AccountNumber, &item.Amount, &item.TransactionType, &occurredAt, &item.Description); err != nil {
			return nil, err
		}
		item.OccurredAt = formatDateTime(sql.NullTime{Time: occurredAt, Valid: true})
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *MarginTransactionService) lockMarginAccountForOwner(ctx context.Context, tx *sql.Tx, userID, companyID *int64) (marginAccount, error) {
	if userID != nil {
		return s.marginAccounts.findByUserID(ctx, tx, *userID, true)
	}
	return s.marginAccounts.findByCompanyID(ctx, tx, *companyID, true)
}

func (s *MarginTransactionService) resolveCheckingAccount(ctx context.Context, tx *sql.Tx, userID int64, requested string) (string, error) {
	if requested != "" {
		return requested, nil
	}
	acc, err := s.accounts.getByOwnerAndCurrency(ctx, tx, userID, "RSD", false)
	if err != nil {
		return "", BadRequest("Klijent userId=%d nema RSD tekuci racun za margin transfer.", userID)
	}
	return acc.AccountNumber, nil
}

func (s *MarginTransactionService) sendToExchange(ctx context.Context, tx *sql.Tx, amount decimal.Decimal) (int64, error) {
	if err := s.accounts.DebitTx(ctx, tx, s.cfg.BankAccountNumber, amount, s.cfg.BankClientID); err != nil {
		return 0, err
	}
	if err := s.accounts.CreditTx(ctx, tx, s.cfg.ExchangeAccountNumber, amount, s.cfg.ExchangeClientID); err != nil {
		return 0, err
	}
	s.counter++
	return s.counter, nil
}

func (s *MarginTransactionService) receiveFromExchange(ctx context.Context, tx *sql.Tx, amount decimal.Decimal) (int64, error) {
	if err := s.accounts.DebitTx(ctx, tx, s.cfg.ExchangeAccountNumber, amount, s.cfg.ExchangeClientID); err != nil {
		return 0, err
	}
	if err := s.accounts.CreditTx(ctx, tx, s.cfg.BankAccountNumber, amount, s.cfg.BankClientID); err != nil {
		return 0, err
	}
	s.counter++
	return s.counter, nil
}

func (s *MarginTransactionService) recordMarginTx(ctx context.Context, tx *sql.Tx, account marginAccount, amount decimal.Decimal, txType, description string) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO margin_transactions (
    account_number, amount, transaction_type, loan_value_after,
    initial_margin_after, description, occurred_at
) VALUES ($1, $2, $3, $4, $5, $6, NOW())
`, account.AccountNumber, amount, txType, account.LoanValue, account.InitialMargin, description)
	return err
}

func validateStockMarginRequest(req StockMarginTransactionRequest) error {
	if (req.UserID == nil) == (req.CompanyID == nil) {
		return BadRequest("Tacno jedan od userId/companyId mora biti postavljen (spec: jedan ce sigurno biti null).")
	}
	if req.Amount.Sign() <= 0 {
		return BadRequest("amount mora biti veci od 0")
	}
	return nil
}
