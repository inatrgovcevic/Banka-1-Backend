package clients

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func sp(s string) *string { return &s }
func ip(i int) *int       { return &i }
func lp(v int64) *int64   { return &v }
func bp(b bool) *bool      { return &b }
func dp(s string) *decimal.Decimal {
	d := decimal.RequireFromString(s)
	return &d
}

// ---------------------------------------------------------------------------
// StockListing
// ---------------------------------------------------------------------------

func TestContractSizeOr_Nil_ReturnsDefault(t *testing.T) {
	t.Parallel()
	l := StockListing{}
	assert.Equal(t, 1, l.ContractSizeOr(1))
}

func TestContractSizeOr_Set_ReturnsValue(t *testing.T) {
	t.Parallel()
	l := StockListing{ContractSize: ip(100)}
	assert.Equal(t, 100, l.ContractSizeOr(1))
}

func TestListingTypeOr_Nil_ReturnsDefault(t *testing.T) {
	t.Parallel()
	l := StockListing{}
	assert.Equal(t, "STOCK", l.ListingTypeOr("STOCK"))
}

func TestListingTypeOr_Set_ReturnsValue(t *testing.T) {
	t.Parallel()
	l := StockListing{ListingType: sp("FOREX")}
	assert.Equal(t, "FOREX", l.ListingTypeOr("STOCK"))
}

func TestCurrency_Nil_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	l := StockListing{}
	assert.Empty(t, l.Currency())
}

func TestCurrency_KnownName_ReturnsISO(t *testing.T) {
	t.Parallel()
	l := StockListing{CurrencyRaw: sp("United States Dollar")}
	assert.Equal(t, "USD", l.Currency())
}

func TestCurrency_UnknownName_ReturnsRaw(t *testing.T) {
	t.Parallel()
	l := StockListing{CurrencyRaw: sp("MXN")}
	assert.Equal(t, "MXN", l.Currency())
}

func TestCurrency_Euro_ReturnsEUR(t *testing.T) {
	t.Parallel()
	l := StockListing{CurrencyRaw: sp("Euro")}
	assert.Equal(t, "EUR", l.Currency())
}

// ---------------------------------------------------------------------------
// ExchangeRate.Converted
// ---------------------------------------------------------------------------

func TestConverted_PrefersConvertedAmount(t *testing.T) {
	t.Parallel()
	rate := ExchangeRate{ConvertedAmount: dp("100"), ToAmount: dp("200")}
	assert.Equal(t, "100", rate.Converted().String())
}

func TestConverted_FallsBackToToAmount(t *testing.T) {
	t.Parallel()
	rate := ExchangeRate{ToAmount: dp("200")}
	assert.Equal(t, "200", rate.Converted().String())
}

func TestConverted_BothNil_ReturnsNil(t *testing.T) {
	t.Parallel()
	rate := ExchangeRate{}
	assert.Nil(t, rate.Converted())
}

// ---------------------------------------------------------------------------
// AccountDetails
// ---------------------------------------------------------------------------

func TestNumber_AccountNumber_ReturnsIt(t *testing.T) {
	t.Parallel()
	a := AccountDetails{AccountNumber: sp("ACC001")}
	assert.Equal(t, "ACC001", a.Number())
}

func TestNumber_FallsBackToBrojRacuna(t *testing.T) {
	t.Parallel()
	a := AccountDetails{BrojRacuna: sp("BR001")}
	assert.Equal(t, "BR001", a.Number())
}

func TestNumber_BothNil_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	a := AccountDetails{}
	assert.Empty(t, a.Number())
}

func TestCurrencyOrEmpty_Nil_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	a := AccountDetails{}
	assert.Empty(t, a.CurrencyOrEmpty())
}

func TestCurrencyOrEmpty_Set_ReturnsValue(t *testing.T) {
	t.Parallel()
	a := AccountDetails{Currency: sp("USD")}
	assert.Equal(t, "USD", a.CurrencyOrEmpty())
}

func TestBalanceOrZero_PrefersBalance(t *testing.T) {
	t.Parallel()
	a := AccountDetails{Balance: dp("500"), AvailableBalance: dp("300")}
	assert.Equal(t, "500", a.BalanceOrZero().String())
}

func TestBalanceOrZero_FallsBackToAvailableBalance(t *testing.T) {
	t.Parallel()
	a := AccountDetails{AvailableBalance: dp("300")}
	assert.Equal(t, "300", a.BalanceOrZero().String())
}

func TestBalanceOrZero_FallsBackToRaspolozivo(t *testing.T) {
	t.Parallel()
	a := AccountDetails{RaspolozivoStanje: dp("200")}
	assert.Equal(t, "200", a.BalanceOrZero().String())
}

func TestBalanceOrZero_FallsBackToStanje(t *testing.T) {
	t.Parallel()
	a := AccountDetails{StanjeRacuna: dp("100")}
	assert.Equal(t, "100", a.BalanceOrZero().String())
}

func TestBalanceOrZero_AllNil_ReturnsZero(t *testing.T) {
	t.Parallel()
	a := AccountDetails{}
	assert.True(t, a.BalanceOrZero().IsZero())
}

func TestAvailableCreditOrZero_Set(t *testing.T) {
	t.Parallel()
	a := AccountDetails{AvailableCredit: dp("1000")}
	assert.Equal(t, "1000", a.AvailableCreditOrZero().String())
}

func TestAvailableCreditOrZero_Nil_ReturnsZero(t *testing.T) {
	t.Parallel()
	a := AccountDetails{}
	assert.True(t, a.AvailableCreditOrZero().IsZero())
}

func TestOwnerIDValue_Set(t *testing.T) {
	t.Parallel()
	a := AccountDetails{OwnerID: lp(42)}
	assert.Equal(t, int64(42), a.OwnerIDValue())
}

func TestOwnerIDValue_Nil_ReturnsZero(t *testing.T) {
	t.Parallel()
	a := AccountDetails{}
	assert.Equal(t, int64(0), a.OwnerIDValue())
}

// ---------------------------------------------------------------------------
// BankAccount.ResolvedID
// ---------------------------------------------------------------------------

func TestResolvedID_AccountID_ReturnsIt(t *testing.T) {
	t.Parallel()
	b := BankAccount{AccountID: lp(5)}
	assert.Equal(t, int64(5), b.ResolvedID())
}

func TestResolvedID_FallsBackToID(t *testing.T) {
	t.Parallel()
	b := BankAccount{ID: lp(10)}
	assert.Equal(t, int64(10), b.ResolvedID())
}

func TestResolvedID_BothNil_ReturnsZero(t *testing.T) {
	t.Parallel()
	b := BankAccount{}
	assert.Equal(t, int64(0), b.ResolvedID())
}

// ---------------------------------------------------------------------------
// ExchangeStatus
// ---------------------------------------------------------------------------

func TestIsAfterHours_True(t *testing.T) {
	t.Parallel()
	e := ExchangeStatus{AfterHours: bp(true)}
	assert.True(t, e.IsAfterHours())
}

func TestIsAfterHours_False(t *testing.T) {
	t.Parallel()
	e := ExchangeStatus{AfterHours: bp(false)}
	assert.False(t, e.IsAfterHours())
}

func TestIsAfterHours_Nil_ReturnsFalse(t *testing.T) {
	t.Parallel()
	e := ExchangeStatus{}
	assert.False(t, e.IsAfterHours())
}

func TestIsClosed_ClosedTrue(t *testing.T) {
	t.Parallel()
	e := ExchangeStatus{Closed: bp(true)}
	assert.True(t, e.IsClosed())
}

func TestIsClosed_OpenFalse(t *testing.T) {
	t.Parallel()
	e := ExchangeStatus{Open: bp(false)}
	assert.True(t, e.IsClosed())
}

func TestIsClosed_OpenTrue_NotClosed(t *testing.T) {
	t.Parallel()
	e := ExchangeStatus{Open: bp(true)}
	assert.False(t, e.IsClosed())
}

func TestIsClosed_AllNil_ReturnsFalse(t *testing.T) {
	t.Parallel()
	e := ExchangeStatus{}
	assert.False(t, e.IsClosed())
}

// ---------------------------------------------------------------------------
// Customer.First / Last
// ---------------------------------------------------------------------------

func TestCustomerFirst_PrefersFirstName(t *testing.T) {
	t.Parallel()
	c := Customer{FirstName: sp("Jovan"), Name: sp("Petar")}
	assert.Equal(t, "Jovan", *c.First())
}

func TestCustomerFirst_FallsBackToName(t *testing.T) {
	t.Parallel()
	c := Customer{Name: sp("Petar")}
	assert.Equal(t, "Petar", *c.First())
}

func TestCustomerFirst_FallsBackToIme(t *testing.T) {
	t.Parallel()
	c := Customer{Ime: sp("Ana")}
	assert.Equal(t, "Ana", *c.First())
}

func TestCustomerFirst_AllNil_ReturnsNil(t *testing.T) {
	t.Parallel()
	c := Customer{}
	assert.Nil(t, c.First())
}

func TestCustomerLast_PrefersLastName(t *testing.T) {
	t.Parallel()
	c := Customer{LastName: sp("Jovic"), Prezime: sp("Petrovic")}
	assert.Equal(t, "Jovic", *c.Last())
}

func TestCustomerLast_FallsBackToPrezime(t *testing.T) {
	t.Parallel()
	c := Customer{Prezime: sp("Petrovic")}
	assert.Equal(t, "Petrovic", *c.Last())
}

func TestCustomerLast_AllNil_ReturnsNil(t *testing.T) {
	t.Parallel()
	c := Customer{}
	assert.Nil(t, c.Last())
}
