package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"banka1/trading-service-go/internal/actuary"
	"banka1/trading-service-go/internal/clients"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// ---- stub actuary repo for http tests ----

type httpStubActuaryRepo struct {
	info    *actuary.ActuaryInfo
	findErr error
	rows    []actuary.ProfitRow
}

func (s *httpStubActuaryRepo) FindOrCreate(_ context.Context, _ int64) (*actuary.ActuaryInfo, error) {
	return s.info, s.findErr
}
func (s *httpStubActuaryRepo) FindByEmployeeID(_ context.Context, _ int64) (*actuary.ActuaryInfo, error) {
	return s.info, s.findErr
}
func (s *httpStubActuaryRepo) UpdateLimit(_ context.Context, _ int64, _ decimal.Decimal) error {
	return nil
}
func (s *httpStubActuaryRepo) ResetLimit(_ context.Context, _ int64) error     { return nil }
func (s *httpStubActuaryRepo) SetNeedApproval(_ context.Context, _ int64, _ bool) error {
	return nil
}
func (s *httpStubActuaryRepo) SumCommissionByActuary(_ context.Context, _, _ time.Time) ([]actuary.ProfitRow, error) {
	return s.rows, nil
}

type httpStubEmpSearcher struct {
	emp  *clients.Employee
	page *clients.EmployeePage
}

func (s *httpStubEmpSearcher) GetEmployee(_ context.Context, _ int64) (*clients.Employee, error) {
	return s.emp, nil
}
func (s *httpStubEmpSearcher) SearchEmployees(_ context.Context, _, _, _, _ *string, _, _ int) (*clients.EmployeePage, error) {
	if s.page != nil {
		return s.page, nil
	}
	return &clients.EmployeePage{Content: []clients.Employee{}, TotalPages: 1}, nil
}

func newHandlersWithActuary(repo actuary.ActuaryRepo, emp actuary.EmployeeSearcher) *Handlers {
	svc := actuary.NewServiceForTest(repo, emp)
	app := &App{Actuary: svc}
	return &Handlers{app: app}
}

// ---- validateLimit ----

func TestValidateLimit_Nil_ReturnsError(t *testing.T) {
	fields := validateLimit(nil)
	assert.NotNil(t, fields)
	assert.Contains(t, fields["limit"], "not be null")
}

func TestValidateLimit_Zero_ReturnsError(t *testing.T) {
	d := decimal.Zero
	fields := validateLimit(&d)
	assert.NotNil(t, fields)
	assert.Contains(t, fields["limit"], "greater than")
}

func TestValidateLimit_Negative_ReturnsError(t *testing.T) {
	d := decimal.NewFromFloat(-1)
	fields := validateLimit(&d)
	assert.NotNil(t, fields)
}

func TestValidateLimit_Positive_ReturnsNil(t *testing.T) {
	d := decimal.NewFromFloat(100)
	fields := validateLimit(&d)
	assert.Nil(t, fields)
}

func TestValidateLimit_SmallPositive_ReturnsNil(t *testing.T) {
	d := decimal.NewFromFloat(0.01)
	fields := validateLimit(&d)
	assert.Nil(t, fields)
}

// ---- actuaryCtx ----

func TestActuaryCtx_CarriesAuthHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/actuaries", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	ctx := actuaryCtx(req)
	assert.NotNil(t, ctx)
}

// ---- ActuaryNeedApproval: early validation paths ----

func TestActuaryNeedApproval_BadPathID_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	req := httptest.NewRequest(http.MethodPut, "/actuaries/agents/abc/need-approval", nil)
	req.SetPathValue("id", "abc")
	w := httptest.NewRecorder()
	h.ActuaryNeedApproval(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestActuaryNeedApproval_InvalidJSON_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	req := httptest.NewRequest(http.MethodPut, "/actuaries/agents/1/need-approval",
		bytes.NewBufferString("{not-json}"))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.ActuaryNeedApproval(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestActuaryNeedApproval_NullNeedApproval_Returns422(t *testing.T) {
	h := &Handlers{app: &App{}}
	req := httptest.NewRequest(http.MethodPut, "/actuaries/agents/1/need-approval",
		bytes.NewBufferString(`{"needApproval": null}`))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.ActuaryNeedApproval(w, req)
	// needApproval must not be null → validation error
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// ---- ActuarySetLimit: early validation paths ----

func TestActuarySetLimit_BadPathID_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	req := httptest.NewRequest(http.MethodPut, "/actuaries/agents/xyz/limit", nil)
	req.SetPathValue("id", "xyz")
	w := httptest.NewRecorder()
	h.ActuarySetLimit(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestActuarySetLimit_InvalidJSON_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	req := httptest.NewRequest(http.MethodPut, "/actuaries/agents/1/limit",
		bytes.NewBufferString("{bad}"))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.ActuarySetLimit(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestActuarySetLimit_NullLimit_Returns422(t *testing.T) {
	h := &Handlers{app: &App{}}
	req := httptest.NewRequest(http.MethodPut, "/actuaries/agents/1/limit",
		bytes.NewBufferString(`{"limit": null}`))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.ActuarySetLimit(w, req)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// ---- ActuaryResetLimit: bad path ID ----

func TestActuaryResetLimit_BadPathID_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	req := httptest.NewRequest(http.MethodPut, "/actuaries/agents/abc/reset-limit", nil)
	req.SetPathValue("id", "abc")
	w := httptest.NewRecorder()
	h.ActuaryResetLimit(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---- ActuaryProfit: invalid datetime ----

func TestActuaryProfit_InvalidFrom_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	req := httptest.NewRequest(http.MethodGet, "/actuaries/profit?from=not-a-date", nil)
	w := httptest.NewRecorder()
	h.ActuaryProfit(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestActuaryProfit_InvalidTo_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	req := httptest.NewRequest(http.MethodGet, "/actuaries/profit?to=bad", nil)
	w := httptest.NewRecorder()
	h.ActuaryProfit(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---- ActuaryBankSummary: invalid datetime ----

func TestActuaryBankSummary_InvalidFrom_Returns400(t *testing.T) {
	h := &Handlers{app: &App{}}
	req := httptest.NewRequest(http.MethodGet, "/actuaries/profit/bank-summary?from=bad", nil)
	w := httptest.NewRecorder()
	h.ActuaryBankSummary(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---- ActuaryAgents with injectable service ----

func TestActuaryAgents_EmptyResult_Returns200(t *testing.T) {
	stub := &httpStubActuaryRepo{}
	emp := &httpStubEmpSearcher{page: &clients.EmployeePage{Content: []clients.Employee{}, TotalPages: 1}}
	h := newHandlersWithActuary(stub, emp)
	req := httptest.NewRequest(http.MethodGet, "/actuaries/agents", nil)
	w := httptest.NewRecorder()
	h.ActuaryAgents(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- ActuaryProfit with injectable service ----

func TestActuaryProfit_Success_Returns200(t *testing.T) {
	stub := &httpStubActuaryRepo{}
	emp := &httpStubEmpSearcher{page: &clients.EmployeePage{Content: []clients.Employee{}, TotalPages: 1}}
	h := newHandlersWithActuary(stub, emp)
	req := httptest.NewRequest(http.MethodGet, "/actuaries/profit", nil)
	w := httptest.NewRecorder()
	h.ActuaryProfit(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- ActuaryBankSummary with injectable service ----

func TestActuaryBankSummary_Success_Returns200(t *testing.T) {
	stub := &httpStubActuaryRepo{}
	emp := &httpStubEmpSearcher{page: &clients.EmployeePage{Content: []clients.Employee{}, TotalPages: 1}}
	h := newHandlersWithActuary(stub, emp)
	req := httptest.NewRequest(http.MethodGet, "/actuaries/profit/bank-summary", nil)
	w := httptest.NewRecorder()
	h.ActuaryBankSummary(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// suppress unused import warnings
var _ = decimal.Zero
var _ = time.Now()
