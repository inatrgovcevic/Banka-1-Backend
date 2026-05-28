package card

import (
	"testing"

	"banka1/banking-core-service-go/internal/decimal"
)

func TestLuhnValidatorValidPANs(t *testing.T) {
	validator := LuhnValidator{}
	for _, pan := range []string{
		"4532015112830366",
		"5425233430109903",
		"374245455400126",
		"6011000990139424",
	} {
		if !validator.IsValid(pan) {
			t.Fatalf("expected PAN %s to be valid", pan)
		}
	}
}

func TestLuhnValidatorRejectsInvalidPANs(t *testing.T) {
	validator := LuhnValidator{}
	for _, pan := range []string{
		"4532015112830367",
		"1234567890123456",
		"0000000000000000",
		"",
		"4532-0151-1283-0366",
	} {
		if validator.IsValid(pan) {
			t.Fatalf("expected PAN %s to be invalid", pan)
		}
	}
}

func TestBrandDetector(t *testing.T) {
	detector := BrandDetector{}
	cases := map[string]string{
		"4532015112830366": "VISA",
		"5425233430109903": "MASTERCARD",
		"5105105105105100": "MASTERCARD",
		"374245455400126":  "AMEX",
		"340000000000009":  "AMEX",
		"9891000000000008": "DINACARD",
		"9999000000000000": "UNKNOWN",
	}
	for pan, want := range cases {
		if got := detector.Detect(pan); got != want {
			t.Fatalf("Detect(%s)=%s, want %s", pan, got, want)
		}
	}
}

func TestCardBrandRules(t *testing.T) {
	cases := []struct {
		number string
		brand  string
	}{
		{"4532015112830366", "VISA"},
		{"5425233430109903", "MASTERCARD"},
		{"9891000000000008", "DINACARD"},
		{"374245455400126", "AMEX"},
	}
	for _, tc := range cases {
		if !MatchesBrand(tc.number, tc.brand) {
			t.Fatalf("MatchesBrand(%s,%s)=false, want true", tc.number, tc.brand)
		}
	}
}

func TestMasking(t *testing.T) {
	if got := MaskCardNumber("4111111111111111"); got != "4111********1111" {
		t.Fatalf("MaskCardNumber got %s", got)
	}
	if got := MaskAccountNumber("111000150000000011"); got != "**************0011" {
		t.Fatalf("MaskAccountNumber got %s", got)
	}
}

func TestMasterCardFeeCalculator(t *testing.T) {
	calc := NewMasterCardFeeCalculator("0.015", "0.30")
	fee := calc.CalculateFee(decimal.MustParse("100"), decimal.MustParse("117.50"))
	if fee.Cmp(decimal.MustParse("36.75")) != 0 {
		t.Fatalf("fee=%s, want 36.75", fee.String())
	}

	fee = calc.CalculateFee(decimal.MustParse("1000"), decimal.One)
	if fee.Cmp(decimal.MustParse("15.30")) != 0 {
		t.Fatalf("fee=%s, want 15.30", fee.String())
	}
}

func TestFXFeeApplier(t *testing.T) {
	applier := FXFeeApplier{MasterCard: NewMasterCardFeeCalculator("0.015", "0.30")}

	sameCurrency := applier.Apply("MASTERCARD", decimal.MustParse("100"), "RSD", "RSD", decimal.One)
	if sameCurrency.Cmp(decimal.MustParse("100")) != 0 {
		t.Fatalf("sameCurrency=%s, want 100", sameCurrency.String())
	}

	mc := applier.Apply("MASTERCARD", decimal.MustParse("100"), "EUR", "RSD", decimal.MustParse("117.50"))
	if mc.Cmp(decimal.MustParse("136.75")) != 0 {
		t.Fatalf("mc=%s, want 136.75", mc.String())
	}

	visa := applier.Apply("VISA", decimal.MustParse("100"), "EUR", "RSD", decimal.MustParse("117.50"))
	if visa.Cmp(decimal.MustParse("100")) != 0 {
		t.Fatalf("visa=%s, want 100", visa.String())
	}
}
