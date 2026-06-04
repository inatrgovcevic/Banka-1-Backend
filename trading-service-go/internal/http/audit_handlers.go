package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/audit"
	"banka1/trading-service-go/internal/clients"

	"banka1/go-platform/httpx"
)

const (
	auditDefaultPageSize = 20
	auditMaxPageSize     = 200
)

// AuditLog ↔ GET /audit (WP-2 / Issue 9) — mirrors AuditLogController: optional
// actionType / actorId / from / to filters, createdAt DESC, Page envelope.
// from/to accept an ISO date (2026-05-18) or date-time (2026-05-18T14:30:00);
// a bare date expands to start-of-day (from) / end-of-day (to).
func (h *Handlers) AuditLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var actionType *string
	if raw := strings.TrimSpace(q.Get("actionType")); raw != "" {
		if !audit.ValidActionTypes[raw] {
			writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Nepoznat actionType: '"+raw+"'"))
			return
		}
		actionType = &raw
	}
	var actorID *int64
	if raw := strings.TrimSpace(q.Get("actorId")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest,
				"Invalid value '"+raw+"' for parameter 'actorId', expected type: Long."))
			return
		}
		actorID = &parsed
	}
	from, ok := parseAuditBound(q.Get("from"), true)
	if !ok {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest,
			"Neispravan datum za 'from': '"+q.Get("from")+"' (ocekivan ISO datum ili datum-vreme)"))
		return
	}
	to, ok := parseAuditBound(q.Get("to"), false)
	if !ok {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest,
			"Neispravan datum za 'to': '"+q.Get("to")+"' (ocekivan ISO datum ili datum-vreme)"))
		return
	}

	page := queryIntDefault(r, "page", 0)
	if page < 0 {
		page = 0
	}
	size := queryIntDefault(r, "size", auditDefaultPageSize)
	if size < 1 {
		size = auditDefaultPageSize
	}
	if size > auditMaxPageSize {
		size = auditMaxPageSize
	}

	repo := h.app.Audit.Repo()
	rows, total, err := repo.Search(r.Context(), repo.Pool(), audit.SearchFilter{
		ActionType: actionType,
		ActorID:    actorID,
		From:       from,
		To:         to,
		Page:       page,
		Size:       size,
	})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	out := make([]api.AuditLogDto, 0, len(rows))
	for i := range rows {
		e := &rows[i]
		out = append(out, api.AuditLogDto{
			ID:         e.ID,
			ActorID:    e.ActorID,
			ActorName:  e.ActorName,
			ActionType: e.ActionType,
			TargetType: e.TargetType,
			TargetID:   e.TargetID,
			Details:    e.Details,
			CreatedAt:  api.NewLocalDateTime(e.CreatedAt),
		})
	}
	httpx.JSON(w, http.StatusOK, api.NewPage(out, page, size, total))
}

// AuditLogLegacy ↔ GET /audit-log — the legacy flat view the existing frontend
// audit page consumes (kept from the pre-port stub, now derived from the
// reshaped audit_log schema). actorName comes straight from the stored
// actor_name (falling back to an employee lookup by actor_id); targetName is
// resolved when target_id is a numeric employee id. actorRole / oldValue are
// no longer stored (the Java schema has actor_name + details) and serialize as
// null; newValue carries the details text so the page keeps showing the change
// description. Newest 100 rows, like the stub.
func (h *Handlers) AuditLogLegacy(w http.ResponseWriter, r *http.Request) {
	repo := h.app.Audit.Repo()
	rows, _, err := repo.Search(r.Context(), repo.Pool(), audit.SearchFilter{Page: 0, Size: 100})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	empCtx := clients.WithCallerAuth(r.Context(), r.Header.Get("Authorization"))
	type resolved struct {
		name string
		role *string
	}
	cache := map[int64]resolved{}
	resolve := func(id int64) resolved {
		if r, ok := cache[id]; ok {
			return r
		}
		out := resolved{name: strconv.FormatInt(id, 10)}
		if emp, err := h.app.Employees.GetEmployee(empCtx, id); err == nil && emp != nil {
			parts := []string{}
			if emp.Ime != nil {
				parts = append(parts, *emp.Ime)
			}
			if emp.Prezime != nil {
				parts = append(parts, *emp.Prezime)
			}
			if len(parts) > 0 {
				out.name = strings.Join(parts, " ")
			}
			out.role = emp.Role
		}
		cache[id] = out
		return out
	}

	out := make([]map[string]any, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		entry := map[string]any{
			"id":         row.ID,
			"actorRole":  nil,
			"actionType": row.ActionType,
			"targetType": row.TargetType,
			"oldValue":   nil,
			"newValue":   row.Details,
			"createdAt":  row.CreatedAt,
		}
		// actorName: prefer the stored actor_name (the producer resolved it);
		// actorRole is not stored in the WP-2 schema, so it is looked up live
		// from user-service for real actors (nil for SYSTEM events).
		switch {
		case row.ActorID != nil:
			res := resolve(*row.ActorID)
			if row.ActorName != nil && *row.ActorName != "" {
				entry["actorName"] = *row.ActorName
			} else {
				entry["actorName"] = res.name
			}
			entry["actorRole"] = res.role
		case row.ActorName != nil && *row.ActorName != "":
			entry["actorName"] = *row.ActorName // SYSTEM actor
		default:
			entry["actorName"] = nil
		}
		if row.TargetID != nil {
			if id, err := strconv.ParseInt(*row.TargetID, 10, 64); err == nil && row.TargetType != nil && *row.TargetType == "EMPLOYEE" {
				entry["targetName"] = resolve(id).name
			} else {
				entry["targetName"] = *row.TargetID
			}
		} else {
			entry["targetName"] = nil
		}
		out = append(out, entry)
	}
	httpx.JSON(w, http.StatusOK, out)
}

// parseAuditBound mirrors AuditLogController.parseRangeBound: date-time first,
// then bare date expanded to start/end of day. Returns (nil, true) when absent.
func parseAuditBound(raw string, startOfDay bool) (*time.Time, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, true
	}
	if parsed, err := time.ParseInLocation("2006-01-02T15:04:05.999999999", value, time.UTC); err == nil {
		return &parsed, true
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.UTC)
	if err != nil {
		return nil, false
	}
	if !startOfDay {
		end := parsed.Add(24*time.Hour - time.Nanosecond)
		return &end, true
	}
	return &parsed, true
}
