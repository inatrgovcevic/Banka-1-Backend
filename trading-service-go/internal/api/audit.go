package api

// AuditLogDto ↔ trading-service audit AuditLogDto (WP-2 / Issue 9) — one row of
// the GET /audit page, newest first. actionType serializes as the enum name.
type AuditLogDto struct {
	ID         int64         `json:"id"`
	ActorID    *int64        `json:"actorId"`
	ActorName  *string       `json:"actorName"`
	ActionType string        `json:"actionType"`
	TargetType *string       `json:"targetType"`
	TargetID   *string       `json:"targetId"`
	Details    *string       `json:"details"`
	CreatedAt  LocalDateTime `json:"createdAt"`
}
