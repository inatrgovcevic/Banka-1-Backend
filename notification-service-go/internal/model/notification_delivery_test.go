package model_test

import (
	"strings"
	"testing"
	"time"

	"Banka1Back/notification-service-go/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeliveryStatus_IsTerminal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status   model.DeliveryStatus
		terminal bool
	}{
		{model.StatusPending, false},
		{model.StatusProcessing, false},
		{model.StatusRetryScheduled, false},
		{model.StatusSucceeded, true},
		{model.StatusFailed, true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.status), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.terminal, tc.status.IsTerminal())
		})
	}
}

func TestDeliveryStatus_IsRecoverable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status      model.DeliveryStatus
		recoverable bool
	}{
		{model.StatusPending, true},
		{model.StatusRetryScheduled, true},
		{model.StatusProcessing, false},
		{model.StatusSucceeded, false},
		{model.StatusFailed, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.status), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.recoverable, tc.status.IsRecoverable())
		})
	}
}

func TestDeliveryStatus_CanTransitionTo_AllowedPaths(t *testing.T) {
	t.Parallel()

	allowed := []struct{ from, to model.DeliveryStatus }{
		{model.StatusPending, model.StatusProcessing},
		{model.StatusPending, model.StatusFailed},
		{model.StatusProcessing, model.StatusSucceeded},
		{model.StatusProcessing, model.StatusRetryScheduled},
		{model.StatusProcessing, model.StatusFailed},
		{model.StatusRetryScheduled, model.StatusProcessing},
		{model.StatusRetryScheduled, model.StatusFailed},
	}

	for _, tc := range allowed {
		tc := tc
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, tc.from.CanTransitionTo(tc.to))
		})
	}
}

func TestDeliveryStatus_CanTransitionTo_ForbiddenPaths(t *testing.T) {
	t.Parallel()

	forbidden := []struct{ from, to model.DeliveryStatus }{
		{model.StatusSucceeded, model.StatusFailed},
		{model.StatusFailed, model.StatusSucceeded},
		{model.StatusFailed, model.StatusRetryScheduled},
		{model.StatusSucceeded, model.StatusPending},
		{model.StatusPending, model.StatusSucceeded},
		{model.StatusPending, model.StatusRetryScheduled},
	}

	for _, tc := range forbidden {
		tc := tc
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			t.Parallel()
			assert.Error(t, tc.from.CanTransitionTo(tc.to))
		})
	}
}

func TestIsEligibleForAttempt_PendingWithBudget(t *testing.T) {
	t.Parallel()
	d := &model.NotificationDelivery{Status: model.StatusPending, AttemptCount: 0, MaxRetries: 4}
	assert.True(t, d.IsEligibleForAttempt(time.Now()))
}

func TestIsEligibleForAttempt_PendingBudgetExhausted(t *testing.T) {
	t.Parallel()
	d := &model.NotificationDelivery{Status: model.StatusPending, AttemptCount: 4, MaxRetries: 4}
	assert.False(t, d.IsEligibleForAttempt(time.Now()))
}

func TestIsEligibleForAttempt_RetryScheduledDue(t *testing.T) {
	t.Parallel()
	past := time.Now().Add(-10 * time.Second)
	d := &model.NotificationDelivery{
		Status: model.StatusRetryScheduled, AttemptCount: 2, MaxRetries: 4, NextAttemptAt: &past,
	}
	assert.True(t, d.IsEligibleForAttempt(time.Now()))
}

func TestIsEligibleForAttempt_RetryScheduledNotYetDue(t *testing.T) {
	t.Parallel()
	future := time.Now().Add(30 * time.Second)
	d := &model.NotificationDelivery{
		Status: model.StatusRetryScheduled, AttemptCount: 1, MaxRetries: 4, NextAttemptAt: &future,
	}
	assert.False(t, d.IsEligibleForAttempt(time.Now()))
}

func TestIsEligibleForAttempt_RetryScheduledMissingNextAttemptAt(t *testing.T) {
	t.Parallel()
	d := &model.NotificationDelivery{
		Status: model.StatusRetryScheduled, AttemptCount: 1, MaxRetries: 4, NextAttemptAt: nil,
	}
	assert.False(t, d.IsEligibleForAttempt(time.Now()))
}

func TestIsEligibleForAttempt_TerminalStates(t *testing.T) {
	t.Parallel()
	for _, status := range []model.DeliveryStatus{model.StatusSucceeded, model.StatusFailed} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()
			d := &model.NotificationDelivery{Status: status, AttemptCount: 0, MaxRetries: 4}
			assert.False(t, d.IsEligibleForAttempt(time.Now()))
		})
	}
}

func TestShouldScheduleRetry_RetryableWithBudget(t *testing.T) {
	t.Parallel()
	d := &model.NotificationDelivery{AttemptCount: 1, MaxRetries: 4}
	assert.True(t, d.ShouldScheduleRetry(true))
}

func TestShouldScheduleRetry_NonRetryableError(t *testing.T) {
	t.Parallel()
	d := &model.NotificationDelivery{AttemptCount: 1, MaxRetries: 4}
	assert.False(t, d.ShouldScheduleRetry(false))
}

func TestShouldScheduleRetry_BudgetExhausted(t *testing.T) {
	t.Parallel()
	d := &model.NotificationDelivery{AttemptCount: 4, MaxRetries: 4}
	assert.False(t, d.ShouldScheduleRetry(true))
}

func TestTrimError_ShortMessageUnchanged(t *testing.T) {
	t.Parallel()
	msg := "SMTP connection refused"
	assert.Equal(t, msg, model.TrimError(msg))
}

func TestTrimError_LongMessageTrimmedToMaxLength(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", model.MaxErrorLength+500)
	result := model.TrimError(long)
	assert.Equal(t, model.MaxErrorLength, len(result))
}

func TestNewPendingDelivery_HasExpectedFields(t *testing.T) {
	t.Parallel()
	d := model.NewPendingDelivery(
		"user@example.com", "Welcome", "<html>Hello</html>",
		model.NotificationTypeEmployeeCreated, 4,
	)
	require.NotEmpty(t, d.DeliveryID)
	assert.Equal(t, model.StatusPending, d.Status)
	assert.Equal(t, "user@example.com", d.RecipientEmail)
	assert.Equal(t, 0, d.AttemptCount)
	assert.Equal(t, 4, d.MaxRetries)
	assert.Nil(t, d.LastError)
	assert.Nil(t, d.NextAttemptAt)
}

func TestNewFailedAudit_HasExpectedFields(t *testing.T) {
	t.Parallel()
	d := model.NewFailedAudit(
		"unknown", model.NotificationType("UNKNOWN"),
		"Unsupported routing key: foo.bar", 4,
	)
	require.NotEmpty(t, d.DeliveryID)
	assert.Equal(t, model.StatusFailed, d.Status)
	assert.Equal(t, "unknown", d.RecipientEmail)
	assert.Equal(t, "", d.Subject)
	assert.NotNil(t, d.LastError)
	assert.Contains(t, *d.LastError, "Unsupported routing key")
}

func TestResolveNotificationType_KnownKeys(t *testing.T) {
	t.Parallel()
	cases := []struct {
		key      string
		expected model.NotificationType
	}{
		{"employee.created", model.NotificationTypeEmployeeCreated},
		{"employee.password_reset", model.NotificationTypeEmployeePasswordReset},
		{"card.blocked", model.NotificationTypeCardBlocked},
		{"credit.approved", model.NotificationTypeCreditApproved},
		{"otc.accepted", model.NotificationTypeOTCAccepted},
		{"tax.collected", model.NotificationTypeTaxCollected},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()
			got, ok := model.ResolveNotificationType(tc.key)
			require.True(t, ok)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestResolveNotificationType_UnknownKey(t *testing.T) {
	t.Parallel()
	_, ok := model.ResolveNotificationType("unknown.event.xyz")
	assert.False(t, ok)
}
