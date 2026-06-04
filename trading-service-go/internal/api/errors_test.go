package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReasonPhrase_KnownCodes(t *testing.T) {
	t.Parallel()
	cases := map[int]string{
		400: "Bad Request",
		401: "Unauthorized",
		403: "Forbidden",
		404: "Not Found",
		405: "Method Not Allowed",
		409: "Conflict",
		500: "Internal Server Error",
	}
	for code, want := range cases {
		assert.Equal(t, want, ReasonPhrase(code), "status %d", code)
	}
}

func TestReasonPhrase_UnknownCode_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", ReasonPhrase(418))
}

func TestNewOrderError_CreatesCorrectError(t *testing.T) {
	t.Parallel()
	err := NewOrderError(404, "resource not found")
	assert.Equal(t, 404, err.Status)
	assert.Equal(t, "resource not found", err.Message)
	assert.Equal(t, ShapeOrder, err.Shape)
	assert.Equal(t, "resource not found", err.Error())
}

func TestNewOtcError_CreatesCorrectError(t *testing.T) {
	t.Parallel()
	err := NewOtcError(409, "state conflict")
	assert.Equal(t, 409, err.Status)
	assert.Equal(t, ShapeOtc, err.Shape)
}

func TestNewOrderValidation_CreatesValidationError(t *testing.T) {
	t.Parallel()
	err := NewOrderValidation(map[string]string{"amount": "must be > 0"})
	assert.Equal(t, 400, err.Status)
	assert.Equal(t, ShapeOrder, err.Shape)
	assert.Equal(t, "must be > 0", err.FieldErrors["amount"])
}

func TestDomainError_Error_ReturnsMessage(t *testing.T) {
	t.Parallel()
	err := &DomainError{Message: "test error"}
	assert.Equal(t, "test error", err.Error())
}
