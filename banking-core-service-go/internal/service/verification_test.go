package service

import (
	"testing"

	"banka1/banking-core-service-go/internal/config"
)

func TestGenerateOTPCodeProducesSixDigits(t *testing.T) {
	for i := 0; i < 50; i++ {
		code, err := generateOTPCode()
		if err != nil {
			t.Fatalf("generateOTPCode() error: %v", err)
		}
		if !otpCodePattern.MatchString(code) {
			t.Fatalf("generateOTPCode()=%q, want six digits", code)
		}
	}
}

func TestHashOTPMatches(t *testing.T) {
	svc := NewVerificationService(nil, config.Config{JWTSecret: "test-secret"}, nil)
	hash, err := svc.hashOTP("123456")
	if err != nil {
		t.Fatalf("hashOTP() error: %v", err)
	}
	ok, err := svc.matchesOTP("123456", hash)
	if err != nil {
		t.Fatalf("matchesOTP() error: %v", err)
	}
	if !ok {
		t.Fatal("matchesOTP()=false, want true")
	}
	ok, err = svc.matchesOTP("654321", hash)
	if err != nil {
		t.Fatalf("matchesOTP() wrong code error: %v", err)
	}
	if ok {
		t.Fatal("matchesOTP()=true for wrong code")
	}
}

func TestValidOperationType(t *testing.T) {
	for _, value := range []string{"PAYMENT", "TRANSFER", "LIMIT_CHANGE", "CARD_REQUEST", "LOAN_REQUEST"} {
		if !validOperationType(value) {
			t.Fatalf("validOperationType(%s)=false", value)
		}
	}
	if validOperationType("UNKNOWN") {
		t.Fatal("validOperationType(UNKNOWN)=true")
	}
}
