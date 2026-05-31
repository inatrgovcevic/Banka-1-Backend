package service

import (
	"fmt"
	"strings"

	"banka1/banking-core-service-go/internal/config"
)

const (
	devDefaultBankAccount      = "111000110000000312"
	devDefaultExchangeAccount  = "111000300000002012"
	devDefaultExchangeClientID = int64(-3)
)

func ValidateStartupConfig(cfg config.Config) error {
	return validateMarginTransferConfig(cfg)
}

func validateMarginTransferConfig(cfg config.Config) error {
	if strings.TrimSpace(cfg.BankAccountNumber) == "" ||
		strings.TrimSpace(cfg.ExchangeAccountNumber) == "" {
		return fmt.Errorf("margin bank/exchange account konfiguracija ima null/blank vrednosti")
	}

	usingBankAccountDev := cfg.BankAccountNumber == devDefaultBankAccount
	usingExchangeAccountDev := cfg.ExchangeAccountNumber == devDefaultExchangeAccount
	usingExchangeClientDev := cfg.ExchangeClientID == devDefaultExchangeClientID

	if !isNonProdProfile(cfg.Profiles) && (usingBankAccountDev || usingExchangeAccountDev || usingExchangeClientDev) {
		return fmt.Errorf("margin bank/exchange account konfiguracija koristi DEV default vrednosti, a aktivni profili nisu dev/local/test")
	}
	return nil
}

func isNonProdProfile(profiles []string) bool {
	if len(profiles) == 0 {
		return true
	}
	for _, profile := range profiles {
		switch strings.ToLower(strings.TrimSpace(profile)) {
		case "dev", "local", "test":
			return true
		}
	}
	return false
}
