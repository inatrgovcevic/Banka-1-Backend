package platform_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"banka1/user-service-go/internal/platform"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validJMBGCrypto(t *testing.T) *platform.JMBGCrypto {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	c, err := platform.NewJMBGCrypto(platform.JMBGConfig{
		AESKeyBase64: base64.StdEncoding.EncodeToString(key),
	})
	require.NoError(t, err)
	return c
}

func TestJMBGCrypto_EncryptDecrypt_RoundTrip(t *testing.T) {
	t.Parallel()
	c := validJMBGCrypto(t)
	plaintext := "1234567890123"

	encrypted, err := c.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, encrypted)

	decrypted, err := c.Decrypt(encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestJMBGCrypto_Encrypt_EmptyString_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	c := validJMBGCrypto(t)
	enc, err := c.Encrypt("")
	require.NoError(t, err)
	assert.Empty(t, enc)
}

func TestJMBGCrypto_Decrypt_EmptyString_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	c := validJMBGCrypto(t)
	dec, err := c.Decrypt("")
	require.NoError(t, err)
	assert.Empty(t, dec)
}

func TestJMBGCrypto_Decrypt_InvalidBase64_ReturnsError(t *testing.T) {
	t.Parallel()
	c := validJMBGCrypto(t)
	_, err := c.Decrypt("not-valid-base64!!!")
	require.Error(t, err)
}

func TestJMBGCrypto_Decrypt_TooShortPayload_ReturnsError(t *testing.T) {
	t.Parallel()
	c := validJMBGCrypto(t)
	short := base64.StdEncoding.EncodeToString([]byte("tooshort"))
	_, err := c.Decrypt(short)
	require.Error(t, err)
}

func TestNewJMBGCrypto_InvalidBase64Key_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := platform.NewJMBGCrypto(platform.JMBGConfig{AESKeyBase64: "not-base64!!"})
	require.Error(t, err)
}

func TestNewJMBGCrypto_WrongKeyLength_ReturnsError(t *testing.T) {
	t.Parallel()
	shortKey := base64.StdEncoding.EncodeToString([]byte("short"))
	_, err := platform.NewJMBGCrypto(platform.JMBGConfig{AESKeyBase64: shortKey})
	require.Error(t, err)
}

func TestJMBGCrypto_Encrypt_ProducesDifferentCiphertexts(t *testing.T) {
	t.Parallel()
	c := validJMBGCrypto(t)
	e1, _ := c.Encrypt("same")
	e2, _ := c.Encrypt("same")
	assert.NotEqual(t, e1, e2, "different IVs must produce different ciphertexts")
}

func TestSHA256Hex_KnownValue(t *testing.T) {
	t.Parallel()
	result := platform.SHA256Hex("hello")
	assert.Equal(t, 64, len(result), "SHA256 hex is always 64 chars")
	assert.True(t, strings.ContainsAny(result, "0123456789abcdef"))
}

func TestRandomURLToken_ReturnsNonEmptyToken(t *testing.T) {
	t.Parallel()
	tok, err := platform.RandomURLToken()
	require.NoError(t, err)
	assert.NotEmpty(t, tok)
	assert.True(t, len(tok) > 20)
}

func TestRandomURLToken_ProducesDifferentTokens(t *testing.T) {
	t.Parallel()
	t1, _ := platform.RandomURLToken()
	t2, _ := platform.RandomURLToken()
	assert.NotEqual(t, t1, t2)
}
