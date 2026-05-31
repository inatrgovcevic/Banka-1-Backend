package platform

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

var springArgon2 = argon2Params{
	memory:      65536,
	iterations:  3,
	parallelism: 1,
	saltLength:  16,
	keyLength:   32,
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, springArgon2.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, springArgon2.iterations, springArgon2.memory, springArgon2.parallelism, springArgon2.keyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		springArgon2.memory,
		springArgon2.iterations,
		springArgon2.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

func VerifyPassword(password, encoded string) bool {
	params, salt, expected, err := parseArgon2id(encoded)
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, params.iterations, params.memory, params.parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func parseArgon2id(encoded string) (argon2Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return argon2Params{}, nil, nil, errors.New("unsupported hash")
	}
	params := argon2Params{}
	for _, item := range strings.Split(parts[3], ",") {
		kv := strings.SplitN(item, "=", 2)
		if len(kv) != 2 {
			return argon2Params{}, nil, nil, errors.New("invalid parameters")
		}
		value, err := strconv.Atoi(kv[1])
		if err != nil {
			return argon2Params{}, nil, nil, err
		}
		switch kv[0] {
		case "m":
			params.memory = uint32(value)
		case "t":
			params.iterations = uint32(value)
		case "p":
			params.parallelism = uint8(value)
		}
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argon2Params{}, nil, nil, err
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argon2Params{}, nil, nil, err
	}
	return params, salt, hash, nil
}
