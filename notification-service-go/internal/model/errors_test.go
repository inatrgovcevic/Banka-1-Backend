package model_test

import (
	"testing"

	"Banka1Back/notification-service-go/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestErrDeliveryNotEligible_Error_ContainsIDAndReason(t *testing.T) {
	t.Parallel()
	err := &model.ErrDeliveryNotEligible{DeliveryID: "abc-123", Reason: "already processing"}
	assert.Contains(t, err.Error(), "abc-123")
	assert.Contains(t, err.Error(), "already processing")
}

func TestErrDeliveryNotFound_Error_ContainsID(t *testing.T) {
	t.Parallel()
	err := &model.ErrDeliveryNotFound{DeliveryID: "xyz-999"}
	assert.Contains(t, err.Error(), "xyz-999")
}

func TestIsPushOnlyNotificationType_PriceAlert_True(t *testing.T) {
	t.Parallel()
	assert.True(t, model.IsPushOnlyNotificationType(model.NotificationTypePriceAlertTriggered))
}

func TestIsPushOnlyNotificationType_OrderRecurringSkipped_True(t *testing.T) {
	t.Parallel()
	assert.True(t, model.IsPushOnlyNotificationType(model.NotificationTypeOrderRecurringSkipped))
}

func TestIsPushOnlyNotificationType_EmailType_False(t *testing.T) {
	t.Parallel()
	assert.False(t, model.IsPushOnlyNotificationType(model.NotificationTypeEmployeeCreated))
}

func TestResolveNotificationType_AllRoutingKeys(t *testing.T) {
	t.Parallel()
	cases := []struct {
		key      string
		expected model.NotificationType
	}{
		{"client.created", model.NotificationTypeClientCreated},
		{"client.password_reset", model.NotificationTypeClientPasswordReset},
		{"client.account_deactivated", model.NotificationTypeClientAccountDeact},
		{"client.verification", model.NotificationTypeVerificationOTP},
		{"account.created", model.NotificationTypeAccountCreated},
		{"account.deactivated", model.NotificationTypeAccountDeactivated},
		{"transaction.completed", model.NotificationTypeTransactionCompleted},
		{"transaction.denied", model.NotificationTypeTransactionDenied},
		{"credit.requested", model.NotificationTypeCreditRequested},
		{"price.alert_triggered", model.NotificationTypePriceAlertTriggered},
		{"order.recurring_skipped", model.NotificationTypeOrderRecurringSkipped},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()
			got, ok := model.ResolveNotificationType(tc.key)
			assert.True(t, ok)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestIgnoredRoutingKeys_ContainsExpectedKeys(t *testing.T) {
	t.Parallel()
	assert.True(t, model.IgnoredRoutingKeys["card.create"])
	assert.True(t, model.IgnoredRoutingKeys["card.deactivate"])
	assert.False(t, model.IgnoredRoutingKeys["employee.created"])
}
