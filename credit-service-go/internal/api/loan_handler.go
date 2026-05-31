package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"Banka1Back/credit-service-go/internal/auth"
	"Banka1Back/credit-service-go/internal/dto"
	"Banka1Back/credit-service-go/internal/model"
	"Banka1Back/credit-service-go/internal/service"
)

type LoanHandler struct {
	loanService *service.LoanService
}

func NewLoanHandler(loanService *service.LoanService) *LoanHandler {
	return &LoanHandler{loanService: loanService}
}

func (h *LoanHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (h *LoanHandler) CreateLoanRequest(w http.ResponseWriter, r *http.Request) {
	var request dto.LoanRequestDTO

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Neispravni podaci", "Request body nije validan JSON.")
		return
	}

	validationErrors := validateLoanRequest(request)
	if len(validationErrors) > 0 {
		writeValidationError(w, validationErrors)
		return
	}

	currentUser, err := auth.ParseBearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Neautorizovan pristup", "Korisnik nije autentifikovan.")
		return
	}

	if !currentUser.HasRole("CLIENT_BASIC") {
		writeError(w, http.StatusForbidden, "ERR_FORBIDDEN", "Zabranjen pristup", "Nemate dozvolu za ovu metodu.")
		return
	}

	response, err := h.loanService.Request(r.Context(), currentUser, request)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ERR_INTERNAL_SERVER", "Serverska greška", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, response)
}

func (h *LoanHandler) FindAllLoanRequests(w http.ResponseWriter, r *http.Request) {
	currentUser, err := auth.ParseBearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Neautorizovan pristup", "Korisnik nije autentifikovan.")
		return
	}

	if !currentUser.HasRole("BASIC") {
		writeError(w, http.StatusForbidden, "ERR_FORBIDDEN", "Zabranjen pristup", "Nemate dozvolu za ovu metodu.")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))

	if size <= 0 {
		size = 10
	}
	if page < 0 {
		page = 0
	}

	var loanType *model.LoanType
	if value := r.URL.Query().Get("vrstaKredita"); value != "" {
		parsed := model.LoanType(value)
		loanType = &parsed
	}

	var accountNumber *string
	if value := r.URL.Query().Get("brojRacuna"); value != "" {
		accountNumber = &value
	}

	response, err := h.loanService.FindAllLoanRequests(r.Context(), loanType, accountNumber, page, size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ERR_INTERNAL_SERVER", "Serverska greška", "Došlo je do neočekivanog problema. Naš tim je obavešten.")
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *LoanHandler) ConfirmLoanRequest(w http.ResponseWriter, r *http.Request) {
	currentUser, err := auth.ParseBearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Neautorizovan pristup", "Korisnik nije autentifikovan.")
		return
	}

	if !currentUser.HasRole("BASIC") {
		writeError(w, http.StatusForbidden, "ERR_FORBIDDEN", "Zabranjen pristup", "Nemate dozvolu za ovu metodu.")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/loans/requests/")
	parts := strings.Split(path, "/")

	if len(parts) != 2 {
		writeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Neispravni argumenti", "Putanja zahteva nije validna.")
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Neispravni argumenti", "ID zahteva za kredit nije validan.")
		return
	}

	var status model.Status
	switch parts[1] {
	case "approve":
		status = model.StatusApproved
	case "decline":
		status = model.StatusDeclined
	default:
		writeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Neispravni argumenti", "Akcija mora biti approve ili decline.")
		return
	}

	message, err := h.loanService.Confirmation(r.Context(), id, status)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Neispravni argumenti", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": message,
	})
}

func (h *LoanHandler) FindClientLoans(w http.ResponseWriter, r *http.Request) {
	currentUser, err := auth.ParseBearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Neautorizovan pristup", "Korisnik nije autentifikovan.")
		return
	}

	if !currentUser.HasRole("CLIENT_BASIC") {
		writeError(w, http.StatusForbidden, "ERR_FORBIDDEN", "Zabranjen pristup", "Nemate dozvolu za ovu metodu.")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))

	if size <= 0 {
		size = 10
	}
	if page < 0 {
		page = 0
	}

	response, err := h.loanService.FindClientLoans(r.Context(), currentUser, page, size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ERR_INTERNAL_SERVER", "Serverska greška", "Došlo je do neočekivanog problema. Naš tim je obavešten.")
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *LoanHandler) GetLoanInfo(w http.ResponseWriter, r *http.Request) {
	currentUser, err := auth.ParseBearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Neautorizovan pristup", "Korisnik nije autentifikovan.")
		return
	}

	if !currentUser.HasAnyRole("CLIENT_BASIC", "BASIC") {
		writeError(w, http.StatusForbidden, "ERR_FORBIDDEN", "Zabranjen pristup", "Nemate dozvolu za ovu metodu.")
		return
	}

	idPart := strings.TrimPrefix(r.URL.Path, "/api/loans/")
	loanID, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Neispravni argumenti", "Broj kredita nije validan.")
		return
	}

	response, err := h.loanService.GetLoanInfo(r.Context(), currentUser, loanID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ERR_VALIDATION", "Neispravni argumenti", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *LoanHandler) FindAllLoans(w http.ResponseWriter, r *http.Request) {
	currentUser, err := auth.ParseBearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Neautorizovan pristup", "Korisnik nije autentifikovan.")
		return
	}

	if !currentUser.HasRole("BASIC") {
		writeError(w, http.StatusForbidden, "ERR_FORBIDDEN", "Zabranjen pristup", "Nemate dozvolu za ovu metodu.")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))

	if size <= 0 {
		size = 10
	}
	if page < 0 {
		page = 0
	}

	var loanType *model.LoanType
	if value := r.URL.Query().Get("vrstaKredita"); value != "" {
		parsed := model.LoanType(value)
		loanType = &parsed
	}

	var accountNumber *string
	if value := r.URL.Query().Get("brojRacuna"); value != "" {
		accountNumber = &value
	}

	var status *model.Status
	if value := r.URL.Query().Get("loanStatus"); value != "" {
		parsed := model.Status(value)
		status = &parsed
	}

	response, err := h.loanService.FindAllLoans(r.Context(), loanType, accountNumber, status, page, size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ERR_INTERNAL_SERVER", "Serverska greška", "Došlo je do neočekivanog problema. Naš tim je obavešten.")
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code string, title string, desc string) {
	writeJSON(w, status, dto.NewErrorResponse(code, title, desc))
}

func writeValidationError(w http.ResponseWriter, validationErrors map[string]string) {
	writeJSON(w, http.StatusBadRequest, dto.NewValidationErrorResponse(validationErrors))
}
