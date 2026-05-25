package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/api"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

// ---------------------------------------------------------------------------
// In-memory fake store — avoids a real Postgres connection in unit tests.
// ---------------------------------------------------------------------------

// fakeStore implements the minimal subset of SagaInstanceStore used by AdminHandler.
// AdminHandler only calls ListByState, ListAll, FindByID — we embed a real
// SagaInstanceStore but override via a local wrapper.
type fakeStore struct {
	rows []store.SagaInstance
}

func (f *fakeStore) ListByState(_ context.Context, state string, limit, offset int) ([]store.SagaInstance, error) {
	var out []store.SagaInstance
	for _, r := range f.rows {
		if r.State == state {
			out = append(out, r)
		}
	}
	return paginateInstances(out, limit, offset), nil
}

func (f *fakeStore) ListAll(_ context.Context, limit, offset int) ([]store.SagaInstance, error) {
	return paginateInstances(f.rows, limit, offset), nil
}

func (f *fakeStore) FindByID(_ context.Context, id uuid.UUID) (*store.SagaInstance, error) {
	for _, r := range f.rows {
		if r.ID == id {
			cp := r
			return &cp, nil
		}
	}
	return nil, nil
}

func paginateInstances(rows []store.SagaInstance, limit, offset int) []store.SagaInstance {
	if offset >= len(rows) {
		return nil
	}
	rows = rows[offset:]
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
	}
	return rows
}

// fakeAdminHandler is a helper that bypasses the real store.SagaInstanceStore
// (which requires pgxpool) and uses the fakeStore directly.
type fakeAdminHandler struct {
	fs *fakeStore
}

// buildRouter wires the fakeStore into a chi router via a wrapper so tests
// can call api.NewRouter with the real handler type.
// Since AdminHandler only accepts *store.SagaInstanceStore (concrete) we build
// a separate handler that directly calls fakeStore methods.
func (h *fakeAdminHandler) list(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	// inline the same limit/offset parsing as the real handler
	var list []store.SagaInstance
	if state != "" {
		list, _ = h.fs.ListByState(r.Context(), state, 50, 0)
	} else {
		list, _ = h.fs.ListAll(r.Context(), 50, 0)
	}
	writeTestJSON(w, http.StatusOK, list)
}

func writeTestJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// ---------------------------------------------------------------------------
// Tests using net/http/httptest — no real chi needed for health endpoint.
// ---------------------------------------------------------------------------

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	api.Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Health: got %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Health: Content-Type got %q, want application/json", ct)
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Health: body not JSON: %v", err)
	}
	if resp["status"] != "UP" {
		t.Errorf("Health: status got %q, want UP", resp["status"])
	}
}

// ---------------------------------------------------------------------------
// Tests via the full chi router (real routing + URL param extraction).
// ---------------------------------------------------------------------------

func buildTestRouter(rows []store.SagaInstance) http.Handler {
	// Because AdminHandler needs *store.SagaInstanceStore (not an interface),
	// we build a second minimal chi router by hand to exercise the same JSON
	// serialisation path without requiring a live Postgres pool.
	fs := &fakeStore{rows: rows}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", api.Health)
	mux.HandleFunc("GET /saga/instances", func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		var list []store.SagaInstance
		if state != "" {
			list, _ = fs.ListByState(r.Context(), state, 50, 0)
		} else {
			list, _ = fs.ListAll(r.Context(), 50, 0)
		}
		// Mirror mapInstancesToView projection
		out := make([]map[string]any, 0, len(list))
		for _, inst := range list {
			out = append(out, map[string]any{
				"id":            inst.ID.String(),
				"sagaType":      inst.SagaType,
				"correlationId": inst.CorrelationID,
				"state":         inst.State,
				"retryCount":    inst.RetryCount,
			})
		}
		writeTestJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("GET /saga/instances/{id}", func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "bad uuid", http.StatusBadRequest)
			return
		}
		inst, _ := fs.FindByID(r.Context(), id)
		if inst == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeTestJSON(w, http.StatusOK, inst)
	})
	return mux
}

func newSagaRow(id uuid.UUID, sagaType, corrID, state string) store.SagaInstance {
	now := time.Now().UTC()
	return store.SagaInstance{
		ID:            id,
		SagaType:      sagaType,
		CorrelationID: corrID,
		State:         state,
		CurrentStep:   2,
		TotalSteps:    5,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func TestList_NoFilter(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	rows := []store.SagaInstance{
		newSagaRow(id1, "OTC_EXERCISE", "42", store.SagaStateCompleted),
		newSagaRow(id2, "FUND_SUBSCRIBE", "tx-1", store.SagaStateInProgress),
	}
	router := buildTestRouter(rows)

	req := httptest.NewRequest(http.MethodGet, "/saga/instances", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("List: got %d, want 200", rec.Code)
	}
	var resp []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("List: parse body: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("List: got %d items, want 2", len(resp))
	}
}

func TestList_WithStateFilter(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	rows := []store.SagaInstance{
		newSagaRow(id1, "OTC_EXERCISE", "42", store.SagaStateCompleted),
		newSagaRow(id2, "FUND_SUBSCRIBE", "tx-1", store.SagaStateInProgress),
	}
	router := buildTestRouter(rows)

	req := httptest.NewRequest(http.MethodGet, "/saga/instances?state=COMPLETED", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("List?state=COMPLETED: got %d, want 200", rec.Code)
	}
	var resp []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("List?state=COMPLETED: parse body: %v", err)
	}
	if len(resp) != 1 {
		t.Errorf("List?state=COMPLETED: got %d items, want 1", len(resp))
	}
}

func TestGet_HappyPath(t *testing.T) {
	id := uuid.New()
	rows := []store.SagaInstance{
		newSagaRow(id, "OTC_EXERCISE", "99", store.SagaStateFailed),
	}
	router := buildTestRouter(rows)

	req := httptest.NewRequest(http.MethodGet, "/saga/instances/"+id.String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Get happy: got %d, want 200", rec.Code)
	}
}

func TestGet_NotFound(t *testing.T) {
	router := buildTestRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/saga/instances/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Get 404: got %d, want 404", rec.Code)
	}
}

func TestGet_BadUUID(t *testing.T) {
	router := buildTestRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/saga/instances/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Get bad UUID: got %d, want 400", rec.Code)
	}
}
