package user

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"banka1/user-service-go/internal/platform"

	gpauth "banka1/go-platform/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testService(repo UserRepo) *Service {
	auth := platform.NewJWTService(platform.JWTConfig{
		Secret:              "test-secret",
		Issuer:              "test",
		IDClaim:             "id",
		RolesClaim:          "roles",
		PermissionsClaim:    "permissions",
		AccessTokenDuration: time.Hour,
	})
	return NewService(repo, auth, &mockPub{}, defaultCfg(), platform.ServicesConfig{}, platform.EmailConfig{
		EmployeeActivateURL:  "https://bank.io/activate?token=",
		ClientActivateURL:    "https://bank.io/client/activate?token=",
	})
}

func testHandlers(repo UserRepo) *Handlers {
	return NewHandlers(testService(repo))
}

func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// ---------------------------------------------------------------------------
// employeeLogin
// ---------------------------------------------------------------------------

func TestEmployeeLoginHandler_ValidCredentials_Returns200(t *testing.T) {
	hash, _ := platform.HashPassword("pass")
	repo := &mockRepo{
		employeeByLoginResult: Employee{
			ID: 1, Email: "emp@bank.io", Ime: "J", Prezime: "J",
			Role: "BASIC", Aktivan: true, PasswordHash: &hash,
		},
		employeePermissions: []string{"BANKING_BASIC"},
	}
	h := testHandlers(repo)

	req := httptest.NewRequest(http.MethodPost, "/employees/auth/login", jsonBody(LoginRequest{Email: "emp@bank.io", Password: "pass"}))
	w := httptest.NewRecorder()
	h.employeeLogin(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEmployeeLoginHandler_InvalidJSON_Returns400(t *testing.T) {
	h := testHandlers(&mockRepo{})
	req := httptest.NewRequest(http.MethodPost, "/employees/auth/login", bytes.NewBufferString("{bad json"))
	w := httptest.NewRecorder()
	h.employeeLogin(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEmployeeLoginHandler_InvalidCredentials_Returns401(t *testing.T) {
	repo := &mockRepo{employeeByLoginErr: ErrInvalidLogin}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodPost, "/employees/auth/login", jsonBody(LoginRequest{Email: "x@y.com", Password: "wrong"}))
	w := httptest.NewRecorder()
	h.employeeLogin(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ---------------------------------------------------------------------------
// clientLogin
// ---------------------------------------------------------------------------

func TestClientLoginHandler_ValidCredentials_Returns200(t *testing.T) {
	hash, _ := platform.HashPassword("cpass")
	repo := &mockRepo{
		clientByEmailResult: Client{
			ID: 5, Email: "c@bank.io", Ime: "A", Prezime: "B",
			Role: "CLIENT_BASIC", Aktivan: true, PasswordHash: &hash,
		},
		clientPermissions: []string{"CLIENT_ACCOUNT_ACCESS"},
	}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodPost, "/clients/auth/login", jsonBody(LoginRequest{Email: "c@bank.io", Password: "cpass"}))
	w := httptest.NewRecorder()
	h.clientLogin(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestClientLoginHandler_InvalidCredentials_Returns401(t *testing.T) {
	repo := &mockRepo{clientByEmailErr: ErrInvalidLogin}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodPost, "/clients/auth/login", jsonBody(LoginRequest{Email: "x@y.com", Password: "w"}))
	w := httptest.NewRecorder()
	h.clientLogin(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ---------------------------------------------------------------------------
// employeeLogout
// ---------------------------------------------------------------------------

func TestEmployeeLogoutHandler_Returns204(t *testing.T) {
	h := testHandlers(&mockRepo{})
	req := httptest.NewRequest(http.MethodDelete, "/employees/auth/logout", jsonBody(LogoutRequest{RefreshToken: "tok"}))
	w := httptest.NewRecorder()
	h.employeeLogout(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// ---------------------------------------------------------------------------
// searchEmployees
// ---------------------------------------------------------------------------

func TestSearchEmployeesHandler_Returns200(t *testing.T) {
	repo := &mockRepo{
		searchEmployeesResult: []Employee{{ID: 1, Ime: "A", Role: "BASIC"}},
		searchEmployeesTotal:  1,
		employeePermissions:   []string{"BANKING_BASIC"},
	}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodGet, "/employees?page=0&size=10", nil)
	w := httptest.NewRecorder()
	h.searchEmployees(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// createEmployee
// ---------------------------------------------------------------------------

func TestCreateEmployeeHandler_ValidRequest_Returns201(t *testing.T) {
	repo := &mockRepo{
		createEmployeeResult: Employee{ID: 10, Ime: "N", Email: "n@bank.io", Role: "BASIC"},
	}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodPost, "/employees", jsonBody(EmployeeCreateRequest{
		Email:         "n@bank.io",
		DatumRodjenja: "1990-01-01",
	}))
	w := httptest.NewRecorder()
	h.createEmployee(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateEmployeeHandler_InvalidEmail_Returns400(t *testing.T) {
	h := testHandlers(&mockRepo{})
	req := httptest.NewRequest(http.MethodPost, "/employees", jsonBody(EmployeeCreateRequest{
		Email:         "not-email",
		DatumRodjenja: "1990-01-01",
	}))
	w := httptest.NewRecorder()
	h.createEmployee(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// getEmployee
// ---------------------------------------------------------------------------

func TestGetEmployeeHandler_ValidID_Returns200(t *testing.T) {
	repo := &mockRepo{
		employeeByIDResult:  Employee{ID: 3, Ime: "M", Role: "AGENT"},
		employeePermissions: []string{"BANKING_BASIC"},
	}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodGet, "/employees/3", nil)
	w := httptest.NewRecorder()
	h.getEmployee(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetEmployeeHandler_InvalidID_Returns400(t *testing.T) {
	h := testHandlers(&mockRepo{})
	req := httptest.NewRequest(http.MethodGet, "/employees/notanid", nil)
	w := httptest.NewRecorder()
	h.getEmployee(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// deleteEmployee
// ---------------------------------------------------------------------------

func TestDeleteEmployeeHandler_ValidID_Returns204(t *testing.T) {
	h := testHandlers(&mockRepo{})
	req := httptest.NewRequest(http.MethodDelete, "/employees/1", nil)
	w := httptest.NewRecorder()
	h.deleteEmployee(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// ---------------------------------------------------------------------------
// updateEmployee
// ---------------------------------------------------------------------------

func TestUpdateEmployeeHandler_ValidRequest_Returns200(t *testing.T) {
	repo := &mockRepo{
		updateEmployeeResult: Employee{ID: 1, Ime: "Updated", Role: "AGENT"},
		employeePermissions:  []string{"BANKING_BASIC"},
	}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodPut, "/employees/1", jsonBody(EmployeeUpdateRequest{}))
	w := httptest.NewRecorder()
	h.updateEmployee(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// editEmployee
// ---------------------------------------------------------------------------

func TestEditEmployeeHandler_WithPrincipal_Returns200(t *testing.T) {
	repo := &mockRepo{
		updateEmployeeResult: Employee{ID: 1, Role: "BASIC"},
		employeePermissions:  []string{"BANKING_BASIC"},
	}
	h := testHandlers(repo)
	ctx := gpauth.WithPrincipal(httptest.NewRequest(http.MethodPut, "/employees/edit", jsonBody(EmployeeUpdateRequest{})).Context(),
		platform.Principal{ID: 1, Role: "BASIC"})
	req := httptest.NewRequest(http.MethodPut, "/employees/edit", jsonBody(EmployeeUpdateRequest{})).WithContext(ctx)
	w := httptest.NewRecorder()
	h.editEmployee(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEditEmployeeHandler_NoPrincipal_Returns401(t *testing.T) {
	h := testHandlers(&mockRepo{})
	req := httptest.NewRequest(http.MethodPut, "/employees/edit", jsonBody(EmployeeUpdateRequest{}))
	w := httptest.NewRecorder()
	h.editEmployee(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ---------------------------------------------------------------------------
// searchClients / createClient / deleteClient
// ---------------------------------------------------------------------------

func TestSearchClientsHandler_Returns200(t *testing.T) {
	repo := &mockRepo{searchClientsResult: []Client{{ID: 1}}, searchClientsTotal: 1}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodGet, "/clients/customers?page=0&size=10", nil)
	w := httptest.NewRecorder()
	h.searchClients(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateClientHandler_ValidRequest_Returns201(t *testing.T) {
	repo := &mockRepo{createClientResult: Client{ID: 5, Ime: "Ana"}}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodPost, "/clients/customers", jsonBody(ClientCreateRequest{Email: "ana@bank.io"}))
	w := httptest.NewRecorder()
	h.createClient(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestDeleteClientHandler_ValidID_Returns204(t *testing.T) {
	h := testHandlers(&mockRepo{})
	req := httptest.NewRequest(http.MethodDelete, "/clients/customers/1", nil)
	w := httptest.NewRecorder()
	h.deleteClient(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteClientHandler_InvalidID_Returns400(t *testing.T) {
	h := testHandlers(&mockRepo{})
	req := httptest.NewRequest(http.MethodDelete, "/clients/customers/notanid", nil)
	w := httptest.NewRecorder()
	h.deleteClient(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// addMarginPermission
// ---------------------------------------------------------------------------

func TestAddMarginPermissionHandler_ValidID_Returns200(t *testing.T) {
	h := testHandlers(&mockRepo{})
	req := httptest.NewRequest(http.MethodPut, "/clients/customers/margin/42", nil)
	w := httptest.NewRecorder()
	h.addMarginPermission(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// interbankUserDisplay
// ---------------------------------------------------------------------------

func TestInterbankUserDisplayHandler_ValidClientPath_Returns200(t *testing.T) {
	repo := &mockRepo{clientByIDResult: Client{ID: 1, Ime: "Ana", Prezime: "Anic"}}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodGet, "/internal/interbank/user/CLIENT/1", nil)
	w := httptest.NewRecorder()
	h.interbankUserDisplay(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInterbankUserDisplayHandler_InvalidPath_Returns400(t *testing.T) {
	h := testHandlers(&mockRepo{})
	req := httptest.NewRequest(http.MethodGet, "/internal/interbank/user/CLIENT", nil)
	w := httptest.NewRecorder()
	h.interbankUserDisplay(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInterbankUserDisplayHandler_InvalidID_Returns400(t *testing.T) {
	h := testHandlers(&mockRepo{})
	req := httptest.NewRequest(http.MethodGet, "/internal/interbank/user/CLIENT/notanid", nil)
	w := httptest.NewRecorder()
	h.interbankUserDisplay(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// actuaryClientIDs
// ---------------------------------------------------------------------------

func TestActuaryClientIDsHandler_Returns200(t *testing.T) {
	repo := &mockRepo{otcClientIDs: []int64{1, 2, 3}}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodGet, "/internal/otc/actuary-client-ids", nil)
	w := httptest.NewRecorder()
	h.actuaryClientIDs(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// clientCheckActivate / employeeCheckActivate
// ---------------------------------------------------------------------------

func TestClientCheckActivateHandler_Returns200(t *testing.T) {
	repo := &mockRepo{confirmationIDResult: 5}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodGet, "/clients/auth/check-activate?token=sometoken", nil)
	w := httptest.NewRecorder()
	h.clientCheckActivate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEmployeeCheckActivateHandler_Returns200(t *testing.T) {
	repo := &mockRepo{confirmationIDResult: 7}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodGet, "/employees/auth/checkActivate?confirmationToken=sometoken", nil)
	w := httptest.NewRecorder()
	h.employeeCheckActivate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// getClientByJMBG
// ---------------------------------------------------------------------------

func TestGetClientByJMBGHandler_Returns200(t *testing.T) {
	repo := &mockRepo{clientByJMBGResult: Client{ID: 1, Ime: "Mira"}}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodGet, "/clients/customers/jmbg/1234567890123", nil)
	w := httptest.NewRecorder()
	h.getClientByJMBG(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// updateClient / getClientByID
// ---------------------------------------------------------------------------

func TestUpdateClientHandler_ValidRequest_Returns200(t *testing.T) {
	repo := &mockRepo{}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodPut, "/clients/customers/1", jsonBody(ClientUpdateRequest{}))
	w := httptest.NewRecorder()
	h.updateClient(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetClientByIDHandler_Returns200(t *testing.T) {
	repo := &mockRepo{clientByIDResult: Client{ID: 1, Ime: "Ana"}}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodGet, "/clients/customers/1", nil)
	w := httptest.NewRecorder()
	h.getClientByID(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// searchQuery / intParam helpers
// ---------------------------------------------------------------------------

func TestSearchQuery_ParsesQueryParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/employees?ime=Jovan&prezime=Jovic&page=2&size=20", nil)
	q := searchQuery(req)
	assert.Equal(t, "Jovan", q.Ime)
	assert.Equal(t, "Jovic", q.Prezime)
	assert.Equal(t, 2, q.Page)
	assert.Equal(t, 20, q.Size)
}

func TestIntParam_InvalidValue_ReturnsDefault(t *testing.T) {
	assert.Equal(t, 5, intParam("notanumber", 5))
}

func TestIntParam_EmptyValue_ReturnsDefault(t *testing.T) {
	assert.Equal(t, 10, intParam("", 10))
}

func TestIntParam_ValidValue_ReturnsParsed(t *testing.T) {
	assert.Equal(t, 42, intParam("42", 0))
}

// ---------------------------------------------------------------------------
// forgotPassword / resendActivation
// ---------------------------------------------------------------------------

func TestEmployeeForgotPasswordHandler_Returns202(t *testing.T) {
	repo := &mockRepo{
		employeeByLoginResult: Employee{ID: 1, Aktivan: true, Email: "e@bank.io"},
	}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodPost, "/employees/auth/forgot-password", jsonBody(EmailRequest{Email: "e@bank.io"}))
	w := httptest.NewRecorder()
	h.employeeForgotPassword(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)
}

func TestClientForgotPasswordHandler_Returns200(t *testing.T) {
	repo := &mockRepo{
		clientByEmailResult: Client{ID: 1, Aktivan: true, Email: "c@bank.io"},
	}
	h := testHandlers(repo)
	req := httptest.NewRequest(http.MethodPost, "/clients/auth/forgot-password", jsonBody(EmailRequest{Email: "c@bank.io"}))
	w := httptest.NewRecorder()
	h.clientForgotPassword(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
