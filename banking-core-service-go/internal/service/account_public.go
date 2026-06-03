package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"banka1/banking-core-service-go/internal/account"
	"banka1/banking-core-service-go/internal/decimal"
)

type Principal struct {
	ID    int64
	Roles []string
}

func (p Principal) IsPrivileged() bool {
	for _, role := range p.Roles {
		upper := strings.ToUpper(role)
		if strings.Contains(upper, "ADMIN") || strings.Contains(upper, "AGENT") || strings.Contains(upper, "SUPERVISOR") || strings.Contains(upper, "SERVICE") || upper == "BASIC" {
			return true
		}
	}
	return false
}

type CurrencyResponse struct {
	ID        int64    `json:"id"`
	Version   *int64   `json:"version,omitempty"`
	Naziv     string   `json:"naziv"`
	Oznaka    string   `json:"oznaka"`
	Simbol    string   `json:"simbol"`
	Countries []string `json:"countries"`
	Opis      string   `json:"opis"`
	Status    string   `json:"status"`
}

type AccountDetailsResponse struct {
	NazivRacuna               string          `json:"nazivRacuna"`
	BrojRacuna                string          `json:"brojRacuna"`
	Vlasnik                   int64           `json:"vlasnik"`
	Tip                       string          `json:"tip,omitempty"`
	RaspolozivoStanje         decimal.Decimal `json:"raspolozivoStanje"`
	RezervisanaSredstva       decimal.Decimal `json:"rezervisanaSredstva"`
	StanjeRacuna              decimal.Decimal `json:"stanjeRacuna"`
	NazivFirme                string          `json:"nazivFirme,omitempty"`
	Currency                  string          `json:"currency"`
	DailyLimit                decimal.Decimal `json:"dailyLimit"`
	MonthlyLimit              decimal.Decimal `json:"monthlyLimit"`
	DailySpending             decimal.Decimal `json:"dailySpending"`
	MonthlySpending           decimal.Decimal `json:"monthlySpending"`
	CreationDate              string          `json:"creationDate,omitempty"`
	ExpirationDate            string          `json:"expirationDate,omitempty"`
	Status                    string          `json:"status"`
	AccountCategory           string          `json:"accountCategory,omitempty"`
	AccountType               string          `json:"accountType,omitempty"`
	Subtype                   string          `json:"subtype,omitempty"`
	CompanyRegistrationNumber string          `json:"companyRegistrationNumber,omitempty"`
	CompanyTaxID              string          `json:"companyTaxId,omitempty"`
	CompanyActivityCode       string          `json:"companyActivityCode,omitempty"`
	CompanyAddress            string          `json:"companyAddress,omitempty"`
	CompanyOwnerID            *int64          `json:"companyOwnerId,omitempty"`
	Cards                     []any           `json:"cards"`
}

type AccountResponse struct {
	ID                int64           `json:"id"`
	NazivRacuna       string          `json:"nazivRacuna"`
	BrojRacuna        string          `json:"brojRacuna"`
	RaspolozivoStanje decimal.Decimal `json:"raspolozivoStanje"`
	Currency          string          `json:"currency"`
	AccountCategory   string          `json:"accountCategory,omitempty"`
	AccountType       string          `json:"accountType,omitempty"`
	Subtype           string          `json:"subtype,omitempty"`
}

type AccountSearchResponse struct {
	ID                   int64           `json:"id"`
	AccountID            int64           `json:"accountId"`
	BrojRacuna           string          `json:"brojRacuna"`
	Ime                  string          `json:"ime"`
	Prezime              string          `json:"prezime"`
	AccountOwnershipType string          `json:"accountOwnershipType,omitempty"`
	TekuciIliDevizni     string          `json:"tekuciIliDevizni,omitempty"`
	Stanje               decimal.Decimal `json:"stanje"`
	RaspolozivoStanje    decimal.Decimal `json:"raspolozivoStanje"`
	RezervisanaSredstva  decimal.Decimal `json:"rezervisanaSredstva"`
	Currency             string          `json:"currency"`
	Status               string          `json:"status"`
	IsSystemAccount      bool            `json:"isSystemAccount"`
	Vlasnik              int64           `json:"vlasnik"`
	Zaposlen             int64           `json:"zaposlen"`
	DnevniLimit          decimal.Decimal `json:"dnevniLimit"`
	MesecniLimit         decimal.Decimal `json:"mesecniLimit"`
	DnevnaPotrosnja      decimal.Decimal `json:"dnevnaPotrosnja"`
	MesecnaPotrosnja     decimal.Decimal `json:"mesecnaPotrosnja"`
	DatumIsteka          string          `json:"datumIsteka,omitempty"`
}

type CompanyResponse struct {
	ID              int64  `json:"id"`
	Naziv           string `json:"naziv"`
	MaticniBroj     string `json:"maticniBroj"`
	PoreskiBroj     string `json:"poreskiBroj"`
	SifraDelatnosti string `json:"sifraDelatnosti"`
	Adresa          string `json:"adresa,omitempty"`
	Vlasnik         int64  `json:"vlasnik"`
}

type CheckingAccountRequest struct {
	NazivRacuna    string          `json:"nazivRacuna"`
	IDVlasnika     *int64          `json:"idVlasnika"`
	JMBG           string          `json:"jmbg"`
	VrstaRacuna    string          `json:"vrstaRacuna"`
	Firma          *CompanyRequest `json:"firma"`
	InitialBalance decimal.Decimal `json:"initialBalance"`
	CreateCard     *bool           `json:"createCard"`
}

type FXAccountRequest struct {
	NazivRacuna    string          `json:"nazivRacuna"`
	IDVlasnika     *int64          `json:"idVlasnika"`
	JMBG           string          `json:"jmbg"`
	CurrencyCode   string          `json:"currencyCode"`
	TipRacuna      string          `json:"tipRacuna"`
	InitialBalance decimal.Decimal `json:"initialBalance"`
	CreateCard     *bool           `json:"createCard"`
	Firma          *CompanyRequest `json:"firma"`
}

type CompanyRequest struct {
	Naziv           string `json:"naziv"`
	MaticniBroj     string `json:"maticniBroj"`
	PoreskiBroj     string `json:"poreskiBroj"`
	SifraDelatnosti string `json:"sifraDelatnosti"`
	Adresa          string `json:"adresa"`
	Vlasnik         int64  `json:"vlasnik"`
}

type UpdateCompanyRequest struct {
	Naziv           string `json:"naziv"`
	SifraDelatnosti string `json:"sifraDelatnosti"`
	Adresa          string `json:"adresa"`
	Vlasnik         *int64 `json:"vlasnik"`
}

type EditAccountNameRequest struct {
	AccountName string `json:"accountName"`
}

type EditAccountLimitRequest struct {
	DailyLimit            decimal.Decimal `json:"dailyLimit"`
	MonthlyLimit          decimal.Decimal `json:"monthlyLimit"`
	VerificationSessionID int64           `json:"verificationSessionId"`
}

type EditStatusRequest struct {
	Status string `json:"status"`
}

type PaymentRequest struct {
	FromAccountNumber string          `json:"fromAccountNumber"`
	ToAccountNumber   string          `json:"toAccountNumber"`
	FromAmount        decimal.Decimal `json:"fromAmount"`
	ToAmount          decimal.Decimal `json:"toAmount"`
	Commission        decimal.Decimal `json:"commission"`
	ClientID          int64           `json:"clientId"`
}

type BankPaymentRequest struct {
	FromAccountNumber string          `json:"fromAccountNumber"`
	ToAccountNumber   string          `json:"toAccountNumber"`
	Amount            decimal.Decimal `json:"amount"`
}

type OneSidedTransactionRequest struct {
	AccountNumber string          `json:"accountNumber"`
	AccountID     *int64          `json:"accountId"`
	Amount        decimal.Decimal `json:"amount"`
	ClientID      int64           `json:"clientId"`
	Description   string          `json:"description"`
}

type CreateSystemAccountRequest struct {
	AccountNumber   string          `json:"accountNumber"`
	OwnerID         int64           `json:"ownerId"`
	CurrencyCode    string          `json:"currencyCode"`
	AccountConcrete string          `json:"accountConcrete"`
	DisplayName     string          `json:"displayName"`
	InitialBalance  decimal.Decimal `json:"initialBalance"`
}

type UpdatedBalanceResponse struct {
	SenderBalance   decimal.Decimal `json:"senderBalance"`
	ReceiverBalance decimal.Decimal `json:"receiverBalance"`
}

type InfoResponse struct {
	FromCurrencyCode string `json:"fromCurrencyCode"`
	ToCurrencyCode   string `json:"toCurrencyCode"`
	FromVlasnik      int64  `json:"fromVlasnik"`
	ToVlasnik        int64  `json:"toVlasnik"`
	FromEmail        string `json:"fromEmail,omitempty"`
	FromUsername     string `json:"fromUsername,omitempty"`
}

type SifraDelatnostiResponse struct {
	Sifra string `json:"sifra"`
	Grana string `json:"grana"`
}

func (s *AccountService) ListSifraDelatnosti(ctx context.Context) ([]SifraDelatnostiResponse, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT sifra, grana FROM sifra_delatnosti_table ORDER BY sifra`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SifraDelatnostiResponse
	for rows.Next() {
		var item SifraDelatnostiResponse
		if err := rows.Scan(&item.Sifra, &item.Grana); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *AccountService) ListCurrencies(ctx context.Context) ([]CurrencyResponse, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, version, naziv, oznaka, simbol, opis, status
  FROM currency_table
 WHERE status = 'ACTIVE'
 ORDER BY id
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CurrencyResponse
	for rows.Next() {
		cur, err := scanCurrency(rows)
		if err != nil {
			return nil, err
		}
		cur.Countries, _ = s.currencyCountries(ctx, cur.ID)
		out = append(out, cur)
	}
	return out, rows.Err()
}

func (s *AccountService) ListCurrenciesPage(ctx context.Context, page, size int) (Page[CurrencyResponse], error) {
	items, total, err := s.queryCurrencies(ctx, page, size)
	if err != nil {
		return Page[CurrencyResponse]{}, err
	}
	return NewPage(items, page, size, total), nil
}

func (s *AccountService) GetCurrency(ctx context.Context, code string) (CurrencyResponse, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, version, naziv, oznaka, simbol, opis, status
  FROM currency_table
 WHERE oznaka = $1
`, strings.ToUpper(code))
	cur, err := scanCurrency(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CurrencyResponse{}, NotFound("Currency nije pronadjen")
		}
		return CurrencyResponse{}, err
	}
	cur.Countries, _ = s.currencyCountries(ctx, cur.ID)
	return cur, nil
}

func (s *AccountService) CreateCheckingAccount(ctx context.Context, principal Principal, req CheckingAccountRequest) (AccountDetailsResponse, error) {
	if strings.TrimSpace(req.NazivRacuna) == "" {
		return AccountDetailsResponse{}, BadRequest("Unesi naziv racuna")
	}
	if req.VrstaRacuna == "" {
		return AccountDetailsResponse{}, BadRequest("Unesi podvrstu racuna")
	}
	ownership, typeVal, err := accountConcrete(req.VrstaRacuna)
	if err != nil {
		return AccountDetailsResponse{}, err
	}
	if err := validateOwnerInput(req.IDVlasnika, req.JMBG); err != nil {
		return AccountDetailsResponse{}, err
	}
	if err := validateCompanyPresence(req.Firma, ownership); err != nil {
		return AccountDetailsResponse{}, err
	}

	client, err := s.resolveClient(ctx, req.IDVlasnika, req.JMBG)
	if err != nil {
		return AccountDetailsResponse{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AccountDetailsResponse{}, err
	}
	defer tx.Rollback()

	currencyID, err := s.currencyID(ctx, tx, "RSD")
	if err != nil {
		return AccountDetailsResponse{}, err
	}
	companyID, err := s.createCompanyIfNeeded(ctx, tx, req.Firma)
	if err != nil {
		return AccountDetailsResponse{}, err
	}
	number, err := account.GenerateMONAS(ctx, tx, typeVal, s.random)
	if err != nil {
		return AccountDetailsResponse{}, err
	}
	id, err := s.insertAccount(ctx, tx, insertAccountParams{
		AccountNumber:    number,
		DisplayName:      req.NazivRacuna,
		OwnerID:          client.ID,
		EmployeeID:       principal.ID,
		FirstName:        client.Name,
		LastName:         client.LastName,
		Username:         client.Username,
		Email:            client.Email,
		CurrencyID:       currencyID,
		Balance:          req.InitialBalance,
		AccountType:      "CHECKING",
		AccountConcrete:  req.VrstaRacuna,
		AccountOwnership: ownership,
		CompanyID:        companyID,
	})
	if err != nil {
		return AccountDetailsResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountDetailsResponse{}, err
	}
	s.afterAccountCreated(ctx, client, number, boolPtrTrue(req.CreateCard))
	return s.GetAccountDetailsByID(ctx, id, nil)
}

func (s *AccountService) CreateFXAccount(ctx context.Context, principal Principal, req FXAccountRequest) (AccountDetailsResponse, error) {
	if strings.TrimSpace(req.NazivRacuna) == "" {
		return AccountDetailsResponse{}, BadRequest("Ne sme biti prazan naziv racuna")
	}
	if strings.EqualFold(req.CurrencyCode, "RSD") {
		return AccountDetailsResponse{}, BadRequest("Ne moze RSD")
	}
	if err := validateOwnerInput(req.IDVlasnika, req.JMBG); err != nil {
		return AccountDetailsResponse{}, err
	}
	ownership := strings.ToUpper(req.TipRacuna)
	if ownership != "PERSONAL" && ownership != "BUSINESS" {
		return AccountDetailsResponse{}, BadRequest("Unesi tip racuna")
	}
	if err := validateCompanyPresence(req.Firma, ownership); err != nil {
		return AccountDetailsResponse{}, err
	}
	client, err := s.resolveClient(ctx, req.IDVlasnika, req.JMBG)
	if err != nil {
		return AccountDetailsResponse{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AccountDetailsResponse{}, err
	}
	defer tx.Rollback()
	currencyID, err := s.currencyID(ctx, tx, strings.ToUpper(req.CurrencyCode))
	if err != nil {
		return AccountDetailsResponse{}, err
	}
	companyID, err := s.createCompanyIfNeeded(ctx, tx, req.Firma)
	if err != nil {
		return AccountDetailsResponse{}, err
	}
	typeVal := map[string]string{"PERSONAL": "21", "BUSINESS": "22"}[ownership]
	number, err := account.GenerateMONAS(ctx, tx, typeVal, s.random)
	if err != nil {
		return AccountDetailsResponse{}, err
	}
	id, err := s.insertAccount(ctx, tx, insertAccountParams{
		AccountNumber:    number,
		DisplayName:      req.NazivRacuna,
		OwnerID:          client.ID,
		EmployeeID:       principal.ID,
		FirstName:        client.Name,
		LastName:         client.LastName,
		Username:         client.Username,
		Email:            client.Email,
		CurrencyID:       currencyID,
		Balance:          req.InitialBalance,
		AccountType:      "FX",
		AccountOwnership: ownership,
		CompanyID:        companyID,
	})
	if err != nil {
		return AccountDetailsResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountDetailsResponse{}, err
	}
	s.afterAccountCreated(ctx, client, number, boolPtrTrue(req.CreateCard))
	return s.GetAccountDetailsByID(ctx, id, nil)
}

func (s *AccountService) SearchAccounts(ctx context.Context, ime, prezime, accountNumber string, page, size int) (Page[AccountSearchResponse], error) {
	offset := page * size
	args := []any{"%" + strings.ToLower(strings.TrimSpace(nullToEmpty(accountNumber))) + "%",
		"%" + strings.ToLower(strings.TrimSpace(nullToEmpty(ime))) + "%",
		"%" + strings.ToLower(strings.TrimSpace(nullToEmpty(prezime))) + "%", size, offset}

	var total int
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
 FROM account_table a
 WHERE LOWER(a.broj_racuna) LIKE $1
   AND LOWER(a.ime_vlasnika_racuna) LIKE $2
   AND LOWER(a.prezime_vlasnika_racuna) LIKE $3
   AND a.deleted = false
`, args[:3]...).Scan(&total); err != nil {
		return Page[AccountSearchResponse]{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT a.id, a.broj_racuna, a.ime_vlasnika_racuna, a.prezime_vlasnika_racuna,
       COALESCE(a.account_ownership_type, ''), a.account_type, a.stanje,
       a.raspolozivo_stanje, COALESCE(c.oznaka, ''), a.status,
       a.vlasnik, a.zaposlen, COALESCE(a.dnevni_limit, 0),
       COALESCE(a.mesecni_limit, 0), COALESCE(a.dnevna_potrosnja, 0),
       COALESCE(a.mesecna_potrosnja, 0), a.datum_isteka
  FROM account_table a
 LEFT JOIN currency_table c ON c.id = a.currency_id
 WHERE LOWER(a.broj_racuna) LIKE $1
   AND LOWER(a.ime_vlasnika_racuna) LIKE $2
   AND LOWER(a.prezime_vlasnika_racuna) LIKE $3
   AND a.deleted = false
 ORDER BY a.prezime_vlasnika_racuna ASC, a.ime_vlasnika_racuna ASC, a.id ASC
 LIMIT $4 OFFSET $5
`, args...)
	if err != nil {
		return Page[AccountSearchResponse]{}, err
	}
	defer rows.Close()
	var content []AccountSearchResponse
	for rows.Next() {
		item, err := scanAccountSearch(rows)
		if err != nil {
			return Page[AccountSearchResponse]{}, err
		}
		content = append(content, item)
	}
	return NewPage(content, page, size, total), rows.Err()
}

func (s *AccountService) GetClientAccountsPage(ctx context.Context, ownerID int64, page, size int, details bool) (any, error) {
	offset := page * size
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM account_table WHERE vlasnik = $1 AND status = 'ACTIVE' AND deleted = false", ownerID).Scan(&total); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT a.id
  FROM account_table a
 WHERE a.vlasnik = $1 AND a.status = 'ACTIVE'
   AND a.deleted = false
 ORDER BY a.id
 LIMIT $2 OFFSET $3
`, ownerID, size, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if details {
		var content []AccountDetailsResponse
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			item, err := s.GetAccountDetailsByID(ctx, id, &ownerID)
			if err != nil {
				return nil, err
			}
			content = append(content, item)
		}
		return NewPage(content, page, size, total), rows.Err()
	}
	var content []AccountResponse
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		item, err := s.GetAccountResponseByID(ctx, id, &ownerID)
		if err != nil {
			return nil, err
		}
		content = append(content, item)
	}
	return NewPage(content, page, size, total), rows.Err()
}

func (s *AccountService) GetAccountDetailsByID(ctx context.Context, id int64, ownerGuard *int64) (AccountDetailsResponse, error) {
	row, err := s.loadAccountViewByID(ctx, id)
	if err != nil {
		return AccountDetailsResponse{}, err
	}
	if ownerGuard != nil && row.OwnerID != *ownerGuard {
		return AccountDetailsResponse{}, BadRequest("Nisi vlasnik racuna")
	}
	resp := row.details()
	resp.Cards = s.cardSummariesForAccount(ctx, row.AccountNumber)
	return resp, nil
}

func (s *AccountService) GetAccountDetailsByNumber(ctx context.Context, accountNumber string, ownerGuard *int64) (AccountDetailsResponse, error) {
	row, err := s.loadAccountViewByNumber(ctx, accountNumber)
	if err != nil {
		return AccountDetailsResponse{}, err
	}
	if ownerGuard != nil && row.OwnerID != *ownerGuard {
		return AccountDetailsResponse{}, BadRequest("Nisi vlasnik racuna")
	}
	resp := row.details()
	resp.Cards = s.cardSummariesForAccount(ctx, row.AccountNumber)
	return resp, nil
}

func (s *AccountService) GetAccountResponseByID(ctx context.Context, id int64, ownerGuard *int64) (AccountResponse, error) {
	row, err := s.loadAccountViewByID(ctx, id)
	if err != nil {
		return AccountResponse{}, err
	}
	if ownerGuard != nil && row.OwnerID != *ownerGuard {
		return AccountResponse{}, BadRequest("Nisi vlasnik racuna")
	}
	return row.summary(), nil
}

func (s *AccountService) EditAccountNameByID(ctx context.Context, principal Principal, id int64, req EditAccountNameRequest) (string, error) {
	return s.editAccountName(ctx, principal, "id", id, "", req)
}

func (s *AccountService) EditAccountNameByNumber(ctx context.Context, principal Principal, accountNumber string, req EditAccountNameRequest) (string, error) {
	return s.editAccountName(ctx, principal, "number", 0, accountNumber, req)
}

func (s *AccountService) EditAccountLimitByID(ctx context.Context, principal Principal, id int64, req EditAccountLimitRequest) (string, error) {
	return s.editAccountLimit(ctx, principal, "id", id, "", req)
}

func (s *AccountService) EditAccountLimitByNumber(ctx context.Context, principal Principal, accountNumber string, req EditAccountLimitRequest) (string, error) {
	return s.editAccountLimit(ctx, principal, "number", 0, accountNumber, req)
}

func (s *AccountService) EditStatus(ctx context.Context, accountNumber string, req EditStatusRequest) (string, error) {
	status := strings.ToUpper(req.Status)
	if status != "ACTIVE" && status != "INACTIVE" {
		return "", BadRequest("Neispravan status")
	}
	row, err := s.loadAccountViewByNumber(ctx, accountNumber)
	if err != nil {
		return "", err
	}
	if row.OwnerID < 0 {
		return "", BadRequest("Sistemski racun banke se ne moze deaktivirati")
	}
	_, err = s.db.ExecContext(ctx, "UPDATE account_table SET status = $1, updated_at = now() WHERE broj_racuna = $2 AND deleted = false", status, accountNumber)
	if err != nil {
		return "", err
	}
	s.afterAccountStatusChanged(ctx, row, status)
	return "Uspesno editovan status", nil
}

func (s *AccountService) GetBankAccounts(ctx context.Context) ([]AccountDetailsResponse, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM account_table WHERE vlasnik = -1 ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccountDetailsResponse
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		item, err := s.GetAccountDetailsByID(ctx, id, nil)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *AccountService) GetBankAccountDetailsByCurrency(ctx context.Context, currency string) (AccountDetailsResponse, error) {
	acc, err := s.accountsByOwnerCurrencyView(ctx, -1, strings.ToUpper(currency))
	if err != nil {
		return AccountDetailsResponse{}, err
	}
	return acc.details(), nil
}

func (s *AccountService) GetCompany(ctx context.Context, id int64) (CompanyResponse, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT c.id, c.naziv, c.maticni_broj, c.poreski_broj, sd.sifra, COALESCE(c.adresa, ''), c.vlasnik
  FROM company_table c
  JOIN sifra_delatnosti_table sd ON sd.id = c.sifra_delatnosti_id
 WHERE c.id = $1
`, id)
	var out CompanyResponse
	if err := row.Scan(&out.ID, &out.Naziv, &out.MaticniBroj, &out.PoreskiBroj, &out.SifraDelatnosti, &out.Adresa, &out.Vlasnik); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CompanyResponse{}, NotFound("Ne postoji firma sa id: %d", id)
		}
		return CompanyResponse{}, err
	}
	return out, nil
}

func (s *AccountService) UpdateCompany(ctx context.Context, id int64, req UpdateCompanyRequest) (CompanyResponse, error) {
	sifraID, err := s.sifraID(ctx, s.db, req.SifraDelatnosti)
	if err != nil {
		return CompanyResponse{}, err
	}
	if req.Vlasnik != nil {
		_, err = s.db.ExecContext(ctx, `
UPDATE company_table
   SET naziv = $1, sifra_delatnosti_id = $2, adresa = $3, vlasnik = $4
 WHERE id = $5
`, req.Naziv, sifraID, req.Adresa, *req.Vlasnik, id)
	} else {
		_, err = s.db.ExecContext(ctx, `
UPDATE company_table
   SET naziv = $1, sifra_delatnosti_id = $2, adresa = $3
 WHERE id = $4
`, req.Naziv, sifraID, req.Adresa, id)
	}
	if err != nil {
		return CompanyResponse{}, err
	}
	return s.GetCompany(ctx, id)
}

func (s *AccountService) Info(ctx context.Context, fromNumber, toNumber string) (InfoResponse, error) {
	from, err := s.loadAccountViewByNumber(ctx, fromNumber)
	if err != nil {
		return InfoResponse{}, BadRequest("Ne postoji from racun")
	}
	if from.Status == "INACTIVE" {
		return InfoResponse{}, BadRequest("FromAccount nije aktivan")
	}
	to, err := s.loadAccountViewByNumber(ctx, toNumber)
	if err != nil {
		return InfoResponse{}, BadRequest("Ne postoji to racun")
	}
	if to.Status == "INACTIVE" {
		return InfoResponse{}, BadRequest("ToAccount nije aktivan")
	}
	return InfoResponse{
		FromCurrencyCode: from.Currency,
		ToCurrencyCode:   to.Currency,
		FromVlasnik:      from.OwnerID,
		ToVlasnik:        to.OwnerID,
		FromEmail:        from.Email,
		FromUsername:     from.Username,
	}, nil
}

func (s *AccountService) Transaction(ctx context.Context, req PaymentRequest, sameOwnerRequired bool) (UpdatedBalanceResponse, error) {
	resp, err := s.applyPaymentOperation(ctx, req, sameOwnerRequired)
	if err != nil {
		return UpdatedBalanceResponse{}, err
	}
	// Java: balansni update commit-uje u svojoj tx; recordPayment je odvojen i best-effort.
	s.recordTransferPayment(ctx, req, sameOwnerRequired)
	return resp, nil
}

func (s *AccountService) ApplyPaymentWithoutRecord(ctx context.Context, req PaymentRequest, sameOwnerRequired bool) (UpdatedBalanceResponse, error) {
	return s.applyPaymentOperation(ctx, req, sameOwnerRequired)
}

// recordTransferPayment upisuje payment_table audit red ("Transfer"/"Payment") u
// odvojenoj transakciji. Best-effort: neuspeh ne utice na vec commit-ovan balans.
func (s *AccountService) recordTransferPayment(ctx context.Context, req PaymentRequest, sameOwner bool) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	from, err := s.getByNumber(ctx, tx, req.FromAccountNumber, false)
	if err != nil {
		return
	}
	to, err := s.getByNumber(ctx, tx, req.ToAccountNumber, false)
	if err != nil {
		return
	}
	purpose := "Payment"
	if sameOwner {
		purpose = "Transfer"
	}
	_ = s.recordPayment(ctx, tx, from, to, req.FromAmount, req.ToAmount, req.Commission, purpose, "")
	_ = tx.Commit()
}

func (s *AccountService) applyPaymentOperation(ctx context.Context, req PaymentRequest, sameOwnerRequired bool) (UpdatedBalanceResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return UpdatedBalanceResponse{}, err
	}
	defer tx.Rollback()

	from, err := s.accountsForTransfer(ctx, tx, req.FromAccountNumber)
	if err != nil {
		return UpdatedBalanceResponse{}, err
	}
	to, err := s.accountsForTransfer(ctx, tx, req.ToAccountNumber)
	if err != nil {
		return UpdatedBalanceResponse{}, err
	}
	if req.ClientID == 0 {
		return UpdatedBalanceResponse{}, BadRequest("Unesi id clienta")
	}
	if from.OwnerID != req.ClientID {
		return UpdatedBalanceResponse{}, BadRequest("Nisi vlasnik racuna")
	}
	if sameOwnerRequired && from.OwnerID != to.OwnerID {
		return UpdatedBalanceResponse{}, BadRequest("Transfer se moze odvijati samo za racune istog vlasnika")
	}
	if !sameOwnerRequired && from.OwnerID == to.OwnerID {
		return UpdatedBalanceResponse{}, BadRequest("Tranzakcija se ne moze odvijati za racune istog vlasnike")
	}
	if err := s.applyPaymentTx(ctx, tx, from, to, req); err != nil {
		return UpdatedBalanceResponse{}, err
	}
	finalFrom, _ := s.getByNumber(ctx, tx, from.AccountNumber, false)
	finalTo, _ := s.getByNumber(ctx, tx, to.AccountNumber, false)
	if err := tx.Commit(); err != nil {
		return UpdatedBalanceResponse{}, err
	}
	return UpdatedBalanceResponse{SenderBalance: finalFrom.BookedBalance, ReceiverBalance: finalTo.BookedBalance}, nil
}

func (s *AccountService) TransactionFromBank(ctx context.Context, req BankPaymentRequest) error {
	if req.FromAccountNumber == "" && req.ToAccountNumber == "" {
		return BadRequest("Los unos")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var sender, recipient accountBalanceRow
	if req.FromAccountNumber == "" {
		recipient, err = s.getByNumber(ctx, tx, req.ToAccountNumber, true)
		if err != nil {
			return err
		}
		sender, err = s.getByOwnerAndCurrency(ctx, tx, -1, recipient.Currency, true)
	} else {
		sender, err = s.getByNumber(ctx, tx, req.FromAccountNumber, true)
		if err != nil {
			return err
		}
		recipient, err = s.getByOwnerAndCurrency(ctx, tx, -1, sender.Currency, true)
	}
	if err != nil {
		return err
	}
	if err := s.DebitTx(ctx, tx, sender.AccountNumber, req.Amount, sender.OwnerID); err != nil {
		return err
	}
	if err := s.CreditTx(ctx, tx, recipient.AccountNumber, req.Amount, recipient.OwnerID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *AccountService) CreditBank(ctx context.Context, currency string, amount decimal.Decimal) error {
	acc, err := s.GetBankAccount(ctx, strings.ToUpper(currency))
	if err != nil {
		return err
	}
	return s.Credit(ctx, acc.AccountNumber, amount, acc.OwnerID)
}

func (s *AccountService) DebitBank(ctx context.Context, currency string, amount decimal.Decimal) error {
	acc, err := s.GetBankAccount(ctx, strings.ToUpper(currency))
	if err != nil {
		return err
	}
	return s.Debit(ctx, acc.AccountNumber, amount, acc.OwnerID)
}

func (s *AccountService) ExchangeBuy(ctx context.Context, req OneSidedTransactionRequest) (UpdatedBalanceResponse, error) {
	return s.exchangeOneSided(ctx, req, true)
}

func (s *AccountService) ExchangeSell(ctx context.Context, req OneSidedTransactionRequest) (UpdatedBalanceResponse, error) {
	return s.exchangeOneSided(ctx, req, false)
}

// exchangeOneSided portuje exchangeBuy/exchangeSell iz AccountServiceImplementation:
// jednostrani debit/credit klijentskog racuna (trade-leg) + audit zapis u payment_table
// ("Stock purchase"/"Stock sale"). Balans se azurira u svojoj transakciji, a payment
// zapis je best-effort u odvojenoj transakciji (Java: "balance update has already
// committed ... must not be rolled back").
func (s *AccountService) exchangeOneSided(ctx context.Context, req OneSidedTransactionRequest, buy bool) (UpdatedBalanceResponse, error) {
	account, err := s.resolveOneSidedAccount(ctx, req)
	if err != nil {
		return UpdatedBalanceResponse{}, err
	}
	if req.Amount.Sign() <= 0 {
		return UpdatedBalanceResponse{}, BadRequest("Iznos mora biti pozitivan")
	}
	if buy {
		if err := s.Debit(ctx, account.AccountNumber, req.Amount, account.OwnerID); err != nil {
			return UpdatedBalanceResponse{}, err
		}
	} else {
		if err := s.Credit(ctx, account.AccountNumber, req.Amount, account.OwnerID); err != nil {
			return UpdatedBalanceResponse{}, err
		}
	}
	s.recordExchangePayment(ctx, account.AccountNumber, account.Currency, req.Amount, buy)
	acc, _ := s.getByNumber(ctx, s.db, account.AccountNumber, false)
	return UpdatedBalanceResponse{SenderBalance: acc.AvailableBalance}, nil
}

// recordExchangePayment upisuje payment_table audit red za exchange buy/sell.
// Best-effort: neuspeh se ignorise i ne utice na vec commit-ovan balans.
func (s *AccountService) recordExchangePayment(ctx context.Context, accountNumber, currency string, amount decimal.Decimal, buy bool) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	account, err := s.getByNumber(ctx, tx, accountNumber, false)
	if err != nil {
		return
	}
	bank, err := s.getByOwnerAndCurrency(ctx, tx, -1, currency, false)
	if err != nil {
		return
	}
	if buy {
		_ = s.recordPayment(ctx, tx, account, bank, amount, amount, decimal.Zero, "Stock purchase", "")
	} else {
		_ = s.recordPayment(ctx, tx, bank, account, amount, amount, decimal.Zero, "Stock sale", "")
	}
	_ = tx.Commit()
}

func (s *AccountService) CreateSystemAccount(ctx context.Context, req CreateSystemAccountRequest) (AccountDetails, error) {
	existing, err := s.GetByNumber(ctx, req.AccountNumber)
	if err == nil {
		return existing, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AccountDetails{}, err
	}
	defer tx.Rollback()
	currencyID, err := s.currencyID(ctx, tx, strings.ToUpper(req.CurrencyCode))
	if err != nil {
		return AccountDetails{}, err
	}
	ownership, _, err := accountConcrete(req.AccountConcrete)
	if err != nil {
		return AccountDetails{}, err
	}
	_, err = s.insertAccount(ctx, tx, insertAccountParams{
		AccountNumber:    req.AccountNumber,
		DisplayName:      req.DisplayName,
		OwnerID:          req.OwnerID,
		EmployeeID:       -1,
		FirstName:        "SYSTEM",
		LastName:         req.DisplayName,
		Username:         fmt.Sprintf("system-%d", req.OwnerID),
		Email:            fmt.Sprintf("system+%d@banka1.local", req.OwnerID),
		CurrencyID:       currencyID,
		Balance:          req.InitialBalance,
		AccountType:      "CHECKING",
		AccountConcrete:  req.AccountConcrete,
		AccountOwnership: ownership,
	})
	if err != nil {
		return AccountDetails{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountDetails{}, err
	}
	return s.GetByNumber(ctx, req.AccountNumber)
}

// helpers continue below

type clientInfo struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	LastName string `json:"lastName"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

func (s *AccountService) resolveClient(ctx context.Context, id *int64, jmbg string) (clientInfo, error) {
	if id == nil && strings.TrimSpace(jmbg) == "" {
		return clientInfo{}, BadRequest("Unesi id ili jmbg")
	}
	path := ""
	if id != nil {
		path = fmt.Sprintf("/clients/customers/%d", *id)
	} else {
		path = "/clients/customers/jmbg/" + url.PathEscape(jmbg)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.cfg.UserServiceURL, "/")+path, nil)
	if err != nil {
		return clientInfo{}, err
	}
	if token, err := s.serviceToken(); err == nil {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.http.Do(req)
	if err == nil && resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var out clientInfo
			if err := json.NewDecoder(resp.Body).Decode(&out); err == nil && out.ID != 0 {
				if out.Username == "" {
					out.Username = out.Name + "_" + out.LastName
				}
				return out, nil
			}
		}
	}
	return clientInfo{}, Internal("Greska sa komunikacijom izmedju servisa")
}

type insertAccountParams struct {
	AccountNumber    string
	DisplayName      string
	OwnerID          int64
	EmployeeID       int64
	FirstName        string
	LastName         string
	Username         string
	Email            string
	CurrencyID       int64
	Balance          decimal.Decimal
	AccountType      string
	AccountConcrete  string
	AccountOwnership string
	CompanyID        sql.NullInt64
}

func (s *AccountService) insertAccount(ctx context.Context, tx *sql.Tx, p insertAccountParams) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
INSERT INTO account_table (
    broj_racuna, ime_vlasnika_racuna, prezime_vlasnika_racuna, email, username,
    naziv_racuna, vlasnik, zaposlen, stanje, raspolozivo_stanje,
    datum_i_vreme_kreiranja, datum_isteka, currency_id, status,
    dnevni_limit, mesecni_limit, dnevna_potrosnja, mesecna_potrosnja,
    company_id, account_type, account_concrete, account_ownership_type
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $9,
    NOW(), CURRENT_DATE + INTERVAL '5 years', $10, 'ACTIVE',
    250000.00, 1000000.00, 0, 0,
    $11, $12, NULLIF($13, ''), NULLIF($14, '')
)
RETURNING id
`, p.AccountNumber, p.FirstName, p.LastName, p.Email, p.Username,
		p.DisplayName, p.OwnerID, p.EmployeeID, p.Balance,
		p.CurrencyID, p.CompanyID, p.AccountType, p.AccountConcrete, p.AccountOwnership).Scan(&id)
	return id, err
}

func (s *AccountService) currencyID(ctx context.Context, runner sqlRunner, code string) (int64, error) {
	var id int64
	var status string
	err := runner.QueryRowContext(ctx, "SELECT id, status FROM currency_table WHERE oznaka = $1", strings.ToUpper(code)).Scan(&id, &status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, BadRequest("Nisu unete valute")
		}
		return 0, err
	}
	if status == "INACTIVE" {
		return 0, BadRequest("Deaktivirana valuta")
	}
	return id, nil
}

func (s *AccountService) createCompanyIfNeeded(ctx context.Context, tx *sql.Tx, req *CompanyRequest) (sql.NullInt64, error) {
	if req == nil {
		return sql.NullInt64{}, nil
	}
	sifraID, err := s.sifraID(ctx, tx, req.SifraDelatnosti)
	if err != nil {
		return sql.NullInt64{}, err
	}
	var id int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO company_table (naziv, maticni_broj, poreski_broj, sifra_delatnosti_id, adresa, vlasnik)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id
`, req.Naziv, req.MaticniBroj, req.PoreskiBroj, sifraID, req.Adresa, req.Vlasnik).Scan(&id)
	return sql.NullInt64{Int64: id, Valid: true}, err
}

func (s *AccountService) sifraID(ctx context.Context, runner sqlRunner, code string) (int64, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(code), ".", "")
	var id int64
	err := runner.QueryRowContext(ctx, "SELECT id FROM sifra_delatnosti_table WHERE sifra = $1", normalized).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, BadRequest("Nije uneta sifra delatnosti")
		}
		return 0, err
	}
	return id, nil
}

func validateOwnerInput(id *int64, jmbg string) error {
	if id == nil && strings.TrimSpace(jmbg) == "" {
		return BadRequest("Unesi id ili jmbg")
	}
	return nil
}

func validateCompanyPresence(company *CompanyRequest, ownership string) error {
	if (company == nil && ownership == "BUSINESS") || (company != nil && ownership == "PERSONAL") {
		return BadRequest("Pogresan tip racuna")
	}
	return nil
}

func accountConcrete(value string) (ownership string, typeVal string, err error) {
	switch strings.ToUpper(value) {
	case "STANDARDNI":
		return "PERSONAL", "11", nil
	case "STEDNI":
		return "PERSONAL", "13", nil
	case "PENZIONERSKI":
		return "PERSONAL", "14", nil
	case "ZA_MLADE":
		return "PERSONAL", "15", nil
	case "ZA_STUDENTE":
		return "PERSONAL", "16", nil
	case "ZA_NEZAPOSLENE":
		return "PERSONAL", "17", nil
	case "DOO", "AD", "FONDACIJA":
		return "BUSINESS", "12", nil
	default:
		return "", "", BadRequest("Nepoznata vrsta racuna")
	}
}

func nullToEmpty(value string) string {
	return value
}

func boolPtrTrue(value *bool) bool {
	return value != nil && *value
}
