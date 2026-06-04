package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"banka1/banking-core-service-go/internal/config"
	"banka1/banking-core-service-go/internal/decimal"
	"banka1/banking-core-service-go/internal/uuid"
)

type TransactionService struct {
	db           *sql.DB
	cfg          config.Config
	accounts     *AccountService
	market       *MarketClient
	verification *VerificationService
	rabbit       *RabbitPublisher
}

func NewTransactionService(db *sql.DB, cfg config.Config, accounts *AccountService, market *MarketClient, verification *VerificationService, rabbit *RabbitPublisher) *TransactionService {
	return &TransactionService{db: db, cfg: cfg, accounts: accounts, market: market, verification: verification, rabbit: rabbit}
}

type NewPaymentRequest struct {
	FromAccountNumber     string          `json:"fromAccountNumber"`
	ToAccountNumber       string          `json:"toAccountNumber"`
	Amount                decimal.Decimal `json:"amount"`
	RecipientName         string          `json:"recipientName"`
	PaymentCode           string          `json:"paymentCode"`
	ReferenceNumber       string          `json:"referenceNumber"`
	PaymentPurpose        string          `json:"paymentPurpose"`
	VerificationSessionID int64           `json:"verificationSessionId"`
}

type NewPaymentResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

type TransactionResponse struct {
	OrderNumber       string           `json:"orderNumber"`
	FromAccountNumber string           `json:"fromAccountNumber"`
	ToAccountNumber   string           `json:"toAccountNumber"`
	InitialAmount     decimal.Decimal  `json:"initialAmount"`
	FinalAmount       decimal.Decimal  `json:"finalAmount"`
	RecipientName     string           `json:"recipientName"`
	PaymentCode       string           `json:"paymentCode"`
	ReferenceNumber   string           `json:"referenceNumber,omitempty"`
	PaymentPurpose    string           `json:"paymentPurpose"`
	Status            string           `json:"status"`
	FromCurrency      string           `json:"fromCurrency"`
	ToCurrency        string           `json:"toCurrency"`
	ExchangeRate      *decimal.Decimal `json:"exchangeRate,omitempty"`
	CreatedAt         string           `json:"createdAt"`
}

type PaymentFilter struct {
	AccountNumber    string
	Status           string
	FromDate         *time.Time
	ToDate           *time.Time
	InitialAmountMin *decimal.Decimal
	InitialAmountMax *decimal.Decimal
	FinalAmountMin   *decimal.Decimal
	FinalAmountMax   *decimal.Decimal
}

type PaymentRecipientRequest struct {
	Naziv      string `json:"naziv"`
	BrojRacuna string `json:"brojRacuna"`
}

type PaymentRecipientResponse struct {
	ID         int64  `json:"id"`
	Naziv      string `json:"naziv"`
	BrojRacuna string `json:"brojRacuna"`
}

func (s *TransactionService) NewPayment(ctx context.Context, principal Principal, req NewPaymentRequest) (NewPaymentResponse, error) {
	if err := req.validate(); err != nil {
		return NewPaymentResponse{}, err
	}
	if !s.cfg.SkipVerification {
		ok, err := s.verificationVerified(ctx, req.VerificationSessionID)
		if err != nil || !ok {
			return NewPaymentResponse{}, Conflict("ERR_VERIFICATION_FAILED", "Verifikacija nije uspela", "Verifikacija nije uspela")
		}
	}
	info, err := s.accounts.Info(ctx, req.FromAccountNumber, req.ToAccountNumber)
	if err != nil {
		return NewPaymentResponse{}, err
	}
	if principal.ID != 0 && info.FromVlasnik != principal.ID && !principal.IsPrivileged() {
		return NewPaymentResponse{}, BadRequest("Ne mozes da saljes pare sa tudjeg racuna")
	}
	conversion, err := s.convert(ctx, req.Amount, info.FromCurrencyCode, info.ToCurrencyCode)
	if err != nil {
		return NewPaymentResponse{}, err
	}
	id, orderNumber, err := s.createPayment(ctx, req, info, conversion)
	if err != nil {
		return NewPaymentResponse{}, err
	}

	status := "DENIED"
	var applyErr error
	payment := PaymentRequest{
		FromAccountNumber: req.FromAccountNumber,
		ToAccountNumber:   req.ToAccountNumber,
		FromAmount:        conversion.FromAmount,
		ToAmount:          conversion.ToAmount,
		Commission:        conversion.Commission,
		ClientID:          info.FromVlasnik,
	}
	sameOwner := info.FromVlasnik == info.ToVlasnik
	for attempt := 0; attempt < 3; attempt++ {
		if _, applyErr = s.accounts.ApplyPaymentWithoutRecord(ctx, payment, sameOwner); applyErr == nil {
			status = "COMPLETED"
			break
		}
	}
	if err := s.finishPayment(ctx, id, status); err != nil {
		return NewPaymentResponse{}, err
	}
	s.publishTransactionEmail(ctx, info.FromUsername, info.FromEmail, status)
	if status == "COMPLETED" {
		_ = orderNumber
		return NewPaymentResponse{Message: "Uspesan payment", Status: status}, nil
	}
	return NewPaymentResponse{}, applyErr
}

func (s *TransactionService) FindByClient(ctx context.Context, clientID int64, page, size int) (Page[TransactionResponse], error) {
	return s.queryPayments(ctx, "recipient_client_id = $1 OR sender_client_id = $1", []any{clientID}, page, size, "created_at DESC")
}

func (s *TransactionService) FindBySenderClient(ctx context.Context, clientID int64, page, size int) (Page[TransactionResponse], error) {
	return s.queryPayments(ctx, "sender_client_id = $1", []any{clientID}, page, size, "created_at DESC")
}

func (s *TransactionService) FindByRecipientClient(ctx context.Context, clientID int64, page, size int) (Page[TransactionResponse], error) {
	return s.queryPayments(ctx, "recipient_client_id = $1", []any{clientID}, page, size, "created_at DESC")
}

func (s *TransactionService) FindForAccount(ctx context.Context, principal Principal, accountNumber string, page, size int, employeeAccess bool) (Page[TransactionResponse], error) {
	if !employeeAccess && principal.ID != 0 && !principal.IsPrivileged() {
		details, err := s.accounts.GetByNumber(ctx, accountNumber)
		if err != nil {
			return Page[TransactionResponse]{}, err
		}
		if details.OwnerID != principal.ID {
			return Page[TransactionResponse]{}, BadRequest("Nisi vlasnik racuna")
		}
	}
	return s.queryPayments(ctx, "from_account_number = $1 OR to_account_number = $1", []any{accountNumber}, page, size, "created_at DESC")
}

func (s *TransactionService) FindPayments(ctx context.Context, principal Principal, filter PaymentFilter, page, size int) (Page[TransactionResponse], error) {
	args := []any{}
	clauses := []string{"1 = 1"}
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filter.AccountNumber != "" {
		if principal.ID != 0 && !principal.IsPrivileged() {
			details, err := s.accounts.GetByNumber(ctx, filter.AccountNumber)
			if err != nil {
				return Page[TransactionResponse]{}, err
			}
			if details.OwnerID != principal.ID {
				return Page[TransactionResponse]{}, BadRequest("Nisi vlasnik racuna")
			}
		}
		args = append(args, filter.AccountNumber)
		n := len(args)
		clauses = append(clauses, fmt.Sprintf("(from_account_number = $%d OR to_account_number = $%d)", n, n))
	}
	if filter.Status != "" {
		switch strings.ToUpper(filter.Status) {
		case "IN_PROGRESS", "COMPLETED", "DENIED":
			add("status = $%d", strings.ToUpper(filter.Status))
		default:
			return Page[TransactionResponse]{}, BadRequest("Nevalidan status")
		}
	}
	if filter.FromDate != nil {
		add("created_at >= $%d", *filter.FromDate)
	}
	if filter.ToDate != nil {
		add("created_at <= $%d", *filter.ToDate)
	}
	if filter.InitialAmountMin != nil {
		add("initial_amount >= $%d", *filter.InitialAmountMin)
	}
	if filter.InitialAmountMax != nil {
		add("initial_amount <= $%d", *filter.InitialAmountMax)
	}
	if filter.FinalAmountMin != nil {
		add("final_amount >= $%d", *filter.FinalAmountMin)
	}
	if filter.FinalAmountMax != nil {
		add("final_amount <= $%d", *filter.FinalAmountMax)
	}
	return s.queryPayments(ctx, strings.Join(clauses, " AND "), args, page, size, "created_at DESC")
}

func (s *TransactionService) ListRecipients(ctx context.Context, ownerID int64, page, size int) (Page[PaymentRecipientResponse], error) {
	where := "owner_client_id = $1 AND deleted = false"
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM payment_recipient WHERE "+where, ownerID).Scan(&total); err != nil {
		return Page[PaymentRecipientResponse]{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, naziv, broj_racuna
  FROM payment_recipient
 WHERE `+where+`
 ORDER BY id
 LIMIT $2 OFFSET $3
`, ownerID, size, page*size)
	if err != nil {
		return Page[PaymentRecipientResponse]{}, err
	}
	defer rows.Close()
	items := []PaymentRecipientResponse{}
	for rows.Next() {
		var item PaymentRecipientResponse
		if err := rows.Scan(&item.ID, &item.Naziv, &item.BrojRacuna); err != nil {
			return Page[PaymentRecipientResponse]{}, err
		}
		items = append(items, item)
	}
	return NewPage(items, page, size, total), rows.Err()
}

func (s *TransactionService) CreateRecipient(ctx context.Context, ownerID int64, req PaymentRecipientRequest) (PaymentRecipientResponse, error) {
	if err := req.validate(); err != nil {
		return PaymentRecipientResponse{}, err
	}
	var id int64
	err := s.db.QueryRowContext(ctx, `
INSERT INTO payment_recipient (owner_client_id, naziv, broj_racuna)
VALUES ($1, $2, $3)
RETURNING id
`, ownerID, strings.TrimSpace(req.Naziv), strings.TrimSpace(req.BrojRacuna)).Scan(&id)
	if err != nil {
		if looksUniqueViolation(err) {
			return PaymentRecipientResponse{}, Conflict("ERR_RECIPIENT_NAME_TAKEN", "Naziv je zauzet", "Naziv: %s", req.Naziv)
		}
		return PaymentRecipientResponse{}, err
	}
	return PaymentRecipientResponse{ID: id, Naziv: strings.TrimSpace(req.Naziv), BrojRacuna: strings.TrimSpace(req.BrojRacuna)}, nil
}

func (s *TransactionService) UpdateRecipient(ctx context.Context, ownerID, id int64, req PaymentRecipientRequest) (PaymentRecipientResponse, error) {
	if err := req.validate(); err != nil {
		return PaymentRecipientResponse{}, err
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE payment_recipient
   SET naziv = $1, broj_racuna = $2, updated_at = CURRENT_TIMESTAMP, version = COALESCE(version, 0) + 1
 WHERE id = $3 AND owner_client_id = $4 AND deleted = false
`, strings.TrimSpace(req.Naziv), strings.TrimSpace(req.BrojRacuna), id, ownerID)
	if err != nil {
		if looksUniqueViolation(err) {
			return PaymentRecipientResponse{}, Conflict("ERR_RECIPIENT_NAME_TAKEN", "Naziv je zauzet", "Naziv: %s", req.Naziv)
		}
		return PaymentRecipientResponse{}, err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return PaymentRecipientResponse{}, NotFound("Primalac placanja nije pronadjen")
	}
	return PaymentRecipientResponse{ID: id, Naziv: strings.TrimSpace(req.Naziv), BrojRacuna: strings.TrimSpace(req.BrojRacuna)}, nil
}

func (s *TransactionService) DeleteRecipient(ctx context.Context, ownerID, id int64) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE payment_recipient
   SET deleted = true, updated_at = CURRENT_TIMESTAMP, version = COALESCE(version, 0) + 1
 WHERE id = $1 AND owner_client_id = $2 AND deleted = false
`, id, ownerID)
	if err != nil {
		return err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return NotFound("Primalac placanja nije pronadjen")
	}
	return nil
}

func (s *TransactionService) verificationVerified(ctx context.Context, sessionID int64) (bool, error) {
	if sessionID == 0 {
		return false, BadRequest("Unesi verification session ID")
	}
	if s.verification == nil {
		return s.accounts.verificationVerified(ctx, sessionID)
	}
	status, err := s.verification.Status(ctx, sessionID)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(status.Status, "VERIFIED"), nil
}

func (s *TransactionService) convert(ctx context.Context, amount decimal.Decimal, from, to string) (ConversionResponse, error) {
	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))
	if strings.EqualFold(from, to) {
		return ConversionResponse{
			FromCurrency: from,
			ToCurrency:   to,
			FromAmount:   amount,
			ToAmount:     amount,
			Rate:         decimal.One,
			Commission:   decimal.Zero,
		}, nil
	}
	return s.market.Convert(ctx, amount, from, to)
}

func (s *TransactionService) createPayment(ctx context.Context, req NewPaymentRequest, info InfoResponse, conversion ConversionResponse) (int64, string, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
INSERT INTO payment_table (
    from_account_number, to_account_number, initial_amount, final_amount, commission,
    sender_client_id, recipient_client_id, recipient_name,
    payment_code, reference_number, payment_purpose, status,
    from_currency, to_currency, exchange_rate, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8,
    $9, NULLIF($10, ''), $11, 'IN_PROGRESS',
    $12, $13, $14, NOW(), NOW(), 0
)
RETURNING id
`, req.FromAccountNumber, req.ToAccountNumber, conversion.FromAmount, conversion.ToAmount, conversion.Commission,
		info.FromVlasnik, info.ToVlasnik, strings.TrimSpace(req.RecipientName),
		strings.TrimSpace(req.PaymentCode), strings.TrimSpace(req.ReferenceNumber), strings.TrimSpace(req.PaymentPurpose),
		conversion.FromCurrency, conversion.ToCurrency, conversion.Rate).Scan(&id)
	if err != nil {
		return 0, "", err
	}
	orderNumber := fmt.Sprintf("BANKA1-%d", id)
	_, err = s.db.ExecContext(ctx, "UPDATE payment_table SET order_number = $1 WHERE id = $2", orderNumber, id)
	return id, orderNumber, err
}

func (s *TransactionService) finishPayment(ctx context.Context, id int64, status string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE payment_table
   SET status = $1, updated_at = CURRENT_TIMESTAMP, version = COALESCE(version, 0) + 1
 WHERE id = $2
`, status, id)
	return err
}

func (s *TransactionService) queryPayments(ctx context.Context, where string, args []any, page, size int, orderBy string) (Page[TransactionResponse], error) {
	if size <= 0 {
		size = 10
	}
	if size > 100 {
		size = 100
	}
	if page < 0 {
		page = 0
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM payment_table WHERE "+where, args...).Scan(&total); err != nil {
		return Page[TransactionResponse]{}, err
	}
	args = append(args, size, page*size)
	rows, err := s.db.QueryContext(ctx, paymentSelectSQL+" WHERE "+where+fmt.Sprintf(" ORDER BY %s LIMIT $%d OFFSET $%d", orderBy, len(args)-1, len(args)), args...)
	if err != nil {
		return Page[TransactionResponse]{}, err
	}
	defer rows.Close()
	items := []TransactionResponse{}
	for rows.Next() {
		item, err := scanPayment(rows)
		if err != nil {
			return Page[TransactionResponse]{}, err
		}
		items = append(items, item)
	}
	return NewPage(items, page, size, total), rows.Err()
}

func scanPayment(row rowScanner) (TransactionResponse, error) {
	var out TransactionResponse
	var created time.Time
	var exchange decimal.Decimal
	var hasExchange bool
	err := row.Scan(
		&out.OrderNumber,
		&out.FromAccountNumber,
		&out.ToAccountNumber,
		&out.InitialAmount,
		&out.FinalAmount,
		&out.RecipientName,
		&out.PaymentCode,
		&out.ReferenceNumber,
		&out.PaymentPurpose,
		&out.Status,
		&out.FromCurrency,
		&out.ToCurrency,
		&exchange,
		&hasExchange,
		&created,
	)
	if err != nil {
		return TransactionResponse{}, err
	}
	if hasExchange {
		out.ExchangeRate = &exchange
	}
	out.CreatedAt = created.Format("2006-01-02T15:04:05")
	return out, nil
}

func (r NewPaymentRequest) validate() error {
	if strings.TrimSpace(r.FromAccountNumber) == "" {
		return BadRequest("Unesi racun posiljaoca")
	}
	if strings.TrimSpace(r.ToAccountNumber) == "" {
		return BadRequest("Unesi racun primaoca")
	}
	if r.Amount.Sign() <= 0 {
		return BadRequest("Unesi iznos")
	}
	paymentCode := strings.TrimSpace(r.PaymentCode)
	if len(paymentCode) != 3 || !strings.HasPrefix(paymentCode, "2") || !allDigits(paymentCode) {
		return BadRequest("Sifra mora poceti sa 2 i imati tacno 3 cifre")
	}
	if strings.TrimSpace(r.RecipientName) == "" {
		return BadRequest("Unesi naziv primaoca")
	}
	if strings.TrimSpace(r.PaymentPurpose) == "" {
		return BadRequest("Unesi svrhu placanja")
	}
	if r.VerificationSessionID == 0 {
		return BadRequest("Unesi verification session ID")
	}
	return nil
}

func (r PaymentRecipientRequest) validate() error {
	if strings.TrimSpace(r.Naziv) == "" {
		return BadRequest("Naziv je obavezan")
	}
	if len(strings.TrimSpace(r.Naziv)) > 100 {
		return BadRequest("Naziv ne sme biti duzi od 100 znakova")
	}
	if strings.TrimSpace(r.BrojRacuna) == "" {
		return BadRequest("Broj racuna je obavezan")
	}
	if len(strings.TrimSpace(r.BrojRacuna)) > 50 {
		return BadRequest("Broj racuna ne sme biti duzi od 50 znakova")
	}
	return nil
}

func (s *TransactionService) publishTransactionEmail(ctx context.Context, username, email, status string) {
	emailType := "TRANSACTION_DENIED"
	routingKey := "transaction.denied"
	if status == "COMPLETED" {
		emailType = "TRANSACTION_COMPLETED"
		routingKey = "transaction.completed"
	}
	s.rabbit.PublishJSONBestEffort(ctx, s.cfg.NotificationExchange, routingKey, accountEmailPayload{
		UserEmail: strings.TrimSpace(email),
		Username:  strings.TrimSpace(username),
		EmailType: emailType,
	})
}

func orderSuffix() string {
	id, err := uuid.New()
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return strings.ToUpper(strings.Split(id, "-")[0])
}

func allDigits(value string) bool {
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return value != ""
}

const paymentSelectSQL = `
SELECT COALESCE(order_number, ''),
       from_account_number,
       to_account_number,
       initial_amount,
       final_amount,
       recipient_name,
       payment_code,
       COALESCE(reference_number, ''),
       payment_purpose,
       status,
       from_currency,
       to_currency,
       COALESCE(exchange_rate, 0),
       exchange_rate IS NOT NULL,
       created_at
  FROM payment_table
`
