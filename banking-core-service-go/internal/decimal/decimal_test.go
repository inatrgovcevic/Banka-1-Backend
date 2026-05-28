package decimal

import "testing"

func TestRoundHalfUp(t *testing.T) {
	cases := map[string]string{
		"1.234":  "1.23",
		"1.235":  "1.24",
		"33.33":  "33.33",
		"-1.235": "-1.24",
	}
	for input, want := range cases {
		got := MustParse(input).Round(2)
		if got.Cmp(MustParse(want)) != 0 {
			t.Fatalf("%s rounded to %s, want %s", input, got.String(), want)
		}
	}
}

func TestArithmetic(t *testing.T) {
	got := MustParse("10000").Mul(MustParse("0.30")).Round(2)
	if got.Cmp(MustParse("3000.00")) != 0 {
		t.Fatalf("bank part=%s, want 3000.00", got.String())
	}

	clientPart := MustParse("10000").Sub(got)
	if clientPart.Cmp(MustParse("7000")) != 0 {
		t.Fatalf("client part=%s, want 7000", clientPart.String())
	}
}
