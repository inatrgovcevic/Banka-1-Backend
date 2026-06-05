package audit

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Audit action types — mirror AuditActionType. Stored as the enum name string.
const (
	ActionOrderApproved             = "ORDER_APPROVED"
	ActionOrderDeclined             = "ORDER_DECLINED"
	ActionAgentLimitChanged         = "AGENT_LIMIT_CHANGED"
	ActionAgentUsedLimitReset       = "AGENT_USED_LIMIT_RESET"
	ActionAgentNeedApprovalChanged  = "AGENT_NEED_APPROVAL_CHANGED"
	ActionEmployeePermissionsChange = "EMPLOYEE_PERMISSIONS_CHANGED"
	ActionTaxRunManual              = "TAX_RUN_MANUAL"
	ActionTaxRunScheduled           = "TAX_RUN_SCHEDULED"
)

// ValidActionTypes mirrors the AuditActionType enum domain — an event with an
// unknown actionType is skipped (listener) or rejected with 400 (query API).
var ValidActionTypes = map[string]bool{
	ActionOrderApproved:             true,
	ActionOrderDeclined:             true,
	ActionAgentLimitChanged:         true,
	ActionAgentUsedLimitReset:       true,
	ActionAgentNeedApprovalChanged:  true,
	ActionEmployeePermissionsChange: true,
	ActionTaxRunManual:              true,
	ActionTaxRunScheduled:           true,
}

// Event mirrors AuditEventDto — the broker message contract every producer
// (order-service Java, user-service-go, this service) shares. timestamp is
// epoch millis (UTC); null falls back to the receive time.
type Event struct {
	ActorID    *int64  `json:"actorId"`
	ActorName  *string `json:"actorName"`
	ActionType string  `json:"actionType"`
	TargetType *string `json:"targetType"`
	TargetID   *string `json:"targetId"`
	Details    *string `json:"details"`
	Timestamp  *int64  `json:"timestamp"`
}

// auditRepo is the subset of *Repository used by Service. *Repository satisfies it.
type auditRepo interface {
	Pool() *pgxpool.Pool
	Insert(ctx context.Context, q Querier, e *Entry) error
	Search(ctx context.Context, q Querier, f SearchFilter) ([]Entry, int64, error)
}

// Service is the audit sink + query backend.
type Service struct {
	repo   auditRepo
	logger *slog.Logger
}

func NewService(repo *Repository, logger *slog.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// Repo exposes the repository for the query handler (the query handler needs
// the concrete *Repository for Search; production always injects *Repository).
func (s *Service) Repo() auditRepo { return s.repo }

// Record mirrors AuditEventListener.onAuditEvent: an unknown/blank actionType
// is logged and skipped (never an error — no poison loop); the timestamp falls
// back to now. Returns the insert error so the consumer can decide requeue;
// direct in-process producers treat it as best-effort via RecordBestEffort.
func (s *Service) Record(ctx context.Context, ev Event) error {
	actionType := strings.TrimSpace(ev.ActionType)
	if actionType == "" || !ValidActionTypes[actionType] {
		s.logger.Warn("audit: unknown actionType — event skipped",
			"actionType", ev.ActionType, "actorId", ev.ActorID, "targetType", ev.TargetType, "targetId", ev.TargetID)
		return nil
	}
	createdAt := time.Now().UTC()
	if ev.Timestamp != nil {
		createdAt = time.UnixMilli(*ev.Timestamp).UTC()
	}
	return s.repo.Insert(ctx, s.repo.Pool(), &Entry{
		ActorID:    ev.ActorID,
		ActorName:  ev.ActorName,
		ActionType: actionType,
		TargetType: ev.TargetType,
		TargetID:   ev.TargetID,
		Details:    ev.Details,
		CreatedAt:  createdAt,
	})
}

// RecordBestEffort wraps Record for in-process producers (order decisions):
// audit must never break the business flow, so errors are logged and swallowed
// — mirroring AuditPublisher's catch-and-log.
func (s *Service) RecordBestEffort(ctx context.Context, ev Event) {
	if err := s.Record(ctx, ev); err != nil {
		s.logger.Warn("audit: record failed", "actionType", ev.ActionType, "error", err)
	}
}
