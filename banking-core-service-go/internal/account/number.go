package account

import (
	"crypto/rand"
	"math/big"
)

type NumberGenerator struct{}

func (NumberGenerator) Generate() (string, error) {
	body := make([]byte, 15)
	for i := range body {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		body[i] = byte('0' + n.Int64())
	}
	check := mod11CheckDigit(string(body))
	return string(append(body, byte('0'+check))), nil
}

func mod11CheckDigit(fifteenDigits string) int {
	sum := 0
	weight := 2
	for i := len(fifteenDigits) - 1; i >= 0; i-- {
		sum += int(fifteenDigits[i]-'0') * weight
		if weight == 7 {
			weight = 2
		} else {
			weight++
		}
	}
	check := 11 - (sum % 11)
	if check >= 10 {
		return 0
	}
	return check
}
