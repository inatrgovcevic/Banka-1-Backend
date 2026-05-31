package model

// RoutingKeyMap maps each RabbitMQ routing key to the corresponding NotificationType.
var RoutingKeyMap = map[string]NotificationType{
	"employee.created":             NotificationTypeEmployeeCreated,
	"employee.password_reset":      NotificationTypeEmployeePasswordReset,
	"employee.account_deactivated": NotificationTypeEmployeeAccountDeact,

	"client.created":             NotificationTypeClientCreated,
	"client.password_reset":      NotificationTypeClientPasswordReset,
	"client.account_deactivated": NotificationTypeClientAccountDeact,

	"verification.otp": NotificationTypeVerificationOTP,

	"card.request_verification": NotificationTypeCardRequestVerification,
	"card.request_success":      NotificationTypeCardRequestSuccess,
	"card.request_failure":      NotificationTypeCardRequestFailure,
	"card.blocked":              NotificationTypeCardBlocked,
	"card.unblocked":            NotificationTypeCardUnblocked,
	"card.deactivated":          NotificationTypeCardDeactivated,

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
}

// ResolveNotificationType returns the NotificationType for a given routing key.
// The second return value is false when the routing key is unknown/unsupported.
func ResolveNotificationType(routingKey string) (NotificationType, bool) {
	nt, ok := RoutingKeyMap[routingKey]
	return nt, ok
}
