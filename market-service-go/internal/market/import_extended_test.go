package market

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// parseOptionalCSVTime
// ---------------------------------------------------------------------------

func TestParseOptionalCSVTime_EmptyValue_ReturnsNil(t *testing.T) {
	t.Parallel()
	result, err := parseOptionalCSVTime("", "field", 1, "test.csv")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestParseOptionalCSVTime_WhitespaceOnly_ReturnsNil(t *testing.T) {
	t.Parallel()
	result, err := parseOptionalCSVTime("   ", "field", 1, "test.csv")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestParseOptionalCSVTime_ValidTime_ReturnsPointer(t *testing.T) {
	t.Parallel()
	result, err := parseOptionalCSVTime("09:30", "openTime", 2, "test.csv")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "09:30:00", *result)
}

func TestParseOptionalCSVTime_InvalidTime_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := parseOptionalCSVTime("invalid", "field", 1, "test.csv")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// normalizeCSVTime
// ---------------------------------------------------------------------------

func TestNormalizeCSVTime_ValidHHmm_ReturnsHHmmss(t *testing.T) {
	t.Parallel()
	result, err := normalizeCSVTime("09:30", "openTime", 1, "test.csv")
	require.NoError(t, err)
	assert.Equal(t, "09:30:00", result)
}

func TestNormalizeCSVTime_Invalid_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := normalizeCSVTime("bad", "openTime", 5, "test.csv")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "openTime")
	assert.Contains(t, err.Error(), "5")
}

// ---------------------------------------------------------------------------
// parseOptionalCSVBool
// ---------------------------------------------------------------------------

func TestParseOptionalCSVBool_Truthy(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"true", "1", "yes", ""} {
		assert.True(t, parseOptionalCSVBool(v), "value=%q", v)
	}
}

func TestParseOptionalCSVBool_Falsy(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"false", "0", "no"} {
		assert.False(t, parseOptionalCSVBool(v), "value=%q", v)
	}
}

func TestParseOptionalCSVBool_Unknown_ReturnsTrue(t *testing.T) {
	t.Parallel()
	assert.True(t, parseOptionalCSVBool("maybe"))
}

// ---------------------------------------------------------------------------
// isBlankRecord
// ---------------------------------------------------------------------------

func TestIsBlankRecord_AllEmpty_ReturnsTrue(t *testing.T) {
	t.Parallel()
	assert.True(t, isBlankRecord([]string{"", "  ", "\t"}))
}

func TestIsBlankRecord_HasContent_ReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, isBlankRecord([]string{"", "NYSE", ""}))
}

func TestIsBlankRecord_EmptySlice_ReturnsTrue(t *testing.T) {
	t.Parallel()
	assert.True(t, isBlankRecord([]string{}))
}

// ---------------------------------------------------------------------------
// normalizeImportCurrency - additional cases
// ---------------------------------------------------------------------------

func TestNormalizeImportCurrency_USD_Recognized(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "USD", normalizeImportCurrency("United States Dollar"))
}

func TestNormalizeImportCurrency_GBP_Recognized(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "GBP", normalizeImportCurrency("Pound Sterling"))
}

func TestNormalizeImportCurrency_Unknown_ReturnsOriginal(t *testing.T) {
	t.Parallel()
	result := normalizeImportCurrency("SomeCurrency")
	assert.Equal(t, "SomeCurrency", result)
}
