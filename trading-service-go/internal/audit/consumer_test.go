package audit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"banka1/go-platform/rabbitmq"
)

func TestHandleAuditMessage_EmptyBody(t *testing.T) {
	stub := &stubAuditRepo{}
	svc := newAuditSvc(stub)
	env := rabbitmq.Envelope{Body: []byte{}}
	result := handleAuditMessage(context.Background(), env, svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if result != rabbitmq.Ack {
		t.Errorf("expected Ack for empty body, got %v", result)
	}
	if len(stub.inserted) != 0 {
		t.Error("should not insert for empty body")
	}
}

func TestHandleAuditMessage_InvalidJSON(t *testing.T) {
	stub := &stubAuditRepo{}
	svc := newAuditSvc(stub)
	env := rabbitmq.Envelope{Body: []byte("not-json")}
	result := handleAuditMessage(context.Background(), env, svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if result != rabbitmq.Reject {
		t.Errorf("expected Reject for invalid JSON, got %v", result)
	}
}

func TestHandleAuditMessage_UnknownActionType_Ack(t *testing.T) {
	stub := &stubAuditRepo{}
	svc := newAuditSvc(stub)
	body, _ := json.Marshal(Event{ActionType: "UNKNOWN_TYPE"})
	env := rabbitmq.Envelope{Body: body}
	result := handleAuditMessage(context.Background(), env, svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if result != rabbitmq.Ack {
		t.Errorf("expected Ack for unknown type (Record returns nil), got %v", result)
	}
}

func TestHandleAuditMessage_ValidType_InsertError_Requeue(t *testing.T) {
	stub := &stubAuditRepo{insertErr: errors.New("db error")}
	svc := newAuditSvc(stub)
	body, _ := json.Marshal(Event{ActionType: ActionOrderApproved})
	env := rabbitmq.Envelope{Body: body}
	result := handleAuditMessage(context.Background(), env, svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if result != rabbitmq.Requeue {
		t.Errorf("expected Requeue for insert error, got %v", result)
	}
}

func TestHandleAuditMessage_ValidType_Success_Ack(t *testing.T) {
	stub := &stubAuditRepo{}
	svc := newAuditSvc(stub)
	body, _ := json.Marshal(Event{ActionType: ActionOrderDeclined})
	env := rabbitmq.Envelope{Body: body}
	result := handleAuditMessage(context.Background(), env, svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if result != rabbitmq.Ack {
		t.Errorf("expected Ack for successful insert, got %v", result)
	}
	if len(stub.inserted) != 1 {
		t.Errorf("expected 1 insert, got %d", len(stub.inserted))
	}
}
