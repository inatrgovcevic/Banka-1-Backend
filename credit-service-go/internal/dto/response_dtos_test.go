package dto_test

import (
	"testing"

	"Banka1Back/credit-service-go/internal/dto"

	"github.com/stretchr/testify/assert"
)

func TestNewErrorResponse_PopulatesFields(t *testing.T) {
	t.Parallel()
	resp := dto.NewErrorResponse("ERR_TEST", "Test Title", "Test description")
	assert.Equal(t, "ERR_TEST", resp.ErrorCode)
	assert.Equal(t, "Test Title", resp.ErrorTitle)
	assert.Equal(t, "Test description", resp.ErrorDesc)
	assert.False(t, resp.Timestamp.IsZero())
}

func TestNewValidationErrorResponse_PopulatesValidationErrors(t *testing.T) {
	t.Parallel()
	errs := map[string]string{"amount": "must be > 0"}
	resp := dto.NewValidationErrorResponse(errs)
	assert.Equal(t, "ERR_VALIDATION", resp.ErrorCode)
	assert.Equal(t, "must be > 0", resp.ValidationErrors["amount"])
}

func TestVerificationStatusResponse_IsVerified_True(t *testing.T) {
	t.Parallel()
	r := dto.VerificationStatusResponse{Status: "VERIFIED"}
	assert.True(t, r.IsVerified())
}

func TestVerificationStatusResponse_IsVerified_False(t *testing.T) {
	t.Parallel()
	r := dto.VerificationStatusResponse{Status: "PENDING"}
	assert.False(t, r.IsVerified())
}
