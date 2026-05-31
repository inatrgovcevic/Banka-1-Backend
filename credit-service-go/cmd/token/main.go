package main

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "ci-test-secret-that-is-long-enough-for-hmac"
	}

	role := os.Getenv("JWT_ROLE")
	if role == "" {
		role = "CLIENT_BASIC"
	}

	claims := jwt.MapClaims{
		"id":       float64(1),
		"sub":      "test@example.com",
		"username": "test-user",
		"roles":    role,
		"exp":      time.Now().Add(time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		panic(err)
	}

	fmt.Println(signed)
}
