package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/api"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
)

// ---------------------------------------------------------------------------
// In-memory AdminStore fake exercising the REAL AdminHandler + chi router.
// ---------------------------------------------------------------------------

type memStore struct {
	mu      sync.Mutex
	rows    map[uuid.UUID]*store.SagaInstance
	listErr error // forced error for ListAll/ListByState
	findErr error // forced error for FindByID/FindByTypeAndCorrelation
	updErr  error // forced error for UpdateOptimistic
}

func newMemStore(rows ...store.SagaInstance) *memStore {
	m := &memStore{rows: make(map[uuid.UUID]*store.SagaInstance)}
	for i := range rows {
		cp := rows[i]
		m.rows[cp.ID] = &cp
	}
	return m
}

func (m *memStore) all() []store.SagaInstance {
	out := make([]store.SagaInstance, 0, len(m.rows))
	for _, r := range m.rows {
		out = append(out, *r)
	}
	return out
}

func (m *memStore) ListByState(_ context.Context, state string, limit, offset int) ([]store.SagaInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listErr != nil {
		return nil, m.listErr
	}
	var out []store.SagaInstance
	for _, r := range m.all() {
		if r.State == state {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memStore) ListAll(_ context.Context, limit, offset int) ([]store.SagaInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.all(), nil
}

func (m *memStore) FindByID(_ context.Context, id uuid.UUID) (*store.SagaInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.findErr != nil {
		return nil, m.findErr
	}
	if r, ok := m.rows[id]; ok {
		cp := *r
		return &cp, nil
	}
	return nil, nil
}

func (m *memStore) FindByTypeAndCorrelation(_ context.Context, sagaType, corr string) (*store.SagaInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.findErr != nil {
		return nil, m.findErr
	}
	for _, r := range m.rows {
		if r.SagaType == sagaType && r.CorrelationID == corr {
			cp := *r
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *memStore) UpdateOptimistic(_ context.Context, inst *store.SagaInstance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updErr != nil {
		return m.updErr
	}
	cp := *inst
	m.rows[inst.ID] = &cp
	inst.Version++
	return nil
}

// stubDispatcher records Dispatch invocations.
type stubDispatcher struct {
	mu     sync.Mutex
	called int
	err    error
}

func (s *stubDispatcher) Dispatch(_ context.Context, _ *store.SagaInstance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.called++
	return s.err
}

func (s *stubDispatcher) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.called
}

func realRow(id uuid.UUID, sagaType, corr, state string) store.SagaInstance {
	now := time.Now().UTC()
	return store.SagaInstance{
		ID:            id,
		SagaType:      sagaType,
		CorrelationID: corr,
		State:         state,
		CurrentStep:   2,
		TotalSteps:    5,
		Payload:       []byte(`{"contractId":42}`),
		CreatedAt:     now,
		UpdatedAt:     now,
		Version:       1,
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestAdminList_NoFilter(t *testing.T) {
	ms := newMemStore(
		realRow(uuid.New(), "OTC_EXERCISE", "1", store.SagaStateCompleted),
		realRow(uuid.New(), "FUND_REDEEM", "2", store.SagaStateInProgress),
	)
	h := api.NewAdminHandlerWithStore(ms, nil)
	router := api.NewRouter(h)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/saga/instances", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var resp []api.SagaInstanceView
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("got %d, want 2", len(resp))
	}
	// Payload decoded as structured object.
	if resp[0].Payload == nil {
		t.Error("expected payload to be decoded")
	}
}

func TestAdminList_StateFilter(t *testing.T) {
	ms := newMemStore(
		realRow(uuid.New(), "OTC_EXERCISE", "1", store.SagaStateCompleted),
		realRow(uuid.New(), "FUND_REDEEM", "2", store.SagaStateInProgress),
	)
	h := api.NewAdminHandlerWithStore(ms, nil)
	router := api.NewRouter(h)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/saga/instances?state=COMPLETED&limit=10&offset=0", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var resp []api.SagaInstanceView
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp) != 1 {
		t.Errorf("got %d, want 1", len(resp))
	}
}

func TestAdminList_LimitClampedAndNegativeOffset(t *testing.T) {
	ms := newMemStore(realRow(uuid.New(), "OTC_EXERCISE", "1", store.SagaStateCompleted))
	h := api.NewAdminHandlerWithStore(ms, nil)
	router := api.NewRouter(h)

	rec := httptest.NewRecorder()
	// limit > 200 clamps to 50; offset < 0 clamps to 0.
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/saga/instances?limit=9999&offset=-5", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
}

func TestAdminList_StoreError(t *testing.T) {
	ms := newMemStore()
	ms.listErr = errors.New("db down")
	h := api.NewAdminHandlerWithStore(ms, nil)
	router := api.NewRouter(h)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/saga/instances", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

func TestAdminGet_HappyPath(t *testing.T) {
	id := uuid.New()
	ms := newMemStore(realRow(id, "OTC_EXERCISE", "99", store.SagaStateFailed))
	h := api.NewAdminHandlerWithStore(ms, nil)
	router := api.NewRouter(h)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/saga/instances/"+id.String(), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var v api.SagaInstanceView
	json.Unmarshal(rec.Body.Bytes(), &v)
	if v.ID != id.String() {
		t.Errorf("id=%q, want %q", v.ID, id.String())
	}
}

func TestAdminGet_BadUUID(t *testing.T) {
	ms := newMemStore()
	h := api.NewAdminHandlerWithStore(ms, nil)
	router := api.NewRouter(h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/saga/instances/not-a-uuid", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
}

func TestAdminGet_NotFound(t *testing.T) {
	ms := newMemStore()
	h := api.NewAdminHandlerWithStore(ms, nil)
	router := api.NewRouter(h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/saga/instances/"+uuid.New().String(), nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rec.Code)
	}
}

func TestAdminGet_StoreError(t *testing.T) {
	ms := newMemStore()
	ms.findErr = errors.New("db down")
	h := api.NewAdminHandlerWithStore(ms, nil)
	router := api.NewRouter(h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/saga/instances/"+uuid.New().String(), nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// ByCorrelation
// ---------------------------------------------------------------------------

func TestAdminByCorrelation_HappyPath(t *testing.T) {
	ms := newMemStore(realRow(uuid.New(), "OTC_EXERCISE", "55", store.SagaStateCompleted))
	h := api.NewAdminHandlerWithStore(ms, nil)
	router := api.NewRouter(h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/saga/instances/by-correlation?sagaType=OTC_EXERCISE&correlationId=55", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
}

func TestAdminByCorrelation_MissingParams(t *testing.T) {
	ms := newMemStore()
	h := api.NewAdminHandlerWithStore(ms, nil)
	router := api.NewRouter(h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/saga/instances/by-correlation?sagaType=OTC_EXERCISE", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
}

func TestAdminByCorrelation_NotFound(t *testing.T) {
	ms := newMemStore()
	h := api.NewAdminHandlerWithStore(ms, nil)
	router := api.NewRouter(h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/saga/instances/by-correlation?sagaType=X&correlationId=Y", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rec.Code)
	}
}

func TestAdminByCorrelation_StoreError(t *testing.T) {
	ms := newMemStore()
	ms.findErr = errors.New("db down")
	h := api.NewAdminHandlerWithStore(ms, nil)
	router := api.NewRouter(h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/saga/instances/by-correlation?sagaType=X&correlationId=Y", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Resume / Retry / Restart (recover)
// ---------------------------------------------------------------------------

func waitForDispatch(d *stubDispatcher) {
	for i := 0; i < 200; i++ {
		if d.calls() > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestAdminResume_HappyPath(t *testing.T) {
	id := uuid.New()
	ms := newMemStore(realRow(id, "OTC_EXERCISE", "1", store.SagaStateInProgress))
	disp := &stubDispatcher{}
	h := api.NewAdminHandlerWithStore(ms, disp)
	router := api.NewRouter(h)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/saga/instances/"+id.String()+"/resume", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("got %d, want 202", rec.Code)
	}
	waitForDispatch(disp)
	if disp.calls() != 1 {
		t.Errorf("dispatch calls=%d, want 1", disp.calls())
	}
}

func TestAdminRetry_ResetsToStarted(t *testing.T) {
	id := uuid.New()
	ms := newMemStore(realRow(id, "OTC_EXERCISE", "1", store.SagaStateFailed))
	disp := &stubDispatcher{}
	h := api.NewAdminHandlerWithStore(ms, disp)
	router := api.NewRouter(h)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/saga/instances/"+id.String()+"/retry", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("got %d, want 202", rec.Code)
	}
	var resp map[string]string
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["state"] != store.SagaStateStarted {
		t.Errorf("state=%q, want STARTED", resp["state"])
	}
	waitForDispatch(disp)
}

func TestAdminRestart_HardReset(t *testing.T) {
	id := uuid.New()
	row := realRow(id, "OTC_EXERCISE", "1", store.SagaStateFailed)
	row.RetryCount = 5
	row.CompensationLog = []byte(`{"x":1}`)
	ms := newMemStore(row)
	disp := &stubDispatcher{}
	h := api.NewAdminHandlerWithStore(ms, disp)
	router := api.NewRouter(h)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/saga/instances/"+id.String()+"/restart", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("got %d, want 202", rec.Code)
	}
	waitForDispatch(disp)

	updated, _ := ms.FindByID(context.Background(), id)
	if updated.RetryCount != 0 {
		t.Errorf("retryCount=%d, want 0 after hard reset", updated.RetryCount)
	}
	if updated.CurrentStep != 0 {
		t.Errorf("currentStep=%d, want 0 after hard reset", updated.CurrentStep)
	}
}

func TestAdminRecover_NoDispatcher(t *testing.T) {
	id := uuid.New()
	ms := newMemStore(realRow(id, "OTC_EXERCISE", "1", store.SagaStateInProgress))
	h := api.NewAdminHandlerWithStore(ms, nil) // nil dispatcher
	router := api.NewRouter(h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/saga/instances/"+id.String()+"/resume", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503", rec.Code)
	}
}

func TestAdminRecover_BadUUID(t *testing.T) {
	ms := newMemStore()
	disp := &stubDispatcher{}
	h := api.NewAdminHandlerWithStore(ms, disp)
	router := api.NewRouter(h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/saga/instances/bad/resume", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
}

func TestAdminRecover_NotFound(t *testing.T) {
	ms := newMemStore()
	disp := &stubDispatcher{}
	h := api.NewAdminHandlerWithStore(ms, disp)
	router := api.NewRouter(h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/saga/instances/"+uuid.New().String()+"/retry", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rec.Code)
	}
}

func TestAdminRecover_FindError(t *testing.T) {
	ms := newMemStore()
	ms.findErr = errors.New("db down")
	disp := &stubDispatcher{}
	h := api.NewAdminHandlerWithStore(ms, disp)
	router := api.NewRouter(h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/saga/instances/"+uuid.New().String()+"/retry", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", rec.Code)
	}
}

func TestAdminRecover_UpdateConflict(t *testing.T) {
	id := uuid.New()
	ms := newMemStore(realRow(id, "OTC_EXERCISE", "1", store.SagaStateFailed))
	ms.updErr = store.ErrOptimisticLockConflict
	disp := &stubDispatcher{}
	h := api.NewAdminHandlerWithStore(ms, disp)
	router := api.NewRouter(h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/saga/instances/"+id.String()+"/retry", nil))
	if rec.Code != http.StatusConflict {
		t.Errorf("got %d, want 409", rec.Code)
	}
}

// Resume on an already-IN_PROGRESS saga skips the state reset (no update needed).
func TestAdminResume_AlreadyInProgress_NoUpdate(t *testing.T) {
	id := uuid.New()
	ms := newMemStore(realRow(id, "OTC_EXERCISE", "1", store.SagaStateInProgress))
	ms.updErr = errors.New("update should not be called")
	disp := &stubDispatcher{}
	h := api.NewAdminHandlerWithStore(ms, disp)
	router := api.NewRouter(h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/saga/instances/"+id.String()+"/resume", nil))
	if rec.Code != http.StatusAccepted {
		t.Errorf("got %d, want 202 (no state change → no UpdateOptimistic)", rec.Code)
	}
	waitForDispatch(disp)
}

// ---------------------------------------------------------------------------
// CORS preflight
// ---------------------------------------------------------------------------

func TestCorsPreflight_Options(t *testing.T) {
	ms := newMemStore()
	h := api.NewAdminHandlerWithStore(ms, nil)
	router := api.NewRouter(h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/saga/instances", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS preflight: got %d, want 204", rec.Code)
	}
}

func TestRouter_HealthEndpoint(t *testing.T) {
	ms := newMemStore()
	h := api.NewAdminHandlerWithStore(ms, nil)
	router := api.NewRouter(h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("got %d, want 200", rec.Code)
	}
}
