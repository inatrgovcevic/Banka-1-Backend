package platform

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

const gcmIVLength = 12

type JMBGCrypto struct {
	gcm cipher.AEAD
}

func NewJMBGCrypto(cfg JMBGConfig) (*JMBGCrypto, error) {
	key, err := base64.StdEncoding.DecodeString(cfg.AESKeyBase64)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("JMBG AES key must decode to 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &JMBGCrypto{gcm: gcm}, nil
}

func (c *JMBGCrypto) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	iv := make([]byte, gcmIVLength)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}
	ciphertext := c.gcm.Seal(nil, iv, []byte(plaintext), nil)
	combined := append(iv, ciphertext...)
	return base64.StdEncoding.EncodeToString(combined), nil
}

func (c *JMBGCrypto) Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	combined, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	if len(combined) < gcmIVLength+c.gcm.Overhead() {
		return "", errors.New("encrypted JMBG payload too short")
	}
	iv := combined[:gcmIVLength]
	ciphertext := combined[gcmIVLength:]
	plaintext, err := c.gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
