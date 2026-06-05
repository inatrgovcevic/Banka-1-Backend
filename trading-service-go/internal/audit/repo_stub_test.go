package audit

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type stubAuditRepo struct {
	insertErr error
	inserted  []*Entry
}

func (s *stubAuditRepo) Pool() *pgxpool.Pool { return nil }
func (s *stubAuditRepo) Insert(_ context.Context, _ Querier, e *Entry) error {
	s.inserted = append(s.inserted, e)
	return s.insertErr
}
func (s *stubAuditRepo) Search(_ context.Context, _ Querier, _ SearchFilter) ([]Entry, int64, error) {
	return nil, 0, nil
}

func newAuditSvc(repo auditRepo) *Service {
	return &Service{repo: repo, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestRecord_ValidType_CallsInsert(t *testing.T) {
	stub := &stubAuditRepo{}
	svc := newAuditSvc(stub)
	err := svc.Record(context.Background(), Event{ActionType: ActionOrderApproved})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stub.inserted) != 1 {
		t.Errorf("Insert called %d times, want 1", len(stub.inserted))
	}
	if stub.inserted[0].ActionType != ActionOrderApproved {
		t.Errorf("ActionType = %q, want %q", stub.inserted[0].ActionType, ActionOrderApproved)
	}
}

func TestRecord_ValidType_InsertError_Propagates(t *testing.T) {
	boom := errors.New("insert failed")
	stub := &stubAuditRepo{insertErr: boom}
	svc := newAuditSvc(stub)
	err := svc.Record(context.Background(), Event{ActionType: ActionOrderDeclined})
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestRecord_WithTimestamp_UsesIt(t *testing.T) {
	stub := &stubAuditRepo{}
	svc := newAuditSvc(stub)
	ts := int64(1700000000000) // epoch millis
	err := svc.Record(context.Background(), Event{ActionType: ActionAgentLimitChanged, Timestamp: &ts})
	if err != nil {
		t.Fatal(err)
	}
	want := time.UnixMilli(ts).UTC()
	if !stub.inserted[0].CreatedAt.Equal(want) {
		t.Errorf("CreatedAt = %v, want %v", stub.inserted[0].CreatedAt, want)
	}
}

func TestRecord_NilTimestamp_UsesNow(t *testing.T) {
	stub := &stubAuditRepo{}
	svc := newAuditSvc(stub)
	before := time.Now().Add(-time.Second)
	err := svc.Record(context.Background(), Event{ActionType: ActionTaxRunManual})
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now().Add(time.Second)
	if stub.inserted[0].CreatedAt.Before(before) || stub.inserted[0].CreatedAt.After(after) {
		t.Error("CreatedAt should be approximately now")
	}
}

func TestRecord_CopiesActorFields(t *testing.T) {
	stub := &stubAuditRepo{}
	svc := newAuditSvc(stub)
	id := int64(99)
	name := "Test Actor"
	tt := "USER"
	tid := "42"
	details := "something happened"
	err := svc.Record(context.Background(), Event{
		ActionType: ActionAgentNeedApprovalChanged,
		ActorID:    &id,
		ActorName:  &name,
		TargetType: &tt,
		TargetID:   &tid,
		Details:    &details,
	})
	if err != nil {
		t.Fatal(err)
	}
	e := stub.inserted[0]
	if e.ActorID == nil || *e.ActorID != 99 {
		t.Error("ActorID mismatch")
	}
	if e.ActorName == nil || *e.ActorName != "Test Actor" {
		t.Error("ActorName mismatch")
	}
	if e.TargetType == nil || *e.TargetType != "USER" {
		t.Error("TargetType mismatch")
	}
}

func TestRecordBestEffort_ValidType_CallsInsert(t *testing.T) {
	stub := &stubAuditRepo{}
	svc := newAuditSvc(stub)
	svc.RecordBestEffort(context.Background(), Event{ActionType: ActionTaxRunScheduled})
	if len(stub.inserted) != 1 {
		t.Errorf("Insert called %d times, want 1", len(stub.inserted))
	}
}

func TestRecordBestEffort_InsertError_Swallowed(t *testing.T) {
	stub := &stubAuditRepo{insertErr: errors.New("db error")}
	svc := newAuditSvc(stub)
	// Must not propagate error
	svc.RecordBestEffort(context.Background(), Event{ActionType: ActionOrderApproved})
}

func TestRecord_AllValidTypes(t *testing.T) {
	for action := range ValidActionTypes {
		stub := &stubAuditRepo{}
		svc := newAuditSvc(stub)
		if err := svc.Record(context.Background(), Event{ActionType: action}); err != nil {
			t.Errorf("Record(%q) returned error: %v", action, err)
		}
		if len(stub.inserted) != 1 {
			t.Errorf("Record(%q): Insert called %d times, want 1", action, len(stub.inserted))
		}
	}
}

func TestServiceRepo_ReturnsRepo(t *testing.T) {
	stub := &stubAuditRepo{}
	svc := newAuditSvc(stub)
	if svc.Repo() != stub {
		t.Error("Repo() should return the injected repo")
	}
}
