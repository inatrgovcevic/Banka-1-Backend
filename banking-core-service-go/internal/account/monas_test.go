package account

import (
	"context"
	"testing"
)

func TestValidateMONAS(t *testing.T) {
	valid := []string{
		"111000150000000011",
		"111000140000000021",
	}
	for _, value := range valid {
		if !ValidateMONAS(value) {
			t.Fatalf("ValidateMONAS(%q)=false, want true", value)
		}
	}
}

func TestValidateMONASRejectsInvalidValues(t *testing.T) {
	invalid := []string{
		"111000150000000012",
		"211000150000000011",
		"111000150000000010",
		"11100015000000011",
		"11100015000000001x",
	}
	for _, value := range invalid {
		if ValidateMONAS(value) {
			t.Fatalf("ValidateMONAS(%q)=true, want false", value)
		}
	}
}

func TestGenerateMONASRejectsUnknownType(t *testing.T) {
	if _, err := GenerateMONAS(context.Background(), nil, "99", nil); err == nil {
		t.Fatal("GenerateMONAS() error=nil, want validation error")
	}
}
