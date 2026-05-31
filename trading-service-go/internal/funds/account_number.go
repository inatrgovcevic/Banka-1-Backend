package funds

import (
	"crypto/rand"
	"math/big"
	"strconv"
	"strings"
)

// GenerateAccountNumber mirrors FundAccountNumberGenerator.generate: 15 random
// digits + a 1-digit check digit (mod-11 weighted) = 16 digits total. The
// `investment_funds.account_number` column has a regex check `^[0-9]{16}$`.
func GenerateAccountNumber() (string, error) {
	var b strings.Builder
	b.Grow(16)
	for i := 0; i < 15; i++ {
		d, err := randomDigit()
		if err != nil {
			return "", err
		}
		b.WriteByte('0' + byte(d))
	}
	prefix := b.String()
	b.WriteByte('0' + byte(checkDigit(prefix)))
	return b.String(), nil
}

// checkDigit mirrors FundAccountNumberGenerator.checkDigit: mod-11 weighted
// (weights cycle 2..7 from the right); the digit is 11-rem, mapped to 0 when ≥10.
func checkDigit(prefix string) int {
	sum := 0
	weight := 2
	for i := len(prefix) - 1; i >= 0; i-- {
		d, _ := strconv.Atoi(string(prefix[i]))
		sum += d * weight
		if weight == 7 {
			weight = 2
		} else {
			weight++
		}
	}
	rem := sum % 11
	cd := 11 - rem
	if cd >= 10 {
		return 0
	}
	return cd
}

func randomDigit() (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}
