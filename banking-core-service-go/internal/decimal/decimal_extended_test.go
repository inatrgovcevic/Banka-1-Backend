package decimal

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Parse / MustParse
// ---------------------------------------------------------------------------

func TestParse_ValidNumbers(t *testing.T) {
	cases := []string{"0", "1", "-1", "123.456", "-0.001", "999999999.99"}
	for _, c := range cases {
		_, err := Parse(c)
		assert.NoError(t, err, "input: %s", c)
	}
}

func TestParse_EmptyString_ReturnsError(t *testing.T) {
	_, err := Parse("")
	require.Error(t, err)
}

func TestParse_InvalidString_ReturnsError(t *testing.T) {
	_, err := Parse("not-a-number")
	require.Error(t, err)
}

func TestMustParse_Panics(t *testing.T) {
	assert.Panics(t, func() { MustParse("bad") })
}

// ---------------------------------------------------------------------------
// NewFromInt
// ---------------------------------------------------------------------------

func TestNewFromInt_PositiveAndNegative(t *testing.T) {
	assert.Equal(t, "100", NewFromInt(100).String())
	assert.Equal(t, "-50", NewFromInt(-50).String())
	assert.Equal(t, "0", NewFromInt(0).String())
}

// ---------------------------------------------------------------------------
// Arithmetic operations
// ---------------------------------------------------------------------------

func TestAdd_TwoPositives(t *testing.T) {
	result := MustParse("1.5").Add(MustParse("2.5"))
	assert.Equal(t, "4", result.String())
}

func TestSub_Positive(t *testing.T) {
	result := MustParse("10").Sub(MustParse("3.5"))
	assert.Equal(t, "6.5", result.String())
}

func TestMul_Fractional(t *testing.T) {
	result := MustParse("3").Mul(MustParse("0.5"))
	assert.Equal(t, "1.5", result.String())
}

func TestNeg_Positive(t *testing.T) {
	result := MustParse("5").Neg()
	assert.Equal(t, "-5", result.String())
}

func TestNeg_Negative(t *testing.T) {
	result := MustParse("-3").Neg()
	assert.Equal(t, "3", result.String())
}

func TestNeg_Zero(t *testing.T) {
	result := Zero.Neg()
	assert.Equal(t, "0", result.String())
}

func TestMin_FirstSmaller(t *testing.T) {
	result := MustParse("3").Min(MustParse("7"))
	assert.Equal(t, "3", result.String())
}

func TestMin_SecondSmaller(t *testing.T) {
	result := MustParse("10").Min(MustParse("2"))
	assert.Equal(t, "2", result.String())
}

func TestMin_Equal(t *testing.T) {
	result := MustParse("5").Min(MustParse("5"))
	assert.Equal(t, "5", result.String())
}

// ---------------------------------------------------------------------------
// Round
// ---------------------------------------------------------------------------

func TestRound_NegativeScale_UsesZero(t *testing.T) {
	result := MustParse("12.345").Round(-1)
	assert.Equal(t, "12", result.String())
}

func TestRound_NegativeNumber(t *testing.T) {
	result := MustParse("-1.555").Round(2)
	assert.Equal(t, "-1.56", result.String())
}

func TestRound_AlreadyRounded(t *testing.T) {
	result := MustParse("1.23").Round(4)
	assert.Equal(t, "1.23", result.String())
}

// ---------------------------------------------------------------------------
// Cmp / Sign / IsZero
// ---------------------------------------------------------------------------

func TestCmp_LessThan(t *testing.T) {
	assert.Equal(t, -1, MustParse("1").Cmp(MustParse("2")))
}

func TestCmp_Equal(t *testing.T) {
	assert.Equal(t, 0, MustParse("5").Cmp(MustParse("5")))
}

func TestCmp_GreaterThan(t *testing.T) {
	assert.Equal(t, 1, MustParse("3").Cmp(MustParse("1")))
}

func TestSign_Positive(t *testing.T) {
	assert.Equal(t, 1, MustParse("0.001").Sign())
}

func TestSign_Negative(t *testing.T) {
	assert.Equal(t, -1, MustParse("-0.5").Sign())
}

func TestSign_Zero(t *testing.T) {
	assert.Equal(t, 0, Zero.Sign())
}

func TestIsZero_True(t *testing.T) {
	assert.True(t, Zero.IsZero())
}

func TestIsZero_False(t *testing.T) {
	assert.False(t, MustParse("0.001").IsZero())
}

// ---------------------------------------------------------------------------
// String / Fixed
// ---------------------------------------------------------------------------

func TestString_NilValue_ReturnsZero(t *testing.T) {
	d := Decimal{}
	assert.Equal(t, "0", d.String())
}

func TestString_TrailingZerosStripped(t *testing.T) {
	assert.Equal(t, "1.5", MustParse("1.50000").String())
}

func TestString_NegativeZero(t *testing.T) {
	d := MustParse("0.0").Neg()
	assert.Equal(t, "0", d.String())
}

func TestFixed_TwoDecimals(t *testing.T) {
	assert.Equal(t, "1.50", MustParse("1.5").Fixed(2))
}

// ---------------------------------------------------------------------------
// JSON marshaling / unmarshaling
// ---------------------------------------------------------------------------

func TestMarshalJSON_ReturnsStringRepresentation(t *testing.T) {
	d := MustParse("3.14")
	b, err := d.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, "3.14", string(b))
}

func TestUnmarshalJSON_FromNumber(t *testing.T) {
	var d Decimal
	require.NoError(t, json.Unmarshal([]byte("42.5"), &d))
	assert.Equal(t, "42.5", d.String())
}

func TestUnmarshalJSON_FromQuotedString(t *testing.T) {
	var d Decimal
	require.NoError(t, json.Unmarshal([]byte(`"100.25"`), &d))
	assert.Equal(t, "100.25", d.String())
}

func TestUnmarshalJSON_Null_SetsZero(t *testing.T) {
	d := MustParse("5")
	require.NoError(t, json.Unmarshal([]byte("null"), &d))
	assert.True(t, d.IsZero())
}

func TestUnmarshalJSON_InvalidNumber_ReturnsError(t *testing.T) {
	var d Decimal
	err := json.Unmarshal([]byte(`"not-a-number"`), &d)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// SQL driver Value / Scan
// ---------------------------------------------------------------------------

func TestValue_ReturnsStringRepresentation(t *testing.T) {
	d := MustParse("99.99")
	v, err := d.Value()
	require.NoError(t, err)
	assert.Equal(t, "99.99", v)
}

func TestScan_FromNil_SetsZero(t *testing.T) {
	var d Decimal
	require.NoError(t, d.Scan(nil))
	assert.True(t, d.IsZero())
}

func TestScan_FromString(t *testing.T) {
	var d Decimal
	require.NoError(t, d.Scan("123.45"))
	assert.Equal(t, "123.45", d.String())
}

func TestScan_FromBytes(t *testing.T) {
	var d Decimal
	require.NoError(t, d.Scan([]byte("50.5")))
	assert.Equal(t, "50.5", d.String())
}

func TestScan_FromInt64(t *testing.T) {
	var d Decimal
	require.NoError(t, d.Scan(int64(42)))
	assert.Equal(t, "42", d.String())
}

func TestScan_FromFloat64(t *testing.T) {
	var d Decimal
	require.NoError(t, d.Scan(float64(3.14)))
	assert.False(t, d.IsZero())
}

func TestScan_FromInvalidType_ReturnsError(t *testing.T) {
	var d Decimal
	err := d.Scan(true)
	require.Error(t, err)
}

func TestScan_FromInvalidString_ReturnsError(t *testing.T) {
	var d Decimal
	err := d.Scan("not-decimal")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Nil Decimal (no value) behavior
// ---------------------------------------------------------------------------

func TestNilDecimal_Cmp(t *testing.T) {
	var d Decimal
	assert.Equal(t, 0, d.Cmp(Zero))
}

func TestNilDecimal_Add(t *testing.T) {
	var d Decimal
	result := d.Add(MustParse("5"))
	assert.Equal(t, "5", result.String())
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

func TestZeroConstant(t *testing.T) {
	assert.True(t, Zero.IsZero())
	assert.Equal(t, "0", Zero.String())
}

func TestOneConstant(t *testing.T) {
	assert.Equal(t, "1", One.String())
	assert.False(t, One.IsZero())
}
