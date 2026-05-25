package decimalx

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
)

func TestMarshal_PlainString_NoScientificNotation(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"0.00000001", `"0.00000001"`},
		{"1234567890.12", `"1234567890.12"`},
		{"-0.5", `"-0.5"`},
		{"0", `"0"`},
		{"100", `"100"`},
	}
	for _, c := range cases {
		d := decimal.RequireFromString(c.in)
		got, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("marshal %s: %v", c.in, err)
		}
		if string(got) != c.want {
			t.Errorf("in=%s got=%s want=%s", c.in, got, c.want)
		}
	}
}

func TestUnmarshal_FromJSONNumberAndString(t *testing.T) {
	var v struct {
		Amount decimal.Decimal `json:"amount"`
	}
	// From string (the form we emit)
	if err := json.Unmarshal([]byte(`{"amount":"0.00000001"}`), &v); err != nil {
		t.Fatalf("string: %v", err)
	}
	if v.Amount.String() != "0.00000001" {
		t.Errorf("string round-trip: got %s", v.Amount.String())
	}
	// From raw JSON number (Java may emit either form)
	if err := json.Unmarshal([]byte(`{"amount":100.50}`), &v); err != nil {
		t.Fatalf("number: %v", err)
	}
	if v.Amount.String() != "100.5" {
		t.Errorf("number form: got %s", v.Amount.String())
	}
}
