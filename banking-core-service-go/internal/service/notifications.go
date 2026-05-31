package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	cardutil "banka1/banking-core-service-go/internal/card"
)

const (
	accountCreatedRoutingKey     = "account.created"
	accountDeactivatedRoutingKey = "account.deactivated"
	cardCreateRoutingKey         = "card.create"
	cardDeactivateRoutingKey     = "card.deactivate"
	cardRequestSuccessRoutingKey = "card.request_success"
	cardBlockedRoutingKey        = "card.blocked"
	cardUnblockedRoutingKey      = "card.unblocked"
	cardDeactivatedRoutingKey    = "card.deactivated"
)

type accountEmailPayload struct {
	UserEmail string `json:"userEmail,omitempty"`
	Username  string `json:"username,omitempty"`
	EmailType string `json:"emailType,omitempty"`
}

type accountCardEventPayload struct {
	ClientID      int64  `json:"clientId"`
	AccountNumber string `json:"accountNumber"`
	EventType     string `json:"eventType"`
}

type cardNotificationPayload struct {
	Username          string            `json:"username,omitempty"`
	UserEmail         string            `json:"userEmail,omitempty"`
	TemplateVariables map[string]string `json:"templateVariables,omitempty"`
}

type verificationNotificationPayload struct {
	UserEmail         string            `json:"userEmail,omitempty"`
	ClientID          int64             `json:"clientId,omitempty"`
	OperationType     string            `json:"operationType,omitempty"`
	SessionID         string            `json:"sessionId,omitempty"`
	TemplateVariables map[string]string `json:"templateVariables,omitempty"`
}

type notificationRecipient struct {
	Name  string
	Email string
}

func (s *AccountService) afterAccountCreated(ctx context.Context, client clientInfo, accountNumber string, createCard bool) {
	s.publishAccountEmail(ctx, client.Username, client.Email, "ACCOUNT_CREATED", accountCreatedRoutingKey)
	if !createCard {
		return
	}
	s.publishAccountCardEvent(ctx, client.ID, accountNumber, "CARD_CREATE", cardCreateRoutingKey)
	if s.automaticCards != nil {
		_, _ = s.automaticCards.CreateAutomaticCard(ctx, AutoCardCreationRequest{
			ClientID:      client.ID,
			AccountNumber: accountNumber,
		})
	}
}

func (s *AccountService) afterAccountStatusChanged(ctx context.Context, account accountView, status string) {
	if !strings.EqualFold(status, "INACTIVE") {
		return
	}
	if strings.TrimSpace(account.Username) != "" && strings.TrimSpace(account.Email) != "" {
		s.publishAccountEmail(ctx, account.Username, account.Email, "ACCOUNT_DEACTIVATED", accountDeactivatedRoutingKey)
	}
	s.publishAccountCardEvent(ctx, account.OwnerID, account.AccountNumber, "CARD_DEACTIVATE", cardDeactivateRoutingKey)
}

func (s *AccountService) publishAccountEmail(ctx context.Context, username, email, emailType, routingKey string) {
	s.rabbit.PublishJSONBestEffort(ctx, s.cfg.NotificationExchange, routingKey, accountEmailPayload{
		UserEmail: strings.TrimSpace(email),
		Username:  strings.TrimSpace(username),
		EmailType: emailType,
	})
}

func (s *AccountService) publishAccountCardEvent(ctx context.Context, clientID int64, accountNumber, eventType, routingKey string) {
	s.rabbit.PublishJSONBestEffort(ctx, s.cfg.NotificationExchange, routingKey, accountCardEventPayload{
		ClientID:      clientID,
		AccountNumber: strings.TrimSpace(accountNumber),
		EventType:     eventType,
	})
}

func (s *CardService) publishRequestSuccess(ctx context.Context, accountNumber string, authorizedPersonID sql.NullInt64, created CardCreationResponse) {
	recipients := s.notificationRecipientsForCard(ctx, accountNumber, authorizedPersonID)
	payload := cardTemplateVariables(created.CardNumber, accountNumber, created.CardName)
	for _, recipient := range recipients {
		s.publishCardNotification(ctx, cardRequestSuccessRoutingKey, recipient, payload)
	}
}

func (s *CardService) publishLifecycleNotification(ctx context.Context, row cardRow, routingKey string) {
	recipients := s.notificationRecipientsForCard(ctx, row.AccountNumber, row.AuthorizedPersonID)
	payload := cardTemplateVariables(row.CardNumber, row.AccountNumber, row.CardName)
	for _, recipient := range recipients {
		s.publishCardNotification(ctx, routingKey, recipient, payload)
	}
}

func (s *CardService) publishCardNotification(ctx context.Context, routingKey string, recipient notificationRecipient, variables map[string]string) {
	if strings.TrimSpace(recipient.Email) == "" {
		return
	}
	s.rabbit.PublishJSONBestEffort(ctx, s.cfg.NotificationExchange, routingKey, cardNotificationPayload{
		Username:          recipient.Name,
		UserEmail:         recipient.Email,
		TemplateVariables: variables,
	})
}

func (s *CardService) notificationRecipientsForCard(ctx context.Context, accountNumber string, authorizedPersonID sql.NullInt64) []notificationRecipient {
	recipients := make([]notificationRecipient, 0, 2)
	seen := map[string]struct{}{}
	if owner, ok := s.accountNotificationRecipient(ctx, accountNumber); ok {
		addNotificationRecipient(&recipients, seen, owner)
	}
	if authorizedPersonID.Valid {
		if authorized, ok := s.authorizedPersonNotificationRecipient(ctx, authorizedPersonID.Int64); ok {
			addNotificationRecipient(&recipients, seen, authorized)
		}
	}
	return recipients
}

func (s *CardService) accountNotificationRecipient(ctx context.Context, accountNumber string) (notificationRecipient, bool) {
	if s.accounts == nil {
		return notificationRecipient{}, false
	}
	account, err := s.accounts.loadAccountViewByNumber(ctx, accountNumber)
	if err != nil || strings.TrimSpace(account.Email) == "" {
		return notificationRecipient{}, false
	}
	return notificationRecipient{
		Name:  accountDisplayName(account),
		Email: strings.TrimSpace(account.Email),
	}, true
}

func (s *CardService) authorizedPersonNotificationRecipient(ctx context.Context, id int64) (notificationRecipient, bool) {
	var firstName, lastName, email string
	err := s.db.QueryRowContext(ctx, `
SELECT COALESCE(first_name, ''), COALESCE(last_name, ''), COALESCE(email, '')
  FROM authorized_persons
 WHERE id = $1
`, id).Scan(&firstName, &lastName, &email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notificationRecipient{}, false
		}
		return notificationRecipient{}, false
	}
	if strings.TrimSpace(email) == "" {
		return notificationRecipient{}, false
	}
	return notificationRecipient{
		Name:  displayName(firstName, lastName, ""),
		Email: strings.TrimSpace(email),
	}, true
}

func (s *VerificationService) publishGeneratedEvent(ctx context.Context, req VerificationGenerateRequest, rawCode string, sessionID int64) {
	s.rabbit.PublishJSONBestEffort(ctx, s.cfg.NotificationExchange, s.cfg.VerificationRoutingKey, verificationNotificationPayload{
		UserEmail:     strings.TrimSpace(req.ClientEmail),
		ClientID:      req.ClientID,
		OperationType: req.OperationType,
		SessionID:     fmt.Sprintf("%d", sessionID),
		TemplateVariables: map[string]string{
			"code": rawCode,
		},
	})
}

func addNotificationRecipient(recipients *[]notificationRecipient, seen map[string]struct{}, recipient notificationRecipient) {
	email := strings.TrimSpace(recipient.Email)
	if email == "" {
		return
	}
	key := strings.ToLower(email)
	if _, exists := seen[key]; exists {
		return
	}
	seen[key] = struct{}{}
	recipient.Email = email
	recipient.Name = strings.TrimSpace(recipient.Name)
	*recipients = append(*recipients, recipient)
}

func cardTemplateVariables(cardNumber, accountNumber, cardName string) map[string]string {
	return map[string]string{
		"cardNumber":    cardutil.MaskCardNumber(cardNumber),
		"accountNumber": cardutil.MaskAccountNumber(accountNumber),
		"cardName":      cardName,
	}
}

func accountDisplayName(account accountView) string {
	return displayName(account.FirstName, account.LastName, account.Username)
}

func displayName(firstName, lastName, fallback string) string {
	name := strings.TrimSpace(strings.TrimSpace(firstName) + " " + strings.TrimSpace(lastName))
	if name != "" {
		return name
	}
	return strings.TrimSpace(fallback)
}
