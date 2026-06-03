package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"banka1/banking-core-service-go/internal/decimal"
	"banka1/banking-core-service-go/internal/uuid"
)

type rowScanner interface {
	Scan(...any) error
}

func scanCurrency(row rowScanner) (CurrencyResponse, error) {
	var out CurrencyResponse
	var version sql.NullInt64
	if err := row.Scan(&out.ID, &version, &out.Naziv, &out.Oznaka, &out.Simbol, &out.Opis, &out.Status); err != nil {
		return CurrencyResponse{}, err
	}
	if version.Valid {
		out.Version = &version.Int64
	}
	return out, nil
}

func (s *AccountService) queryCurrencies(ctx context.Context, page, size int) ([]CurrencyResponse, int, error) {
	if size <= 0 {
		size = 10
	}
	offset := page * size
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM currency_table WHERE status = 'ACTIVE'").Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, version, naziv, oznaka, simbol, opis, status
  FROM currency_table
 WHERE status = 'ACTIVE'
 ORDER BY id
 LIMIT $1 OFFSET $2
`, size, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]CurrencyResponse, 0, size)
	for rows.Next() {
		cur, err := scanCurrency(rows)
		if err != nil {
			return nil, 0, err
		}
		cur.Countries, _ = s.currencyCountries(ctx, cur.ID)
		out = append(out, cur)
	}
	return out, total, rows.Err()
}

func (s *AccountService) currencyCountries(ctx context.Context, currencyID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT country FROM currency_countries WHERE currency_id = $1 ORDER BY country", currencyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var country string
		if err := rows.Scan(&country); err != nil {
			return nil, err
		}
		out = append(out, country)
	}
	return out, rows.Err()
}

type accountView struct {
	ID               int64
	DisplayName      string
	AccountNumber    string
	OwnerID          int64
	EmployeeID       int64
	FirstName        string
	LastName         string
	Email            string
	Username         string
	AvailableBalance decimal.Decimal
	BookedBalance    decimal.Decimal
	Currency         string
	DailyLimit       decimal.Decimal
	MonthlyLimit     decimal.Decimal
	DailySpending    decimal.Decimal
	MonthlySpending  decimal.Decimal
	CreatedAt        sql.NullTime
	ExpiresAt        sql.NullTime
	Status           string
	AccountType      string
	AccountConcrete  string
	AccountOwnership string
	CompanyID        sql.NullInt64
	CompanyName      string
	CompanyRegNumber string
	CompanyTaxID     string
	CompanyActivity  string
	CompanyAddress   string
	CompanyOwnerID   sql.NullInt64
}

func (s *AccountService) loadAccountViewByID(ctx context.Context, id int64) (accountView, error) {
	return scanAccountView(s.db.QueryRowContext(ctx, accountViewSQL+" WHERE a.id = $1 AND a.deleted = false", id))
}

func (s *AccountService) loadAccountViewByNumber(ctx context.Context, accountNumber string) (accountView, error) {
	return scanAccountView(s.db.QueryRowContext(ctx, accountViewSQL+" WHERE a.broj_racuna = $1 AND a.deleted = false", accountNumber))
}

func (s *AccountService) accountsByOwnerCurrencyView(ctx context.Context, ownerID int64, currency string) (accountView, error) {
	return scanAccountView(s.db.QueryRowContext(ctx, accountViewSQL+`
 WHERE a.vlasnik = $1 AND cur.oznaka = $2
   AND a.deleted = false
 ORDER BY a.id
 LIMIT 1
`, ownerID, strings.ToUpper(currency)))
}

func scanAccountView(row rowScanner) (accountView, error) {
	var out accountView
	if err := row.Scan(
		&out.ID,
		&out.DisplayName,
		&out.AccountNumber,
		&out.OwnerID,
		&out.EmployeeID,
		&out.FirstName,
		&out.LastName,
		&out.Email,
		&out.Username,
		&out.AvailableBalance,
		&out.BookedBalance,
		&out.Currency,
		&out.DailyLimit,
		&out.MonthlyLimit,
		&out.DailySpending,
		&out.MonthlySpending,
		&out.CreatedAt,
		&out.ExpiresAt,
		&out.Status,
		&out.AccountType,
		&out.AccountConcrete,
		&out.AccountOwnership,
		&out.CompanyID,
		&out.CompanyName,
		&out.CompanyRegNumber,
		&out.CompanyTaxID,
		&out.CompanyActivity,
		&out.CompanyAddress,
		&out.CompanyOwnerID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return accountView{}, NotFound("Ne postoji racun")
		}
		return accountView{}, err
	}
	return out, nil
}

func (v accountView) details() AccountDetailsResponse {
	accountCategory, accountType, subtype, tip := v.classification()
	out := AccountDetailsResponse{
		NazivRacuna:         v.DisplayName,
		BrojRacuna:          v.AccountNumber,
		Vlasnik:             v.OwnerID,
		Tip:                 tip,
		RaspolozivoStanje:   v.AvailableBalance,
		RezervisanaSredstva: v.BookedBalance.Sub(v.AvailableBalance),
		StanjeRacuna:        v.BookedBalance,
		Currency:            v.Currency,
		DailyLimit:          v.DailyLimit,
		MonthlyLimit:        v.MonthlyLimit,
		DailySpending:       v.DailySpending,
		MonthlySpending:     v.MonthlySpending,
		CreationDate:        formatDateTime(v.CreatedAt),
		ExpirationDate:      formatDate(v.ExpiresAt),
		Status:              v.Status,
		AccountCategory:     accountCategory,
		AccountType:         accountType,
		Subtype:             subtype,
		Cards:               []any{},
	}
	if v.CompanyID.Valid {
		out.NazivFirme = v.CompanyName
		out.CompanyRegistrationNumber = v.CompanyRegNumber
		out.CompanyTaxID = v.CompanyTaxID
		out.CompanyActivityCode = v.CompanyActivity
		out.CompanyAddress = v.CompanyAddress
		if v.CompanyOwnerID.Valid {
			out.CompanyOwnerID = &v.CompanyOwnerID.Int64
		}
	}
	return out
}

func (v accountView) summary() AccountResponse {
	accountCategory, accountType, subtype, _ := v.classification()
	return AccountResponse{
		ID:                v.ID,
		NazivRacuna:       v.DisplayName,
		BrojRacuna:        v.AccountNumber,
		RaspolozivoStanje: v.AvailableBalance,
		Currency:          v.Currency,
		AccountCategory:   accountCategory,
		AccountType:       accountType,
		Subtype:           subtype,
	}
}

func (v accountView) classification() (category, accountType, subtype, tip string) {
	if strings.EqualFold(v.AccountType, "FX") {
		return "FOREIGN_CURRENCY", strings.ToUpper(v.AccountOwnership), "", "devizni"
	}
	ownership := strings.ToUpper(v.AccountOwnership)
	subtype = strings.ToUpper(v.AccountConcrete)
	if ownership == "" && subtype != "" {
		derived, _, err := accountConcrete(subtype)
		if err == nil {
			ownership = derived
		}
	}
	return "CHECKING", ownership, subtype, "tekuci"
}

func scanAccountSearch(row rowScanner) (AccountSearchResponse, error) {
	var out AccountSearchResponse
	var ownership, accountType string
	var expires sql.NullTime
	if err := row.Scan(
		&out.ID,
		&out.BrojRacuna,
		&out.Ime,
		&out.Prezime,
		&ownership,
		&accountType,
		&out.Stanje,
		&out.RaspolozivoStanje,
		&out.Currency,
		&out.Status,
		&out.Vlasnik,
		&out.Zaposlen,
		&out.DnevniLimit,
		&out.MesecniLimit,
		&out.DnevnaPotrosnja,
		&out.MesecnaPotrosnja,
		&expires,
	); err != nil {
		return AccountSearchResponse{}, err
	}
	out.AccountID = out.ID
	out.RezervisanaSredstva = out.Stanje.Sub(out.RaspolozivoStanje)
	out.AccountOwnershipType = strings.ToUpper(ownership)
	out.IsSystemAccount = out.Vlasnik < 0
	if strings.EqualFold(accountType, "FX") {
		out.TekuciIliDevizni = "devizni"
	} else {
		out.TekuciIliDevizni = "tekuci"
	}
	out.DatumIsteka = formatDate(expires)
	return out, nil
}

func (s *AccountService) editAccountName(ctx context.Context, principal Principal, key string, id int64, accountNumber string, req EditAccountNameRequest) (string, error) {
	name := strings.TrimSpace(req.AccountName)
	if name == "" {
		return "", BadRequest("Unesi naziv racuna")
	}
	view, err := s.accountForEdit(ctx, key, id, accountNumber)
	if err != nil {
		return "", err
	}
	if view.OwnerID != principal.ID {
		return "", BadRequest("Nisi vlasnik racuna")
	}
	if strings.EqualFold(view.DisplayName, name) {
		return "", BadRequest("Ime ne sme biti isto")
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
      FROM account_table
     WHERE vlasnik = $1
       AND LOWER(naziv_racuna) = LOWER($2)
       AND id <> $3
       AND deleted = false
)
`, view.OwnerID, name, view.ID).Scan(&exists); err != nil {
		return "", err
	}
	if exists {
		return "", BadRequest("Vlasnik poseduje racun sa ovim imenom")
	}
	_, err = s.db.ExecContext(ctx, "UPDATE account_table SET naziv_racuna = $1, updated_at = now() WHERE id = $2 AND deleted = false", name, view.ID)
	if err != nil {
		return "", err
	}
	return "Uspesno editovano ime", nil
}

func (s *AccountService) editAccountLimit(ctx context.Context, principal Principal, key string, id int64, accountNumber string, req EditAccountLimitRequest) (string, error) {
	if req.DailyLimit.Sign() <= 0 || req.MonthlyLimit.Sign() <= 0 {
		return "", BadRequest("Limit mora biti veci od 0")
	}
	if req.DailyLimit.Cmp(req.MonthlyLimit) > 0 {
		return "", BadRequest("Dnevni limit mora biti manji ili jednak od mesecnog")
	}
	view, err := s.accountForEdit(ctx, key, id, accountNumber)
	if err != nil {
		return "", err
	}
	if view.OwnerID != principal.ID {
		return "", BadRequest("Nisi vlasnik racuna")
	}
	if !s.cfg.SkipVerification {
		ok, err := s.verificationVerified(ctx, req.VerificationSessionID)
		if err != nil || !ok {
			return "", Conflict("ERR_VERIFICATION_FAILED", "Verifikacija nije uspela", "Verifikacija nije uspela")
		}
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE account_table
   SET dnevni_limit = $1,
       mesecni_limit = $2,
       updated_at = now()
 WHERE id = $3
   AND deleted = false
`, req.DailyLimit, req.MonthlyLimit, view.ID)
	if err != nil {
		return "", err
	}
	return "Uspesno setovani limiti", nil
}

func (s *AccountService) accountForEdit(ctx context.Context, key string, id int64, accountNumber string) (accountView, error) {
	if key == "id" {
		return s.loadAccountViewByID(ctx, id)
	}
	return s.loadAccountViewByNumber(ctx, accountNumber)
}

func (s *AccountService) cardSummariesForAccount(ctx context.Context, accountNumber string) []any {
	rows, err := s.db.QueryContext(ctx, cardSelectSQL+" WHERE account_number = $1 AND deleted = false ORDER BY id", accountNumber)
	if err != nil {
		return []any{}
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		row, err := scanCard(rows)
		if err != nil {
			return []any{}
		}
		out = append(out, row.internalSummary())
	}
	return out
}

func (s *AccountService) verificationVerified(ctx context.Context, sessionID int64) (bool, error) {
	if sessionID == 0 {
		return false, BadRequest("Unesi verification session ID")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.cfg.VerificationURL, "/")+fmt.Sprintf("/%d/status", sessionID), nil)
	if err != nil {
		return false, err
	}
	if token, err := s.serviceToken(); err == nil {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, nil
	}
	var decoded struct {
		Status   string `json:"status"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return false, err
	}
	return decoded.Verified || strings.EqualFold(decoded.Status, "VERIFIED"), nil
}

func (s *AccountService) accountsForTransfer(ctx context.Context, tx *sql.Tx, accountNumber string) (accountBalanceRow, error) {
	row, err := s.getByNumber(ctx, tx, accountNumber, true)
	if err != nil {
		return accountBalanceRow{}, err
	}
	if err := row.validateMutable(accountNumber, row.OwnerID); err != nil {
		return accountBalanceRow{}, err
	}
	return row, nil
}

func (s *AccountService) applyPaymentTx(ctx context.Context, tx *sql.Tx, from, to accountBalanceRow, req PaymentRequest) error {
	if req.FromAmount.Sign() <= 0 || req.ToAmount.Sign() <= 0 {
		return BadRequest("Iznos mora biti veci od 0")
	}
	if req.Commission.Sign() < 0 {
		return BadRequest("Minimalni commission je 0")
	}
	if strings.EqualFold(from.Currency, to.Currency) {
		if err := s.DebitTx(ctx, tx, from.AccountNumber, req.FromAmount.Add(req.Commission), from.OwnerID); err != nil {
			return err
		}
		if err := s.CreditTx(ctx, tx, to.AccountNumber, req.ToAmount, to.OwnerID); err != nil {
			return err
		}
		if req.Commission.Sign() > 0 {
			bank, err := s.getByOwnerAndCurrency(ctx, tx, -1, from.Currency, true)
			if err != nil {
				return err
			}
			if err := s.CreditTx(ctx, tx, bank.AccountNumber, req.Commission, bank.OwnerID); err != nil {
				return err
			}
		}
		return nil
	}
	if to.OwnerID == -1 {
		if err := s.DebitTx(ctx, tx, from.AccountNumber, req.FromAmount, from.OwnerID); err != nil {
			return err
		}
		return s.CreditTx(ctx, tx, to.AccountNumber, req.ToAmount, to.OwnerID)
	}
	bankSender, err := s.getByOwnerAndCurrency(ctx, tx, -1, from.Currency, true)
	if err != nil {
		return err
	}
	bankTarget, err := s.getByOwnerAndCurrency(ctx, tx, -1, to.Currency, true)
	if err != nil {
		return err
	}
	// Commission is in the source currency — deduct it from the sender along with FromAmount.
	// The receiver always gets the full converted ToAmount.
	senderDebit := req.FromAmount.Add(req.Commission)
	if err := s.DebitTx(ctx, tx, from.AccountNumber, senderDebit, from.OwnerID); err != nil {
		return err
	}
	if err := s.CreditTx(ctx, tx, bankSender.AccountNumber, senderDebit, bankSender.OwnerID); err != nil {
		return err
	}
	if err := s.DebitTx(ctx, tx, bankTarget.AccountNumber, req.ToAmount, bankTarget.OwnerID); err != nil {
		return err
	}
	return s.CreditTx(ctx, tx, to.AccountNumber, req.ToAmount, to.OwnerID)
}

func (s *AccountService) recordPayment(ctx context.Context, tx *sql.Tx, from, to accountBalanceRow, fromAmount, toAmount, commission decimal.Decimal, purpose, reference string) error {
	orderNumber, err := uuid.New()
	if err != nil {
		orderNumber = fmt.Sprintf("go-%d", time.Now().UnixNano())
	}
	if reference == "" {
		reference = orderNumber
	}
	recipientName := strings.TrimSpace(to.Username)
	if recipientName == "" {
		recipientName = to.AccountNumber
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO payment_table (
    from_account_number, to_account_number, initial_amount, final_amount, commission,
    sender_client_id, recipient_client_id, recipient_name,
    payment_code, reference_number, payment_purpose, status,
    from_currency, to_currency, order_number, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8,
    '289', $9, $10, 'COMPLETED',
    $11, $12, $13, NOW(), NOW(), 0
)
`, from.AccountNumber, to.AccountNumber, fromAmount, toAmount, commission,
		from.OwnerID, to.OwnerID, recipientName,
		reference, purpose,
		from.Currency, to.Currency, orderNumber)
	if err != nil {
		return nil
	}
	return nil
}

func (s *AccountService) resolveOneSidedAccount(ctx context.Context, req OneSidedTransactionRequest) (accountBalanceRow, error) {
	if strings.TrimSpace(req.AccountNumber) != "" {
		row, err := s.getByNumber(ctx, s.db, req.AccountNumber, false)
		if err != nil {
			return accountBalanceRow{}, err
		}
		if err := row.validateMutable(req.AccountNumber, row.OwnerID); err != nil {
			return accountBalanceRow{}, err
		}
		return row, nil
	}
	if req.AccountID != nil {
		row, err := s.getByID(ctx, s.db, *req.AccountID)
		if err != nil {
			return accountBalanceRow{}, err
		}
		if err := row.validateMutable(row.AccountNumber, row.OwnerID); err != nil {
			return accountBalanceRow{}, err
		}
		return row, nil
	}
	return accountBalanceRow{}, BadRequest("OneSidedTransactionDto mora imati accountNumber ili accountId")
}

func formatDateTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02T15:04:05")
}

func formatDate(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02")
}

const accountViewSQL = `
SELECT a.id,
       a.naziv_racuna,
       a.broj_racuna,
       a.vlasnik,
       a.zaposlen,
       COALESCE(a.ime_vlasnika_racuna, ''),
       COALESCE(a.prezime_vlasnika_racuna, ''),
       COALESCE(a.email, ''),
       COALESCE(a.username, ''),
       a.raspolozivo_stanje,
       a.stanje,
       COALESCE(cur.oznaka, ''),
       COALESCE(a.dnevni_limit, 0),
       COALESCE(a.mesecni_limit, 0),
       COALESCE(a.dnevna_potrosnja, 0),
       COALESCE(a.mesecna_potrosnja, 0),
       a.datum_i_vreme_kreiranja,
       a.datum_isteka,
       a.status,
       COALESCE(a.account_type, ''),
       COALESCE(a.account_concrete, ''),
       COALESCE(a.account_ownership_type, ''),
       a.company_id,
       COALESCE(co.naziv, ''),
       COALESCE(co.maticni_broj, ''),
       COALESCE(co.poreski_broj, ''),
       COALESCE(sd.sifra, ''),
       COALESCE(co.adresa, ''),
       co.vlasnik
  FROM account_table a
  LEFT JOIN currency_table cur ON cur.id = a.currency_id
  LEFT JOIN company_table co ON co.id = a.company_id
  LEFT JOIN sifra_delatnosti_table sd ON sd.id = co.sifra_delatnosti_id
`
