package auth

import (
	"errors"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

func ParseBearerToken(authHeader string) (User, error) {
	if authHeader == "" {
		return User{}, errors.New("missing authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return User{}, errors.New("invalid authorization header")
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return User{}, errors.New("missing JWT_SECRET")
	}

	token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return User{}, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return User{}, errors.New("invalid claims")
	}

	user := User{}

	if idValue, ok := claims["id"].(float64); ok {
		user.ID = int64(idValue)
	}

	if emailValue, ok := claims["sub"].(string); ok {
		user.Email = emailValue
	}

	if usernameValue, ok := claims["username"].(string); ok {
		user.Username = usernameValue
	}

	if roleValue, ok := claims["roles"].(string); ok {
		user.Role = roleValue
	}

	if roleArray, ok := claims["roles"].([]interface{}); ok && len(roleArray) > 0 {
		if firstRole, ok := roleArray[0].(string); ok {
			user.Role = firstRole
		}
	}

	return user, nil
}
