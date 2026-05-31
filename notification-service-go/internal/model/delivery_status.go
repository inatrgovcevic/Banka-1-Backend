package model

import "fmt"

// DeliveryStatus represents the lifecycle state of a single notification delivery attempt.
//
//	PENDING → PROCESSING → SUCCEEDED
//	PENDING → PROCESSING → RETRY_SCHEDULED → PROCESSING → ...
//	PENDING → PROCESSING → FAILED
type DeliveryStatus string

const (
	StatusPending        DeliveryStatus = "PENDING"
	StatusProcessing     DeliveryStatus = "PROCESSING"
	StatusRetryScheduled DeliveryStatus = "RETRY_SCHEDULED"
	StatusSucceeded      DeliveryStatus = "SUCCEEDED"
	StatusFailed         DeliveryStatus = "FAILED"
)

func (s DeliveryStatus) IsTerminal() bool {
	return s == StatusSucceeded || s == StatusFailed
}

func (s DeliveryStatus) IsRecoverable() bool {
	return s == StatusPending || s == StatusRetryScheduled
}

func (s DeliveryStatus) CanTransitionTo(next DeliveryStatus) error {
	allowed := validTransitions[s]
	for _, a := range allowed {
		if a == next {
			return nil
		}
	}
	return fmt.Errorf("illegal status transition: %s → %s", s, next)
}

var validTransitions = map[DeliveryStatus][]DeliveryStatus{
	StatusPending:        {StatusProcessing, StatusFailed},
	StatusProcessing:     {StatusSucceeded, StatusRetryScheduled, StatusFailed},
	StatusRetryScheduled: {StatusProcessing, StatusFailed},
	StatusSucceeded:      {},
	StatusFailed:         {},
}

// NotificationType is the strongly-typed event category resolved from the AMQP routing key.
type NotificationType string

const (
	NotificationTypeEmployeeCreated         NotificationType = "EMPLOYEE_CREATED"
	NotificationTypeEmployeePasswordReset   NotificationType = "EMPLOYEE_PASSWORD_RESET"
	NotificationTypeEmployeeAccountDeact    NotificationType = "EMPLOYEE_ACCOUNT_DEACTIVATED"
	NotificationTypeClientCreated           NotificationType = "CLIENT_CREATED"
	NotificationTypeClientPasswordReset     NotificationType = "CLIENT_PASSWORD_RESET"
	NotificationTypeClientAccountDeact      NotificationType = "CLIENT_ACCOUNT_DEACTIVATED"
	NotificationTypeVerificationOTP         NotificationType = "VERIFICATION_OTP"
	NotificationTypeCardRequestVerification NotificationType = "CARD_REQUEST_VERIFICATION"
	NotificationTypeCardRequestSuccess      NotificationType = "CARD_REQUEST_SUCCESS"
	NotificationTypeCardRequestFailure      NotificationType = "CARD_REQUEST_FAILURE"
	NotificationTypeCardBlocked             NotificationType = "CARD_BLOCKED"
	NotificationTypeCardUnblocked           NotificationType = "CARD_UNBLOCKED"
	NotificationTypeCardDeactivated         NotificationType = "CARD_DEACTIVATED"
	NotificationTypeCreditApproved          NotificationType = "CREDIT_APPROVED"
	NotificationTypeCreditDeclined          NotificationType = "CREDIT_DECLINED"
	NotificationTypeCreditInstallmentFailed NotificationType = "CREDIT_INSTALLMENT_FAILED"
	NotificationTypeOrderApproved           NotificationType = "ORDER_APPROVED"
	NotificationTypeOrderDeclined           NotificationType = "ORDER_DECLINED"
	NotificationTypeTaxCollected            NotificationType = "TAX_COLLECTED"
	NotificationTypeOTCCounterOffered       NotificationType = "OTC_COUNTER_OFFERED"
	NotificationTypeOTCAccepted             NotificationType = "OTC_ACCEPTED"
	NotificationTypeOTCCanceled             NotificationType = "OTC_CANCELED"
	NotificationTypeOTCExpiryReminder       NotificationType = "OTC_EXPIRY_REMINDER"
)

// ErrDeliveryNotEligible is returned by MarkProcessing when the target delivery
// is not in an eligible state for processing.
type ErrDeliveryNotEligible struct {
	DeliveryID string
	Reason     string
}

func (e *ErrDeliveryNotEligible) Error() string {
	return "delivery " + e.DeliveryID + " not eligible for processing: " + e.Reason
}

// ErrDeliveryNotFound is returned when a lookup by primary key yields no result.
type ErrDeliveryNotFound struct {
	DeliveryID string
}

func (e *ErrDeliveryNotFound) Error() string {
	return "delivery not found: " + e.DeliveryID
}