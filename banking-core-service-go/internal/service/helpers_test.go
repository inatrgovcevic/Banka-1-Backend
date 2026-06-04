package service

import (
	"database/sql"
	"testing"
	"time"

	"banka1/banking-core-service-go/internal/decimal"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// formatDateTime / formatDate
// ---------------------------------------------------------------------------

func TestFormatDateTime_ValidTime(t *testing.T) {
	t.Parallel()
	nt := sql.NullTime{Valid: true, Time: time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)}
	result := formatDateTime(nt)
	assert.Equal(t, "2024-03-15T14:30:00", result)
}

func TestFormatDateTime_NullTime_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", formatDateTime(sql.NullTime{Valid: false}))
}

func TestFormatDate_ValidTime(t *testing.T) {
	t.Parallel()
	nt := sql.NullTime{Valid: true, Time: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)}
	assert.Equal(t, "2025-12-31", formatDate(nt))
}

func TestFormatDate_NullTime_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", formatDate(sql.NullTime{Valid: false}))
}

// ---------------------------------------------------------------------------
// accountView.classification()
// ---------------------------------------------------------------------------

func TestAccountViewClassification_FXAccount(t *testing.T) {
	t.Parallel()
	v := accountView{AccountType: "FX", AccountOwnership: "personal"}
	category, accountType, subtype, tip := v.classification()
	assert.Equal(t, "FOREIGN_CURRENCY", category)
	assert.Equal(t, "PERSONAL", accountType)
	assert.Equal(t, "", subtype)
	assert.Equal(t, "devizni", tip)
}

func TestAccountViewClassification_CheckingStandardni(t *testing.T) {
	t.Parallel()
	v := accountView{AccountConcrete: "STANDARDNI"}
	category, accountType, subtype, tip := v.classification()
	assert.Equal(t, "CHECKING", category)
	assert.Equal(t, "PERSONAL", accountType)
	assert.Equal(t, "STANDARDNI", subtype)
	assert.Equal(t, "tekuci", tip)
}

func TestAccountViewClassification_CheckingWithOwnership(t *testing.T) {
	t.Parallel()
	v := accountView{AccountOwnership: "business", AccountConcrete: "DOO"}
	category, accountType, subtype, tip := v.classification()
	assert.Equal(t, "CHECKING", category)
	assert.Equal(t, "BUSINESS", accountType)
	assert.Equal(t, "DOO", subtype)
	assert.Equal(t, "tekuci", tip)
}

func TestAccountViewClassification_UnknownConcrete(t *testing.T) {
	t.Parallel()
	v := accountView{AccountConcrete: "UNKNOWN_TYPE"}
	category, _, _, _ := v.classification()
	assert.Equal(t, "CHECKING", category)
}

// ---------------------------------------------------------------------------
// accountView.summary()
// ---------------------------------------------------------------------------

func TestAccountViewSummary_PopulatesFields(t *testing.T) {
	t.Parallel()
	v := accountView{
		ID:               1,
		DisplayName:      "Moj Racun",
		AccountNumber:    "1110001000000000511",
		AvailableBalance: decimal.MustParse("5000"),
		Currency:         "RSD",
		AccountConcrete:  "STANDARDNI",
	}
	s := v.summary()
	assert.Equal(t, int64(1), s.ID)
	assert.Equal(t, "Moj Racun", s.NazivRacuna)
	assert.Equal(t, "1110001000000000511", s.BrojRacuna)
	assert.Equal(t, "RSD", s.Currency)
	assert.Equal(t, "PERSONAL", s.AccountType)
}

// ---------------------------------------------------------------------------
// accountView.details() — basic fields
// ---------------------------------------------------------------------------

func TestAccountViewDetails_BasicFields(t *testing.T) {
	t.Parallel()
	v := accountView{
		ID:               5,
		DisplayName:      "Racun 5",
		AccountNumber:    "1110001000000000511",
		OwnerID:          42,
		AvailableBalance: decimal.MustParse("1000"),
		BookedBalance:    decimal.MustParse("1100"),
		Currency:         "RSD",
		Status:           "ACTIVE",
		AccountType:      "CHECKING",
		AccountConcrete:  "STANDARDNI",
	}
	d := v.details()
	assert.Equal(t, "1110001000000000511", d.BrojRacuna)
	assert.Equal(t, int64(42), d.Vlasnik)
	assert.Equal(t, "RSD", d.Currency)
	assert.Equal(t, "ACTIVE", d.Status)
}

func TestAccountViewDetails_WithCompany(t *testing.T) {
	t.Parallel()
	companyID := int64(10)
	ownerID := int64(20)
	v := accountView{
		AccountConcrete: "DOO",
		CompanyID:       sql.NullInt64{Valid: true, Int64: companyID},
		CompanyName:     "Firma DOO",
		CompanyRegNumber: "12345678",
		CompanyOwnerID:  sql.NullInt64{Valid: true, Int64: ownerID},
	}
	d := v.details()
	assert.Equal(t, "Firma DOO", d.NazivFirme)
	assert.Equal(t, "12345678", d.CompanyRegistrationNumber)
	require_not_nil_int64(t, d.CompanyOwnerID, ownerID)
}

func require_not_nil_int64(t *testing.T, ptr *int64, want int64) {
	t.Helper()
	if ptr == nil {
		t.Fatal("expected non-nil int64 pointer")
	}
	if *ptr != want {
		t.Fatalf("expected %d, got %d", want, *ptr)
	}
}

func TestAccountViewDetails_NoCompanyID(t *testing.T) {
	t.Parallel()
	v := accountView{
		AccountConcrete: "STANDARDNI",
		CompanyID:       sql.NullInt64{Valid: false},
	}
	d := v.details()
	assert.Empty(t, d.NazivFirme)
	assert.Nil(t, d.CompanyOwnerID)
}
