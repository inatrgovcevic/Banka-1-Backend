package clients

import "github.com/shopspring/decimal"

// StockListing mirrors order-service StockListingDto (the market-service listing
// response). Stock-service serializes the PK as "listingId". Nullable option
// fields are pointers.
type StockListing struct {
	ID                  int64            `json:"listingId"`
	Ticker              *string          `json:"ticker"`
	Name                *string          `json:"name"`
	Price               *decimal.Decimal `json:"price"`
	Ask                 *decimal.Decimal `json:"ask"`
	Bid                 *decimal.Decimal `json:"bid"`
	CurrencyRaw         *string          `json:"currency"`
	ListingType         *string          `json:"listingType"`
	Volume              *int64           `json:"volume"`
	SettlementDate      *string          `json:"settlementDate"` // ISO_LOCAL_DATE "2006-01-02"
	UnderlyingListingID *int64           `json:"underlyingListingId"`
	StrikePrice         *decimal.Decimal `json:"strikePrice"`
	OptionType          *string          `json:"optionType"`
	// P3 order fields. Stock-service serializes the exchange PK as
	// "stockExchangeId" (matches StockListingDto.@JsonProperty).
	ExchangeID        *int64           `json:"stockExchangeId"`
	ContractSize      *int             `json:"contractSize"`
	UnderlyingPrice   *decimal.Decimal `json:"underlyingPrice"`
	MaintenanceMargin *decimal.Decimal `json:"maintenanceMargin"`
}

// ContractSizeOr returns the listing contract size, or def when stock-service
// omitted it (Java treats the Integer as set; this guards a null).
func (l StockListing) ContractSizeOr(def int) int {
	if l.ContractSize == nil {
		return def
	}
	return *l.ContractSize
}

// ListingTypeOr returns the listing type or def (Java defaults a null
// listingType to STOCK in the margin/portfolio paths).
func (l StockListing) ListingTypeOr(def string) string {
	if l.ListingType == nil {
		return def
	}
	return *l.ListingType
}

// currencyNameToISO mirrors StockListingDto.CURRENCY_NAME_TO_ISO so getCurrency()
// behaves identically when resolving the option's settlement currency.
var currencyNameToISO = map[string]string{
	"United States Dollar":   "USD",
	"US Dollar":              "USD",
	"Euro":                   "EUR",
	"British Pound":          "GBP",
	"British Pound Sterling": "GBP",
	"Japanese Yen":           "JPY",
	"Canadian Dollar":        "CAD",
	"Australian Dollar":      "AUD",
	"Swiss Franc":            "CHF",
	"Serbian Dinar":          "RSD",
}

// Currency normalizes a currency name to its ISO code, mirroring
// StockListingDto.getCurrency().
func (l StockListing) Currency() string {
	if l.CurrencyRaw == nil {
		return ""
	}
	if iso, ok := currencyNameToISO[*l.CurrencyRaw]; ok {
		return iso
	}
	return *l.CurrencyRaw
}

// ExchangeRate mirrors order-service ExchangeRateDto. The market-service response
// may carry the converted amount as "convertedAmount" or "toAmount" (@JsonAlias).
type ExchangeRate struct {
	ConvertedAmount *decimal.Decimal `json:"convertedAmount"`
	ToAmount        *decimal.Decimal `json:"toAmount"`
	Commission      *decimal.Decimal `json:"commission"`
}

// Converted returns the converted amount, preferring convertedAmount then toAmount.
func (e ExchangeRate) Converted() *decimal.Decimal {
	if e.ConvertedAmount != nil {
		return e.ConvertedAmount
	}
	return e.ToAmount
}

// AccountDetails mirrors order-service AccountDetailsDto. accountNumber may arrive
// as "accountNumber" or "brojRacuna" (@JsonAlias); balance may arrive as
// "balance"/"availableBalance"/"raspolozivoStanje"/"stanjeRacuna" (@JsonAlias).
type AccountDetails struct {
	AccountNumber   *string          `json:"accountNumber"`
	BrojRacuna      *string          `json:"brojRacuna"`
	Currency        *string          `json:"currency"`
	OwnerID         *int64           `json:"ownerId"`
	AvailableCredit *decimal.Decimal `json:"availableCredit"`
	// Balance aliases — first non-nil wins (see BalanceOrZero).
	Balance           *decimal.Decimal `json:"balance"`
	AvailableBalance  *decimal.Decimal `json:"availableBalance"`
	RaspolozivoStanje *decimal.Decimal `json:"raspolozivoStanje"`
	StanjeRacuna      *decimal.Decimal `json:"stanjeRacuna"`
}

// Number returns the account number from whichever alias is populated.
func (a AccountDetails) Number() string {
	if a.AccountNumber != nil {
		return *a.AccountNumber
	}
	if a.BrojRacuna != nil {
		return *a.BrojRacuna
	}
	return ""
}

// CurrencyOrEmpty returns the account currency or "".
func (a AccountDetails) CurrencyOrEmpty() string {
	if a.Currency != nil {
		return *a.Currency
	}
	return ""
}

// BalanceOrZero returns the balance from whichever @JsonAlias key is populated,
// or zero (mirrors Java's `balance == null ? ZERO : balance`).
func (a AccountDetails) BalanceOrZero() decimal.Decimal {
	for _, v := range []*decimal.Decimal{a.Balance, a.AvailableBalance, a.RaspolozivoStanje, a.StanjeRacuna} {
		if v != nil {
			return *v
		}
	}
	return decimal.Zero
}

// AvailableCreditOrZero returns availableCredit or zero.
func (a AccountDetails) AvailableCreditOrZero() decimal.Decimal {
	if a.AvailableCredit != nil {
		return *a.AvailableCredit
	}
	return decimal.Zero
}

// OwnerIDValue returns the owner id or 0.
func (a AccountDetails) OwnerIDValue() int64 {
	if a.OwnerID != nil {
		return *a.OwnerID
	}
	return 0
}

// BankAccount mirrors order-service BankAccountDto: the bank account PK, which may
// arrive as "accountId" or "id" (@JsonAlias).
type BankAccount struct {
	AccountID *int64 `json:"accountId"`
	ID        *int64 `json:"id"`
}

// ResolvedID returns the account PK from whichever alias is populated (0 if none).
func (b BankAccount) ResolvedID() int64 {
	if b.AccountID != nil {
		return *b.AccountID
	}
	if b.ID != nil {
		return *b.ID
	}
	return 0
}

// Employee mirrors order-service EmployeeDto (user-service employee record).
type Employee struct {
	ID        int64   `json:"id"`
	Ime       *string `json:"ime"`
	Prezime   *string `json:"prezime"`
	Email     *string `json:"email"`
	Username  *string `json:"username"`
	Pozicija  *string `json:"pozicija"`
	Departman *string `json:"departman"`
	Aktivan   bool    `json:"aktivan"`
	Role      *string `json:"role"`
}

// EmployeePage mirrors order-service EmployeePageResponse.
type EmployeePage struct {
	Content       []Employee `json:"content"`
	TotalPages    int        `json:"totalPages"`
	TotalElements int64      `json:"totalElements"`
}

// Payment mirrors order-service dto.client.PaymentDto — the body POSTed to
// banking-core /internal/accounts/transaction. Field names are load-bearing.
type Payment struct {
	FromAccountNumber string          `json:"fromAccountNumber"`
	ToAccountNumber   string          `json:"toAccountNumber"`
	FromAmount        decimal.Decimal `json:"fromAmount"`
	ToAmount          decimal.Decimal `json:"toAmount"`
	Commission        decimal.Decimal `json:"commission"`
	ClientID          int64           `json:"clientId"`
}

// ExchangeStatus mirrors order-service ExchangeStatusDto: GET
// /api/stock-exchanges/{id}/status. All three flags are nullable.
type ExchangeStatus struct {
	Open       *bool `json:"open"`
	AfterHours *bool `json:"afterHours"`
	Closed     *bool `json:"closed"`
}

// IsAfterHours mirrors ExchangeStatusDto.isAfterHours().
func (e ExchangeStatus) IsAfterHours() bool {
	return e.AfterHours != nil && *e.AfterHours
}

// IsClosed mirrors OrderCreationServiceImpl.resolveExchangeWindow: closed when
// closed==true OR open==false.
func (e ExchangeStatus) IsClosed() bool {
	return (e.Closed != nil && *e.Closed) || (e.Open != nil && !*e.Open)
}

// OneSidedTransaction mirrors order-service dto.client.OneSidedTransactionDto —
// the body POSTed to banking-core /internal/accounts/exchange/{buy,sell} for the
// one-sided trade leg (GHI #199). Field names are load-bearing.
type OneSidedTransaction struct {
	AccountNumber string          `json:"accountNumber"`
	AccountID     int64           `json:"accountId"`
	Amount        decimal.Decimal `json:"amount"`
	ClientID      int64           `json:"clientId"`
	Description   string          `json:"description"`
}

// Customer mirrors order-service CustomerDto (the user-service /clients/customers
// record). order-service declares @JsonAlias on the name fields
// (firstName/name/ime, lastName/prezime), so the alias keys are carried as
// pointers and First()/Last() reproduce that precedence.
type Customer struct {
	ID        int64   `json:"id"`
	FirstName *string `json:"firstName"`
	Name      *string `json:"name"`
	Ime       *string `json:"ime"`
	LastName  *string `json:"lastName"`
	Prezime   *string `json:"prezime"`
	Email     *string `json:"email"`
}

// First returns the resolved first name (firstName, then name, then ime), or nil
// — mirroring CustomerDto.getFirstName() with its @JsonAlias keys. The pointer is
// preserved so a tax tracking row renders JSON null exactly as Java does.
func (c Customer) First() *string {
	for _, v := range []*string{c.FirstName, c.Name, c.Ime} {
		if v != nil {
			return v
		}
	}
	return nil
}

// Last returns the resolved last name (lastName, then prezime), or nil.
func (c Customer) Last() *string {
	for _, v := range []*string{c.LastName, c.Prezime} {
		if v != nil {
			return v
		}
	}
	return nil
}

// CustomerPage mirrors order-service CustomerPageResponse (the paginated
// /clients/customers response).
type CustomerPage struct {
	Content       []Customer `json:"content"`
	TotalPages    int        `json:"totalPages"`
	TotalElements int64      `json:"totalElements"`
}
