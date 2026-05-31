package template_test

import (
	"strings"
	"testing"

	"Banka1Back/notification-service-go/internal/model"
	"Banka1Back/notification-service-go/internal/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRenderer(overrides map[string]template.EmailTemplate) *template.Renderer {
	return template.NewRenderer(template.NewDefaultTemplateRegistry(overrides))
}

func TestRegistry_Resolve_KnownType_ReturnsTemplate(t *testing.T) {
	t.Parallel()
	reg := template.NewDefaultTemplateRegistry(nil)

	got, err := reg.Resolve(model.NotificationTypeEmployeeCreated)
	require.NoError(t, err)
	assert.Equal(t, "Activation Email", got.Subject)
	assert.Contains(t, got.BodyTemplate, "{{name}}")
	assert.Contains(t, got.BodyTemplate, "{{activationLink}}")
}

func TestRegistry_Resolve_UnknownType_ReturnsError(t *testing.T) {
	t.Parallel()
	reg := template.NewDefaultTemplateRegistry(nil)

	_, err := reg.Resolve(model.NotificationType("DOES_NOT_EXIST"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ERR_NOTIFICATION_004")
}

func TestRegistry_Resolve_OverrideReplacesDefault(t *testing.T) {
	t.Parallel()
	overrides := map[string]template.EmailTemplate{
		"EMPLOYEE_CREATED": {Subject: "Custom Subject", BodyTemplate: "Custom body"},
	}
	reg := template.NewDefaultTemplateRegistry(overrides)

	got, err := reg.Resolve(model.NotificationTypeEmployeeCreated)
	require.NoError(t, err)
	assert.Equal(t, "Custom Subject", got.Subject)
}

func TestRegistry_AllDefaultNotificationTypesHaveTemplates(t *testing.T) {
	t.Parallel()
	reg := template.NewDefaultTemplateRegistry(nil)

	allTypes := []model.NotificationType{
		model.NotificationTypeEmployeeCreated,
		model.NotificationTypeEmployeePasswordReset,
		model.NotificationTypeEmployeeAccountDeact,
		model.NotificationTypeClientCreated,
		model.NotificationTypeClientPasswordReset,
		model.NotificationTypeClientAccountDeact,
		model.NotificationTypeVerificationOTP,
		model.NotificationTypeCardRequestVerification,
		model.NotificationTypeCardRequestSuccess,
		model.NotificationTypeCardRequestFailure,
		model.NotificationTypeCardBlocked,
		model.NotificationTypeCardUnblocked,
		model.NotificationTypeCardDeactivated,
		model.NotificationTypeCreditApproved,
		model.NotificationTypeCreditDeclined,
		model.NotificationTypeCreditInstallmentFailed,
		model.NotificationTypeOrderApproved,
		model.NotificationTypeOrderDeclined,
		model.NotificationTypeTaxCollected,
		model.NotificationTypeOTCCounterOffered,
		model.NotificationTypeOTCAccepted,
		model.NotificationTypeOTCCanceled,
		model.NotificationTypeOTCExpiryReminder,
	}

	for _, nt := range allTypes {
		nt := nt
		t.Run(string(nt), func(t *testing.T) {
			t.Parallel()
			_, err := reg.Resolve(nt)
			assert.NoError(t, err, "default template must exist for %q", nt)
		})
	}
}

func TestRenderer_Resolve_SubstitutesPlaceholders(t *testing.T) {
	t.Parallel()
	r := newRenderer(map[string]template.EmailTemplate{
		"EMPLOYEE_CREATED": {
			Subject:      "Dobrodosli, {{name}}",
			BodyTemplate: "Zdravo {{name}}, link: {{activationLink}}",
		},
	})

	got, err := r.Resolve(
		model.NotificationTypeEmployeeCreated,
		"alice@bank.io",
		"Alice",
		map[string]string{"activationLink": "https://bank.io/activate/abc"},
	)
	require.NoError(t, err)
	assert.Equal(t, "alice@bank.io", got.RecipientEmail)
	assert.Equal(t, "Dobrodosli, Alice", got.Subject)
	assert.Equal(t, "Zdravo Alice, link: https://bank.io/activate/abc", got.Body)
}

func TestRenderer_Resolve_UsernameInjectsNameAndUsernameAliases(t *testing.T) {
	t.Parallel()
	r := newRenderer(map[string]template.EmailTemplate{
		"EMPLOYEE_CREATED": {
			Subject:      "Hi {{name}}",
			BodyTemplate: "Username: {{username}}, Name: {{name}}",
		},
	})

	got, err := r.Resolve(
		model.NotificationTypeEmployeeCreated,
		"bob@bank.io",
		"Bob",
		map[string]string{},
	)
	require.NoError(t, err)
	assert.Equal(t, "Hi Bob", got.Subject)
	assert.Equal(t, "Username: Bob, Name: Bob", got.Body)
}

func TestRenderer_Resolve_ExplicitTemplateVarOverridesUsernameAlias(t *testing.T) {
	t.Parallel()
	r := newRenderer(map[string]template.EmailTemplate{
		"EMPLOYEE_CREATED": {Subject: "subj", BodyTemplate: "Name: {{name}}"},
	})

	got, err := r.Resolve(
		model.NotificationTypeEmployeeCreated,
		"carol@bank.io",
		"Carol",
		map[string]string{"name": "ExplicitName"},
	)
	require.NoError(t, err)
	assert.Equal(t, "Name: ExplicitName", got.Body)
}

func TestRenderer_Resolve_UnknownPlaceholdersLeftIntact(t *testing.T) {
	t.Parallel()
	r := newRenderer(map[string]template.EmailTemplate{
		"EMPLOYEE_CREATED": {Subject: "subj", BodyTemplate: "Vrednost: {{missingKey}}"},
	})

	got, err := r.Resolve(
		model.NotificationTypeEmployeeCreated,
		"x@bank.io",
		"",
		map[string]string{},
	)
	require.NoError(t, err)
	assert.Contains(t, got.Body, "{{missingKey}}")
}

func TestRenderer_Resolve_MissingTemplate_ReturnsError(t *testing.T) {
	t.Parallel()
	r := newRenderer(nil)

	_, err := r.Resolve(model.NotificationType("NONEXISTENT_TYPE"), "x@bank.io", "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ERR_NOTIFICATION_004")
}

func TestRenderer_Resolve_EmptyRecipientEmail_ReturnsError(t *testing.T) {
	t.Parallel()
	r := newRenderer(nil)

	_, err := r.Resolve(model.NotificationTypeEmployeeCreated, "", "Bob", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ERR_NOTIFICATION_003")
}

func TestRenderer_EscapesHTMLInVariableValues(t *testing.T) {
	t.Parallel()
	r := newRenderer(map[string]template.EmailTemplate{
		"EMPLOYEE_CREATED": {Subject: "subj", BodyTemplate: "Link: {{activationLink}}"},
	})

	got, err := r.Resolve(
		model.NotificationTypeEmployeeCreated,
		"x@bank.io",
		"",
		map[string]string{"activationLink": `<script>alert("xss")</script>`},
	)
	require.NoError(t, err)
	assert.NotContains(t, got.Body, "<script>")
	assert.Contains(t, got.Body, "&lt;script&gt;")
	assert.Contains(t, got.Body, "&quot;xss&quot;")
}

func TestRenderer_EscapesAmpersand(t *testing.T) {
	t.Parallel()
	r := newRenderer(map[string]template.EmailTemplate{
		"EMPLOYEE_CREATED": {Subject: "subj", BodyTemplate: "Param: {{param}}"},
	})

	got, err := r.Resolve(
		model.NotificationTypeEmployeeCreated, "x@bank.io", "",
		map[string]string{"param": "a&b"},
	)
	require.NoError(t, err)
	assert.Contains(t, got.Body, "a&amp;b")
}

func TestDefaultTemplates_SampleVariablesRenderWithoutLeftoverPlaceholders(t *testing.T) {
	t.Parallel()

	vars := map[string]string{
		"name": "Marko", "username": "Marko",
		"activationLink": "https://bank.io/activate",
		"resetLink":      "https://bank.io/reset",
		"code":           "123456",
		"cardName":       "Visa Platinum", "cardNumber": "4111111111111111",
		"accountNumber": "123-456", "verificationCode": "987654", "reason": "insufficient funds",
		"creditId": "CR-42", "approvedAmount": "5000 RSD", "installmentAmount": "100 RSD", "hours": "24",
		"orderId": "ORD-1", "listingId": "LST-2", "orderType": "LIMIT", "direction": "BUY", "supervisorId": "S-3",
		"transactionId": "TX-99", "tax": "50", "taxRsd": "5000",
		"offerId": "OTC-10", "stockTicker": "AAPL", "amount": "100", "pricePerStock": "150",
		"premium": "5", "status": "OPEN", "expiryDate": "2026-01-01", "timestamp": "2025-01-01T00:00:00Z",
		"eventType": "CANCELED", "contractId": "C-55", "reminderDays": "7",
	}

	r := newRenderer(nil)

	for _, nt := range []model.NotificationType{
		model.NotificationTypeEmployeeCreated,
		model.NotificationTypeEmployeePasswordReset,
		model.NotificationTypeEmployeeAccountDeact,
		model.NotificationTypeClientCreated,
		model.NotificationTypeClientPasswordReset,
		model.NotificationTypeClientAccountDeact,
		model.NotificationTypeVerificationOTP,
		model.NotificationTypeCardRequestVerification,
		model.NotificationTypeCardRequestSuccess,
		model.NotificationTypeCardRequestFailure,
		model.NotificationTypeCardBlocked,
		model.NotificationTypeCardUnblocked,
		model.NotificationTypeCardDeactivated,
		model.NotificationTypeCreditApproved,
		model.NotificationTypeCreditDeclined,
		model.NotificationTypeCreditInstallmentFailed,
		model.NotificationTypeOrderApproved,
		model.NotificationTypeOrderDeclined,
		model.NotificationTypeTaxCollected,
		model.NotificationTypeOTCCounterOffered,
		model.NotificationTypeOTCAccepted,
		model.NotificationTypeOTCCanceled,
		model.NotificationTypeOTCExpiryReminder,
	} {
		nt := nt
		t.Run(string(nt), func(t *testing.T) {
			t.Parallel()
			got, err := r.Resolve(nt, "user@bank.io", "Marko", vars)
			require.NoError(t, err)
			assert.False(t,
				strings.Contains(got.Body, "{{") || strings.Contains(got.Subject, "{{"),
				"rendered output for %s must not contain unfilled placeholders", nt,
			)
		})
	}
}
