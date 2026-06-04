package service_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"Banka1Back/notification-service-go/internal/config"
	"Banka1Back/notification-service-go/internal/dto"
	"Banka1Back/notification-service-go/internal/model"
	"Banka1Back/notification-service-go/internal/service"
	"Banka1Back/notification-service-go/internal/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Push-specific stubs
// ---------------------------------------------------------------------------

type stubTokenStore struct {
	token *model.FcmToken
	err   error
}

func (s *stubTokenStore) FindByClientId(_ context.Context, _ int64) (*model.FcmToken, error) {
	return s.token, s.err
}

type stubPushSender struct {
	sendNotifErr error
	sendDataErr  error
	notifCalls   int
	dataCalls    int
	lastDataMap  map[string]string
}

func (s *stubPushSender) SendNotification(_ context.Context, _, _, _ string) error {
	s.notifCalls++
	return s.sendNotifErr
}

func (s *stubPushSender) SendData(_ context.Context, _ string, data map[string]string) error {
	s.dataCalls++
	s.lastDataMap = data
	return s.sendDataErr
}

func newPushService(
	store service.DeliveryStore,
	tokenStore service.FcmTokenStore,
	pushSender service.PushSender,
) *service.NotificationService {
	registry := template.NewDefaultTemplateRegistry(nil)
	renderer := template.NewRenderer(registry)
	retryCfg := config.RetryConfig{MaxRetries: 4, DelaySeconds: 5}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))

	return service.NewNotificationServiceWithSender(
		store,
		renderer,
		&stubSender{},
		&stubScheduler{},
		retryCfg,
		log,
		service.WithExec(syncExec),
		service.WithPush(tokenStore, pushSender),
	)
}

// ---------------------------------------------------------------------------
// Push-only notification type: PRICE_ALERT_TRIGGERED
// ---------------------------------------------------------------------------

func TestHandleIncoming_PriceAlertTriggered_CallsSendData(t *testing.T) {
	t.Parallel()

	push := &stubPushSender{}
	token := &model.FcmToken{Token: "device-token-123"}
	tokenStore := &stubTokenStore{token: token}
	store := &stubStore{}

	svc := newPushService(store, tokenStore, push)

	req := &dto.NotificationRequest{
		ClientID: 42,
		TemplateVariables: map[string]string{
			"ticker":    "AAPL",
			"price":     "150.00",
			"threshold": "145.00",
			"condition": "ABOVE",
		},
	}

	err := svc.HandleIncoming(context.Background(), req, model.NotificationTypePriceAlertTriggered)
	require.NoError(t, err)
	assert.Equal(t, 0, push.notifCalls, "SendNotification must not be called for price alerts")
	assert.Equal(t, 1, push.dataCalls, "SendData must be called for price alerts")
	assert.Empty(t, store.created, "no email delivery record for push-only type")

	assert.Equal(t, "PRICE_ALERT_TRIGGERED", push.lastDataMap["type"])
	assert.Equal(t, "AAPL", push.lastDataMap["ticker"])
	assert.Equal(t, "150.00", push.lastDataMap["triggeredPrice"])
}

func TestHandleIncoming_PriceAlertTriggered_SendDataFailure_NoError(t *testing.T) {
	t.Parallel()

	push := &stubPushSender{sendDataErr: assert.AnError}
	token := &model.FcmToken{Token: "device-token-123"}
	svc := newPushService(&stubStore{}, &stubTokenStore{token: token}, push)

	req := &dto.NotificationRequest{
		ClientID:          42,
		TemplateVariables: map[string]string{"ticker": "GOOG", "price": "100"},
	}

	err := svc.HandleIncoming(context.Background(), req, model.NotificationTypePriceAlertTriggered)
	require.NoError(t, err, "push send failure must not be propagated as error")
	assert.Equal(t, 1, push.dataCalls)
}

// ---------------------------------------------------------------------------
// Push-only notification type: ORDER_RECURRING_SKIPPED
// ---------------------------------------------------------------------------

func TestHandleIncoming_OrderRecurringSkipped_CallsSendNotification(t *testing.T) {
	t.Parallel()

	push := &stubPushSender{}
	token := &model.FcmToken{Token: "tok-abc"}
	svc := newPushService(&stubStore{}, &stubTokenStore{token: token}, push)

	req := &dto.NotificationRequest{
		ClientID:          7,
		TemplateVariables: map[string]string{"orderId": "ORD-1", "reason": "insufficient funds"},
	}

	err := svc.HandleIncoming(context.Background(), req, model.NotificationTypeOrderRecurringSkipped)
	require.NoError(t, err)
	assert.Equal(t, 1, push.notifCalls)
	assert.Equal(t, 0, push.dataCalls)
}

func TestHandleIncoming_OrderRecurringSkipped_SendFailure_NoError(t *testing.T) {
	t.Parallel()

	push := &stubPushSender{sendNotifErr: assert.AnError}
	token := &model.FcmToken{Token: "tok-abc"}
	svc := newPushService(&stubStore{}, &stubTokenStore{token: token}, push)

	req := &dto.NotificationRequest{ClientID: 7}

	err := svc.HandleIncoming(context.Background(), req, model.NotificationTypeOrderRecurringSkipped)
	require.NoError(t, err, "push send failure must not be propagated")
}

// ---------------------------------------------------------------------------
// Push skipped when FCM not configured
// ---------------------------------------------------------------------------

func TestHandleIncoming_PushOnly_NoPushConfigured_SkipsGracefully(t *testing.T) {
	t.Parallel()

	registry := template.NewDefaultTemplateRegistry(nil)
	renderer := template.NewRenderer(registry)
	retryCfg := config.RetryConfig{MaxRetries: 4, DelaySeconds: 5}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))

	svc := service.NewNotificationServiceWithSender(
		&stubStore{},
		renderer,
		&stubSender{},
		&stubScheduler{},
		retryCfg,
		log,
		service.WithExec(syncExec),
		// no WithPush → FCM not configured
	)

	req := &dto.NotificationRequest{ClientID: 1}
	err := svc.HandleIncoming(context.Background(), req, model.NotificationTypePriceAlertTriggered)
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Push skipped when clientId is 0
// ---------------------------------------------------------------------------

func TestHandleIncoming_PushOnly_MissingClientID_SkipsGracefully(t *testing.T) {
	t.Parallel()

	push := &stubPushSender{}
	svc := newPushService(&stubStore{}, &stubTokenStore{token: &model.FcmToken{Token: "tok"}}, push)

	req := &dto.NotificationRequest{ClientID: 0}
	err := svc.HandleIncoming(context.Background(), req, model.NotificationTypePriceAlertTriggered)
	require.NoError(t, err)
	assert.Equal(t, 0, push.dataCalls, "no push when clientId is 0")
}

// ---------------------------------------------------------------------------
// Push skipped when token lookup returns nil
// ---------------------------------------------------------------------------

func TestHandleIncoming_PushOnly_NoTokenForClient_SkipsGracefully(t *testing.T) {
	t.Parallel()

	push := &stubPushSender{}
	svc := newPushService(&stubStore{}, &stubTokenStore{token: nil}, push)

	req := &dto.NotificationRequest{ClientID: 5}
	err := svc.HandleIncoming(context.Background(), req, model.NotificationTypePriceAlertTriggered)
	require.NoError(t, err)
	assert.Equal(t, 0, push.dataCalls)
}

// ---------------------------------------------------------------------------
// Push skipped when token lookup returns empty token string
// ---------------------------------------------------------------------------

func TestHandleIncoming_PushOnly_EmptyToken_SkipsGracefully(t *testing.T) {
	t.Parallel()

	push := &stubPushSender{}
	svc := newPushService(&stubStore{}, &stubTokenStore{token: &model.FcmToken{Token: ""}}, push)

	req := &dto.NotificationRequest{ClientID: 5}
	err := svc.HandleIncoming(context.Background(), req, model.NotificationTypePriceAlertTriggered)
	require.NoError(t, err)
	assert.Equal(t, 0, push.dataCalls)
}

// ---------------------------------------------------------------------------
// Push skipped when token store returns an error
// ---------------------------------------------------------------------------

func TestHandleIncoming_PushOnly_TokenStoreError_SkipsGracefully(t *testing.T) {
	t.Parallel()

	push := &stubPushSender{}
	svc := newPushService(&stubStore{}, &stubTokenStore{err: assert.AnError}, push)

	req := &dto.NotificationRequest{ClientID: 5}
	err := svc.HandleIncoming(context.Background(), req, model.NotificationTypePriceAlertTriggered)
	require.NoError(t, err, "token store error must not be propagated")
	assert.Equal(t, 0, push.dataCalls)
}
