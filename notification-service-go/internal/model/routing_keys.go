package model

// IgnoredRoutingKeys contains internal command events that the notification-service
// intentionally acknowledges without sending an email (no userEmail in payload).
var IgnoredRoutingKeys = map[string]bool{
	"card.create":    true,
	"card.deactivate": true,
}

// RoutingKeyMap maps each RabbitMQ routing key to the corresponding NotificationType.
var RoutingKeyMap = map[string]NotificationType{
	"employee.created":             NotificationTypeEmployeeCreated,
	"employee.password_reset":      NotificationTypeEmployeePasswordReset,
	"employee.account_deactivated": NotificationTypeEmployeeAccountDeact,

	"client.created":             NotificationTypeClientCreated,
	"client.password_reset":      NotificationTypeClientPasswordReset,
	"client.account_deactivated": NotificationTypeClientAccountDeact,

	"verification.otp":    NotificationTypeVerificationOTP,
	"client.verification": NotificationTypeVerificationOTP,

	"card.request_verification": NotificationTypeCardRequestVerification,
	"card.request_success":      NotificationTypeCardRequestSuccess,
	"card.request_failure":      NotificationTypeCardRequestFailure,
	"card.blocked":              NotificationTypeCardBlocked,
	"card.unblocked":            NotificationTypeCardUnblocked,
	"card.deactivated":          NotificationTypeCardDeactivated,

	"credit.requested":          NotificationTypeCreditRequested,
	"credit.approved":           NotificationTypeCreditApproved,
	"credit.declined":           NotificationTypeCreditDeclined,
	"credit.installment_failed": NotificationTypeCreditInstallmentFailed,

	"order.approved": NotificationTypeOrderApproved,
	"order.declined": NotificationTypeOrderDeclined,

	"tax.collected": NotificationTypeTaxCollected,

	"otc.countered":       NotificationTypeOTCCounterOffered,
	"otc.accepted":        NotificationTypeOTCAccepted,
	"otc.canceled":        NotificationTypeOTCCanceled,
	"otc.expiry_reminder": NotificationTypeOTCExpiryReminder,

	"account.created":     NotificationTypeAccountCreated,
	"account.deactivated": NotificationTypeAccountDeactivated,

	"transaction.completed": NotificationTypeTransactionCompleted,
	"transaction.denied":    NotificationTypeTransactionDenied,

	"price.alert_triggered":   NotificationTypePriceAlertTriggered,
	"order.recurring_skipped": NotificationTypeOrderRecurringSkipped,
}

// PushOnlyNotificationTypes contains notification types that are mobile-push
// exclusive and may be produced without a recipient email address.
var PushOnlyNotificationTypes = map[NotificationType]bool{
	NotificationTypePriceAlertTriggered:  true,
	NotificationTypeOrderRecurringSkipped: true,
}

// IsPushOnlyNotificationType reports whether the given notification type is
// push-only and may arrive without a recipient email address.
func IsPushOnlyNotificationType(nt NotificationType) bool {
	return PushOnlyNotificationTypes[nt]
}

// ResolveNotificationType returns the NotificationType for a given routing key.
// The second return value is false when the routing key is unknown/unsupported.
func ResolveNotificationType(routingKey string) (NotificationType, bool) {
	nt, ok := RoutingKeyMap[routingKey]
	return nt, ok
}
