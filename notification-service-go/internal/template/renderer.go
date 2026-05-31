package template

import (
	"fmt"
	"strings"

	"Banka1Back/notification-service-go/internal/model"
)

// ResolvedEmail is the rendered, SMTP-ready email produced by the Renderer.
type ResolvedEmail struct {
	RecipientEmail string
	Subject        string
	Body           string
}

// Renderer resolves a notification request into a ResolvedEmail by looking up
// the registered EmailTemplate and substituting all {{key}} placeholders.
type Renderer struct {
	registry *TemplateRegistry
}

func NewRenderer(registry *TemplateRegistry) *Renderer {
	return &Renderer{registry: registry}
}

// Resolve renders the email template for notificationType using the variables
// and username from the incoming payload.
func (r *Renderer) Resolve(
	notificationType model.NotificationType,
	recipientEmail string,
	username string,
	templateVars map[string]string,
) (*ResolvedEmail, error) {
	if strings.TrimSpace(recipientEmail) == "" {
		return nil, fmt.Errorf("ERR_NOTIFICATION_003: recipientEmail is required")
	}

	tmpl, err := r.registry.Resolve(notificationType)
	if err != nil {
		return nil, err
	}

	vars := buildVariables(username, templateVars)

	return &ResolvedEmail{
		RecipientEmail: recipientEmail,
		Subject:        renderTemplate(tmpl.Subject, vars),
		Body:           renderTemplate(tmpl.BodyTemplate, vars),
	}, nil
}

func buildVariables(username string, incoming map[string]string) map[string]string {
	vars := make(map[string]string, len(incoming)+2)
	for k, v := range incoming {
		vars[k] = v
	}
	uname := strings.TrimSpace(username)
	if uname != "" {
		if _, exists := vars["username"]; !exists {
			vars["username"] = uname
		}
		if _, exists := vars["name"]; !exists {
			vars["name"] = uname
		}
	}
	return vars
}

func renderTemplate(tmpl string, vars map[string]string) string {
	if len(vars) == 0 {
		return tmpl
	}
	result := tmpl
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{"+k+"}}", escapeHTML(v))
	}
	return result
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
