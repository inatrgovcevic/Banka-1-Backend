package user

import (
	"net/http"
	"strconv"
	"strings"

	"banka1/user-service-go/internal/platform"
)

type Handlers struct {
	service *Service
}

func NewHandlers(service *Service) *Handlers {
	return &Handlers{service: service}
}

func (h *Handlers) RegisterRoutes(m *platform.Mux, auth *platform.JWTService) {
	m.HandleFunc(http.MethodPost, "/employees/auth/login", h.employeeLogin)
	m.HandleFunc(http.MethodPost, "/employees/auth/refresh", h.employeeRefresh)
	m.HandleFunc(http.MethodGet, "/employees/auth/checkActivate", h.employeeCheckActivate)
	m.HandleFunc(http.MethodGet, "/employees/auth/checkResetPassword", h.employeeCheckActivate)
	m.HandleFunc(http.MethodPost, "/employees/auth/activate", h.employeeActivate)
	m.HandleFunc(http.MethodPost, "/employees/auth/resetPassword", h.employeeActivate)
	m.HandleFunc(http.MethodDelete, "/employees/auth/logout", h.employeeLogout)
	m.HandleFunc(http.MethodPost, "/employees/auth/forgot-password", h.employeeForgotPassword)
	m.HandleFunc(http.MethodPost, "/employees/auth/resend-activation", h.employeeResendActivation)

	m.Handle(http.MethodPost, "/employees", auth.Middleware(platform.RequireAnyRole("ADMIN")(http.HandlerFunc(h.createEmployee))))
	m.Handle(http.MethodGet, "/employees", auth.Middleware(platform.RequireAnyRole("BASIC", "ADMIN", "SUPERVISOR", "AGENT")(http.HandlerFunc(h.searchEmployees))))
	m.Handle(http.MethodGet, "/employees/search", auth.Middleware(platform.RequireAnyRole("BASIC", "ADMIN", "SUPERVISOR", "AGENT")(http.HandlerFunc(h.searchEmployees))))
	m.Handle(http.MethodPut, "/employees/edit", auth.Middleware(http.HandlerFunc(h.editEmployee)))
	m.Handle(http.MethodGet, "/employees/", auth.Middleware(platform.RequireAnyRole("BASIC", "ADMIN", "SUPERVISOR", "AGENT", "SERVICE")(http.HandlerFunc(h.getEmployee))))
	m.Handle(http.MethodPut, "/employees/", auth.Middleware(platform.RequireAnyRole("AGENT", "ADMIN", "SUPERVISOR")(http.HandlerFunc(h.updateEmployee))))
	m.Handle(http.MethodDelete, "/employees/", auth.Middleware(platform.RequireAnyRole("ADMIN")(http.HandlerFunc(h.deleteEmployee))))

	m.HandleFunc(http.MethodPost, "/clients/auth/login", h.clientLogin)
	m.HandleFunc(http.MethodGet, "/clients/auth/check-activate", h.clientCheckActivate)
	m.HandleFunc(http.MethodPost, "/clients/auth/activate", h.clientActivate)
	m.HandleFunc(http.MethodPost, "/clients/auth/reset-password", h.clientActivate)
	m.HandleFunc(http.MethodPost, "/clients/auth/forgot-password", h.clientForgotPassword)
	m.HandleFunc(http.MethodPost, "/clients/auth/resend-activation", h.clientResendActivation)

	m.Handle(http.MethodPost, "/clients/customers", auth.Middleware(platform.RequireAnyRole("AGENT", "ADMIN", "SUPERVISOR")(http.HandlerFunc(h.createClient))))
	m.Handle(http.MethodGet, "/clients/customers", auth.Middleware(platform.RequireAnyRole("AGENT", "SUPERVISOR", "ADMIN", "SERVICE")(http.HandlerFunc(h.searchClients))))
	m.Handle(http.MethodGet, "/clients/customers/search", auth.Middleware(platform.RequireAnyRole("AGENT", "ADMIN", "SUPERVISOR")(http.HandlerFunc(h.searchClients))))
	m.Handle(http.MethodPut, "/clients/customers/margin/", auth.Middleware(platform.RequireAnyRole("SERVICE")(http.HandlerFunc(h.addMarginPermission))))
	m.Handle(http.MethodGet, "/clients/customers/jmbg/", auth.Middleware(platform.RequireAnyRole("SERVICE", "ADMIN")(http.HandlerFunc(h.getClientByJMBG))))
	m.Handle(http.MethodGet, "/clients/customers/", auth.Middleware(platform.RequireAnyRole("SERVICE", "ADMIN", "CLIENT_BASIC", "CLIENT_TRADING")(http.HandlerFunc(h.getClientByID))))
	m.Handle(http.MethodPut, "/clients/customers/", auth.Middleware(platform.RequireAnyRole("AGENT", "ADMIN", "SUPERVISOR")(http.HandlerFunc(h.updateClient))))
	m.Handle(http.MethodDelete, "/clients/customers/", auth.Middleware(platform.RequireAnyRole("ADMIN")(http.HandlerFunc(h.deleteClient))))

	m.Handle(http.MethodGet, "/internal/interbank/user/", auth.Middleware(platform.RequireAnyRole("SERVICE")(http.HandlerFunc(h.interbankUserDisplay))))
	m.Handle(http.MethodGet, "/internal/otc/actuary-client-ids", auth.Middleware(platform.RequireAnyRole("SERVICE", "AGENT", "SUPERVISOR", "ADMIN")(http.HandlerFunc(h.actuaryClientIDs))))
}

func (h *Handlers) employeeLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.service.EmployeeLogin(r.Context(), req)
	writeResult(w, resp, err, http.StatusOK)
}

func (h *Handlers) employeeRefresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.service.EmployeeRefresh(r.Context(), req.RefreshToken)
	writeResult(w, resp, err, http.StatusOK)
}

func (h *Handlers) employeeLogout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if !decode(w, r, &req) {
		return
	}
	if err := h.service.Logout(r.Context(), req.RefreshToken); err != nil {
		writeError(w, err)
		return
	}
	platform.NoContent(w, http.StatusNoContent)
}

func (h *Handlers) employeeCheckActivate(w http.ResponseWriter, r *http.Request) {
	id, err := h.service.CheckEmployeeToken(r.Context(), r.URL.Query().Get("confirmationToken"))
	writeResult(w, id, err, http.StatusOK)
}

func (h *Handlers) employeeActivate(w http.ResponseWriter, r *http.Request) {
	var req ActivateRequest
	if !decode(w, r, &req) {
		return
	}
	err := h.service.ActivateEmployee(r.Context(), req)
	writeResult(w, "Lozinka je uspesno promenjena.", err, http.StatusOK)
}

func (h *Handlers) employeeForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req EmailRequest
	if !decode(w, r, &req) {
		return
	}
	err := h.service.EmployeeForgotPassword(r.Context(), req.Email)
	writeResult(w, "Poslat mejl", err, http.StatusAccepted)
}

func (h *Handlers) employeeResendActivation(w http.ResponseWriter, r *http.Request) {
	var req EmailRequest
	if !decode(w, r, &req) {
		return
	}
	err := h.service.EmployeeResendActivation(r.Context(), req.Email)
	writeResult(w, "Poslat mejl", err, http.StatusAccepted)
}

func (h *Handlers) clientLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.service.ClientLogin(r.Context(), req)
	writeResult(w, resp, err, http.StatusOK)
}

func (h *Handlers) clientCheckActivate(w http.ResponseWriter, r *http.Request) {
	id, err := h.service.CheckClientToken(r.Context(), r.URL.Query().Get("token"))
	writeResult(w, CheckActivateResponse{ID: id}, err, http.StatusOK)
}

func (h *Handlers) clientActivate(w http.ResponseWriter, r *http.Request) {
	var req ActivateRequest
	if !decode(w, r, &req) {
		return
	}
	err := h.service.ActivateClient(r.Context(), req)
	writeResult(w, "Lozinka je uspesno promenjena.", err, http.StatusOK)
}

func (h *Handlers) clientForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req EmailRequest
	if !decode(w, r, &req) {
		return
	}
	err := h.service.ClientForgotPassword(r.Context(), req.Email)
	writeResult(w, "Poslat mejl", err, http.StatusOK)
}

func (h *Handlers) clientResendActivation(w http.ResponseWriter, r *http.Request) {
	var req EmailRequest
	if !decode(w, r, &req) {
		return
	}
	err := h.service.ClientResendActivation(r.Context(), req.Email)
	writeResult(w, "Poslat mejl", err, http.StatusOK)
}

func (h *Handlers) searchEmployees(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.SearchEmployees(r.Context(), searchQuery(r))
	writeResult(w, resp, err, http.StatusOK)
}

func (h *Handlers) createEmployee(w http.ResponseWriter, r *http.Request) {
	var req EmployeeCreateRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.service.CreateEmployee(r.Context(), req)
	writeResult(w, resp, err, http.StatusCreated)
}

func (h *Handlers) getEmployee(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "/employees/")
	if !ok {
		return
	}
	resp, err := h.service.GetEmployee(r.Context(), id)
	writeResult(w, resp, err, http.StatusOK)
}

func (h *Handlers) updateEmployee(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "/employees/")
	if !ok {
		return
	}
	var req EmployeeUpdateRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.service.UpdateEmployee(r.Context(), id, req)
	status := http.StatusOK
	if req.Aktivan != nil && !*req.Aktivan {
		status = http.StatusAccepted
	}
	writeResult(w, resp, err, status)
}

func (h *Handlers) editEmployee(w http.ResponseWriter, r *http.Request) {
	principal, ok := platform.PrincipalFromContext(r.Context())
	if !ok {
		platform.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing principal")
		return
	}
	var req EmployeeUpdateRequest
	if !decode(w, r, &req) {
		return
	}
	req.Role = nil
	req.Aktivan = nil
	resp, err := h.service.UpdateEmployee(r.Context(), principal.ID, req)
	writeResult(w, resp, err, http.StatusOK)
}

func (h *Handlers) deleteEmployee(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "/employees/")
	if !ok {
		return
	}
	if err := h.service.DeleteEmployee(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	platform.NoContent(w, http.StatusNoContent)
}

func (h *Handlers) searchClients(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.SearchClients(r.Context(), searchQuery(r))
	writeResult(w, resp, err, http.StatusOK)
}

func (h *Handlers) createClient(w http.ResponseWriter, r *http.Request) {
	var req ClientCreateRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.service.CreateClient(r.Context(), req)
	writeResult(w, resp, err, http.StatusCreated)
}

func (h *Handlers) updateClient(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "/clients/customers/")
	if !ok {
		return
	}
	var req ClientUpdateRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.service.UpdateClient(r.Context(), id, req)
	writeResult(w, resp, err, http.StatusOK)
}

func (h *Handlers) deleteClient(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "/clients/customers/")
	if !ok {
		return
	}
	if err := h.service.DeleteClient(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	platform.NoContent(w, http.StatusNoContent)
}

func (h *Handlers) addMarginPermission(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "/clients/customers/margin/")
	if !ok {
		return
	}
	if err := h.service.AddMarginPermission(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	platform.NoContent(w, http.StatusOK)
}

func (h *Handlers) getClientByJMBG(w http.ResponseWriter, r *http.Request) {
	jmbg := strings.TrimPrefix(r.URL.Path, "/clients/customers/jmbg/")
	resp, err := h.service.GetClientInfoByJMBG(r.Context(), jmbg)
	writeResult(w, resp, err, http.StatusOK)
}

func (h *Handlers) getClientByID(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "/clients/customers/")
	if !ok {
		return
	}
	principal, _ := platform.PrincipalFromContext(r.Context())
	resp, err := h.service.GetClientInfo(r.Context(), id, principal)
	writeResult(w, resp, err, http.StatusOK)
}

func (h *Handlers) interbankUserDisplay(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/internal/interbank/user/"), "/")
	if len(parts) != 2 {
		platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Expected /internal/interbank/user/{type}/{id}")
		return
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid id")
		return
	}
	resp, err := h.service.InterbankUserDisplay(r.Context(), strings.ToUpper(parts[0]), id)
	writeResult(w, resp, err, http.StatusOK)
}

func (h *Handlers) actuaryClientIDs(w http.ResponseWriter, r *http.Request) {
	// Compatibility endpoint for OTC callers. The Java controller currently serves
	// as a narrow adapter; returning an empty list is safe until the exact actuary
	// assignment source is ported from trading/order logic.
	platform.JSON(w, http.StatusOK, []int64{})
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := platform.DecodeJSON(r, dst); err != nil {
		platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body")
		return false
	}
	return true
}

func writeResult(w http.ResponseWriter, value any, err error, status int) {
	if err != nil {
		writeError(w, err)
		return
	}
	platform.JSON(w, status, value)
}

func writeError(w http.ResponseWriter, err error) {
	status, code, message := handleServiceError(err)
	platform.Error(w, status, code, message)
}

func pathID(w http.ResponseWriter, r *http.Request, prefix string) (int64, bool) {
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	if raw == "" || strings.Contains(raw, "/") {
		platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid id")
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid id")
		return 0, false
	}
	return id, true
}

func searchQuery(r *http.Request) SearchQuery {
	values := r.URL.Query()
	return SearchQuery{
		Ime:       values.Get("ime"),
		Prezime:   values.Get("prezime"),
		Email:     values.Get("email"),
		Pozicija:  values.Get("pozicija"),
		Departman: values.Get("departman"),
		Query:     values.Get("query"),
		Page:      intParam(values.Get("page"), 0),
		Size:      intParam(values.Get("size"), 10),
	}
}

func intParam(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
