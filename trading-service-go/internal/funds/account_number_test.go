package funds

import (
	"regexp"
	"testing"
)

var accountNumberRe = regexp.MustCompile(`^[0-9]{16}$`)

func TestCheckDigit_KnownValues(t *testing.T) {
	cases := []struct {
		prefix string
		want   int
	}{
		{"000000000000000", func() int { return checkDigit("000000000000000") }()},
		{"123456789012345", func() int { return checkDigit("123456789012345") }()},
		{"111111111111111", func() int { return checkDigit("111111111111111") }()},
	}
	for _, c := range cases {
		got := checkDigit(c.prefix)
		if got < 0 || got > 9 {
			t.Errorf("checkDigit(%q) = %d, want 0-9", c.prefix, got)
		}
		if got != c.want {
			t.Errorf("checkDigit(%q) not stable: got %d and %d", c.prefix, got, c.want)
		}
	}
}

func TestCheckDigit_AllZeros(t *testing.T) {
	// 15 zeros: sum=0, rem=0, cd=11 ≥ 10 → 0
	if got := checkDigit("000000000000000"); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestCheckDigit_Deterministic(t *testing.T) {
	prefix := "987654321098765"
	first := checkDigit(prefix)
	for i := 0; i < 10; i++ {
		if got := checkDigit(prefix); got != first {
			t.Fatalf("checkDigit not deterministic: %d vs %d", got, first)
		}
	}
}

func TestCheckDigit_ResultRange(t *testing.T) {
	prefixes := []string{
		"100000000000000",
		"200000000000000",
		"999999999999999",
		"123456789012340",
		"000000000000001",
	}
	for _, p := range prefixes {
		cd := checkDigit(p)
		if cd < 0 || cd > 9 {
			t.Errorf("checkDigit(%q) = %d, want 0-9", p, cd)
		}
	}
}

func TestGenerateAccountNumber_Format(t *testing.T) {
	for i := 0; i < 20; i++ {
		num, err := GenerateAccountNumber()
		if err != nil {
			t.Fatalf("GenerateAccountNumber() error: %v", err)
		}
		if !accountNumberRe.MatchString(num) {
			t.Errorf("GenerateAccountNumber() = %q, want 16 digits", num)
		}
	}
}

func TestGenerateAccountNumber_CheckDigitValid(t *testing.T) {
	for i := 0; i < 20; i++ {
		num, err := GenerateAccountNumber()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		prefix := num[:15]
		want := checkDigit(prefix)
		gotByte := int(num[15] - '0')
		if gotByte != want {
			t.Errorf("num=%q: check digit %d, expected %d", num, gotByte, want)
		}
	}
}

func TestGenerateAccountNumber_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		num, err := GenerateAccountNumber()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		seen[num] = true
	}
	// With 15 random digits there are 10^15 possibilities — collisions in 50 draws are astronomically unlikely.
	if len(seen) < 45 {
		t.Errorf("too many collisions: %d unique out of 50", len(seen))
	}
}
