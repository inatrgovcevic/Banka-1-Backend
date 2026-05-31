package account

import "testing"

func TestGenerateProduces16Digits(t *testing.T) {
	gen := NumberGenerator{}
	for i := 0; i < 100; i++ {
		value, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		if len(value) != 16 {
			t.Fatalf("len(%s)=%d, want 16", value, len(value))
		}
		for _, ch := range value {
			if ch < '0' || ch > '9' {
				t.Fatalf("Generate()=%s, want only digits", value)
			}
		}
	}
}

func TestGenerateProducesDifferentValues(t *testing.T) {
	gen := NumberGenerator{}
	a, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	b, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if a == b {
		t.Fatalf("two generated numbers are equal: %s", a)
	}
}
