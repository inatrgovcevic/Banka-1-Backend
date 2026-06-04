package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"Banka1Back/credit-service-go/internal/api"
	"Banka1Back/credit-service-go/internal/auth"
	"Banka1Back/credit-service-go/internal/dto"
	"Banka1Back/credit-service-go/internal/model"

	"github.com/golang-jwt/jwt/v5"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Stub service
// ---------------------------------------------------------------------------

type stubCreditService struct {
	requestResult      dto.LoanRequestResponseDTO
	requestErr         error
	allRequestsResult  dto.PageResponse[model.LoanRequest]
	allRequestsErr     error
	confirmResult      string
	confirmErr         error
	clientLoansResult  dto.PageResponse[dto.LoanResponseDTO]
	clientLoansErr     error
	loanInfoResult     dto.LoanInfoResponseDTO
	loanInfoErr        error
	allLoansResult     dto.PageResponse[dto.LoanResponseDTO]
	allLoansErr        error
}

func (s *stubCreditService) Request(_ context.Context, _ auth.User, _ dto.LoanRequestDTO) (dto.LoanRequestResponseDTO, error) {
	return s.requestResult, s.requestErr
}

func (s *stubCreditService) FindAllLoanRequests(_ context.Context, _ *model.LoanType, _ *string, _ int, _ int) (dto.PageResponse[model.LoanRequest], error) {
	return s.allRequestsResult, s.allRequestsErr
}

func (s *stubCreditService) Confirmation(_ context.Context, _ int64, _ model.Status) (string, error) {
	return s.confirmResult, s.confirmErr
}

func (s *stubCreditService) FindClientLoans(_ context.Context, _ auth.User, _ int, _ int) (dto.PageResponse[dto.LoanResponseDTO], error) {
	return s.clientLoansResult, s.clientLoansErr
}

func (s *stubCreditService) GetLoanInfo(_ context.Context, _ auth.User, _ int64) (dto.LoanInfoResponseDTO, error) {
	return s.loanInfoResult, s.loanInfoErr
}

func (s *stubCreditService) FindAllLoans(_ context.Context, _ *model.LoanType, _ *string, _ *model.Status, _ int, _ int) (dto.PageResponse[dto.LoanResponseDTO], error) {
	return s.allLoansResult, s.allLoansErr
}

// ---------------------------------------------------------------------------
// JWT helper
// ---------------------------------------------------------------------------

const testSecret = "test-secret-key"

func makeToken(id int64, role string) string {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":       float64(id),
		"sub":      "user@bank.io",
		"username": "testuser",
		"roles":    role,
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	tok, _ := t.SignedString([]byte(testSecret))
	return "Bearer " + tok
}

func withSecret(t *testing.T) {
	t.Helper()
	os.Setenv("JWT_SECRET", testSecret)
	t.Cleanup(func() { os.Unsetenv("JWT_SECRET") })
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func TestHealth_Returns200OK(t *testing.T) {
	t.Parallel()
	h := api.NewLoanHandler(&stubCreditService{})
	w := httptest.NewRecorder()
	h.Health(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// CreateLoanRequest
// ---------------------------------------------------------------------------

func TestCreateLoanRequest_ValidRequest_Returns201(t *testing.T) {
	withSecret(t)
	svc := &stubCreditService{
		requestResult: dto.LoanRequestResponseDTO{ID: 1, CreatedAt: time.Now()},
	}
	h := api.NewLoanHandler(svc)

	body := dto.LoanRequestDTO{
		LoanType:                model.LoanGotovinski,
		InterestType:            model.InterestFixed,
		Amount:                  decimal.NewFromInt(50000),
		Currency:                model.CurrencyRSD,
		Purpose:                 "house",
		MonthlySalary:           decimal.NewFromInt(1000),
		EmploymentStatus:        model.EmploymentPermanent,
		CurrentEmploymentPeriod: 12,
		RepaymentPeriod:         24,
		ContactPhone:            "060123456",
		AccountNumber:           "1234567890123456789",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/loans/requests", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", makeToken(1, "CLIENT_BASIC"))
	w := httptest.NewRecorder()

	h.CreateLoanRequest(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateLoanRequest_InvalidJSON_Returns400(t *testing.T) {
	t.Parallel()
	h := api.NewLoanHandler(&stubCreditService{})
	req := httptest.NewRequest(http.MethodPost, "/api/loans/requests", bytes.NewBufferString("{invalid}"))
	w := httptest.NewRecorder()
	h.CreateLoanRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateLoanRequest_ValidationFails_Returns400(t *testing.T) {
	withSecret(t)
	h := api.NewLoanHandler(&stubCreditService{})

	body := dto.LoanRequestDTO{} // all fields empty/zero → validation errors
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/loans/requests", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", makeToken(1, "CLIENT_BASIC"))
	w := httptest.NewRecorder()
	h.CreateLoanRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateLoanRequest_NoToken_Returns401(t *testing.T) {
	t.Parallel()
	h := api.NewLoanHandler(&stubCreditService{})
	body := validBody()
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/loans/requests", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()
	h.CreateLoanRequest(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateLoanRequest_WrongRole_Returns403(t *testing.T) {
	withSecret(t)
	h := api.NewLoanHandler(&stubCreditService{})
	body := validBody()
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/loans/requests", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", makeToken(1, "AGENT"))
	w := httptest.NewRecorder()
	h.CreateLoanRequest(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func validBody() dto.LoanRequestDTO {
	return dto.LoanRequestDTO{
		LoanType:                model.LoanGotovinski,
		InterestType:            model.InterestFixed,
		Amount:                  decimal.NewFromInt(50000),
		Currency:                model.CurrencyRSD,
		Purpose:                 "house",
		MonthlySalary:           decimal.NewFromInt(1000),
		EmploymentStatus:        model.EmploymentPermanent,
		CurrentEmploymentPeriod: 12,
		RepaymentPeriod:         24,
		ContactPhone:            "060123456",
		AccountNumber:           "1234567890123456789",
	}
}

// ---------------------------------------------------------------------------
// FindAllLoanRequests
// ---------------------------------------------------------------------------

func TestFindAllLoanRequests_WithValidAdmin_Returns200(t *testing.T) {
	withSecret(t)
	svc := &stubCreditService{
		allRequestsResult: dto.PageResponse[model.LoanRequest]{Content: []model.LoanRequest{{}}, TotalElements: 1},
	}
	h := api.NewLoanHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/loans/requests?page=0&size=10", nil)
	req.Header.Set("Authorization", makeToken(1, "ADMIN"))
	w := httptest.NewRecorder()
	h.FindAllLoanRequests(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFindAllLoanRequests_WithLoanTypeFilter_Returns200(t *testing.T) {
	withSecret(t)
	svc := &stubCreditService{}
	h := api.NewLoanHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/loans/requests?vrstaKredita=GOTOVINSKI&brojRacuna=123", nil)
	req.Header.Set("Authorization", makeToken(1, "SUPERVISOR"))
	w := httptest.NewRecorder()
	h.FindAllLoanRequests(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFindAllLoanRequests_NoToken_Returns401(t *testing.T) {
	t.Parallel()
	h := api.NewLoanHandler(&stubCreditService{})
	w := httptest.NewRecorder()
	h.FindAllLoanRequests(w, httptest.NewRequest(http.MethodGet, "/api/loans/requests", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestFindAllLoanRequests_WrongRole_Returns403(t *testing.T) {
	withSecret(t)
	h := api.NewLoanHandler(&stubCreditService{})
	req := httptest.NewRequest(http.MethodGet, "/api/loans/requests", nil)
	req.Header.Set("Authorization", makeToken(1, "CLIENT_BASIC"))
	w := httptest.NewRecorder()
	h.FindAllLoanRequests(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ---------------------------------------------------------------------------
// ConfirmLoanRequest
// ---------------------------------------------------------------------------

func TestConfirmLoanRequest_Approve_Returns200(t *testing.T) {
	withSecret(t)
	svc := &stubCreditService{confirmResult: "ODOBREN ZAHTEV"}
	h := api.NewLoanHandler(svc)
	req := httptest.NewRequest(http.MethodPut, "/api/loans/requests/1/approve", nil)
	req.Header.Set("Authorization", makeToken(1, "ADMIN"))
	w := httptest.NewRecorder()
	h.ConfirmLoanRequest(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestConfirmLoanRequest_Decline_Returns200(t *testing.T) {
	withSecret(t)
	svc := &stubCreditService{confirmResult: "ODBIJEN ZAHTEV"}
	h := api.NewLoanHandler(svc)
	req := httptest.NewRequest(http.MethodPut, "/api/loans/requests/1/decline", nil)
	req.Header.Set("Authorization", makeToken(1, "ADMIN"))
	w := httptest.NewRecorder()
	h.ConfirmLoanRequest(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestConfirmLoanRequest_InvalidAction_Returns400(t *testing.T) {
	withSecret(t)
	h := api.NewLoanHandler(&stubCreditService{})
	req := httptest.NewRequest(http.MethodPut, "/api/loans/requests/1/cancel", nil)
	req.Header.Set("Authorization", makeToken(1, "ADMIN"))
	w := httptest.NewRecorder()
	h.ConfirmLoanRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestConfirmLoanRequest_InvalidID_Returns400(t *testing.T) {
	withSecret(t)
	h := api.NewLoanHandler(&stubCreditService{})
	req := httptest.NewRequest(http.MethodPut, "/api/loans/requests/notanumber/approve", nil)
	req.Header.Set("Authorization", makeToken(1, "ADMIN"))
	w := httptest.NewRecorder()
	h.ConfirmLoanRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestConfirmLoanRequest_NoToken_Returns401(t *testing.T) {
	t.Parallel()
	h := api.NewLoanHandler(&stubCreditService{})
	w := httptest.NewRecorder()
	h.ConfirmLoanRequest(w, httptest.NewRequest(http.MethodPut, "/api/loans/requests/1/approve", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestConfirmLoanRequest_WrongRole_Returns403(t *testing.T) {
	withSecret(t)
	h := api.NewLoanHandler(&stubCreditService{})
	req := httptest.NewRequest(http.MethodPut, "/api/loans/requests/1/approve", nil)
	req.Header.Set("Authorization", makeToken(1, "CLIENT_BASIC"))
	w := httptest.NewRecorder()
	h.ConfirmLoanRequest(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestConfirmLoanRequest_InvalidPathSegments_Returns400(t *testing.T) {
	withSecret(t)
	h := api.NewLoanHandler(&stubCreditService{})
	req := httptest.NewRequest(http.MethodPut, "/api/loans/requests/1", nil)
	req.Header.Set("Authorization", makeToken(1, "ADMIN"))
	w := httptest.NewRecorder()
	h.ConfirmLoanRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// FindClientLoans
// ---------------------------------------------------------------------------

func TestFindClientLoans_ValidClient_Returns200(t *testing.T) {
	withSecret(t)
	svc := &stubCreditService{
		clientLoansResult: dto.PageResponse[dto.LoanResponseDTO]{Content: []dto.LoanResponseDTO{{}}},
	}
	h := api.NewLoanHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/loans/client?page=0&size=10", nil)
	req.Header.Set("Authorization", makeToken(1, "CLIENT_BASIC"))
	w := httptest.NewRecorder()
	h.FindClientLoans(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFindClientLoans_NoToken_Returns401(t *testing.T) {
	t.Parallel()
	h := api.NewLoanHandler(&stubCreditService{})
	w := httptest.NewRecorder()
	h.FindClientLoans(w, httptest.NewRequest(http.MethodGet, "/api/loans/client", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestFindClientLoans_WrongRole_Returns403(t *testing.T) {
	withSecret(t)
	h := api.NewLoanHandler(&stubCreditService{})
	req := httptest.NewRequest(http.MethodGet, "/api/loans/client", nil)
	req.Header.Set("Authorization", makeToken(1, "AGENT"))
	w := httptest.NewRecorder()
	h.FindClientLoans(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ---------------------------------------------------------------------------
// GetLoanInfo
// ---------------------------------------------------------------------------

func TestGetLoanInfo_ValidRequest_Returns200(t *testing.T) {
	withSecret(t)
	svc := &stubCreditService{
		loanInfoResult: dto.LoanInfoResponseDTO{Loan: dto.LoanResponseDTO{LoanNumber: 1}},
	}
	h := api.NewLoanHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/loans/5", nil)
	req.Header.Set("Authorization", makeToken(1, "CLIENT_BASIC"))
	w := httptest.NewRecorder()
	h.GetLoanInfo(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetLoanInfo_InvalidID_Returns400(t *testing.T) {
	withSecret(t)
	h := api.NewLoanHandler(&stubCreditService{})
	req := httptest.NewRequest(http.MethodGet, "/api/loans/notanumber", nil)
	req.Header.Set("Authorization", makeToken(1, "CLIENT_BASIC"))
	w := httptest.NewRecorder()
	h.GetLoanInfo(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetLoanInfo_NoToken_Returns401(t *testing.T) {
	t.Parallel()
	h := api.NewLoanHandler(&stubCreditService{})
	w := httptest.NewRecorder()
	h.GetLoanInfo(w, httptest.NewRequest(http.MethodGet, "/api/loans/1", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ---------------------------------------------------------------------------
// FindAllLoans
// ---------------------------------------------------------------------------

func TestFindAllLoans_ValidAdmin_Returns200(t *testing.T) {
	withSecret(t)
	svc := &stubCreditService{
		allLoansResult: dto.PageResponse[dto.LoanResponseDTO]{Content: []dto.LoanResponseDTO{}},
	}
	h := api.NewLoanHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/loans/all?vrstaKredita=AUTO&loanStatus=ACTIVE", nil)
	req.Header.Set("Authorization", makeToken(1, "ADMIN"))
	w := httptest.NewRecorder()
	h.FindAllLoans(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFindAllLoans_DefaultPagination(t *testing.T) {
	withSecret(t)
	svc := &stubCreditService{}
	h := api.NewLoanHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/loans/all", nil)
	req.Header.Set("Authorization", makeToken(1, "BASIC"))
	w := httptest.NewRecorder()
	h.FindAllLoans(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFindAllLoans_NoToken_Returns401(t *testing.T) {
	t.Parallel()
	h := api.NewLoanHandler(&stubCreditService{})
	w := httptest.NewRecorder()
	h.FindAllLoans(w, httptest.NewRequest(http.MethodGet, "/api/loans/all", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestFindAllLoans_WrongRole_Returns403(t *testing.T) {
	withSecret(t)
	h := api.NewLoanHandler(&stubCreditService{})
	req := httptest.NewRequest(http.MethodGet, "/api/loans/all", nil)
	req.Header.Set("Authorization", makeToken(1, "CLIENT_BASIC"))
	w := httptest.NewRecorder()
	h.FindAllLoans(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ---------------------------------------------------------------------------
// writeJSON / writeError helpers (tested via handler responses)
// ---------------------------------------------------------------------------

func TestWriteJSON_SetsContentTypeJSON(t *testing.T) {
	withSecret(t)
	svc := &stubCreditService{}
	h := api.NewLoanHandler(svc)
	w := httptest.NewRecorder()
	h.Health(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	// Health doesn't set JSON content-type, but any 200 success handler does
	// Test via FindAllLoans which calls writeJSON
	req := httptest.NewRequest(http.MethodGet, "/api/loans/all", nil)
	req.Header.Set("Authorization", makeToken(1, "ADMIN"))
	w2 := httptest.NewRecorder()
	h.FindAllLoans(w2, req)
	assert.Equal(t, "application/json", w2.Header().Get("Content-Type"))
}

func TestCreateLoanRequest_ServiceError_Returns500(t *testing.T) {
	withSecret(t)
	svc := &stubCreditService{requestErr: assert.AnError}
	h := api.NewLoanHandler(svc)
	body := validBody()
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/loans/requests", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", makeToken(1, "CLIENT_BASIC"))
	w := httptest.NewRecorder()
	h.CreateLoanRequest(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetLoanInfo_WrongRole_Returns403(t *testing.T) {
	withSecret(t)
	h := api.NewLoanHandler(&stubCreditService{})
	req := httptest.NewRequest(http.MethodGet, "/api/loans/1", nil)
	req.Header.Set("Authorization", makeToken(1, "AGENT"))
	w := httptest.NewRecorder()
	h.GetLoanInfo(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestConfirmLoanRequest_ServiceError_Returns400(t *testing.T) {
	withSecret(t)
	svc := &stubCreditService{confirmErr: assert.AnError}
	h := api.NewLoanHandler(svc)
	req := httptest.NewRequest(http.MethodPut, "/api/loans/requests/1/approve", nil)
	req.Header.Set("Authorization", makeToken(1, "ADMIN"))
	w := httptest.NewRecorder()
	h.ConfirmLoanRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFindClientLoans_ServiceError_Returns500(t *testing.T) {
	withSecret(t)
	svc := &stubCreditService{clientLoansErr: assert.AnError}
	h := api.NewLoanHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/loans/client", nil)
	req.Header.Set("Authorization", makeToken(1, "CLIENT_TRADING"))
	w := httptest.NewRecorder()
	h.FindClientLoans(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
