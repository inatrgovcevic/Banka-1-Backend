// Package api exposes the saga-orchestrator admin HTTP interface.
//
// Routes (all unauthenticated — network-level guarded; admin port is internal-only):
//
//	GET  /health                             — liveness probe
//	GET  /saga/instances                     — list instances (?state=&limit=&offset=)
//	GET  /saga/instances/{id}                — get single instance by UUID
//	GET  /saga/instances/by-correlation      — lookup by (sagaType, correlationId)
//	POST /saga/instances/{id}/resume         — crash recovery: re-run IN_PROGRESS saga
//	POST /saga/instances/{id}/retry          — re-run a FAILED/COMPENSATED saga (new corrID suffix)
//	POST /saga/instances/{id}/restart        — hard reset + re-run from scratch
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

// SagaDispatcher is the subset of saga.Orchestrator needed by the admin handler.
// Defined as an interface so that tests can inject a stub without importing the
// saga package (which would create a cycle).
type SagaDispatcher interface {
	Dispatch(ctx context.Context, inst *store.SagaInstance) error
}

// AdminHandler serves the admin HTTP endpoints.
type AdminHandler struct {
	store      *store.SagaInstanceStore
	dispatcher SagaDispatcher
}

// NewAdminHandler creates an AdminHandler backed by the given store and
// dispatcher. Pass nil for dispatcher to disable the recovery endpoints
// (they return 503).
func NewAdminHandler(s *store.SagaInstanceStore, d SagaDispatcher) *AdminHandler {
	return &AdminHandler{store: s, dispatcher: d}
}

// List handles GET /saga/instances.
// Query params:
//   - state   — filter by SagaState (STARTED, IN_PROGRESS, COMPENSATING, COMPLETED, FAILED)
//   - limit   — max rows returned (1–200; default 50)
//   - offset  — pagination offset (default 0)
func (h *AdminHandler) List(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	var (
		list []store.SagaInstance
		err  error
	)
	if state != "" {
		list, err = h.store.ListByState(r.Context(), state, limit, offset)
	} else {
		list, err = h.store.ListAll(r.Context(), limit, offset)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, mapInstancesToView(list))
}

// Get handles GET /saga/instances/{id}.
// Returns 400 if the id is not a valid UUID, 404 if not found.
func (h *AdminHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "bad uuid: "+err.Error(), http.StatusBadRequest)
		return
	}

	inst, err := h.store.FindByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if inst == nil {
		http.Error(w, "saga instance not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, mapInstanceToView(*inst))
}

// Health handles GET /health. Returns HTTP 200 with {"status":"UP"}.
// Intentionally does NOT check the DB — it is a pure liveness probe.
func Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"UP"}`))
}

// corsPreflight short-circuits OPTIONS requests with 204 No Content so the
// browser preflight succeeds. Actual Access-Control-* response headers are
// emitted by nginx api-gateway via `add_header ... always` directives.
func corsPreflight(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ByCorrelation handles GET /saga/instances/by-correlation.
// Query params: sagaType (required), correlationId (required).
// Returns 400 if either param is missing, 404 if not found.
func (h *AdminHandler) ByCorrelation(w http.ResponseWriter, r *http.Request) {
	sagaType := r.URL.Query().Get("sagaType")
	correlationID := r.URL.Query().Get("correlationId")
	if sagaType == "" || correlationID == "" {
		http.Error(w, "sagaType and correlationId are required", http.StatusBadRequest)
		return
	}
	inst, err := h.store.FindByTypeAndCorrelation(r.Context(), sagaType, correlationID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if inst == nil {
		http.Error(w, "saga instance not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, mapInstanceToView(*inst))
}

// Resume handles POST /saga/instances/{id}/resume.
// Treats the saga as a crash recovery: leaves state as-is and re-dispatches.
// Works best when the saga is IN_PROGRESS (same exerciseCorrID, idempotent).
func (h *AdminHandler) Resume(w http.ResponseWriter, r *http.Request) {
	h.recover(w, r, store.SagaStateInProgress, false)
}

// Retry handles POST /saga/instances/{id}/retry.
// Resets state to STARTED so the next dispatch increments RetryCount and uses
// a new exerciseCorrID — suitable for FAILED sagas.
func (h *AdminHandler) Retry(w http.ResponseWriter, r *http.Request) {
	h.recover(w, r, store.SagaStateStarted, false)
}

// Restart handles POST /saga/instances/{id}/restart.
// Hard reset: clears CompensationLog, resets RetryCount to 0, sets state to
// STARTED, then re-dispatches (RetryCount becomes 1 after the handler's
// increment, so a fresh exerciseCorrID suffix is used).
func (h *AdminHandler) Restart(w http.ResponseWriter, r *http.Request) {
	h.recover(w, r, store.SagaStateStarted, true)
}

// recover is the shared implementation for Resume/Retry/Restart.
func (h *AdminHandler) recover(w http.ResponseWriter, r *http.Request, targetState string, hardReset bool) {
	if h.dispatcher == nil {
		http.Error(w, "recovery not available", http.StatusServiceUnavailable)
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "bad uuid: "+err.Error(), http.StatusBadRequest)
		return
	}
	inst, err := h.store.FindByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if inst == nil {
		http.Error(w, "saga instance not found", http.StatusNotFound)
		return
	}

	// Apply state reset if requested.
	if inst.State != targetState || hardReset {
		inst.State = targetState
		if hardReset {
			inst.CompensationLog = nil
			inst.RetryCount = 0
			inst.CurrentStep = 0
		}
		if err := h.store.UpdateOptimistic(r.Context(), inst); err != nil {
			http.Error(w, "state reset failed: "+err.Error(), http.StatusConflict)
			return
		}
	}

	// Dispatch runs asynchronously so the HTTP response is not blocked.
	go func() {
		_ = h.dispatcher.Dispatch(context.Background(), inst)
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"sagaId": inst.ID.String(),
		"state":  inst.State,
	})
}

// NewRouter assembles the admin chi.Router. Health is unauthenticated and
// responds quickly. Admin endpoints are also unauthenticated for now because
// the admin port (8095) is only reachable inside the Docker network.
func NewRouter(admin *AdminHandler) http.Handler {
	r := chi.NewRouter()
	r.Use(corsPreflight)
	r.Get("/health", Health)
	r.Route("/saga", func(r chi.Router) {
		r.Get("/instances", admin.List)
		r.Get("/instances/by-correlation", admin.ByCorrelation)
		r.Get("/instances/{id}", admin.Get)
		r.Post("/instances/{id}/resume", admin.Resume)
		r.Post("/instances/{id}/retry", admin.Retry)
		r.Post("/instances/{id}/restart", admin.Restart)
	})
	return r
}

// ---------------------------------------------------------------------------
// View types (JSON-serializable projection of SagaInstance)
// ---------------------------------------------------------------------------

// SagaInstanceView is the public JSON representation of a saga_instance row.
// Payload and CompensationLog are decoded from JSONB to avoid double-encoding.
type SagaInstanceView struct {
	ID              string `json:"id"`
	SagaType        string `json:"sagaType"`
	CorrelationID   string `json:"correlationId"`
	CurrentStep     int    `json:"currentStep"`
	TotalSteps      int    `json:"totalSteps"`
	State           string `json:"state"`
	Payload         any    `json:"payload,omitempty"`
	CompensationLog any    `json:"compensationLog,omitempty"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
	RetryCount      int    `json:"retryCount"`
	Version         int64  `json:"version"`
}

func mapInstanceToView(inst store.SagaInstance) SagaInstanceView {
	out := SagaInstanceView{
		ID:            inst.ID.String(),
		SagaType:      inst.SagaType,
		CorrelationID: inst.CorrelationID,
		CurrentStep:   inst.CurrentStep,
		TotalSteps:    inst.TotalSteps,
		State:         inst.State,
		CreatedAt:     inst.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     inst.UpdatedAt.Format(time.RFC3339),
		RetryCount:    inst.RetryCount,
		Version:       inst.Version,
	}
	// Decode JSONB bytes so the caller receives a structured object, not a
	// base64-encoded string.
	if len(inst.Payload) > 0 {
		var p any
		if json.Unmarshal(inst.Payload, &p) == nil {
			out.Payload = p
		}
	}
	if len(inst.CompensationLog) > 0 {
		var cl any
		if json.Unmarshal(inst.CompensationLog, &cl) == nil {
			out.CompensationLog = cl
		}
	}
	return out
}

func mapInstancesToView(list []store.SagaInstance) []SagaInstanceView {
	out := make([]SagaInstanceView, 0, len(list))
	for _, inst := range list {
		out = append(out, mapInstanceToView(inst))
	}
	return out
}

// writeJSON serialises body to JSON, sets Content-Type, and writes status.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
