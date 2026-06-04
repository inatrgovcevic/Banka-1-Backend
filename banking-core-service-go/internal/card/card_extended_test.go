package card

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// CalculateCheckDigit
// ---------------------------------------------------------------------------

func TestCalculateCheckDigit_ValidPayload_ReturnsDigit(t *testing.T) {
	t.Parallel()
	// 453201511283036 → the check digit that makes it a valid Luhn number
	payload := "453201511283036"
	digit, ok := CalculateCheckDigit(payload)
	require.True(t, ok)
	validator := LuhnValidator{}
	assert.True(t, validator.IsValid(payload+string(digit)))
}

func TestCalculateCheckDigit_ShortPayload_FindsDigit(t *testing.T) {
	t.Parallel()
	digit, ok := CalculateCheckDigit("123456789012345")
	assert.True(t, ok)
	_ = digit
}

// ---------------------------------------------------------------------------
// CardName
// ---------------------------------------------------------------------------

func TestCardName_KnownBrands(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"VISA":       "Visa Debit",
		"MASTERCARD": "MasterCard Debit",
		"DINACARD":   "DinaCard Debit",
		"AMEX":       "AmEx Debit",
	}
	for brand, want := range cases {
		assert.Equal(t, want, CardName(brand))
	}
}

func TestCardName_UnknownBrand_ReturnsBrandDebit(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "MAESTRO Debit", CardName("MAESTRO"))
}

// ---------------------------------------------------------------------------
// CardNumberLength
// ---------------------------------------------------------------------------

func TestCardNumberLength_AMEX_Returns15(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 15, CardNumberLength("AMEX"))
	assert.Equal(t, 15, CardNumberLength("amex"))
}

func TestCardNumberLength_OtherBrands_Returns16(t *testing.T) {
	t.Parallel()
	for _, brand := range []string{"VISA", "MASTERCARD", "DINACARD", "MAESTRO"} {
		assert.Equal(t, 16, CardNumberLength(brand))
	}
}

// ---------------------------------------------------------------------------
// MaskCardNumber / MaskAccountNumber
// ---------------------------------------------------------------------------

func TestMaskCardNumber_16Digits(t *testing.T) {
	t.Parallel()
	masked := MaskCardNumber("4532015112830366")
	assert.Equal(t, "4532", masked[:4])
	assert.Equal(t, "0366", masked[len(masked)-4:])
	assert.Contains(t, masked, "****")
}

func TestMaskCardNumber_ShortNumber_Unchanged(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "12345678", MaskCardNumber("12345678"))
}

func TestMaskAccountNumber_LongNumber(t *testing.T) {
	t.Parallel()
	masked := MaskAccountNumber("1234567890123456789")
	assert.True(t, len(masked) == len("1234567890123456789"))
	assert.Equal(t, "6789", masked[len(masked)-4:])
}

func TestMaskAccountNumber_ShortNumber_Unchanged(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "1234", MaskAccountNumber("1234"))
}

// ---------------------------------------------------------------------------
// MatchesBrand
// ---------------------------------------------------------------------------

func TestMatchesBrand_VISA_Valid(t *testing.T) {
	t.Parallel()
	assert.True(t, MatchesBrand("4532015112830366", "VISA"))
}

func TestMatchesBrand_MASTERCARD_Valid(t *testing.T) {
	t.Parallel()
	assert.True(t, MatchesBrand("5425233430109903", "MASTERCARD"))
}

func TestMatchesBrand_DINACARD_Valid(t *testing.T) {
	t.Parallel()
	assert.True(t, MatchesBrand("9891000000000000", "DINACARD"))
}

func TestMatchesBrand_AMEX_Valid(t *testing.T) {
	t.Parallel()
	assert.True(t, MatchesBrand("340000000000009", "AMEX"))
}

func TestMatchesBrand_UnknownBrand_ReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, MatchesBrand("1234567890123456", "MAESTRO"))
}

func TestMatchesBrand_NonDigits_ReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, MatchesBrand("45320151128303XX", "VISA"))
}

func TestMatchesBrand_WrongLength_ReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, MatchesBrand("453201511283", "VISA"))
}

// ---------------------------------------------------------------------------
// Detect - MAESTRO (currently 0% for that branch)
// ---------------------------------------------------------------------------

func TestBrandDetector_Maestro_Detected(t *testing.T) {
	t.Parallel()
	detector := BrandDetector{}
	// MAESTRO starts with 50 or 56-69 (2-digit prefix)
	assert.Equal(t, "MAESTRO", detector.Detect("5018000000000000"))
}

func TestBrandDetector_Maestro_56Prefix(t *testing.T) {
	t.Parallel()
	detector := BrandDetector{}
	assert.Equal(t, "MAESTRO", detector.Detect("5600000000000000"))
}

// ---------------------------------------------------------------------------
// hasPrefixRange
// ---------------------------------------------------------------------------

func TestHasPrefixRange_TooShort_ReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, hasPrefixRange("4", 4, 2221, 2720))
}

func TestHasPrefixRange_InRange(t *testing.T) {
	t.Parallel()
	assert.True(t, hasPrefixRange("2500000000000000", 4, 2221, 2720))
}

func TestHasPrefixRange_OutOfRange(t *testing.T) {
	t.Parallel()
	assert.False(t, hasPrefixRange("2800000000000000", 4, 2221, 2720))
}

// ---------------------------------------------------------------------------
// allDigits
// ---------------------------------------------------------------------------

func TestAllDigits_AllDigits_ReturnsTrue(t *testing.T) {
	t.Parallel()
	assert.True(t, allDigits("1234567890"))
}

func TestAllDigits_WithSpace_ReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, allDigits("123 456"))
}

func TestAllDigits_Empty_ReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, allDigits(""))
}
