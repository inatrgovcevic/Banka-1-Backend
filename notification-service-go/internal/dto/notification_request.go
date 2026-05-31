package dto

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NotificationRequest is the message payload consumed from the AMQP queue.
// JSON field-name aliases match @JsonAlias declarations in the Spring Boot service
// so that existing producers need no changes when the consumer switches to Go.
type NotificationRequest struct {
	Username          string            `json:"username"`
	UserEmail         string            `json:"userEmail"`
	TemplateVariables map[string]string `json:"templateVariables"`
	ClientID          int64             `json:"clientId"`
	OperationType     string            `json:"operationType"`
	SessionID         string            `json:"sessionId"`
}

var userEmailAliases = []string{"userEmail", "email", "recipientEmail"}
var templateVarAliases = []string{"templateVariables", "params", "data", "payload", "userData"}

// UnmarshalJSON supports the field-name aliases from Spring Boot @JsonAlias annotations.
func (r *NotificationRequest) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("NotificationRequest: %w", err)
	}

	if v, ok := raw["username"]; ok {
		if err := json.Unmarshal(v, &r.Username); err != nil {
			return fmt.Errorf("NotificationRequest.username: %w", err)
		}
	}
	if v, ok := raw["clientId"]; ok {
		if err := json.Unmarshal(v, &r.ClientID); err != nil {
			return fmt.Errorf("NotificationRequest.clientId: %w", err)
		}
	}
	if v, ok := raw["operationType"]; ok {
		if err := json.Unmarshal(v, &r.OperationType); err != nil {
			return fmt.Errorf("NotificationRequest.operationType: %w", err)
		}
	}
	if v, ok := raw["sessionId"]; ok {
		if err := json.Unmarshal(v, &r.SessionID); err != nil {
			return fmt.Errorf("NotificationRequest.sessionId: %w", err)
		}
	}

	for _, key := range userEmailAliases {
		if v, ok := raw[key]; ok {
			if err := json.Unmarshal(v, &r.UserEmail); err != nil {
				return fmt.Errorf("NotificationRequest.%s: %w", key, err)
			}
			break
		}
	}

	for _, key := range templateVarAliases {
		if v, ok := raw[key]; ok {
			if err := json.Unmarshal(v, &r.TemplateVariables); err != nil {
				return fmt.Errorf("NotificationRequest.%s: %w", key, err)
			}
			break
		}
	}

	if r.TemplateVariables == nil {
		r.TemplateVariables = make(map[string]string)
	}
	return nil
}

// Validate checks the minimum payload shape required for email delivery.
func (r *NotificationRequest) Validate() error {
	if strings.TrimSpace(r.UserEmail) == "" {
		return fmt.Errorf("userEmail is required (ERR_NOTIFICATION_003)")
	}
	return nil
}

// EffectiveUsername returns the trimmed username or empty string.
func (r *NotificationRequest) EffectiveUsername() string {
	return strings.TrimSpace(r.Username)
}
