package account

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
)

const (
	monasFixedPrefix      = "1110001"
	monasFixedPrefixSum   = 4
	maxGenerationAttempts = 1024
)

var monasTypePattern = regexp.MustCompile(`^(1[1-7]|2[1-2])$`)

type ExistsChecker interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func GenerateMONAS(ctx context.Context, db ExistsChecker, typeVal string, random *rand.Rand) (string, error) {
	if !monasTypePattern.MatchString(typeVal) {
		return "", fmt.Errorf("invalid MONAS account type %q", typeVal)
	}
	if random == nil {
		random = rand.New(rand.NewSource(rand.Int63()))
	}
	for attempt := 0; attempt < maxGenerationAttempts; attempt++ {
		randomPart := make([]byte, 0, 9)
		sum := monasFixedPrefixSum + int(typeVal[0]-'0') + int(typeVal[1]-'0')
		for i := 0; i < 8; i++ {
			d := random.Intn(10)
			randomPart = append(randomPart, byte('0'+d))
			sum += d
		}
		last := (11 - sum%11) % 11
		if last == 10 {
			continue
		}
		randomPart = append(randomPart, byte('0'+last))
		candidate := monasFixedPrefix + string(randomPart) + typeVal

		var exists bool
		if err := db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM account_table WHERE broj_racuna = $1)", candidate).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", ErrMONASGenerationFailed
}

var ErrMONASGenerationFailed = errors.New("nije moguce generisati jedinstven broj racuna")

func ValidateMONAS(number string) bool {
	if len(number) != 18 || !allDigits(number) {
		return false
	}
	if number[:len(monasFixedPrefix)] != monasFixedPrefix {
		return false
	}
	if !monasTypePattern.MatchString(number[16:18]) {
		return false
	}
	return DigitSum(number)%11 == 0
}

func DigitSum(digits string) int {
	sum := 0
	for _, ch := range digits {
		if ch >= '0' && ch <= '9' {
			sum += int(ch - '0')
		}
	}
	return sum
}

func allDigits(value string) bool {
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
