package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	MaxErrorLength   = 1000
	UnknownRecipient = "unknown"
)

// NotificationDelivery is the entity for the `notification_deliveries` table.
// It is the single source of truth for the entire delivery retry lifecycle.
type NotificationDelivery struct {
	DeliveryID       string
	RecipientEmail   string
	Subject          string
	Body             string
	Status           DeliveryStatus
	NotificationType string
	AttemptCount     int
	MaxRetries       int
	LastError        *string
	NextAttemptAt    *time.Time
	LastAttemptAt    *time.Time
	SentAt           *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IsEligibleForAttempt returns true when this delivery can legally transition to PROCESSING.
func (nd *NotificationDelivery) IsEligibleForAttempt(now time.Time) bool {
	if nd.Status.IsTerminal() {
		return false
	}
	if nd.AttemptCount >= nd.MaxRetries {
		return false
	}
	if nd.Status == StatusRetryScheduled {
		if nd.NextAttemptAt == nil {
			return false
		}
		return !nd.NextAttemptAt.After(now)
	}
	return nd.Status == StatusPending || nd.Status == StatusProcessing
}

// ShouldScheduleRetry returns true when a failed attempt should result in RETRY_SCHEDULED.
func (nd *NotificationDelivery) ShouldScheduleRetry(retryable bool) bool {
	if !retryable {
		return false
	}
	return nd.AttemptCount < nd.MaxRetries
}

// TrimError truncates an error string to MaxErrorLength.
func TrimError(errMsg string) string {
	if len(errMsg) <= MaxErrorLength {
		return errMsg
	}
	return errMsg[:MaxErrorLength]
}

// NewPendingDelivery constructs a new NotificationDelivery in PENDING status.
func NewPendingDelivery(
	recipientEmail, subject, body string,
	notificationType NotificationType,
	maxRetries int,
) *NotificationDelivery {
	return &NotificationDelivery{
		DeliveryID:       uuid.New().String(),
		RecipientEmail:   recipientEmail,
		Subject:          subject,
		Body:             body,
		Status:           StatusPending,
		NotificationType: string(notificationType),
		AttemptCount:     0,
		MaxRetries:       maxRetries,
	}
}

// NewFailedAudit constructs a terminal FAILED record for messages that cannot
// enter the normal delivery lifecycle (unsupported routing key, missing payload).
func NewFailedAudit(
	recipientEmail string,
	notificationType NotificationType,
	errMsg string,
	maxRetries int,
) *NotificationDelivery {
	trimmed := TrimError(errMsg)
	return &NotificationDelivery{
		DeliveryID:       uuid.New().String(),
		RecipientEmail:   recipientEmail,
		Subject:          "",
		Body:             "",
		Status:           StatusFailed,
		NotificationType: string(notificationType),
		AttemptCount:     0,
		MaxRetries:       maxRetries,
		LastError:        &trimmed,
	}
}