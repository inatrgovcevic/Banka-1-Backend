package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"Banka1Back/notification-service-go/internal/config"
	"Banka1Back/notification-service-go/internal/dto"
	"Banka1Back/notification-service-go/internal/model"
	"Banka1Back/notification-service-go/internal/smtp"
	"Banka1Back/notification-service-go/internal/template"
)

// DeliveryStore is the persistence port used by NotificationService.
// Implemented by *store.NotificationDeliveryStore.
type DeliveryStore interface {
	Create(ctx context.Context, delivery *model.NotificationDelivery) error
	FindByDeliveryID(ctx context.Context, deliveryID string) (*model.NotificationDelivery, error)
	FindDueRetries(ctx context.Context, now time.Time, limit int) ([]*model.NotificationDelivery, error)
	MarkProcessing(ctx context.Context, deliveryID string) error
	MarkSucceeded(ctx context.Context, deliveryID string, attemptedAt time.Time) error
	MarkFailedOrRetry(ctx context.Context, deliveryID string, attemptedAt time.Time, errMsg string, retryable bool, retryDelaySecs int) (time.Time, error)
	PersistFailedAudit(ctx context.Context, delivery *model.NotificationDelivery) error
}

// FcmTokenStore is the persistence port for looking up FCM device tokens.
type FcmTokenStore interface {
	FindByClientId(ctx context.Context, clientId int64) (*model.FcmToken, error)
}

// RetryScheduler feeds a deliveryID back into the retry work queue.
type RetryScheduler interface {
	Schedule(deliveryID string, at time.Time)
}

// EmailSender is the port interface for SMTP delivery.
// *smtp.Sender implements it implicitly.
type EmailSender interface {
	SendEmail(to, subject, body string) error
}

// PushSender is the port interface for FCM push delivery.
type PushSender interface {
	SendNotification(ctx context.Context, deviceToken, title, body string) error
	SendData(ctx context.Context, deviceToken string, data map[string]string) error
}

// NotificationService orchestrates the complete notification delivery lifecycle:
// template rendering → DB persistence → SMTP send / FCM push → retry scheduling.
type NotificationService struct {
	store      DeliveryStore
	renderer   *template.Renderer
	sender     EmailSender
	scheduler  RetryScheduler
	retryCfg   config.RetryConfig
	log        *slog.Logger
	exec       func(f func())
	tokenStore FcmTokenStore
	pushSender PushSender
}

// Option is a functional option for NotificationService.
type Option func(*NotificationService)

// WithExec replaces the goroutine launcher. Used in tests to run AttemptDelivery synchronously.
func WithExec(exec func(f func())) Option {
	return func(s *NotificationService) { s.exec = exec }
}

// WithPush configures the FCM token store and push sender for push notification delivery.
func WithPush(tokenStore FcmTokenStore, pushSender PushSender) Option {
	return func(s *NotificationService) {
		s.tokenStore = tokenStore
		s.pushSender = pushSender
	}
}

// NewNotificationService constructs a fully-wired NotificationService.
func NewNotificationService(
	store DeliveryStore,
	renderer *template.Renderer,
	sender *smtp.Sender,
	scheduler RetryScheduler,
	retryCfg config.RetryConfig,
	log *slog.Logger,
	opts ...Option,
) *NotificationService {
	return NewNotificationServiceWithSender(store, renderer, sender, scheduler, retryCfg, log, opts...)
}

// NewNotificationServiceWithSender is the primary constructor that accepts the
// EmailSender interface — used by both production (with *smtp.Sender) and tests (with a stub).
func NewNotificationServiceWithSender(
	store DeliveryStore,
	renderer *template.Renderer,
	sender EmailSender,
	scheduler RetryScheduler,
	retryCfg config.RetryConfig,
	log *slog.Logger,
	opts ...Option,
) *NotificationService {
	svc := &NotificationService{
		store:     store,
		renderer:  renderer,
		sender:    sender,
		scheduler: scheduler,
		retryCfg:  retryCfg,
		log:       log,
		exec:      func(f func()) { go f() },
	}
	for _, o := range opts {
		o(svc)
	}
	return svc
}

// HandleIncoming processes a successfully routed AMQP message.
//
// Flow for push-only notification types (PRICE_ALERT_TRIGGERED,
// ORDER_RECURRING_SKIPPED, and the order lifecycle events ORDER_CREATED /
// ORDER_DONE / ORDER_PARTIAL_FILL / ORDER_AUTO_CANCELLED):
//  1. Render subject + body templates (no email required).
//  2. Send an FCM push notification.
//
// Flow for all other notification types:
//  1. Render subject + body from template registry.
//  2. Persist a PENDING delivery record.
//  3. Launch AttemptDelivery asynchronously.
//
// Push-only types skip the email delivery channel entirely.
//
// A non-nil error causes the Consumer to NACK the message.
func (s *NotificationService) HandleIncoming(
	ctx context.Context,
	req *dto.NotificationRequest,
	notificationType model.NotificationType,
) error {
	if model.IsPushOnlyNotificationType(notificationType) {
		return s.handlePushOnlyNotification(ctx, req, notificationType)
	}

	resolved, err := s.renderer.Resolve(
		notificationType,
		req.UserEmail,
		req.EffectiveUsername(),
		req.TemplateVariables,
	)
	if err != nil {
		if errors.Is(err, template.ErrEmailRequired) {
			s.log.Error("email is required for non-push notification type — message will be NACKed",
				"notification_type", notificationType,
				"recipient", req.UserEmail,
				"error", err,
			)
		} else {
			s.log.Error("template resolution failed — message will be NACKed",
				"notification_type", notificationType,
				"recipient", req.UserEmail,
				"error", err,
			)
		}
		return fmt.Errorf("resolve template [%s]: %w", notificationType, err)
	}

	delivery := model.NewPendingDelivery(
		resolved.RecipientEmail,
		resolved.Subject,
		resolved.Body,
		notificationType,
		s.retryCfg.MaxRetries,
	)

	if err := s.store.Create(ctx, delivery); err != nil {
		return fmt.Errorf("persist pending delivery: %w", err)
	}

	s.log.Info("delivery record created",
		"delivery_id", delivery.DeliveryID,
		"notification_type", notificationType,
		"recipient", resolved.RecipientEmail,
	)

	deliveryID := delivery.DeliveryID
	s.exec(func() { s.AttemptDelivery(deliveryID) })

	return nil
}

func (s *NotificationService) handlePushOnlyNotification(
	ctx context.Context,
	req *dto.NotificationRequest,
	notificationType model.NotificationType,
) error {
	resolved, err := s.renderer.ResolveTemplates(
		notificationType,
		"",
		req.EffectiveUsername(),
		req.TemplateVariables,
	)
	if err != nil {
		s.log.Error("push-only template resolution failed — message will be NACKed",
			"notification_type", notificationType,
			"error", err,
		)
		return fmt.Errorf("resolve push template [%s]: %w", notificationType, err)
	}

	s.tryPushDelivery(ctx, req, notificationType, resolved.Subject, resolved.Body)
	return nil
}

func (s *NotificationService) tryPushDelivery(
	ctx context.Context,
	req *dto.NotificationRequest,
	notificationType model.NotificationType,
	subject string,
	body string,
) {
	if s.tokenStore == nil || s.pushSender == nil {
		s.log.Debug("push delivery skipped — FCM not configured",
			"notification_type", notificationType,
		)
		return
	}

	if req.ClientID == 0 {
		s.log.Warn("push delivery skipped — missing clientId",
			"notification_type", notificationType,
		)
		return
	}

	token, err := s.tokenStore.FindByClientId(ctx, req.ClientID)
	if err != nil {
		s.log.Warn("push delivery skipped — token lookup error",
			"notification_type", notificationType,
			"client_id", req.ClientID,
			"error", err,
		)
		return
	}
	if token == nil || token.Token == "" {
		s.log.Debug("push delivery skipped — no FCM token for client",
			"notification_type", notificationType,
			"client_id", req.ClientID,
		)
		return
	}

	switch notificationType {
	case model.NotificationTypePriceAlertTriggered:
		s.sendPriceAlertPush(ctx, req, token.Token, subject, body)
	case model.NotificationTypeOrderRecurringSkipped,
		model.NotificationTypeOrderCreated,
		model.NotificationTypeOrderDone,
		model.NotificationTypeOrderPartialFill,
		model.NotificationTypeOrderAutoCancelled:
		if err := s.pushSender.SendNotification(ctx, token.Token, subject, body); err != nil {
			s.log.Warn("push notification send failed",
				"notification_type", notificationType,
				"client_id", req.ClientID,
				"error", err,
			)
		} else {
			s.log.Info("push notification sent successfully",
				"notification_type", notificationType,
				"client_id", req.ClientID,
			)
		}
	default:
		s.log.Debug("no push action defined for notification type",
			"notification_type", notificationType,
		)
	}
}

func (s *NotificationService) sendPriceAlertPush(
	ctx context.Context,
	req *dto.NotificationRequest,
	deviceToken string,
	subject string,
	body string,
) {
	vars := req.TemplateVariables
	if len(vars) == 0 {
		vars = make(map[string]string)
	}

	data := map[string]string{
		"type":           "PRICE_ALERT_TRIGGERED",
		"title":          subject,
		"body":           body,
		"ticker":         vars["ticker"],
		"threshold":      vars["threshold"],
		"triggeredPrice": vars["price"],
		"condition":      vars["condition"],
	}

	if err := s.pushSender.SendData(ctx, deviceToken, data); err != nil {
		s.log.Warn("price alert push send failed",
			"client_id", req.ClientID,
			"ticker", vars["ticker"],
			"error", err,
		)
	} else {
		s.log.Info("price alert push sent successfully",
			"client_id", req.ClientID,
			"ticker", vars["ticker"],
		)
	}
}

// PersistUnsupportedAudit creates a terminal FAILED record for messages whose
// routing key could not be resolved to a NotificationType.
func (s *NotificationService) PersistUnsupportedAudit(
	ctx context.Context,
	req *dto.NotificationRequest,
	routingKey string,
) error {
	errMsg := "Unsupported routing key: " + routingKey
	recipient := req.UserEmail
	if recipient == "" {
		recipient = model.UnknownRecipient
	}

	audit := model.NewFailedAudit(
		recipient,
		model.NotificationType("UNKNOWN"),
		errMsg,
		s.retryCfg.MaxRetries,
	)

	if err := s.store.PersistFailedAudit(ctx, audit); err != nil {
		s.log.Error("failed to persist unsupported-routing-key audit",
			"routing_key", routingKey,
			"error", err,
		)
		return err
	}
	s.log.Info("unsupported routing key audited",
		"routing_key", routingKey,
		"delivery_id", audit.DeliveryID,
	)
	return nil
}

// AttemptDelivery executes one SMTP send attempt for the given delivery.
// Called by HandleIncoming (initial attempt) and by the retry scheduler.
//
// Flow:
//  1. Load the delivery; skip if not found or not eligible.
//  2. Mark as PROCESSING (optimistic lock — prevents concurrent duplicates).
//  3. Send email via SMTP.
//  4. On success: mark SUCCEEDED.
//  5. On failure: classify error; mark RETRY_SCHEDULED or FAILED.
func (s *NotificationService) AttemptDelivery(deliveryID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	delivery, err := s.store.FindByDeliveryID(ctx, deliveryID)
	if err != nil {
		s.log.Error("AttemptDelivery: load failed", "delivery_id", deliveryID, "error", err)
		return
	}
	if delivery == nil {
		s.log.Warn("AttemptDelivery: delivery not found", "delivery_id", deliveryID)
		return
	}

	now := time.Now().UTC()
	if !delivery.IsEligibleForAttempt(now) {
		s.log.Debug("AttemptDelivery: delivery not eligible — skipping",
			"delivery_id", deliveryID,
			"status", delivery.Status,
			"attempt_count", delivery.AttemptCount,
			"max_retries", delivery.MaxRetries,
		)
		return
	}

	if err := s.store.MarkProcessing(ctx, deliveryID); err != nil {
		var notElig *model.ErrDeliveryNotEligible
		if errors.As(err, &notElig) {
			s.log.Debug("AttemptDelivery: lost race to another worker", "delivery_id", deliveryID)
			return
		}
		s.log.Error("AttemptDelivery: MarkProcessing failed", "delivery_id", deliveryID, "error", err)
		return
	}

	s.log.Info("attempting email delivery",
		"delivery_id", deliveryID,
		"recipient", delivery.RecipientEmail,
		"attempt", delivery.AttemptCount+1,
		"max_retries", delivery.MaxRetries,
	)

	attemptedAt := time.Now().UTC()
	sendErr := s.sender.SendEmail(delivery.RecipientEmail, delivery.Subject, delivery.Body)

	if sendErr == nil {
		if err := s.store.MarkSucceeded(ctx, deliveryID, attemptedAt); err != nil {
			s.log.Error("AttemptDelivery: MarkSucceeded failed after send",
				"delivery_id", deliveryID, "error", err)
		} else {
			s.log.Info("email delivered successfully",
				"delivery_id", deliveryID,
				"recipient", delivery.RecipientEmail,
			)
		}
		return
	}

	retryable := smtp.IsRetryable(sendErr)
	errMsg := model.TrimError(sendErr.Error())

	s.log.Warn("SMTP delivery failed",
		"delivery_id", deliveryID,
		"recipient", delivery.RecipientEmail,
		"retryable", retryable,
		"error", sendErr,
	)

	nextAt, err := s.store.MarkFailedOrRetry(
		ctx, deliveryID, attemptedAt, errMsg, retryable, s.retryCfg.DelaySeconds,
	)
	if err != nil {
		s.log.Error("AttemptDelivery: MarkFailedOrRetry failed",
			"delivery_id", deliveryID, "error", err)
		return
	}

	if !nextAt.IsZero() {
		s.scheduler.Schedule(deliveryID, nextAt)
		s.log.Info("delivery scheduled for retry",
			"delivery_id", deliveryID,
			"next_attempt_at", nextAt,
		)
	} else {
		s.log.Error("delivery permanently failed",
			"delivery_id", deliveryID,
			"recipient", delivery.RecipientEmail,
		)
	}
}
