package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"banka1/banking-core-service-go/internal/card"
	"banka1/banking-core-service-go/internal/config"
	"banka1/banking-core-service-go/internal/decimal"
)

var cardBrands = []string{"VISA", "MASTERCARD", "DINACARD", "AMEX"}

type CardService struct {
	db       *sql.DB
	cfg      config.Config
	accounts *AccountService
	rabbit   *RabbitPublisher
}

func NewCardService(db *sql.DB, cfg config.Config, accounts *AccountService, rabbit *RabbitPublisher) *CardService {
	return &CardService{db: db, cfg: cfg, accounts: accounts, rabbit: rabbit}
}

type AutoCardCreationRequest struct {
	ClientID              int64  `json:"clientId"`
	AccountNumber         string `json:"accountNumber"`
	AccountCurrency       string `json:"accountCurrency,omitempty"`
	AccountCategory       string `json:"accountCategory,omitempty"`
	AccountType           string `json:"accountType,omitempty"`
	AccountSubtype        string `json:"accountSubtype,omitempty"`
	OwnerFirstName        string `json:"ownerFirstName,omitempty"`
	OwnerLastName         string `json:"ownerLastName,omitempty"`
	OwnerEmail            string `json:"ownerEmail,omitempty"`
	OwnerUsername         string `json:"ownerUsername,omitempty"`
	AccountExpirationDate string `json:"accountExpirationDate,omitempty"`
}

type ClientCardRequest struct {
	AccountNumber  string          `json:"accountNumber"`
	CardBrand      string          `json:"cardBrand"`
	CardLimit      decimal.Decimal `json:"cardLimit"`
	VerificationID int64           `json:"verificationId"`
}

type BusinessCardRequest struct {
	AccountNumber      string                   `json:"accountNumber"`
	RecipientType      string                   `json:"recipientType"`
	AuthorizedPersonID *int64                   `json:"authorizedPersonId"`
	AuthorizedPerson   *AuthorizedPersonRequest `json:"authorizedPerson"`
	CardBrand          string                   `json:"cardBrand"`
	CardLimit          decimal.Decimal          `json:"cardLimit"`
	VerificationID     int64                    `json:"verificationId"`
}

type AuthorizedPersonRequest struct {
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	DateOfBirth string `json:"dateOfBirth"`
	Gender      string `json:"gender"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Address     string `json:"address"`
}

type UpdateCardLimitRequest struct {
	CardLimit decimal.Decimal `json:"cardLimit"`
}

type CardCreationResponse struct {
	CardID         int64  `json:"cardId"`
	CardNumber     string `json:"cardNumber"`
	PlainCVV       string `json:"plainCvv"`
	ExpirationDate string `json:"expirationDate"`
	CardName       string `json:"cardName"`
}

type CardRequestResponse struct {
	Status                string                `json:"status"`
	Message               string                `json:"message"`
	VerificationRequestID *int64                `json:"verificationRequestId"`
	CreatedCard           *CardCreationResponse `json:"createdCard"`
}

type CardSummaryResponse struct {
	ID               int64  `json:"id"`
	MaskedCardNumber string `json:"maskedCardNumber"`
	AccountNumber    string `json:"accountNumber"`
}

type CardDetailResponse struct {
	ID             int64           `json:"id"`
	CardNumber     string          `json:"cardNumber"`
	CardType       string          `json:"cardType"`
	CardName       string          `json:"cardName"`
	CreationDate   string          `json:"creationDate"`
	ExpirationDate string          `json:"expirationDate"`
	AccountNumber  string          `json:"accountNumber"`
	CardLimit      decimal.Decimal `json:"cardLimit"`
	Status         string          `json:"status"`
}

type CardAdminSummaryResponse struct {
	ID            int64           `json:"id"`
	CardNumber    string          `json:"cardNumber"`
	Brand         string          `json:"brand"`
	Status        string          `json:"status"`
	AccountNumber string          `json:"accountNumber"`
	ClientID      int64           `json:"clientId"`
	CardLimit     decimal.Decimal `json:"cardLimit"`
}

type CardInternalSummaryResponse struct {
	ID            int64  `json:"id"`
	CardNumber    string `json:"cardNumber"`
	CardType      string `json:"cardType"`
	Status        string `json:"status"`
	ExpiryDate    string `json:"expiryDate"`
	AccountNumber string `json:"accountNumber"`
}

type cardRow struct {
	ID                 int64
	CardNumber         string
	CardType           string
	CardName           string
	CreationDate       time.Time
	ExpirationDate     time.Time
	AccountNumber      string
	ClientID           int64
	AuthorizedPersonID sql.NullInt64
	CVV                string
	CardLimit          decimal.Decimal
	Status             string
}

type cardCommand struct {
	AccountNumber      string
	CardBrand          string
	CardLimit          decimal.Decimal
	ClientID           int64
	AuthorizedPersonID sql.NullInt64
}

func (s *CardService) CreateAutomaticCard(ctx context.Context, req AutoCardCreationRequest) (CardCreationResponse, error) {
	if strings.TrimSpace(req.AccountNumber) == "" {
		return CardCreationResponse{}, cardBusinessError(400, "ERR_CARD_001", "Invalid account number", "Account number must not be blank.")
	}
	if req.ClientID == 0 {
		return CardCreationResponse{}, cardBusinessError(400, "ERR_CARD_009", "Invalid client ID", "Client ID must be provided.")
	}
	limit := decimal.MustParse(s.cfg.CardDefaultLimit)
	result, err := s.createCard(ctx, cardCommand{
		AccountNumber: strings.TrimSpace(req.AccountNumber),
		CardBrand:     randomCardBrand(),
		CardLimit:     limit,
		ClientID:      req.ClientID,
	})
	if err != nil {
		return CardCreationResponse{}, err
	}
	return result, nil
}

func (s *CardService) RequestPersonalCard(ctx context.Context, principal Principal, req ClientCardRequest) (CardRequestResponse, error) {
	if err := s.validateCardRequest(req.AccountNumber, req.CardBrand, req.CardLimit); err != nil {
		return CardRequestResponse{}, err
	}
	accountDetails, err := s.accounts.GetAccountDetailsByNumber(ctx, strings.TrimSpace(req.AccountNumber), nil)
	if err != nil {
		return CardRequestResponse{}, err
	}
	if strings.EqualFold(accountDetails.AccountType, "BUSINESS") {
		return CardRequestResponse{}, cardBusinessError(422, "ERR_CARD_016", "Invalid account type", "Business accounts must use the /request/business endpoint.")
	}
	if principal.ID != 0 && principal.ID != accountDetails.Vlasnik {
		return CardRequestResponse{}, cardBusinessError(403, "ERR_CARD_007", "Access denied", "You do not own this account.")
	}
	if err := s.ensureVerification(ctx, req.VerificationID); err != nil {
		return CardRequestResponse{}, err
	}
	if err := s.enforcePersonalLimit(ctx, strings.TrimSpace(req.AccountNumber), accountDetails.Vlasnik); err != nil {
		return CardRequestResponse{}, err
	}
	created, err := s.createCard(ctx, cardCommand{
		AccountNumber: strings.TrimSpace(req.AccountNumber),
		CardBrand:     req.CardBrand,
		CardLimit:     req.CardLimit,
		ClientID:      accountDetails.Vlasnik,
	})
	if err != nil {
		return CardRequestResponse{}, err
	}
	s.publishRequestSuccess(ctx, strings.TrimSpace(req.AccountNumber), sql.NullInt64{}, created)
	return completedCardResponse(created), nil
}

func (s *CardService) RequestBusinessCard(ctx context.Context, principal Principal, req BusinessCardRequest) (CardRequestResponse, error) {
	if err := s.validateCardRequest(req.AccountNumber, req.CardBrand, req.CardLimit); err != nil {
		return CardRequestResponse{}, err
	}
	accountDetails, err := s.accounts.GetAccountDetailsByNumber(ctx, strings.TrimSpace(req.AccountNumber), nil)
	if err != nil {
		return CardRequestResponse{}, err
	}
	if !strings.EqualFold(accountDetails.AccountType, "BUSINESS") {
		return CardRequestResponse{}, cardBusinessError(422, "ERR_CARD_016", "Invalid account type", "Personal accounts must use the /request endpoint.")
	}
	if principal.ID != 0 && principal.ID != accountDetails.Vlasnik {
		return CardRequestResponse{}, cardBusinessError(403, "ERR_CARD_007", "Access denied", "You do not own this account.")
	}
	recipientType := strings.ToUpper(strings.TrimSpace(req.RecipientType))
	if recipientType != "OWNER" && recipientType != "AUTHORIZED_PERSON" {
		return CardRequestResponse{}, cardBusinessError(400, "ERR_CARD_015", "Invalid request state", "Recipient type must be provided.")
	}
	if err := s.ensureVerification(ctx, req.VerificationID); err != nil {
		return CardRequestResponse{}, err
	}

	var authorizedPersonID sql.NullInt64
	if recipientType == "AUTHORIZED_PERSON" {
		id, err := s.resolveAuthorizedPerson(ctx, req)
		if err != nil {
			return CardRequestResponse{}, err
		}
		authorizedPersonID = sql.NullInt64{Int64: id, Valid: true}
	}
	if err := s.enforceBusinessLimit(ctx, strings.TrimSpace(req.AccountNumber), accountDetails.Vlasnik, authorizedPersonID); err != nil {
		return CardRequestResponse{}, err
	}
	created, err := s.createCard(ctx, cardCommand{
		AccountNumber:      strings.TrimSpace(req.AccountNumber),
		CardBrand:          req.CardBrand,
		CardLimit:          req.CardLimit,
		ClientID:           accountDetails.Vlasnik,
		AuthorizedPersonID: authorizedPersonID,
	})
	if err != nil {
		return CardRequestResponse{}, err
	}
	if authorizedPersonID.Valid {
		_, _ = s.db.ExecContext(ctx, `
INSERT INTO authorized_person_card_ids (authorized_person_id, card_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING
`, authorizedPersonID.Int64, created.CardID)
	}
	s.publishRequestSuccess(ctx, strings.TrimSpace(req.AccountNumber), authorizedPersonID, created)
	return completedCardResponse(created), nil
}

func (s *CardService) GetCardsForClient(ctx context.Context, clientID int64) ([]CardSummaryResponse, error) {
	rows, err := s.db.QueryContext(ctx, cardSelectSQL+" WHERE client_id = $1 AND deleted = false ORDER BY id", clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CardSummaryResponse
	for rows.Next() {
		row, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row.summary())
	}
	return out, rows.Err()
}

func (s *CardService) GetCardsByAccount(ctx context.Context, accountNumber string) ([]CardSummaryResponse, error) {
	rows, err := s.db.QueryContext(ctx, cardSelectSQL+" WHERE account_number = $1 AND deleted = false ORDER BY id", accountNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CardSummaryResponse
	for rows.Next() {
		row, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row.summary())
	}
	return out, rows.Err()
}

func (s *CardService) GetInternalCardsByAccount(ctx context.Context, accountNumber string) ([]CardInternalSummaryResponse, error) {
	rows, err := s.db.QueryContext(ctx, cardSelectSQL+" WHERE account_number = $1 AND deleted = false ORDER BY id", accountNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CardInternalSummaryResponse
	for rows.Next() {
		row, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row.internalSummary())
	}
	return out, rows.Err()
}

func (s *CardService) GetCardByID(ctx context.Context, id int64) (CardDetailResponse, error) {
	row, err := s.findCard(ctx, id)
	if err != nil {
		return CardDetailResponse{}, err
	}
	return row.detail(), nil
}

func (s *CardService) GetClientIDByCardID(ctx context.Context, id int64) (int64, error) {
	row, err := s.findCard(ctx, id)
	if err != nil {
		return 0, err
	}
	return row.ClientID, nil
}

func (s *CardService) BlockCard(ctx context.Context, id int64) error {
	return s.transitionCard(ctx, id, "BLOCKED")
}

func (s *CardService) UnblockCard(ctx context.Context, id int64) error {
	return s.transitionCard(ctx, id, "ACTIVE")
}

func (s *CardService) DeactivateCard(ctx context.Context, id int64) error {
	return s.transitionCard(ctx, id, "DEACTIVATED")
}

func (s *CardService) UpdateCardLimit(ctx context.Context, id int64, limit decimal.Decimal) error {
	if limit.Sign() < 0 {
		return cardBusinessError(400, "ERR_CARD_006", "Invalid card limit", "Card limit must be zero or greater.")
	}
	if _, err := s.findCard(ctx, id); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, "UPDATE cards SET card_limit = $1, version = COALESCE(version, 0) + 1, updated_at = now() WHERE id = $2 AND deleted = false", limit, id)
	return err
}

func (s *CardService) GetAllCards(ctx context.Context, page, size int, status, search string) (Page[CardAdminSummaryResponse], error) {
	if page < 0 {
		page = 0
	}
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}
	status = strings.ToUpper(strings.TrimSpace(status))
	if status != "ACTIVE" && status != "BLOCKED" && status != "DEACTIVATED" {
		status = ""
	}
	search = strings.TrimSpace(search)
	args := []any{}
	where := []string{"deleted = false"}
	if status != "" {
		args = append(args, status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		where = append(where, fmt.Sprintf("(LOWER(card_number) LIKE $%d OR LOWER(account_number) LIKE $%d OR LOWER(card_name) LIKE $%d)", len(args), len(args), len(args)))
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cards WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return Page[CardAdminSummaryResponse]{}, err
	}
	args = append(args, size, page*size)
	rows, err := s.db.QueryContext(ctx, cardSelectSQL+" WHERE "+whereSQL+fmt.Sprintf(" ORDER BY id LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return Page[CardAdminSummaryResponse]{}, err
	}
	defer rows.Close()
	var content []CardAdminSummaryResponse
	for rows.Next() {
		row, err := scanCard(rows)
		if err != nil {
			return Page[CardAdminSummaryResponse]{}, err
		}
		content = append(content, row.adminSummary())
	}
	return NewPage(content, page, size, total), rows.Err()
}

func (s *CardService) createCard(ctx context.Context, cmd cardCommand) (CardCreationResponse, error) {
	if strings.TrimSpace(cmd.AccountNumber) == "" {
		return CardCreationResponse{}, cardBusinessError(400, "ERR_CARD_001", "Invalid account number", "Account number must not be blank.")
	}
	brand := strings.ToUpper(strings.TrimSpace(cmd.CardBrand))
	if !isSupportedCardBrand(brand) {
		return CardCreationResponse{}, cardBusinessError(400, "ERR_CARD_008", "Invalid card brand", "Card brand must be provided.")
	}
	if cmd.ClientID == 0 {
		return CardCreationResponse{}, cardBusinessError(400, "ERR_CARD_009", "Invalid client ID", "Client ID must be provided.")
	}
	if cmd.CardLimit.Sign() < 0 {
		return CardCreationResponse{}, cardBusinessError(400, "ERR_CARD_002", "Invalid card limit", "Card limit must be zero or greater.")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CardCreationResponse{}, err
	}
	defer tx.Rollback()

	cardNumber, err := s.generateCardNumber(ctx, tx, brand)
	if err != nil {
		return CardCreationResponse{}, err
	}
	plainCVV, hashCVV, err := generateCVV()
	if err != nil {
		return CardCreationResponse{}, err
	}
	creationDate := time.Now().UTC()
	expirationDate := creationDate.AddDate(5, 0, 0)
	var id int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO cards (
    card_number, card_type, card_name, creation_date, expiration_date,
    account_number, client_id, authorized_person_id, cvv, card_limit, status
) VALUES (
    $1, 'DEBIT', $2, $3, $4,
    $5, $6, $7, $8, $9, 'ACTIVE'
)
RETURNING id
`, cardNumber, card.CardName(brand), creationDate, expirationDate,
		strings.TrimSpace(cmd.AccountNumber), cmd.ClientID, cmd.AuthorizedPersonID, hashCVV, cmd.CardLimit).Scan(&id)
	if err != nil {
		return CardCreationResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return CardCreationResponse{}, err
	}
	return CardCreationResponse{
		CardID:         id,
		CardNumber:     cardNumber,
		PlainCVV:       plainCVV,
		ExpirationDate: expirationDate.Format("2006-01-02"),
		CardName:       card.CardName(brand),
	}, nil
}

func (s *CardService) generateCardNumber(ctx context.Context, tx *sql.Tx, brand string) (string, error) {
	for attempt := 0; attempt < 20; attempt++ {
		prefix, err := randomBrandPrefix(brand)
		if err != nil {
			return "", err
		}
		payloadLength := card.CardNumberLength(brand) - len(prefix) - 1
		digits, err := randomDigits(payloadLength)
		if err != nil {
			return "", err
		}
		payload := prefix + digits
		check, ok := card.CalculateCheckDigit(payload)
		if !ok {
			continue
		}
		number := payload + string(check)
		if !card.MatchesBrand(number, brand) || !(card.LuhnValidator{}).IsValid(number) {
			continue
		}
		var exists bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM cards WHERE card_number = $1)", number).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return number, nil
		}
	}
	return "", cardBusinessError(500, "ERR_CARD_003", "Card number generation failed", "Could not generate a unique card number after 20 attempts.")
}

func (s *CardService) validateCardRequest(accountNumber, brand string, limit decimal.Decimal) error {
	if strings.TrimSpace(accountNumber) == "" {
		return cardBusinessError(400, "ERR_CARD_001", "Invalid account number", "Account number must not be blank.")
	}
	if !isSupportedCardBrand(brand) {
		return cardBusinessError(400, "ERR_CARD_008", "Invalid card brand", "Card brand must be provided.")
	}
	if limit.Sign() < 0 {
		return cardBusinessError(400, "ERR_CARD_002", "Invalid card limit", "Card limit must be zero or greater.")
	}
	return nil
}

func (s *CardService) ensureVerification(ctx context.Context, verificationID int64) error {
	if verificationID == 0 {
		return cardBusinessError(400, "ERR_CARD_015", "Invalid request state", "Verification ID must be provided.")
	}
	ok, err := s.accounts.verificationVerified(ctx, verificationID)
	if err != nil || !ok {
		return cardBusinessError(400, "ERR_CARD_015", "Invalid request state", "Verification is not completed.")
	}
	return nil
}

func (s *CardService) enforcePersonalLimit(ctx context.Context, accountNumber string, clientID int64) error {
	var count int
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
  FROM cards
 WHERE account_number = $1
   AND client_id = $2
   AND authorized_person_id IS NULL
   AND status <> 'DEACTIVATED'
   AND deleted = false
`, accountNumber, clientID).Scan(&count)
	if err != nil {
		return err
	}
	if count >= 2 {
		return cardBusinessError(422, "ERR_CARD_010", "Maximum card limit reached", "Personal accounts can have at most 2 active cards.")
	}
	return nil
}

func (s *CardService) enforceBusinessLimit(ctx context.Context, accountNumber string, clientID int64, authorizedPersonID sql.NullInt64) error {
	var count int
	var err error
	if authorizedPersonID.Valid {
		err = s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
  FROM cards
 WHERE account_number = $1
   AND authorized_person_id = $2
   AND status <> 'DEACTIVATED'
   AND deleted = false
`, accountNumber, authorizedPersonID.Int64).Scan(&count)
	} else {
		err = s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
  FROM cards
 WHERE account_number = $1
   AND client_id = $2
   AND authorized_person_id IS NULL
   AND status <> 'DEACTIVATED'
   AND deleted = false
`, accountNumber, clientID).Scan(&count)
	}
	if err != nil {
		return err
	}
	if count >= 1 {
		return cardBusinessError(422, "ERR_CARD_010", "Maximum card limit reached", "Business accounts can have at most 1 active card per person.")
	}
	return nil
}

func (s *CardService) resolveAuthorizedPerson(ctx context.Context, req BusinessCardRequest) (int64, error) {
	if req.AuthorizedPersonID != nil {
		var id int64
		err := s.db.QueryRowContext(ctx, "SELECT id FROM authorized_persons WHERE id = $1", *req.AuthorizedPersonID).Scan(&id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, cardBusinessError(404, "ERR_CARD_014", "Authorized person not found", "Authorized person with ID %d was not found.", *req.AuthorizedPersonID)
			}
			return 0, err
		}
		return id, nil
	}
	if req.AuthorizedPerson == nil {
		return 0, cardBusinessError(400, "ERR_CARD_015", "Invalid request state", "Authorized-person details must be provided for this request.")
	}
	ap := req.AuthorizedPerson
	if strings.TrimSpace(ap.Email) == "" || strings.TrimSpace(ap.FirstName) == "" || strings.TrimSpace(ap.LastName) == "" || strings.TrimSpace(ap.DateOfBirth) == "" {
		return 0, cardBusinessError(400, "ERR_CARD_015", "Invalid request state", "Authorized-person details must be provided for this request.")
	}
	var existing int64
	err := s.db.QueryRowContext(ctx, `
SELECT id
  FROM authorized_persons
 WHERE LOWER(email) = LOWER($1)
   AND LOWER(first_name) = LOWER($2)
   AND LOWER(last_name) = LOWER($3)
   AND date_of_birth = $4
`, strings.TrimSpace(ap.Email), strings.TrimSpace(ap.FirstName), strings.TrimSpace(ap.LastName), strings.TrimSpace(ap.DateOfBirth)).Scan(&existing)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	var id int64
	err = s.db.QueryRowContext(ctx, `
INSERT INTO authorized_persons (first_name, last_name, date_of_birth, gender, email, phone, address)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id
`, strings.TrimSpace(ap.FirstName), strings.TrimSpace(ap.LastName), strings.TrimSpace(ap.DateOfBirth),
		strings.ToUpper(strings.TrimSpace(ap.Gender)), strings.TrimSpace(ap.Email), strings.TrimSpace(ap.Phone), strings.TrimSpace(ap.Address)).Scan(&id)
	return id, err
}

func (s *CardService) findCard(ctx context.Context, id int64) (cardRow, error) {
	row, err := scanCard(s.db.QueryRowContext(ctx, cardSelectSQL+" WHERE id = $1 AND deleted = false", id))
	if err != nil {
		return cardRow{}, err
	}
	return row, nil
}

func (s *CardService) transitionCard(ctx context.Context, id int64, target string) error {
	row, err := s.findCard(ctx, id)
	if err != nil {
		return err
	}
	current := row.Status
	allowed := (current == "ACTIVE" && (target == "BLOCKED" || target == "DEACTIVATED")) ||
		(current == "BLOCKED" && (target == "ACTIVE" || target == "DEACTIVATED"))
	if !allowed {
		return cardBusinessError(422, "ERR_CARD_005", "Invalid status transition", "Transition from %s to %s is not allowed.", current, target)
	}
	_, err = s.db.ExecContext(ctx, "UPDATE cards SET status = $1, version = COALESCE(version, 0) + 1, updated_at = now() WHERE id = $2 AND deleted = false", target, id)
	if err == nil {
		switch target {
		case "BLOCKED":
			s.publishLifecycleNotification(ctx, row, cardBlockedRoutingKey)
		case "ACTIVE":
			s.publishLifecycleNotification(ctx, row, cardUnblockedRoutingKey)
		case "DEACTIVATED":
			s.publishLifecycleNotification(ctx, row, cardDeactivatedRoutingKey)
		}
	}
	return err
}

func scanCard(row rowScanner) (cardRow, error) {
	var out cardRow
	if err := row.Scan(
		&out.ID,
		&out.CardNumber,
		&out.CardType,
		&out.CardName,
		&out.CreationDate,
		&out.ExpirationDate,
		&out.AccountNumber,
		&out.ClientID,
		&out.AuthorizedPersonID,
		&out.CVV,
		&out.CardLimit,
		&out.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return cardRow{}, cardBusinessError(404, "ERR_CARD_004", "Card not found", "Card was not found.")
		}
		return cardRow{}, err
	}
	return out, nil
}

func (r cardRow) summary() CardSummaryResponse {
	return CardSummaryResponse{
		ID:               r.ID,
		MaskedCardNumber: card.MaskCardNumber(r.CardNumber),
		AccountNumber:    r.AccountNumber,
	}
}

func (r cardRow) detail() CardDetailResponse {
	return CardDetailResponse{
		ID:             r.ID,
		CardNumber:     r.CardNumber,
		CardType:       r.CardType,
		CardName:       r.CardName,
		CreationDate:   r.CreationDate.Format("2006-01-02"),
		ExpirationDate: r.ExpirationDate.Format("2006-01-02"),
		AccountNumber:  r.AccountNumber,
		CardLimit:      r.CardLimit,
		Status:         r.Status,
	}
}

func (r cardRow) adminSummary() CardAdminSummaryResponse {
	return CardAdminSummaryResponse{
		ID:            r.ID,
		CardNumber:    card.MaskCardNumber(r.CardNumber),
		Brand:         r.CardName,
		Status:        r.Status,
		AccountNumber: r.AccountNumber,
		ClientID:      r.ClientID,
		CardLimit:     r.CardLimit,
	}
}

func (r cardRow) internalSummary() CardInternalSummaryResponse {
	return CardInternalSummaryResponse{
		ID:            r.ID,
		CardNumber:    card.MaskCardNumber(r.CardNumber),
		CardType:      r.CardName,
		Status:        r.Status,
		ExpiryDate:    r.ExpirationDate.Format("2006-01-02"),
		AccountNumber: r.AccountNumber,
	}
}

func randomBrandPrefix(brand string) (string, error) {
	switch strings.ToUpper(brand) {
	case "VISA":
		return "4", nil
	case "DINACARD":
		return "9891", nil
	case "AMEX":
		if n, err := secureInt(2); err == nil && n == 0 {
			return "34", nil
		}
		return "37", nil
	case "MASTERCARD":
		if n, err := secureInt(2); err == nil && n == 0 {
			prefix, err := secureRange(51, 55)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d", prefix), nil
		}
		prefix, err := secureRange(2221, 2720)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d", prefix), nil
	default:
		return "", cardBusinessError(400, "ERR_CARD_008", "Invalid card brand", "Card brand must be provided.")
	}
}

func randomCardBrand() string {
	n, err := secureInt(len(cardBrands))
	if err != nil {
		return "VISA"
	}
	return cardBrands[n]
}

func randomDigits(length int) (string, error) {
	var b strings.Builder
	for i := 0; i < length; i++ {
		n, err := secureInt(10)
		if err != nil {
			return "", err
		}
		b.WriteByte(byte('0' + n))
	}
	return b.String(), nil
}

func generateCVV() (plain string, hashed string, err error) {
	n, err := secureInt(1000)
	if err != nil {
		return "", "", err
	}
	plain = fmt.Sprintf("%03d", n)
	sum := sha256.Sum256([]byte(plain))
	return plain, "sha256:" + hex.EncodeToString(sum[:]), nil
}

func secureRange(min, max int) (int, error) {
	n, err := secureInt(max - min + 1)
	if err != nil {
		return 0, err
	}
	return min + n, nil
}

func secureInt(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

func isSupportedCardBrand(brand string) bool {
	switch strings.ToUpper(strings.TrimSpace(brand)) {
	case "VISA", "MASTERCARD", "DINACARD", "AMEX":
		return true
	default:
		return false
	}
}

func completedCardResponse(created CardCreationResponse) CardRequestResponse {
	return CardRequestResponse{
		Status:      "COMPLETED",
		Message:     "Card created successfully.",
		CreatedCard: &created,
	}
}

func cardBusinessError(status int, code, title, format string, args ...any) *Error {
	return &Error{Status: status, Code: code, Title: title, Message: fmt.Sprintf(format, args...)}
}

const cardSelectSQL = `
SELECT id,
       card_number,
       card_type,
       card_name,
       creation_date,
       expiration_date,
       account_number,
       client_id,
       authorized_person_id,
       cvv,
       card_limit,
       status
  FROM cards
`
