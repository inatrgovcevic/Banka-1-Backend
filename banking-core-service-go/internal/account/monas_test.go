package account

import (
	"context"
	"testing"
)

func TestValidateMONAS(t *testing.T) {
	valid := []string{
		"1110001000000000511", // type 11, random=000000000, check=5, sum=11
		"1110001000000000421", // type 21, random=000000000, check=4, sum=11
	}
	for _, value := range valid {
		if !ValidateMONAS(value) {
			t.Fatalf("ValidateMONAS(%q)=false, want true", value)
		}
	}
}

func TestValidateMONASRejectsInvalidValues(t *testing.T) {
	invalid := []string{
		"1110001000000000512", // wrong check digit (sum=12, not divisible by 11)
		"2110001000000000511", // wrong prefix
		"1110001000000000510", // wrong check digit (sum=10, not divisible by 11)
		"111000100000000051",  // 18 digits — too short
		"11100010000000005x1", // non-digit character
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
