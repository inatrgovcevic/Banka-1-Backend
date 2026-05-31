package card

import (
	"strconv"
	"strings"

	"banka1/banking-core-service-go/internal/decimal"
)

type LuhnValidator struct{}

func (LuhnValidator) IsValid(pan string) bool {
	if pan == "" {
		return false
	}
	sum := 0
	doubleNext := false
	allZero := true
	for i := len(pan) - 1; i >= 0; i-- {
		ch := pan[i]
		if ch < '0' || ch > '9' {
			return false
		}
		digit := int(ch - '0')
		if digit != 0 {
			allZero = false
		}
		if doubleNext {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		doubleNext = !doubleNext
	}
	return !allZero && sum%10 == 0
}

func CalculateCheckDigit(payload string) (byte, bool) {
	validator := LuhnValidator{}
	for digit := byte('0'); digit <= '9'; digit++ {
		if validator.IsValid(payload + string(digit)) {
			return digit, true
		}
	}
	return 0, false
}

type BrandDetector struct{}

func (BrandDetector) Detect(pan string) string {
	if strings.HasPrefix(pan, "9891") {
		return "DINACARD"
	}
	if strings.HasPrefix(pan, "4") {
		return "VISA"
	}
	if hasPrefixRange(pan, 2, 51, 55) || hasPrefixRange(pan, 4, 2221, 2720) {
		return "MASTERCARD"
	}
	if strings.HasPrefix(pan, "34") || strings.HasPrefix(pan, "37") {
		return "AMEX"
	}
	if strings.HasPrefix(pan, "50") || hasPrefixRange(pan, 2, 56, 69) {
		return "MAESTRO"
	}
	return "UNKNOWN"
}

func MatchesBrand(pan, brand string) bool {
	if !allDigits(pan) {
		return false
	}
	switch strings.ToUpper(brand) {
	case "VISA":
		return len(pan) == 16 && strings.HasPrefix(pan, "4")
	case "MASTERCARD":
		return len(pan) == 16 && (hasPrefixRange(pan, 2, 51, 55) || hasPrefixRange(pan, 4, 2221, 2720))
	case "DINACARD":
		return len(pan) == 16 && strings.HasPrefix(pan, "9891")
	case "AMEX":
		return len(pan) == 15 && (strings.HasPrefix(pan, "34") || strings.HasPrefix(pan, "37"))
	default:
		return false
	}
}

func CardName(brand string) string {
	switch strings.ToUpper(brand) {
	case "VISA":
		return "Visa Debit"
	case "MASTERCARD":
		return "MasterCard Debit"
	case "DINACARD":
		return "DinaCard Debit"
	case "AMEX":
		return "AmEx Debit"
	default:
		return brand + " Debit"
	}
}

func CardNumberLength(brand string) int {
	if strings.EqualFold(brand, "AMEX") {
		return 15
	}
	return 16
}

func MaskCardNumber(cardNumber string) string {
	if len(cardNumber) <= 8 {
		return cardNumber
	}
	return cardNumber[:4] + strings.Repeat("*", len(cardNumber)-8) + cardNumber[len(cardNumber)-4:]
}

func MaskAccountNumber(accountNumber string) string {
	if len(accountNumber) <= 4 {
		return accountNumber
	}
	return strings.Repeat("*", len(accountNumber)-4) + accountNumber[len(accountNumber)-4:]
}

type MasterCardFeeCalculator struct {
	FXFeePercent decimal.Decimal
	NetworkFee   decimal.Decimal
}

func NewMasterCardFeeCalculator(feePercent, networkFee string) MasterCardFeeCalculator {
	return MasterCardFeeCalculator{
		FXFeePercent: decimal.MustParse(feePercent),
		NetworkFee:   decimal.MustParse(networkFee),
	}
}

func (c MasterCardFeeCalculator) CalculateFee(transactionAmount, eurToTxRate decimal.Decimal) decimal.Decimal {
	percentFee := transactionAmount.Mul(c.FXFeePercent).Round(2)
	networkFee := c.NetworkFee.Mul(eurToTxRate).Round(2)
	return percentFee.Add(networkFee)
}

type FXFeeApplier struct {
	MasterCard MasterCardFeeCalculator
}

func (a FXFeeApplier) Apply(cardBrand string, transactionAmount decimal.Decimal, originCurrency, cardCurrency string, eurToTxRate decimal.Decimal) decimal.Decimal {
	if originCurrency == "" || cardCurrency == "" || strings.EqualFold(originCurrency, cardCurrency) {
		return transactionAmount
	}
	if strings.EqualFold(cardBrand, "MASTERCARD") {
		return transactionAmount.Add(a.MasterCard.CalculateFee(transactionAmount, eurToTxRate))
	}
	return transactionAmount
}

func hasPrefixRange(pan string, width, min, max int) bool {
	if len(pan) < width {
		return false
	}
	n, err := strconv.Atoi(pan[:width])
	if err != nil {
		return false
	}
	return n >= min && n <= max
}

func allDigits(value string) bool {
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return value != ""
}
