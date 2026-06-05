package actuary

import (
	"context"
	"errors"
	"testing"
	"time"

	"banka1/trading-service-go/internal/clients"

	"github.com/shopspring/decimal"
)

// ---- stubs ----

type stubActuaryRepo struct {
	info        *ActuaryInfo
	findErr     error
	updateErr   error
	resetErr    error
	approvalErr error
	profitRows  []ProfitRow
	profitErr   error
}

func (s *stubActuaryRepo) FindOrCreate(_ context.Context, _ int64) (*ActuaryInfo, error) {
	return s.info, s.findErr
}
func (s *stubActuaryRepo) FindByEmployeeID(_ context.Context, _ int64) (*ActuaryInfo, error) {
	return s.info, s.findErr
}
func (s *stubActuaryRepo) UpdateLimit(_ context.Context, _ int64, _ decimal.Decimal) error {
	return s.updateErr
}
func (s *stubActuaryRepo) ResetLimit(_ context.Context, _ int64) error {
	return s.resetErr
}
func (s *stubActuaryRepo) SetNeedApproval(_ context.Context, _ int64, _ bool) error {
	return s.approvalErr
}
func (s *stubActuaryRepo) SumCommissionByActuary(_ context.Context, _, _ time.Time) ([]ProfitRow, error) {
	return s.profitRows, s.profitErr
}

type stubEmployeeSearcher struct {
	emp       *clients.Employee
	empErr    error
	page      *clients.EmployeePage
	pageErr   error
	callCount int
}

func (s *stubEmployeeSearcher) GetEmployee(_ context.Context, _ int64) (*clients.Employee, error) {
	return s.emp, s.empErr
}
func (s *stubEmployeeSearcher) SearchEmployees(_ context.Context, _, _, _, _ *string, _, _ int) (*clients.EmployeePage, error) {
	s.callCount++
	return s.page, s.pageErr
}

func agentRole() *string { r := "AGENT"; return &r }
func adminRole() *string { r := "ADMIN"; return &r }
func sp(s string) *string { return &s }

func newTestActuaryService(repo actuaryRepo, emp employeeSearcher) *Service {
	return &Service{
		repo:      repo,
		employees: emp,
		auditor:   nil,
	}
}

// ---- roleMatches ----

func TestRoleMatches_AgentMatch(t *testing.T) {
	r := "AGENT"
	if !roleMatches(&r, "AGENT") {
		t.Error("expected match")
	}
}

func TestRoleMatches_CaseInsensitive(t *testing.T) {
	r := "agent"
	if !roleMatches(&r, "AGENT") {
		t.Error("expected case-insensitive match")
	}
}

func TestRoleMatches_Whitespace(t *testing.T) {
	r := "  AGENT  "
	if !roleMatches(&r, "AGENT") {
		t.Error("expected match after trim")
	}
}

func TestRoleMatches_NoMatch(t *testing.T) {
	r := "SUPERVISOR"
	if roleMatches(&r, "AGENT") {
		t.Error("expected no match")
	}
}

func TestRoleMatches_NilRole(t *testing.T) {
	if roleMatches(nil, "AGENT") {
		t.Error("nil role should not match")
	}
}

// ---- toDto ----

func TestToDto_Basic(t *testing.T) {
	name := "Marko"
	surname := "Markovic"
	lim := decimal.NewFromFloat(1000)
	emp := clients.Employee{ID: 5, Ime: &name, Prezime: &surname, Role: agentRole()}
	info := &ActuaryInfo{EmployeeID: 5, Limit: &lim, UsedLimit: decimal.NewFromFloat(200), NeedApproval: true}
	dto := toDto(emp, info)
	if dto.EmployeeID != 5 {
		t.Errorf("EmployeeID = %d, want 5", dto.EmployeeID)
	}
	if dto.Limit == nil || !dto.Limit.Equal(lim) {
		t.Errorf("Limit mismatch")
	}
	if !dto.NeedApproval {
		t.Error("NeedApproval should be true")
	}
}

func TestToDto_NilLimit(t *testing.T) {
	emp := clients.Employee{ID: 1, Role: agentRole()}
	info := &ActuaryInfo{EmployeeID: 1, Limit: nil}
	dto := toDto(emp, info)
	if dto.Limit != nil {
		t.Error("Limit should be nil")
	}
}

// ---- SetLimit ----

func TestSetLimit_EmployeeNotFound(t *testing.T) {
	emp := &stubEmployeeSearcher{empErr: clients.ErrNotFound}
	svc := newTestActuaryService(&stubActuaryRepo{}, emp)
	err := svc.SetLimit(context.Background(), 1, "SUPERVISOR", 99, decimal.NewFromFloat(100))
	if err == nil {
		t.Error("expected error for not-found employee")
	}
}

func TestSetLimit_AdminRole_Returns404(t *testing.T) {
	emp := &stubEmployeeSearcher{emp: &clients.Employee{ID: 99, Role: adminRole()}}
	svc := newTestActuaryService(&stubActuaryRepo{}, emp)
	err := svc.SetLimit(context.Background(), 1, "SUPERVISOR", 99, decimal.NewFromFloat(100))
	if err == nil {
		t.Error("expected error for admin")
	}
}

func TestSetLimit_NotAgentRole_Returns404(t *testing.T) {
	sup := "SUPERVISOR"
	emp := &stubEmployeeSearcher{emp: &clients.Employee{ID: 99, Role: &sup}}
	svc := newTestActuaryService(&stubActuaryRepo{}, emp)
	err := svc.SetLimit(context.Background(), 1, "SUPERVISOR", 99, decimal.NewFromFloat(100))
	if err == nil {
		t.Error("expected error for non-agent role")
	}
}

func TestSetLimit_LimitLowerThanUsed_Returns404(t *testing.T) {
	used := decimal.NewFromFloat(500)
	info := &ActuaryInfo{EmployeeID: 99, UsedLimit: used}
	emp := &stubEmployeeSearcher{emp: &clients.Employee{ID: 99, Role: agentRole()}}
	repo := &stubActuaryRepo{info: info}
	svc := newTestActuaryService(repo, emp)
	err := svc.SetLimit(context.Background(), 1, "SUPERVISOR", 99, decimal.NewFromFloat(100))
	if err == nil {
		t.Error("expected error for limit < used")
	}
}

func TestSetLimit_Success(t *testing.T) {
	info := &ActuaryInfo{EmployeeID: 99, UsedLimit: decimal.NewFromFloat(100)}
	emp := &stubEmployeeSearcher{emp: &clients.Employee{ID: 99, Role: agentRole()}}
	repo := &stubActuaryRepo{info: info}
	svc := newTestActuaryService(repo, emp)
	err := svc.SetLimit(context.Background(), 1, "SUPERVISOR", 99, decimal.NewFromFloat(500))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---- ResetLimit ----

func TestResetLimit_EmployeeNotFound(t *testing.T) {
	emp := &stubEmployeeSearcher{empErr: clients.ErrNotFound}
	svc := newTestActuaryService(&stubActuaryRepo{}, emp)
	err := svc.ResetLimit(context.Background(), 1, "SUPERVISOR", 99)
	if err == nil {
		t.Error("expected error for not-found employee")
	}
}

func TestResetLimit_AdminRole_Returns404(t *testing.T) {
	emp := &stubEmployeeSearcher{emp: &clients.Employee{ID: 99, Role: adminRole()}}
	svc := newTestActuaryService(&stubActuaryRepo{}, emp)
	err := svc.ResetLimit(context.Background(), 1, "SUPERVISOR", 99)
	if err == nil {
		t.Error("expected error for admin")
	}
}

func TestResetLimit_Success(t *testing.T) {
	emp := &stubEmployeeSearcher{emp: &clients.Employee{ID: 99, Role: agentRole()}}
	info := &ActuaryInfo{EmployeeID: 99, UsedLimit: decimal.NewFromFloat(100)}
	repo := &stubActuaryRepo{info: info}
	svc := newTestActuaryService(repo, emp)
	if err := svc.ResetLimit(context.Background(), 1, "SUPERVISOR", 99); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---- SetNeedApproval ----

func TestSetNeedApproval_AdminRole_Returns404(t *testing.T) {
	emp := &stubEmployeeSearcher{emp: &clients.Employee{ID: 99, Role: adminRole()}}
	svc := newTestActuaryService(&stubActuaryRepo{}, emp)
	if err := svc.SetNeedApproval(context.Background(), 99, true); err == nil {
		t.Error("expected error for admin")
	}
}

func TestSetNeedApproval_NotAgent_Returns404(t *testing.T) {
	sup := "SUPERVISOR"
	emp := &stubEmployeeSearcher{emp: &clients.Employee{ID: 99, Role: &sup}}
	svc := newTestActuaryService(&stubActuaryRepo{}, emp)
	if err := svc.SetNeedApproval(context.Background(), 99, true); err == nil {
		t.Error("expected error for non-agent")
	}
}

func TestSetNeedApproval_Success(t *testing.T) {
	emp := &stubEmployeeSearcher{emp: &clients.Employee{ID: 99, Role: agentRole()}}
	svc := newTestActuaryService(&stubActuaryRepo{}, emp)
	if err := svc.SetNeedApproval(context.Background(), 99, false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSetNeedApproval_EmployeeNotFound(t *testing.T) {
	emp := &stubEmployeeSearcher{empErr: clients.ErrNotFound}
	svc := newTestActuaryService(&stubActuaryRepo{}, emp)
	if err := svc.SetNeedApproval(context.Background(), 99, true); err == nil {
		t.Error("expected error for not-found")
	}
}

// ---- ProfitByActuary ----

func TestProfitByActuary_RepoError(t *testing.T) {
	boom := errors.New("db boom")
	repo := &stubActuaryRepo{profitErr: boom}
	emp := &stubEmployeeSearcher{pageErr: errors.New("emp error"), page: &clients.EmployeePage{}}
	svc := newTestActuaryService(repo, emp)
	_, err := svc.ProfitByActuary(context.Background(), nil, nil)
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestProfitByActuary_NoRows_ReturnsEmpty(t *testing.T) {
	repo := &stubActuaryRepo{profitRows: nil}
	emp := &stubEmployeeSearcher{page: &clients.EmployeePage{Content: []clients.Employee{}, TotalPages: 1}}
	svc := newTestActuaryService(repo, emp)
	out, err := svc.ProfitByActuary(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty, got %d entries", len(out))
	}
}

func TestProfitByActuary_WithRows(t *testing.T) {
	rows := []ProfitRow{
		{UserID: 1, TotalCommission: decimal.NewFromFloat(500), TransactionCount: 10},
		{UserID: 2, TotalCommission: decimal.NewFromFloat(200), TransactionCount: 5},
	}
	repo := &stubActuaryRepo{profitRows: rows}
	emp := &stubEmployeeSearcher{page: &clients.EmployeePage{Content: []clients.Employee{}, TotalPages: 1}}
	svc := newTestActuaryService(repo, emp)
	out, err := svc.ProfitByActuary(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Errorf("got %d rows, want 2", len(out))
	}
}

// ---- BankProfitSummary ----

func TestBankProfitSummary_AggregatesCorrectly(t *testing.T) {
	rows := []ProfitRow{
		{UserID: 1, TotalCommission: decimal.NewFromFloat(300), TransactionCount: 5},
		{UserID: 2, TotalCommission: decimal.NewFromFloat(200), TransactionCount: 3},
	}
	repo := &stubActuaryRepo{profitRows: rows}
	emp := &stubEmployeeSearcher{page: &clients.EmployeePage{Content: []clients.Employee{}, TotalPages: 1}}
	svc := newTestActuaryService(repo, emp)
	summary, err := svc.BankProfitSummary(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !summary.TotalCommission.Equal(decimal.NewFromFloat(500)) {
		t.Errorf("TotalCommission = %v, want 500", summary.TotalCommission)
	}
	if summary.TransactionCount != 8 {
		t.Errorf("TransactionCount = %d, want 8", summary.TransactionCount)
	}
	if summary.DistinctActuaries != 2 {
		t.Errorf("DistinctActuaries = %d, want 2", summary.DistinctActuaries)
	}
}

func TestBankProfitSummary_RepoError(t *testing.T) {
	boom := errors.New("boom")
	repo := &stubActuaryRepo{profitErr: boom}
	emp := &stubEmployeeSearcher{page: &clients.EmployeePage{}}
	svc := newTestActuaryService(repo, emp)
	_, err := svc.BankProfitSummary(context.Background(), nil, nil)
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

// ---- GetAgents ----

func TestGetAgents_PageOutOfRange(t *testing.T) {
	emp := &stubEmployeeSearcher{page: &clients.EmployeePage{Content: []clients.Employee{}, TotalPages: 1}}
	svc := newTestActuaryService(&stubActuaryRepo{}, emp)
	page, err := svc.GetAgents(context.Background(), nil, nil, nil, nil, 5, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Content) != 0 {
		t.Errorf("expected empty page for out-of-range page index")
	}
}

func TestGetAgents_FilterAgentsOnly(t *testing.T) {
	agentEmp := clients.Employee{ID: 1, Role: agentRole()}
	supEmp := clients.Employee{ID: 2, Role: sp("SUPERVISOR")}
	page := &clients.EmployeePage{Content: []clients.Employee{agentEmp, supEmp}, TotalPages: 1}
	emp := &stubEmployeeSearcher{page: page}
	info := &ActuaryInfo{EmployeeID: 1}
	repo := &stubActuaryRepo{info: info}
	svc := newTestActuaryService(repo, emp)
	result, err := svc.GetAgents(context.Background(), nil, nil, nil, nil, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 {
		t.Errorf("expected 1 agent, got %d", len(result.Content))
	}
}

func TestGetAgents_SearchError(t *testing.T) {
	boom := errors.New("search fail")
	emp := &stubEmployeeSearcher{pageErr: boom}
	svc := newTestActuaryService(&stubActuaryRepo{}, emp)
	_, err := svc.GetAgents(context.Background(), nil, nil, nil, nil, 0, 10)
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestNewService_NilAuditor(t *testing.T) {
	svc := NewService(nil, nil, nil)
	if svc == nil {
		t.Error("NewService returned nil")
	}
}

func TestProfitByActuary_WithAgentsFromEmployees(t *testing.T) {
	// commission rows from DB
	rows := []ProfitRow{
		{UserID: 1, TotalCommission: decimal.NewFromFloat(300), TransactionCount: 3},
	}
	repo := &stubActuaryRepo{profitRows: rows}

	// employees page has agent (1) already in rows + a new agent (2) with zero commission
	page := &clients.EmployeePage{
		Content: []clients.Employee{
			{ID: 1, Role: agentRole()},
			{ID: 2, Role: agentRole()},
			{ID: 3, Role: sp("SUPERVISOR")},
		},
		TotalPages: 1,
	}
	emp := &stubEmployeeSearcher{page: page}
	svc := newTestActuaryService(repo, emp)
	out, err := svc.ProfitByActuary(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// agent 1 from DB rows, agent 2 from employees with zero commission, supervisor 3 included
	if len(out) < 2 {
		t.Errorf("expected at least 2 entries, got %d", len(out))
	}
}

func TestGetAgents_SingleTermSearch(t *testing.T) {
	// Same ime and prezime → single-term search (two passes)
	name := "Marko"
	agentEmp := clients.Employee{ID: 5, Role: agentRole(), Ime: &name, Prezime: &name}
	page := &clients.EmployeePage{Content: []clients.Employee{agentEmp}, TotalPages: 1}
	emp := &stubEmployeeSearcher{page: page}
	info := &ActuaryInfo{EmployeeID: 5}
	repo := &stubActuaryRepo{info: info}
	svc := newTestActuaryService(repo, emp)
	result, err := svc.GetAgents(context.Background(), nil, &name, &name, nil, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	// single-term does two passes but deduplicates by seen map
	if len(result.Content) != 1 {
		t.Errorf("expected 1 agent (deduped), got %d", len(result.Content))
	}
}

func TestGetAgents_RepoFindError(t *testing.T) {
	boom := errors.New("repo error")
	agentEmp := clients.Employee{ID: 5, Role: agentRole()}
	page := &clients.EmployeePage{Content: []clients.Employee{agentEmp}, TotalPages: 1}
	emp := &stubEmployeeSearcher{page: page}
	repo := &stubActuaryRepo{findErr: boom}
	svc := newTestActuaryService(repo, emp)
	_, err := svc.GetAgents(context.Background(), nil, nil, nil, nil, 0, 10)
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestFetchEmployeeOrNotFound_GenericError(t *testing.T) {
	boom := errors.New("generic error")
	emp := &stubEmployeeSearcher{empErr: boom}
	svc := newTestActuaryService(&stubActuaryRepo{}, emp)
	_, err := svc.fetchEmployeeOrNotFound(context.Background(), 1)
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

func TestSetLimit_FindOrCreateError(t *testing.T) {
	boom := errors.New("db error")
	emp := &stubEmployeeSearcher{emp: &clients.Employee{ID: 99, Role: agentRole()}}
	repo := &stubActuaryRepo{findErr: boom}
	svc := newTestActuaryService(repo, emp)
	err := svc.SetLimit(context.Background(), 1, "SUPERVISOR", 99, decimal.NewFromFloat(500))
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}
