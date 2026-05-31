package smtp_test

import (
	"errors"
	"fmt"
	"net/textproto"
	"testing"

	"Banka1Back/notification-service-go/internal/smtp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsRetryable_NilError_ReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, smtp.IsRetryable(nil))
}

func TestIsRetryable_MailAuthError_ReturnsFalse(t *testing.T) {
	t.Parallel()
	err := &smtp.MailAuthError{Cause: errors.New("535 credentials invalid")}
	assert.False(t, smtp.IsRetryable(err))
}

func TestIsRetryable_PermanentSMTPError_ReturnsFalse(t *testing.T) {
	t.Parallel()
	err := &smtp.PermanentSMTPError{Code: 550, Msg: "No such user here"}
	assert.False(t, smtp.IsRetryable(err))
}

func TestIsRetryable_TextprotoError535_ReturnsFalse(t *testing.T) {
	t.Parallel()
	wrapped := &smtp.MailAuthError{Cause: &textproto.Error{Code: 535, Msg: "5.7.8 Username and Password not accepted"}}
	assert.False(t, smtp.IsRetryable(wrapped))
}

func TestIsRetryable_TransientError452_ReturnsTrue(t *testing.T) {
	t.Parallel()
	err := &textproto.Error{Code: 452, Msg: "Insufficient system storage"}
	assert.True(t, smtp.IsRetryable(err))
}

func TestIsRetryable_NetworkError_ReturnsTrue(t *testing.T) {
	t.Parallel()
	err := errors.New("dial tcp: connection refused")
	assert.True(t, smtp.IsRetryable(err))
}

func TestIsRetryable_GenericError_ReturnsTrue(t *testing.T) {
	t.Parallel()
	err := errors.New("unexpected EOF")
	assert.True(t, smtp.IsRetryable(err))
}

func TestMailAuthError_ImplementsErrorInterface(t *testing.T) {
	t.Parallel()
	cause := errors.New("535 invalid credentials")
	authErr := &smtp.MailAuthError{Cause: cause}
	require.Error(t, authErr)
	assert.Contains(t, authErr.Error(), "SMTP authentication failed")
}

func TestMailAuthError_Unwrap_ExposesCause(t *testing.T) {
	t.Parallel()
	cause := errors.New("535 invalid credentials")
	authErr := &smtp.MailAuthError{Cause: cause}

	assert.Equal(t, cause.Error(), errors.Unwrap(authErr).Error())
}

func TestPermanentSMTPError_ErrorMessageContainsCode(t *testing.T) {
	t.Parallel()
	permErr := &smtp.PermanentSMTPError{Code: 554, Msg: "Transaction failed"}
	assert.Contains(t, permErr.Error(), "554")
	assert.Contains(t, permErr.Error(), "Transaction failed")
}

func TestIsRetryable_WrappedMailAuthError_StillReturnsFalse(t *testing.T) {
	t.Parallel()
	inner := &smtp.MailAuthError{Cause: errors.New("535")}
	wrapped := fmt.Errorf("outer: %w", inner)
	assert.False(t, smtp.IsRetryable(wrapped))
}

func TestIsRetryable_WrappedPermanentSMTPError_StillReturnsFalse(t *testing.T) {
	t.Parallel()
	inner := &smtp.PermanentSMTPError{Code: 550, Msg: "Rejected"}
	wrapped := fmt.Errorf("outer: %w", inner)
	assert.False(t, smtp.IsRetryable(wrapped))
}
