package user

import (
	"context"
	"testing"

	gpauth "banka1/go-platform/auth"
	"banka1/user-service-go/internal/platform"
)

type recordingNotificationPublisher struct {
	routingKeys []string
	payloads    []any
}

func (p *recordingNotificationPublisher) PublishEmail(ctx context.Context, routingKey string, payload platform.EmailNotification) error {
	return nil
}

func (p *recordingNotificationPublisher) Publish(ctx context.Context, routingKey string, payload any) error {
	p.routingKeys = append(p.routingKeys, routingKey)
	p.payloads = append(p.payloads, payload)
	return nil
}

func (p *recordingNotificationPublisher) Close() {}

func TestPublishPermissionAuditIfChanged(t *testing.T) {
	pub := &recordingNotificationPublisher{}
	service := &Service{pub: pub}
	ctx := gpauth.WithPrincipal(context.Background(), platform.Principal{ID: 10, Email: "admin@example.com"})

	service.publishPermissionAuditIfChanged(ctx, Employee{ID: 42}, []string{"READ"}, []string{"READ", "MARGIN_TRADE"})

	if len(pub.routingKeys) != 1 || pub.routingKeys[0] != "audit.employee_permissions_changed" {
		t.Fatalf("unexpected routing keys: %#v", pub.routingKeys)
	}
	event, ok := pub.payloads[0].(auditEvent)
	if !ok {
		t.Fatalf("expected auditEvent payload, got %T", pub.payloads[0])
	}
	if event.ActorID == nil || *event.ActorID != 10 || event.ActorName != "admin@example.com" {
		t.Fatalf("unexpected actor fields: %#v", event)
	}
	if event.ActionType != "EMPLOYEE_PERMISSIONS_CHANGED" || event.TargetType != "EMPLOYEE" || event.TargetID != "42" {
		t.Fatalf("unexpected audit payload: %#v", event)
	}
}

func TestPublishPermissionAuditActorNameFallsBackToSubject(t *testing.T) {
	pub := &recordingNotificationPublisher{}
	service := &Service{pub: pub}
	// Login tokens carry the email in "sub" and no email claim — the actor
	// name must fall back to the subject, not USER_<id>.
	ctx := gpauth.WithPrincipal(context.Background(), platform.Principal{ID: 10, Subject: "admin@banka.com"})

	service.publishPermissionAuditIfChanged(ctx, Employee{ID: 42}, []string{"READ"}, []string{"READ", "MARGIN_TRADE"})

	if len(pub.payloads) != 1 {
		t.Fatalf("expected one audit event, got %#v", pub.payloads)
	}
	event, ok := pub.payloads[0].(auditEvent)
	if !ok {
		t.Fatalf("expected auditEvent payload, got %T", pub.payloads[0])
	}
	if event.ActorName != "admin@banka.com" {
		t.Fatalf("unexpected actor name: %q", event.ActorName)
	}
}

func TestPublishPermissionAuditIfUnchangedDoesNothing(t *testing.T) {
	pub := &recordingNotificationPublisher{}
	service := &Service{pub: pub}

	service.publishPermissionAuditIfChanged(context.Background(), Employee{ID: 42}, []string{"READ", "READ"}, []string{"READ"})

	if len(pub.routingKeys) != 0 {
		t.Fatalf("expected no audit event, got %#v", pub.routingKeys)
	}
}
