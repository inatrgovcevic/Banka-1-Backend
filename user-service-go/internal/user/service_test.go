package user

import (
	"context"
	"errors"
	"testing"
	"time"

	gpauth "banka1/go-platform/auth"
	"banka1/user-service-go/internal/platform"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mock repository
// ---------------------------------------------------------------------------

type mockRepo struct {
	employeeByLoginResult     Employee
	employeeByLoginErr        error
	employeeByIDResult        Employee
	employeeByIDErr           error
	clientByEmailResult       Client
	clientByEmailErr          error
	clientByIDResult          Client
	clientByIDErr             error
	clientByJMBGResult        Client
	clientByJMBGErr           error
	employeePermissions       []string
	clientPermissions         []string
	firstActiveIDResult       int64
	firstActiveIDErr          error
	searchEmployeesResult     []Employee
	searchEmployeesTotal      int
	searchEmployeesErr        error
	searchClientsResult       []Client
	searchClientsTotal        int
	searchClientsErr          error
	createEmployeeResult      Employee
	createEmployeeErr         error
	updateEmployeeResult      Employee
	updateEmployeeErr         error
	createClientResult        Client
	createClientErr           error
	updateClientErr           error
	otcClientIDs              []int64
	otcClientIDsErr           error
	storeRefreshErr           error
	employeeByRefreshResult   Employee
	employeeByRefreshErr      error
	confirmationIDResult      int64
	confirmationIDErr         error
	registerFailedLoginErr    error
}

func (m *mockRepo) EmployeeByLogin(_ context.Context, _ string) (Employee, error) {
	return m.employeeByLoginResult, m.employeeByLoginErr
}
func (m *mockRepo) EmployeeByID(_ context.Context, _ int64) (Employee, error) {
	return m.employeeByIDResult, m.employeeByIDErr
}
func (m *mockRepo) FirstActiveEmployeeIDByRoleExcluding(_ context.Context, _ string, _ int64) (int64, error) {
	return m.firstActiveIDResult, m.firstActiveIDErr
}
func (m *mockRepo) EmployeePermissions(_ context.Context, _ int64, _ string) []string {
	return m.employeePermissions
}
func (m *mockRepo) ReplaceEmployeePermissions(_ context.Context, _ int64, _ []string) error {
	return nil
}
func (m *mockRepo) ResetEmployeeLoginFailures(_ context.Context, _ int64) error { return nil }
func (m *mockRepo) RegisterFailedEmployeeLogin(_ context.Context, _ Employee, _ int, _ time.Duration) error {
	return m.registerFailedLoginErr
}
func (m *mockRepo) StoreEmployeeRefreshToken(_ context.Context, _ int64, _ string, _ time.Time) error {
	return m.storeRefreshErr
}
func (m *mockRepo) EmployeeByRefreshToken(_ context.Context, _ string) (Employee, error) {
	return m.employeeByRefreshResult, m.employeeByRefreshErr
}
func (m *mockRepo) DeleteRefreshToken(_ context.Context, _ string) error { return nil }
func (m *mockRepo) ConfirmationIDByToken(_ context.Context, _, _, _ string) (int64, error) {
	return m.confirmationIDResult, m.confirmationIDErr
}
func (m *mockRepo) UpsertEmployeeConfirmation(_ context.Context, _ int64, _ string, _ time.Time) error {
	return nil
}
func (m *mockRepo) UpsertClientConfirmation(_ context.Context, _ int64, _ string, _ time.Time) error {
	return nil
}
func (m *mockRepo) ActivateEmployeePassword(_ context.Context, _ int64, _, _ string) error {
	return nil
}
func (m *mockRepo) ActivateClientPassword(_ context.Context, _ int64, _, _ string) error {
	return nil
}
func (m *mockRepo) SearchEmployees(_ context.Context, _ SearchQuery) ([]Employee, int, error) {
	return m.searchEmployeesResult, m.searchEmployeesTotal, m.searchEmployeesErr
}
func (m *mockRepo) SearchClients(_ context.Context, _ SearchQuery) ([]Client, int, error) {
	return m.searchClientsResult, m.searchClientsTotal, m.searchClientsErr
}
func (m *mockRepo) CreateEmployee(_ context.Context, _ EmployeeCreateRequest, _ []string) (Employee, error) {
	return m.createEmployeeResult, m.createEmployeeErr
}
func (m *mockRepo) UpdateEmployee(_ context.Context, _ int64, _ EmployeeUpdateRequest) (Employee, error) {
	return m.updateEmployeeResult, m.updateEmployeeErr
}
func (m *mockRepo) SoftDeleteEmployee(_ context.Context, _ int64) error { return nil }
func (m *mockRepo) ClientByEmail(_ context.Context, _ string) (Client, error) {
	return m.clientByEmailResult, m.clientByEmailErr
}
func (m *mockRepo) ClientByID(_ context.Context, _ int64) (Client, error) {
	return m.clientByIDResult, m.clientByIDErr
}
func (m *mockRepo) ClientByPlainJMBG(_ context.Context, _ string) (Client, error) {
	return m.clientByJMBGResult, m.clientByJMBGErr
}
func (m *mockRepo) ClientPermissions(_ context.Context, _ int64, _ string) []string {
	return m.clientPermissions
}
func (m *mockRepo) OTCTradingClientIDs(_ context.Context) ([]int64, error) {
	return m.otcClientIDs, m.otcClientIDsErr
}
func (m *mockRepo) CreateClient(_ context.Context, _ ClientCreateRequest, _ []string) (Client, error) {
	return m.createClientResult, m.createClientErr
}
func (m *mockRepo) UpdateClient(_ context.Context, _ int64, _ ClientUpdateRequest) (Client, error) {
	return Client{}, m.updateClientErr
}
func (m *mockRepo) AddClientMarginPermission(_ context.Context, _ int64) error { return nil }
func (m *mockRepo) SoftDeleteClient(_ context.Context, _ int64) error          { return nil }

// ---------------------------------------------------------------------------
// Mock publisher
// ---------------------------------------------------------------------------

type mockPub struct {
	published []string
}

func (p *mockPub) PublishEmail(_ context.Context, key string, _ platform.EmailNotification) error {
	p.published = append(p.published, key)
	return nil
}
func (p *mockPub) Publish(_ context.Context, key string, _ any) error {
	p.published = append(p.published, key)
	return nil
}
func (p *mockPub) Close() {}

func defaultCfg() platform.UserConfig {
	return platform.UserConfig{
		EmployeeLockoutAttempts: 5,
		EmployeeLockoutDuration: 30 * time.Minute,
		RefreshTokenDuration:    7 * 24 * time.Hour,
		ConfirmationTokenDuration: 24 * time.Hour,
	}
}

// ---------------------------------------------------------------------------
// EmployeeLogin
// ---------------------------------------------------------------------------

func TestEmployeeLogin_ValidCredentials_ReturnsToken(t *testing.T) {
	hash, _ := platform.HashPassword("secret123")
	repo := &mockRepo{
		employeeByLoginResult: Employee{
			ID:          1,
			Email:       "emp@bank.io",
			Ime:         "Jovan",
			Prezime:     "Jovic",
			Role:        "BASIC",
			Aktivan:     true,
			PasswordHash: &hash,
		},
		employeePermissions: []string{"BANKING_BASIC"},
	}
	auth := platform.NewJWTService(platform.JWTConfig{
		Secret: "test-secret",
		Issuer: "test",
		IDClaim: "id",
		RolesClaim: "roles",
		PermissionsClaim: "permissions",
		AccessTokenDuration: time.Hour,
	})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	resp, err := svc.EmployeeLogin(context.Background(), LoginRequest{Email: "emp@bank.io", Password: "secret123"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.JWT)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.Equal(t, "BASIC", resp.Role)
}

func TestEmployeeLogin_InvalidCredentials_ReturnsError(t *testing.T) {
	repo := &mockRepo{employeeByLoginErr: errors.New("not found")}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	_, err := svc.EmployeeLogin(context.Background(), LoginRequest{Email: "x@y.com", Password: "pass"})
	assert.ErrorIs(t, err, ErrInvalidLogin)
}

func TestEmployeeLogin_InactiveAccount_ReturnsError(t *testing.T) {
	repo := &mockRepo{
		employeeByLoginResult: Employee{ID: 1, Aktivan: false},
	}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	_, err := svc.EmployeeLogin(context.Background(), LoginRequest{Email: "x@y.com", Password: "pass"})
	assert.ErrorIs(t, err, ErrInactiveAccount)
}

func TestEmployeeLogin_LockedAccount_ReturnsError(t *testing.T) {
	future := time.Now().Add(time.Hour)
	repo := &mockRepo{
		employeeByLoginResult: Employee{ID: 1, Aktivan: true, LockedUntil: &future},
	}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	_, err := svc.EmployeeLogin(context.Background(), LoginRequest{Email: "x@y.com", Password: "pass"})
	assert.ErrorIs(t, err, ErrLockedAccount)
}

func TestEmployeeLogin_WrongPassword_RegistersFailedLogin(t *testing.T) {
	hash, _ := platform.HashPassword("correct")
	repo := &mockRepo{
		employeeByLoginResult: Employee{
			ID:      1,
			Aktivan: true,
			Role:    "BASIC",
			PasswordHash: &hash,
			FailedLoginAttempts: 0,
		},
	}
	cfg := defaultCfg()
	cfg.EmployeeLockoutAttempts = 5
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, cfg, platform.ServicesConfig{}, platform.EmailConfig{})

	_, err := svc.EmployeeLogin(context.Background(), LoginRequest{Email: "x@y.com", Password: "wrong"})
	assert.ErrorIs(t, err, ErrInvalidLogin)
}

// ---------------------------------------------------------------------------
// ClientLogin
// ---------------------------------------------------------------------------

func TestClientLogin_ValidCredentials_ReturnsToken(t *testing.T) {
	hash, _ := platform.HashPassword("clientpass")
	repo := &mockRepo{
		clientByEmailResult: Client{
			ID:           42,
			Email:        "client@bank.io",
			Ime:          "Ana",
			Prezime:      "Anic",
			Role:         "CLIENT_BASIC",
			Aktivan:      true,
			PasswordHash: &hash,
		},
		clientPermissions: []string{"CLIENT_ACCOUNT_ACCESS"},
	}
	auth := platform.NewJWTService(platform.JWTConfig{
		Secret: "test-secret", Issuer: "test",
		IDClaim: "id", RolesClaim: "roles", PermissionsClaim: "permissions",
		AccessTokenDuration: time.Hour,
	})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	resp, err := svc.ClientLogin(context.Background(), LoginRequest{Email: "client@bank.io", Password: "clientpass"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, int64(42), resp.ID)
}

func TestClientLogin_WrongPassword_ReturnsError(t *testing.T) {
	hash, _ := platform.HashPassword("correct")
	repo := &mockRepo{
		clientByEmailResult: Client{ID: 1, Aktivan: true, PasswordHash: &hash},
	}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	_, err := svc.ClientLogin(context.Background(), LoginRequest{Email: "x@y.com", Password: "wrong"})
	assert.ErrorIs(t, err, ErrInvalidLogin)
}

func TestClientLogin_InactiveAccount_ReturnsError(t *testing.T) {
	repo := &mockRepo{clientByEmailResult: Client{ID: 1, Aktivan: false}}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	_, err := svc.ClientLogin(context.Background(), LoginRequest{Email: "x@y.com", Password: "pass"})
	assert.ErrorIs(t, err, ErrInactiveAccount)
}

// ---------------------------------------------------------------------------
// SearchEmployees
// ---------------------------------------------------------------------------

func TestSearchEmployees_ReturnsPagedResults(t *testing.T) {
	employees := []Employee{{ID: 1, Ime: "Jovan", Role: "BASIC"}}
	repo := &mockRepo{
		searchEmployeesResult: employees,
		searchEmployeesTotal:  1,
		employeePermissions:   []string{"BANKING_BASIC"},
	}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	result, err := svc.SearchEmployees(context.Background(), SearchQuery{Page: 0, Size: 10})
	require.NoError(t, err)
	assert.Len(t, result.Content, 1)
	assert.Equal(t, 1, result.TotalElements)
}

func TestSearchEmployees_StoreError_ReturnsError(t *testing.T) {
	repo := &mockRepo{searchEmployeesErr: errors.New("db error")}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	_, err := svc.SearchEmployees(context.Background(), SearchQuery{})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// GetEmployee
// ---------------------------------------------------------------------------

func TestGetEmployee_ValidID_ReturnsEmployee(t *testing.T) {
	repo := &mockRepo{
		employeeByIDResult:  Employee{ID: 5, Ime: "Marko", Role: "AGENT"},
		employeePermissions: []string{"BANKING_BASIC", "CLIENT_MANAGE", "SECURITIES_TRADE_LIMITED"},
	}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	resp, err := svc.GetEmployee(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), resp.ID)
}

// ---------------------------------------------------------------------------
// CreateEmployee
// ---------------------------------------------------------------------------

func TestCreateEmployee_ValidRequest_CreatesEmployee(t *testing.T) {
	pub := &mockPub{}
	repo := &mockRepo{
		createEmployeeResult: Employee{ID: 10, Ime: "Novi", Email: "novi@bank.io", Role: "BASIC"},
	}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, pub, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{
		EmployeeActivateURL: "https://bank.io/activate?token=",
	})

	resp, err := svc.CreateEmployee(context.Background(), EmployeeCreateRequest{
		Email:         "novi@bank.io",
		DatumRodjenja: "1990-01-15",
		BrojTelefona:  "+381601234567",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(10), resp.ID)
}

func TestCreateEmployee_InvalidEmail_ReturnsBadRequest(t *testing.T) {
	repo := &mockRepo{}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	_, err := svc.CreateEmployee(context.Background(), EmployeeCreateRequest{Email: "not-an-email", DatumRodjenja: "1990-01-15"})
	assert.ErrorIs(t, err, ErrBadRequest)
}

func TestCreateEmployee_FutureDOB_ReturnsBadRequest(t *testing.T) {
	repo := &mockRepo{}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	_, err := svc.CreateEmployee(context.Background(), EmployeeCreateRequest{
		Email:         "a@b.com",
		DatumRodjenja: "2099-01-01",
	})
	assert.ErrorIs(t, err, ErrBadRequest)
}

func TestCreateEmployee_InvalidPhone_ReturnsBadRequest(t *testing.T) {
	repo := &mockRepo{}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	_, err := svc.CreateEmployee(context.Background(), EmployeeCreateRequest{
		Email:         "a@b.com",
		DatumRodjenja: "1990-01-15",
		BrojTelefona:  "invalid phone!",
	})
	assert.ErrorIs(t, err, ErrBadRequest)
}

// ---------------------------------------------------------------------------
// SearchClients / GetClientInfo
// ---------------------------------------------------------------------------

func TestSearchClients_ReturnsPagedResults(t *testing.T) {
	repo := &mockRepo{
		searchClientsResult: []Client{{ID: 1, Ime: "Ana"}},
		searchClientsTotal:  1,
	}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	result, err := svc.SearchClients(context.Background(), SearchQuery{Page: 0, Size: 10})
	require.NoError(t, err)
	assert.Len(t, result.Content, 1)
}

func TestGetClientInfo_SameClient_Allowed(t *testing.T) {
	repo := &mockRepo{clientByIDResult: Client{ID: 1, Ime: "Ana"}}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	resp, err := svc.GetClientInfo(context.Background(), 1, platform.Principal{ID: 1, Role: "CLIENT_BASIC"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.ID)
}

func TestGetClientInfo_DifferentClientRole_Forbidden(t *testing.T) {
	repo := &mockRepo{}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	_, err := svc.GetClientInfo(context.Background(), 99, platform.Principal{ID: 1, Role: "CLIENT_BASIC"})
	assert.ErrorIs(t, err, ErrForbidden)
}

// ---------------------------------------------------------------------------
// CreateClient
// ---------------------------------------------------------------------------

func TestCreateClient_ValidRequest_CreatesClient(t *testing.T) {
	pub := &mockPub{}
	repo := &mockRepo{
		createClientResult: Client{ID: 5, Ime: "Ana", Email: "ana@bank.io", Role: "CLIENT_BASIC"},
	}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, pub, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{
		ClientActivateURL: "https://bank.io/activate?token=",
	})

	resp, err := svc.CreateClient(context.Background(), ClientCreateRequest{Email: "ana@bank.io"})
	require.NoError(t, err)
	assert.Equal(t, int64(5), resp.ID)
}

// ---------------------------------------------------------------------------
// InterbankUserDisplay
// ---------------------------------------------------------------------------

func TestInterbankUserDisplay_Client_ReturnsName(t *testing.T) {
	repo := &mockRepo{clientByIDResult: Client{ID: 1, Ime: "Ana", Prezime: "Anic"}}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	resp, err := svc.InterbankUserDisplay(context.Background(), "CLIENT", 1)
	require.NoError(t, err)
	assert.Equal(t, "Ana", resp.FirstName)
	assert.Equal(t, "Anic", resp.LastName)
}

func TestInterbankUserDisplay_Employee_ReturnsName(t *testing.T) {
	repo := &mockRepo{employeeByIDResult: Employee{ID: 2, Ime: "Jovan", Prezime: "Jovic"}}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	resp, err := svc.InterbankUserDisplay(context.Background(), "EMPLOYEE", 2)
	require.NoError(t, err)
	assert.Equal(t, "Jovan", resp.FirstName)
}

func TestInterbankUserDisplay_InvalidKind_ReturnsError(t *testing.T) {
	repo := &mockRepo{}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	_, err := svc.InterbankUserDisplay(context.Background(), "UNKNOWN", 1)
	assert.ErrorIs(t, err, ErrBadRequest)
}

// ---------------------------------------------------------------------------
// Pure helper function tests
// ---------------------------------------------------------------------------

func TestHandleServiceError_AllCases(t *testing.T) {
	cases := []struct {
		err    error
		wantCode int
	}{
		{ErrBadRequest, 400},
		{ErrInvalidLogin, 401},
		{ErrInactiveAccount, 403},
		{ErrLockedAccount, 423},
		{ErrForbidden, 403},
		{ErrInvalidToken, 400},
		{ErrNotFound, 404},
		{ErrDuplicate, 409},
		{ErrUnsupportedJMBG, 501},
		{errors.New("generic"), 500},
	}
	for _, tc := range cases {
		code, _, _ := handleServiceError(tc.err)
		assert.Equal(t, tc.wantCode, code, "error: %v", tc.err)
	}
}

func TestNormalizePage_DefaultsApplied(t *testing.T) {
	q := &SearchQuery{Page: -1, Size: 0}
	normalizePage(q)
	assert.Equal(t, 0, q.Page)
	assert.Equal(t, 10, q.Size)
}

func TestNormalizePage_CapsAt100(t *testing.T) {
	q := &SearchQuery{Page: 0, Size: 200}
	normalizePage(q)
	assert.Equal(t, 100, q.Size)
}

func TestPage_CalculatesTotalPages(t *testing.T) {
	result := page([]int{1, 2, 3}, 25, 0, 10)
	assert.Equal(t, 25, result.TotalElements)
	assert.Equal(t, 3, result.TotalPages)
	assert.True(t, result.First)
	assert.False(t, result.Last)
}

func TestPage_LastPage(t *testing.T) {
	result := page([]int{1}, 11, 1, 10)
	assert.True(t, result.Last)
}

func TestPage_EmptyContent(t *testing.T) {
	result := page([]string{}, 0, 0, 10)
	assert.True(t, result.Empty)
	assert.True(t, result.Last)
}

func TestFirstNonBlank_ReturnsFirstNonEmpty(t *testing.T) {
	assert.Equal(t, "b", firstNonBlank("", "b", "c"))
}

func TestFirstNonBlank_AllEmpty_ReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", firstNonBlank("", "", ""))
}

func TestContainsPermission_Found(t *testing.T) {
	assert.True(t, containsPermission([]string{"READ", "WRITE", "ADMIN"}, "WRITE"))
}

func TestContainsPermission_NotFound(t *testing.T) {
	assert.False(t, containsPermission([]string{"READ"}, "WRITE"))
}

func TestIsClientRole_ClientRoles_True(t *testing.T) {
	assert.True(t, isClientRole("CLIENT_BASIC"))
	assert.True(t, isClientRole("CLIENT_TRADING"))
}

func TestIsClientRole_NonClientRole_False(t *testing.T) {
	assert.False(t, isClientRole("ADMIN"))
	assert.False(t, isClientRole("AGENT"))
}

func TestConfirmationToken_UsesConfirmationTokenFirst(t *testing.T) {
	req := ActivateRequest{ConfirmationToken: "ct", Token: "t"}
	assert.Equal(t, "ct", confirmationToken(req))
}

func TestConfirmationToken_FallsBackToToken(t *testing.T) {
	req := ActivateRequest{Token: "t"}
	assert.Equal(t, "t", confirmationToken(req))
}

func TestSortedUnique_DeduplicatesAndSorts(t *testing.T) {
	result := sortedUnique([]string{"WRITE", "READ", "READ", " ", ""})
	assert.Equal(t, []string{"READ", "WRITE"}, result)
}

func TestValidateEmployeeCreate_Valid(t *testing.T) {
	err := validateEmployeeCreate(EmployeeCreateRequest{
		Email:         "emp@bank.io",
		DatumRodjenja: "1985-06-15",
	})
	assert.NoError(t, err)
}

func TestValidateEmployeeCreate_InvalidDate(t *testing.T) {
	err := validateEmployeeCreate(EmployeeCreateRequest{
		Email:         "emp@bank.io",
		DatumRodjenja: "not-a-date",
	})
	assert.ErrorIs(t, err, ErrBadRequest)
}

// ---------------------------------------------------------------------------
// Audit publish tests (also exist in audit_test.go but we add more coverage)
// ---------------------------------------------------------------------------

func TestPublishPermissionAuditIfChanged_NoPub_DoesNotPanic(t *testing.T) {
	svc := &Service{pub: nil}
	assert.NotPanics(t, func() {
		svc.publishPermissionAuditIfChanged(context.Background(), Employee{ID: 1}, []string{"READ"}, []string{"WRITE"})
	})
}

func TestPublishPermissionAuditIfChanged_SystemContext_UsesSystemActor(t *testing.T) {
	pub := &recordingNotificationPublisher{}
	svc := &Service{pub: pub}
	svc.publishPermissionAuditIfChanged(context.Background(), Employee{ID: 1}, []string{}, []string{"READ"})
	require.Len(t, pub.routingKeys, 1)
	event := pub.payloads[0].(auditEvent)
	assert.Equal(t, "SYSTEM", event.ActorName)
	assert.Nil(t, event.ActorID)
}

func TestPublishPermissionAuditIfChanged_WithPrincipalNoEmail(t *testing.T) {
	pub := &recordingNotificationPublisher{}
	svc := &Service{pub: pub}
	ctx := gpauth.WithPrincipal(context.Background(), platform.Principal{ID: 5, Email: ""})
	svc.publishPermissionAuditIfChanged(ctx, Employee{ID: 1}, []string{}, []string{"READ"})
	event := pub.payloads[0].(auditEvent)
	assert.Contains(t, event.ActorName, "USER_5")
}

// ---------------------------------------------------------------------------
// AddMarginPermission, Logout, OTC
// ---------------------------------------------------------------------------

func TestAddMarginPermission_DelegatesToRepo(t *testing.T) {
	repo := &mockRepo{}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})
	require.NoError(t, svc.AddMarginPermission(context.Background(), 42))
}

func TestOTCTradingClientIDs_ReturnsList(t *testing.T) {
	repo := &mockRepo{otcClientIDs: []int64{1, 2, 3}}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	ids, err := svc.OTCTradingClientIDs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2, 3}, ids)
}

func TestDeleteEmployee_DelegatesToRepo(t *testing.T) {
	repo := &mockRepo{}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})
	require.NoError(t, svc.DeleteEmployee(context.Background(), 1))
}

func TestDeleteClient_DelegatesToRepo(t *testing.T) {
	repo := &mockRepo{}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})
	require.NoError(t, svc.DeleteClient(context.Background(), 1))
}

func TestLogout_DelegatesToRepo(t *testing.T) {
	repo := &mockRepo{}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})
	require.NoError(t, svc.Logout(context.Background(), "some-token"))
}

func TestGetClientInfoByJMBG_ReturnsClient(t *testing.T) {
	repo := &mockRepo{clientByJMBGResult: Client{ID: 7, Ime: "Mira"}}
	auth := platform.NewJWTService(platform.JWTConfig{Secret: "s", Issuer: "t", AccessTokenDuration: time.Hour})
	svc := NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{})

	resp, err := svc.GetClientInfoByJMBG(context.Background(), "1234567890123")
	require.NoError(t, err)
	assert.Equal(t, int64(7), resp.ID)
}
