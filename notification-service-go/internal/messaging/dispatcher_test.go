package messaging_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"testing"

	"Banka1Back/notification-service-go/internal/dto"
	"Banka1Back/notification-service-go/internal/messaging"
	"Banka1Back/notification-service-go/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

type stubDeliveryHandler struct {
	lastReq              *dto.NotificationRequest
	lastNotificationType model.NotificationType
	lastRoutingKey       string
	handleIncomingCalls  int
	persistAuditCalls    int
	handleIncomingErr    error
	persistAuditErr      error
}

func (s *stubDeliveryHandler) HandleIncoming(
	_ context.Context,
	req *dto.NotificationRequest,
	nt model.NotificationType,
) error {
	s.handleIncomingCalls++
	s.lastReq = req
	s.lastNotificationType = nt
	return s.handleIncomingErr
}

func (s *stubDeliveryHandler) PersistUnsupportedAudit(
	_ context.Context,
	req *dto.NotificationRequest,
	routingKey string,
) error {
	s.persistAuditCalls++
	s.lastReq = req
	s.lastRoutingKey = routingKey
	return s.persistAuditErr
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

func newDispatcher(h *stubDeliveryHandler) *messaging.Dispatcher {
	return messaging.NewDispatcher(h, silentLogger())
}

// ---------------------------------------------------------------------------
// NotificationRequest.UnmarshalJSON — alias handling
// ---------------------------------------------------------------------------

func TestNotificationRequest_UnmarshalJSON_PrimaryFieldNames(t *testing.T) {
	t.Parallel()
	raw := `{
		"username": "Alice",
		"userEmail": "alice@example.com",
		"templateVariables": {"activationLink": "https://bank.io/activate"},
		"clientId": 42,
		"operationType": "PAYMENT",
		"sessionId": "sess-001"
	}`

	var req dto.NotificationRequest
	require.NoError(t, json.Unmarshal([]byte(raw), &req))

	assert.Equal(t, "Alice", req.Username)
	assert.Equal(t, "alice@example.com", req.UserEmail)
	assert.Equal(t, "https://bank.io/activate", req.TemplateVariables["activationLink"])
	assert.Equal(t, int64(42), req.ClientID)
	assert.Equal(t, "PAYMENT", req.OperationType)
	assert.Equal(t, "sess-001", req.SessionID)
}

func TestNotificationRequest_UnmarshalJSON_EmailAlias_email(t *testing.T) {
	t.Parallel()
	raw := `{"email": "bob@example.com", "username": "Bob"}`

	var req dto.NotificationRequest
	require.NoError(t, json.Unmarshal([]byte(raw), &req))
	assert.Equal(t, "bob@example.com", req.UserEmail)
}

func TestNotificationRequest_UnmarshalJSON_EmailAlias_recipientEmail(t *testing.T) {
	t.Parallel()
	raw := `{"recipientEmail": "carol@example.com"}`

	var req dto.NotificationRequest
	require.NoError(t, json.Unmarshal([]byte(raw), &req))
	assert.Equal(t, "carol@example.com", req.UserEmail)
}

func TestNotificationRequest_UnmarshalJSON_PrimaryEmailTakesPrecedence(t *testing.T) {
	t.Parallel()
	raw := `{"userEmail": "primary@example.com", "email": "alias@example.com"}`

	var req dto.NotificationRequest
	require.NoError(t, json.Unmarshal([]byte(raw), &req))
	assert.Equal(t, "primary@example.com", req.UserEmail)
}

func TestNotificationRequest_UnmarshalJSON_TemplateVarAlias_params(t *testing.T) {
	t.Parallel()
	raw := `{"userEmail": "x@x.com", "params": {"code": "123456"}}`

	var req dto.NotificationRequest
	require.NoError(t, json.Unmarshal([]byte(raw), &req))
	assert.Equal(t, "123456", req.TemplateVariables["code"])
}

func TestNotificationRequest_UnmarshalJSON_TemplateVarAlias_data(t *testing.T) {
	t.Parallel()
	raw := `{"userEmail": "x@x.com", "data": {"cardNumber": "4111111111111111"}}`

	var req dto.NotificationRequest
	require.NoError(t, json.Unmarshal([]byte(raw), &req))
	assert.Equal(t, "4111111111111111", req.TemplateVariables["cardNumber"])
}

func TestNotificationRequest_UnmarshalJSON_NilTemplateVariables_InitialisedToEmpty(t *testing.T) {
	t.Parallel()
	raw := `{"userEmail": "x@x.com"}`

	var req dto.NotificationRequest
	require.NoError(t, json.Unmarshal([]byte(raw), &req))
	assert.NotNil(t, req.TemplateVariables)
	assert.Empty(t, req.TemplateVariables)
}

func TestNotificationRequest_UnmarshalJSON_MalformedJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	raw := `{"userEmail": 12345}`

	var req dto.NotificationRequest
	err := json.Unmarshal([]byte(raw), &req)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// NotificationRequest.Validate
// ---------------------------------------------------------------------------

func TestValidate_MissingUserEmail_ReturnsError(t *testing.T) {
	t.Parallel()
	req := &dto.NotificationRequest{Username: "Alice"}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "userEmail is required")
}

func TestValidate_BlankUserEmail_ReturnsError(t *testing.T) {
	t.Parallel()
	req := &dto.NotificationRequest{UserEmail: "   "}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "userEmail is required")
}

func TestValidate_ValidPayload_ReturnsNil(t *testing.T) {
	t.Parallel()
	req := &dto.NotificationRequest{UserEmail: "alice@example.com"}
	assert.NoError(t, req.Validate())
}

// ---------------------------------------------------------------------------
// Dispatcher.Handle — routing key resolution
// ---------------------------------------------------------------------------

func TestDispatcher_KnownRoutingKey_CallsHandleIncoming(t *testing.T) {
	t.Parallel()
	h := &stubDeliveryHandler{}
	d := newDispatcher(h)

	req := &dto.NotificationRequest{
		UserEmail:         "emp@bank.io",
		Username:          "Jovan",
		TemplateVariables: map[string]string{"activationLink": "https://bank.io/activate/token"},
	}

	err := d.Handle(context.Background(), req, "employee.created")
	require.NoError(t, err)
	assert.Equal(t, 1, h.handleIncomingCalls)
	assert.Equal(t, 0, h.persistAuditCalls)
	assert.Equal(t, model.NotificationTypeEmployeeCreated, h.lastNotificationType)
}

func TestDispatcher_RoutingKeyMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		routingKey string
		wantType   model.NotificationType
	}{
		{"employee.created", model.NotificationTypeEmployeeCreated},
		{"employee.password_reset", model.NotificationTypeEmployeePasswordReset},
		{"employee.account_deactivated", model.NotificationTypeEmployeeAccountDeact},
		{"client.created", model.NotificationTypeClientCreated},
		{"client.password_reset", model.NotificationTypeClientPasswordReset},
		{"client.account_deactivated", model.NotificationTypeClientAccountDeact},
		{"card.blocked", model.NotificationTypeCardBlocked},
		{"card.unblocked", model.NotificationTypeCardUnblocked},
		{"card.deactivated", model.NotificationTypeCardDeactivated},
		{"card.request_verification", model.NotificationTypeCardRequestVerification},
		{"card.request_success", model.NotificationTypeCardRequestSuccess},
		{"card.request_failure", model.NotificationTypeCardRequestFailure},
		{"credit.approved", model.NotificationTypeCreditApproved},
		{"credit.declined", model.NotificationTypeCreditDeclined},
		{"credit.installment_failed", model.NotificationTypeCreditInstallmentFailed},
		{"order.approved", model.NotificationTypeOrderApproved},
		{"order.declined", model.NotificationTypeOrderDeclined},
		{"tax.collected", model.NotificationTypeTaxCollected},
		{"otc.countered", model.NotificationTypeOTCCounterOffered},
		{"otc.accepted", model.NotificationTypeOTCAccepted},
		{"otc.canceled", model.NotificationTypeOTCCanceled},
		{"otc.expiry_reminder", model.NotificationTypeOTCExpiryReminder},
		{"verification.otp", model.NotificationTypeVerificationOTP},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.routingKey, func(t *testing.T) {
			t.Parallel()
			h := &stubDeliveryHandler{}
			disp := newDispatcher(h)
			req := &dto.NotificationRequest{UserEmail: "user@bank.io"}

			err := disp.Handle(context.Background(), req, tc.routingKey)
			require.NoError(t, err)
			assert.Equal(t, tc.wantType, h.lastNotificationType)
		})
	}
}

func TestDispatcher_UnknownRoutingKey_PersistsAuditAndReturnsNil(t *testing.T) {
	t.Parallel()
	h := &stubDeliveryHandler{}
	d := newDispatcher(h)

	req := &dto.NotificationRequest{UserEmail: "x@x.com"}
	err := d.Handle(context.Background(), req, "unknown.event.xyz")

	require.NoError(t, err)
	assert.Equal(t, 0, h.handleIncomingCalls)
	assert.Equal(t, 1, h.persistAuditCalls)
	assert.Equal(t, "unknown.event.xyz", h.lastRoutingKey)
}

func TestDispatcher_UnknownRoutingKey_AuditDBError_PropagatesError(t *testing.T) {
	t.Parallel()
	h := &stubDeliveryHandler{persistAuditErr: errors.New("db connection lost")}
	d := newDispatcher(h)

	req := &dto.NotificationRequest{UserEmail: "x@x.com"}
	err := d.Handle(context.Background(), req, "future.event")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "persist unsupported audit")
}

func TestDispatcher_MissingUserEmail_ReturnsValidationError(t *testing.T) {
	t.Parallel()
	h := &stubDeliveryHandler{}
	d := newDispatcher(h)

	req := &dto.NotificationRequest{Username: "Alice"}
	err := d.Handle(context.Background(), req, "employee.created")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "payload validation failed")
	assert.Equal(t, 0, h.handleIncomingCalls)
}

func TestDispatcher_HandleIncomingError_PropagatesError(t *testing.T) {
	t.Parallel()
	h := &stubDeliveryHandler{handleIncomingErr: errors.New("database unavailable")}
	d := newDispatcher(h)

	req := &dto.NotificationRequest{UserEmail: "emp@bank.io"}
	err := d.Handle(context.Background(), req, "employee.created")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "handle incoming")
}

func TestEffectiveUsername_TrimmedAndReturned(t *testing.T) {
	t.Parallel()
	req := &dto.NotificationRequest{Username: "  Jovan  "}
	assert.Equal(t, "Jovan", req.EffectiveUsername())
}

func TestEffectiveUsername_EmptyWhenAbsent(t *testing.T) {
	t.Parallel()
	req := &dto.NotificationRequest{}
	assert.Equal(t, "", req.EffectiveUsername())
}
