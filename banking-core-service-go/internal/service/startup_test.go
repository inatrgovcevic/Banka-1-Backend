package service

import (
	"testing"

	"banka1/banking-core-service-go/internal/config"
)

func TestValidateStartupConfigAllowsDevDefaultsWithoutActiveProfile(t *testing.T) {
	cfg := config.Config{
		BankAccountNumber:     devDefaultBankAccount,
		ExchangeAccountNumber: devDefaultExchangeAccount,
		BankClientID:          -1,
		ExchangeClientID:      devDefaultExchangeClientID,
	}
	if err := ValidateStartupConfig(cfg); err != nil {
		t.Fatalf("ValidateStartupConfig() error = %v, want nil", err)
	}
}

func TestValidateStartupConfigRejectsDevDefaultsInProdProfile(t *testing.T) {
	cfg := config.Config{
		Profiles:              []string{"prod"},
		BankAccountNumber:     devDefaultBankAccount,
		ExchangeAccountNumber: devDefaultExchangeAccount,
		BankClientID:          -1,
		ExchangeClientID:      devDefaultExchangeClientID,
	}
	if err := ValidateStartupConfig(cfg); err == nil {
		t.Fatal("ValidateStartupConfig() error = nil, want prod dev-default rejection")
	}
}

func TestValidateStartupConfigAllowsExplicitValuesInProdProfile(t *testing.T) {
	cfg := config.Config{
		Profiles:              []string{"prod"},
		BankAccountNumber:     "1234567812345670",
		ExchangeAccountNumber: "9876543212345674",
		BankClientID:          -1,
		ExchangeClientID:      -3,
	}
	if err := ValidateStartupConfig(cfg); err == nil {
		t.Fatal("ValidateStartupConfig() error = nil, want exchange client dev-default rejection")
	}
	cfg.ExchangeClientID = -33
	if err := ValidateStartupConfig(cfg); err != nil {
		t.Fatalf("ValidateStartupConfig() error = %v, want nil", err)
	}
}
