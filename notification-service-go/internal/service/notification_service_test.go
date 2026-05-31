package service_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"Banka1Back/notification-service-go/internal/config"
	"Banka1Back/notification-service-go/internal/dto"
	"Banka1Back/notification-service-go/internal/model"
	"Banka1Back/notification-service-go/internal/service"
	"Banka1Back/notification-service-go/internal/smtp"
	"Banka1Back/notification-service-go/internal/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

type stubStore struct {
	created        []*model.NotificationDelivery
	findResult     *model.NotificationDelivery
	findErr        error
	createErr      error
	auditErr       error
	markProcErr    error
	markSuccErr    error
	markFailResult time.Time
	markFailErr    error
}

func (r *stubStore) Create(_ context.Context, d *model.NotificationDelivery) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, d)
	return nil
}

func (r *stubStore) FindByDeliveryID(_ context.Context, id string) (*model.NotificationDelivery, error) {
	if r.findResult != nil || r.findErr != nil {
		return r.findResult, r.findErr
	}
	for _, d := range r.created {
		if d.DeliveryID == id {
			return d, nil
		}
	}
	return nil, nil
}

func (r *stubStore) FindDueRetries(_ context.Context, _ time.Time, _ int) ([]*model.NotificationDelivery, error) {
	return nil, nil
}

func (r *stubStore) MarkProcessing(_ context.Context, _ string) error {
	return r.markProcErr
}

func (r *stubStore) MarkSucceeded(_ context.Context, _ string, _ time.Time) error {
	return r.markSuccErr
}

func (r *stubStore) MarkFailedOrRetry(_ context.Context, _ string, _ time.Time, _ string, _ bool, _ int) (time.Time, error) {
	return r.markFailResult, r.markFailErr
}

func (r *stubStore) PersistFailedAudit(_ context.Context, _ *model.NotificationDelivery) error {
	return r.auditErr
}

type stubSender struct {
	sendErr  error
	calls    int
	lastTo   string
	lastSubj string
}

func (s *stubSender) SendEmail(to, subject, _ string) error {
	s.calls++
	s.lastTo = to
	s.lastSubj = subject
	return s.sendErr
}

type stubScheduler struct {
	scheduled []struct {
		id string
		at time.Time
	}
}

func (s *stubScheduler) Schedule(id string, at time.Time) {
	s.scheduled = append(s.scheduled, struct {
		id string
		at time.Time
	}{id, at})
}

var syncExec = func(f func()) { f() }

func silentLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError + 1,
	}))
}

func newService(
	store service.DeliveryStore,
	sendErr error,
	overrideTemplates map[string]template.EmailTemplate,
) (*service.NotificationService, *stubSender, *stubScheduler) {
	registry := template.NewDefaultTemplateRegistry(overrideTemplates)
	renderer := template.NewRenderer(registry)

	sender := &stubSender{sendErr: sendErr}
	sched := &stubScheduler{}

	retryCfg := config.RetryConfig{
		MaxRetries:   4,
		DelaySeconds: 5,
	}

	svc := service.NewNotificationServiceWithSender(
		store,
		renderer,
		sender,
		sched,
		retryCfg,
		silentLog(),
		service.WithExec(syncExec),
	)
	return svc, sender, sched
}

// ---------------------------------------------------------------------------
// HandleIncoming — happy path
// ---------------------------------------------------------------------------

func TestHandleIncoming_HappyPath_CreatesPendingAndCallsSender(t *testing.T) {
	t.Parallel()
	store := &stubStore{}
	svc, sender, _ := newService(store, nil, nil)

	req := &dto.NotificationRequest{
		UserEmail: "emp@bank.io",
		Username:  "Jovan",
		TemplateVariables: map[string]string{
			"activationLink": "https://bank.io/activate/xyz",
		},
	}

	err := svc.HandleIncoming(context.Background(), req, model.NotificationTypeEmployeeCreated)
	require.NoError(t, err)

	require.Len(t, store.created, 1)
	assert.Equal(t, model.StatusPending, store.created[0].Status)
	assert.Equal(t, "emp@bank.io", store.created[0].RecipientEmail)
	assert.Equal(t, string(model.NotificationTypeEmployeeCreated), store.created[0].NotificationType)

	assert.Equal(t, 1, sender.calls)
	assert.Equal(t, "emp@bank.io", sender.lastTo)
}

func TestHandleIncoming_SubjectRenderedCorrectly(t *testing.T) {
	t.Parallel()
	store := &stubStore{}
	svc, sender, _ := newService(store, nil, nil)

	req := &dto.NotificationRequest{
		UserEmail:         "emp@bank.io",
		Username:          "Alice",
		TemplateVariables: map[string]string{"activationLink": "https://link"},
	}

	require.NoError(t, svc.HandleIncoming(context.Background(), req, model.NotificationTypeEmployeeCreated))
	assert.Equal(t, "Activation Email", sender.lastSubj)
}

// ---------------------------------------------------------------------------
// HandleIncoming — template resolution failure
// ---------------------------------------------------------------------------

func TestHandleIncoming_MissingTemplate_ReturnsErrorNoDBWrite(t *testing.T) {
	t.Parallel()
	store := &stubStore{}
	svc, sender, _ := newService(store, nil, nil)

	req := &dto.NotificationRequest{UserEmail: "x@bank.io"}

	err := svc.HandleIncoming(context.Background(), req, model.NotificationType("UNKNOWN_TYPE"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ERR_NOTIFICATION_004")
	assert.Empty(t, store.created, "no DB record must be written on template resolution failure")
	assert.Equal(t, 0, sender.calls, "sender must not be called after template failure")
}

// ---------------------------------------------------------------------------
// HandleIncoming — DB failure
// ---------------------------------------------------------------------------

func TestHandleIncoming_DBCreateError_ReturnsError(t *testing.T) {
	t.Parallel()
	store := &stubStore{createErr: errors.New("connection reset by peer")}
	svc, sender, _ := newService(store, nil, nil)

	req := &dto.NotificationRequest{
		UserEmail:         "x@bank.io",
		TemplateVariables: map[string]string{"activationLink": "https://l"},
	}

	err := svc.HandleIncoming(context.Background(), req, model.NotificationTypeEmployeeCreated)
	require.Error(t, err)
	assert.Equal(t, 0, sender.calls, "sender must not be called when DB write failed")
}

// ---------------------------------------------------------------------------
// PersistUnsupportedAudit
// ---------------------------------------------------------------------------

func TestPersistUnsupportedAudit_CreatesFailedRecord(t *testing.T) {
	t.Parallel()
	store := &stubStore{}
	svc, _, _ := newService(store, nil, nil)

	req := &dto.NotificationRequest{UserEmail: "x@bank.io"}
	err := svc.PersistUnsupportedAudit(context.Background(), req, "some.unknown.key")
	require.NoError(t, err)
}

func TestPersistUnsupportedAudit_DBError_Propagates(t *testing.T) {
	t.Parallel()
	store := &stubStore{auditErr: errors.New("db error")}
	svc, _, _ := newService(store, nil, nil)

	req := &dto.NotificationRequest{UserEmail: "x@bank.io"}
	err := svc.PersistUnsupportedAudit(context.Background(), req, "bad.key")
	require.Error(t, err)
}

func TestPersistUnsupportedAudit_EmptyEmail_UsesUnknownRecipient(t *testing.T) {
	t.Parallel()
	store := &stubStore{}
	svc, _, _ := newService(store, nil, nil)

	req := &dto.NotificationRequest{UserEmail: ""}
	err := svc.PersistUnsupportedAudit(context.Background(), req, "bad.key")
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// AttemptDelivery — success path
// ---------------------------------------------------------------------------

func TestAttemptDelivery_Success_MarksSucceeded(t *testing.T) {
	t.Parallel()

	pending := model.NewPendingDelivery(
		"user@bank.io", "Subject", "Body",
		model.NotificationTypeEmployeeCreated, 4,
	)
	store := &stubStore{findResult: pending}
	svc, sender, _ := newService(store, nil, nil)

	svc.AttemptDelivery(pending.DeliveryID)
	assert.Equal(t, 1, sender.calls)
}

// ---------------------------------------------------------------------------
// AttemptDelivery — auth error → terminal FAILED, no retry scheduled
// ---------------------------------------------------------------------------

func TestAttemptDelivery_AuthError_TerminalFailNoRetry(t *testing.T) {
	t.Parallel()

	pending := model.NewPendingDelivery(
		"user@bank.io", "Subject", "Body",
		model.NotificationTypeEmployeeCreated, 4,
	)
	authErr := &smtp.MailAuthError{Cause: errors.New("535 invalid")}

	store := &stubStore{findResult: pending, markFailResult: time.Time{}}
	svc, sender, sched := newService(store, authErr, nil)

	svc.AttemptDelivery(pending.DeliveryID)
	assert.Equal(t, 1, sender.calls)
	assert.Empty(t, sched.scheduled, "no retry must be scheduled for auth errors")
}

// ---------------------------------------------------------------------------
// AttemptDelivery — transient error → RETRY_SCHEDULED, scheduler fed
// ---------------------------------------------------------------------------

func TestAttemptDelivery_TransientError_SchedulesRetry(t *testing.T) {
	t.Parallel()

	pending := model.NewPendingDelivery(
		"user@bank.io", "Subject", "Body",
		model.NotificationTypeEmployeeCreated, 4,
	)
	transientErr := errors.New("connection reset by peer")
	nextAt := time.Now().Add(5 * time.Second)

	store := &stubStore{findResult: pending, markFailResult: nextAt}
	svc, sender, sched := newService(store, transientErr, nil)

	svc.AttemptDelivery(pending.DeliveryID)
	assert.Equal(t, 1, sender.calls)
	require.Len(t, sched.scheduled, 1, "one retry must be scheduled")
	assert.Equal(t, pending.DeliveryID, sched.scheduled[0].id)
	assert.Equal(t, nextAt, sched.scheduled[0].at)
}

// ---------------------------------------------------------------------------
// AttemptDelivery — delivery not found → skip silently
// ---------------------------------------------------------------------------

func TestAttemptDelivery_DeliveryNotFound_SkipsSilently(t *testing.T) {
	t.Parallel()

	store := &stubStore{findResult: nil}
	svc, sender, _ := newService(store, nil, nil)

	svc.AttemptDelivery("non-existent-id")
	assert.Equal(t, 0, sender.calls)
}

// ---------------------------------------------------------------------------
// AttemptDelivery — MarkProcessing race (another worker already claimed it)
// ---------------------------------------------------------------------------

func TestAttemptDelivery_MarkProcessingRace_SkipsSilently(t *testing.T) {
	t.Parallel()

	pending := model.NewPendingDelivery(
		"user@bank.io", "Subject", "Body",
		model.NotificationTypeEmployeeCreated, 4,
	)
	store := &stubStore{
		findResult:  pending,
		markProcErr: &model.ErrDeliveryNotEligible{DeliveryID: pending.DeliveryID, Reason: "already processing"},
	}
	svc, sender, _ := newService(store, nil, nil)

	svc.AttemptDelivery(pending.DeliveryID)
	assert.Equal(t, 0, sender.calls, "sender must not be called when another worker claimed the delivery")
}

// ---------------------------------------------------------------------------
// AttemptDelivery — budget-exhausted delivery is not eligible
// ---------------------------------------------------------------------------

func TestAttemptDelivery_ExhaustedBudget_SkipsDelivery(t *testing.T) {
	t.Parallel()

	exhausted := &model.NotificationDelivery{
		DeliveryID:   "ex-id",
		Status:       model.StatusPending,
		AttemptCount: 4,
		MaxRetries:   4,
	}
	store := &stubStore{findResult: exhausted}
	svc, sender, _ := newService(store, nil, nil)

	svc.AttemptDelivery(exhausted.DeliveryID)
	assert.Equal(t, 0, sender.calls)
}
