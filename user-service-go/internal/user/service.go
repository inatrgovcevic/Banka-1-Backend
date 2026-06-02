package user

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"banka1/user-service-go/internal/platform"
)

var (
	emailPattern = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)
	phonePattern = regexp.MustCompile(`^\+?[0-9]+$`)
)

type Service struct {
	repo  *Repository
	auth  *platform.JWTService
	pub   platform.NotificationPublisher
	cfg   platform.UserConfig
	email platform.EmailConfig
}

func NewService(repo *Repository, auth *platform.JWTService, pub platform.NotificationPublisher, cfg platform.UserConfig, email platform.EmailConfig) *Service {
	return &Service{repo: repo, auth: auth, pub: pub, cfg: cfg, email: email}
}

func (s *Service) EmployeeLogin(ctx context.Context, req LoginRequest) (TokenResponse, error) {
	login := firstNonBlank(req.Email, req.Username)
	employee, err := s.repo.EmployeeByLogin(ctx, login)
	if err != nil {
		return TokenResponse{}, ErrInvalidLogin
	}
	if !employee.Aktivan {
		return TokenResponse{}, ErrInactiveAccount
	}
	if employee.LockedUntil != nil && employee.LockedUntil.After(time.Now()) {
		return TokenResponse{}, ErrLockedAccount
	}
	if employee.PasswordHash == nil || !platform.VerifyPassword(req.Password, *employee.PasswordHash) {
		_ = s.repo.RegisterFailedEmployeeLogin(ctx, employee, s.cfg.EmployeeLockoutAttempts, s.cfg.EmployeeLockoutDuration)
		if employee.FailedLoginAttempts+1 >= s.cfg.EmployeeLockoutAttempts {
			return TokenResponse{}, ErrLockedAccount
		}
		return TokenResponse{}, ErrInvalidLogin
	}
	permissions := s.repo.EmployeePermissions(ctx, employee.ID, employee.Role)
	token, err := s.auth.GenerateAccessToken(employee.ID, employee.Email, employee.Role, permissions)
	if err != nil {
		return TokenResponse{}, err
	}
	refresh, err := platform.RandomURLToken()
	if err != nil {
		return TokenResponse{}, err
	}
	refreshHash := platform.SHA256Hex(refresh)
	if err := s.repo.StoreEmployeeRefreshToken(ctx, employee.ID, refreshHash, time.Now().Add(s.cfg.RefreshTokenDuration)); err != nil {
		return TokenResponse{}, err
	}
	_ = s.repo.ResetEmployeeLoginFailures(ctx, employee.ID)
	return TokenResponse{JWT: token, RefreshToken: refresh, Role: employee.Role, Permissions: permissions}, nil
}

func (s *Service) EmployeeRefresh(ctx context.Context, refresh string) (TokenResponse, error) {
	employee, err := s.repo.EmployeeByRefreshToken(ctx, platform.SHA256Hex(refresh))
	if err != nil {
		return TokenResponse{}, ErrInvalidToken
	}
	permissions := s.repo.EmployeePermissions(ctx, employee.ID, employee.Role)
	token, err := s.auth.GenerateAccessToken(employee.ID, employee.Email, employee.Role, permissions)
	if err != nil {
		return TokenResponse{}, err
	}
	nextRefresh, err := platform.RandomURLToken()
	if err != nil {
		return TokenResponse{}, err
	}
	_ = s.repo.DeleteRefreshToken(ctx, platform.SHA256Hex(refresh))
	if err := s.repo.StoreEmployeeRefreshToken(ctx, employee.ID, platform.SHA256Hex(nextRefresh), time.Now().Add(s.cfg.RefreshTokenDuration)); err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{JWT: token, RefreshToken: nextRefresh, Role: employee.Role, Permissions: permissions}, nil
}

func (s *Service) ClientLogin(ctx context.Context, req LoginRequest) (ClientLoginResponse, error) {
	client, err := s.repo.ClientByEmail(ctx, req.Email)
	if err != nil {
		return ClientLoginResponse{}, ErrInvalidLogin
	}
	if !client.Aktivan {
		return ClientLoginResponse{}, ErrInactiveAccount
	}
	if client.PasswordHash == nil || !platform.VerifyPassword(req.Password, *client.PasswordHash) {
		return ClientLoginResponse{}, ErrInvalidLogin
	}
	permissions := s.repo.ClientPermissions(ctx, client.ID, client.Role)
	token, err := s.auth.GenerateAccessToken(client.ID, client.Email, client.Role, permissions)
	if err != nil {
		return ClientLoginResponse{}, err
	}
	return ClientLoginResponse{Token: token, ID: client.ID, Ime: client.Ime, Prezime: client.Prezime, Email: client.Email}, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.repo.DeleteRefreshToken(ctx, platform.SHA256Hex(token))
}

func (s *Service) CheckEmployeeToken(ctx context.Context, token string) (int64, error) {
	return s.repo.ConfirmationIDByToken(ctx, "confirmation_token", "zaposlen_id", platform.SHA256Hex(token))
}

func (s *Service) CheckClientToken(ctx context.Context, token string) (int64, error) {
	return s.repo.ConfirmationIDByToken(ctx, "client_confirmation_token", "klijent_id", platform.SHA256Hex(token))
}

func (s *Service) ActivateEmployee(ctx context.Context, req ActivateRequest) error {
	token := confirmationToken(req)
	hash, err := platform.HashPassword(req.Password)
	if err != nil {
		return err
	}
	return s.repo.ActivateEmployeePassword(ctx, req.ID, platform.SHA256Hex(token), hash)
}

func (s *Service) ActivateClient(ctx context.Context, req ActivateRequest) error {
	token := confirmationToken(req)
	hash, err := platform.HashPassword(req.Password)
	if err != nil {
		return err
	}
	return s.repo.ActivateClientPassword(ctx, req.ID, platform.SHA256Hex(token), hash)
}

func (s *Service) EmployeeForgotPassword(ctx context.Context, email string) error {
	employee, err := s.repo.EmployeeByLogin(ctx, email)
	if err != nil {
		return ErrNotFound
	}
	if !employee.Aktivan {
		return ErrInactiveAccount
	}
	token, err := platform.RandomURLToken()
	if err != nil {
		return err
	}
	if err := s.repo.UpsertEmployeeConfirmation(ctx, employee.ID, platform.SHA256Hex(token), time.Now().Add(s.cfg.ConfirmationTokenDuration)); err != nil {
		return err
	}
	return s.publishEmail(ctx, "employee.password_reset", "EMPLOYEE_PASSWORD_RESET", employee.Ime, employee.Email, s.email.ResetPasswordURL+token)
}

func (s *Service) EmployeeResendActivation(ctx context.Context, email string) error {
	employee, err := s.repo.EmployeeByLogin(ctx, email)
	if err != nil {
		return ErrNotFound
	}
	if employee.Aktivan {
		return nil
	}
	token, err := platform.RandomURLToken()
	if err != nil {
		return err
	}
	if err := s.repo.UpsertEmployeeConfirmation(ctx, employee.ID, platform.SHA256Hex(token), time.Now().Add(s.cfg.ConfirmationTokenDuration)); err != nil {
		return err
	}
	return s.publishEmail(ctx, "employee.created", "EMPLOYEE_CREATED", employee.Ime, employee.Email, s.email.ActivateURL+token)
}

func (s *Service) ClientForgotPassword(ctx context.Context, email string) error {
	client, err := s.repo.ClientByEmail(ctx, email)
	if err != nil {
		return ErrNotFound
	}
	if !client.Aktivan {
		return ErrInactiveAccount
	}
	token, err := platform.RandomURLToken()
	if err != nil {
		return err
	}
	if err := s.repo.UpsertClientConfirmation(ctx, client.ID, platform.SHA256Hex(token), time.Now().Add(s.cfg.ConfirmationTokenDuration)); err != nil {
		return err
	}
	return s.publishEmail(ctx, "client.password_reset", "CLIENT_PASSWORD_RESET", client.Ime, client.Email, s.email.ResetPasswordURL+token)
}

func (s *Service) ClientResendActivation(ctx context.Context, email string) error {
	client, err := s.repo.ClientByEmail(ctx, email)
	if err != nil {
		return ErrNotFound
	}
	if client.Aktivan {
		return nil
	}
	token, err := platform.RandomURLToken()
	if err != nil {
		return err
	}
	if err := s.repo.UpsertClientConfirmation(ctx, client.ID, platform.SHA256Hex(token), time.Now().Add(s.cfg.ConfirmationTokenDuration)); err != nil {
		return err
	}
	return s.publishEmail(ctx, "client.created", "CLIENT_CREATED", client.Ime, client.Email, s.email.ActivateURL+token)
}

func (s *Service) SearchEmployees(ctx context.Context, query SearchQuery) (PageResponse[EmployeeResponse], error) {
	normalizePage(&query)
	employees, total, err := s.repo.SearchEmployees(ctx, query)
	if err != nil {
		return PageResponse[EmployeeResponse]{}, err
	}
	content := make([]EmployeeResponse, 0, len(employees))
	for _, employee := range employees {
		employee.Permissions = s.repo.EmployeePermissions(ctx, employee.ID, employee.Role)
		content = append(content, employeeDTO(employee))
	}
	return page(content, total, query.Page, query.Size), nil
}

func (s *Service) GetEmployee(ctx context.Context, id int64) (EmployeeResponse, error) {
	employee, err := s.repo.EmployeeByID(ctx, id)
	if err != nil {
		return EmployeeResponse{}, err
	}
	employee.Permissions = s.repo.EmployeePermissions(ctx, employee.ID, employee.Role)
	return employeeDTO(employee), nil
}

func (s *Service) CreateEmployee(ctx context.Context, req EmployeeCreateRequest) (EmployeeResponse, error) {
	if err := validateEmployeeCreate(req); err != nil {
		return EmployeeResponse{}, err
	}
	permissions := employeePermissions(defaultString(req.Role, "BASIC"))
	employee, err := s.repo.CreateEmployee(ctx, req, permissions)
	if err != nil {
		return EmployeeResponse{}, err
	}
	employee.Permissions = permissions
	token, err := platform.RandomURLToken()
	if err == nil {
		if err := s.repo.UpsertEmployeeConfirmation(ctx, employee.ID, platform.SHA256Hex(token), time.Now().Add(s.cfg.ConfirmationTokenDuration)); err == nil {
			_ = s.publishEmail(ctx, "employee.created", "EMPLOYEE_CREATED", employee.Ime, employee.Email, s.email.ActivateURL+token)
		}
	}
	return employeeDTO(employee), nil
}

func validateEmployeeCreate(req EmployeeCreateRequest) error {
	if !emailPattern.MatchString(strings.TrimSpace(req.Email)) {
		return ErrBadRequest
	}
	phone := strings.TrimSpace(req.BrojTelefona)
	if phone != "" && !phonePattern.MatchString(phone) {
		return ErrBadRequest
	}
	dob, err := time.Parse("2006-01-02", strings.TrimSpace(req.DatumRodjenja))
	if err != nil {
		return ErrBadRequest
	}
	today := time.Now()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	dobDay := time.Date(dob.Year(), dob.Month(), dob.Day(), 0, 0, 0, 0, today.Location())
	if dobDay.After(today) {
		return ErrBadRequest
	}
	return nil
}

func (s *Service) UpdateEmployee(ctx context.Context, id int64, req EmployeeUpdateRequest) (EmployeeResponse, error) {
	current, err := s.repo.EmployeeByID(ctx, id)
	if err != nil {
		return EmployeeResponse{}, err
	}
	if principal, ok := platform.PrincipalFromContext(ctx); ok &&
		principal.Role == "ADMIN" &&
		current.Role == "ADMIN" &&
		principal.ID != current.ID {
		return EmployeeResponse{}, ErrForbidden
	}
	employee, err := s.repo.UpdateEmployee(ctx, id, req)
	if err != nil {
		return EmployeeResponse{}, err
	}
	if req.Role != nil || req.Margin != nil {
		permissions := employeePermissions(employee.Role)
		if req.Margin != nil && *req.Margin && !containsPermission(permissions, "MARGIN_TRADE") {
			permissions = append(permissions, "MARGIN_TRADE")
		}
		if err := s.repo.ReplaceEmployeePermissions(ctx, employee.ID, permissions); err != nil {
			return EmployeeResponse{}, err
		}
	}
	employee.Permissions = s.repo.EmployeePermissions(ctx, employee.ID, employee.Role)
	if req.Aktivan != nil && !*req.Aktivan {
		_ = s.publishEmail(ctx, "employee.account_deactivated", "EMPLOYEE_ACCOUNT_DEACTIVATED", employee.Ime, employee.Email, "")
	}
	return employeeDTO(employee), nil
}

func (s *Service) DeleteEmployee(ctx context.Context, id int64) error {
	employee, _ := s.repo.EmployeeByID(ctx, id)
	if err := s.repo.SoftDeleteEmployee(ctx, id); err != nil {
		return err
	}
	if employee.ID != 0 {
		_ = s.publishEmail(ctx, "employee.account_deactivated", "EMPLOYEE_ACCOUNT_DEACTIVATED", employee.Ime, employee.Email, "")
	}
	return nil
}

func (s *Service) SearchClients(ctx context.Context, query SearchQuery) (PageResponse[ClientResponse], error) {
	normalizePage(&query)
	clients, total, err := s.repo.SearchClients(ctx, query)
	if err != nil {
		return PageResponse[ClientResponse]{}, err
	}
	content := make([]ClientResponse, 0, len(clients))
	for _, client := range clients {
		client.Permissions = s.repo.ClientPermissions(ctx, client.ID, client.Role)
		content = append(content, clientDTO(client))
	}
	return page(content, total, query.Page, query.Size), nil
}

func (s *Service) GetClientInfo(ctx context.Context, id int64, principal platform.Principal) (ClientInfoResponse, error) {
	if isClientRole(principal.Role) && principal.ID != id {
		return ClientInfoResponse{}, ErrForbidden
	}
	client, err := s.repo.ClientByID(ctx, id)
	if err != nil {
		return ClientInfoResponse{}, err
	}
	return clientInfoDTO(client), nil
}

func (s *Service) GetClientInfoByJMBG(ctx context.Context, jmbg string) (ClientInfoResponse, error) {
	client, err := s.repo.ClientByPlainJMBG(ctx, jmbg)
	if err != nil {
		return ClientInfoResponse{}, err
	}
	return clientInfoDTO(client), nil
}

func (s *Service) CreateClient(ctx context.Context, req ClientCreateRequest) (ClientResponse, error) {
	permissions := clientPermissions(defaultString(req.Role, "CLIENT_BASIC"))
	client, err := s.repo.CreateClient(ctx, req, permissions)
	if err != nil {
		return ClientResponse{}, err
	}
	client.Permissions = permissions
	token, err := platform.RandomURLToken()
	if err == nil {
		if err := s.repo.UpsertClientConfirmation(ctx, client.ID, platform.SHA256Hex(token), time.Now().Add(s.cfg.ConfirmationTokenDuration)); err == nil {
			_ = s.publishEmail(ctx, "client.created", "CLIENT_CREATED", client.Ime, client.Email, s.email.ActivateURL+token)
		}
	}
	return clientDTO(client), nil
}

func (s *Service) UpdateClient(ctx context.Context, id int64, req ClientUpdateRequest) (ClientResponse, error) {
	client, err := s.repo.UpdateClient(ctx, id, req)
	if err != nil {
		return ClientResponse{}, err
	}
	client.Permissions = s.repo.ClientPermissions(ctx, client.ID, client.Role)
	return clientDTO(client), nil
}

func (s *Service) AddMarginPermission(ctx context.Context, id int64) error {
	return s.repo.AddClientMarginPermission(ctx, id)
}

func (s *Service) DeleteClient(ctx context.Context, id int64) error {
	client, _ := s.repo.ClientByID(ctx, id)
	if err := s.repo.SoftDeleteClient(ctx, id); err != nil {
		return err
	}
	if client.ID != 0 {
		_ = s.publishEmail(ctx, "client.account_deactivated", "CLIENT_ACCOUNT_DEACTIVATED", client.Ime, client.Email, "")
	}
	return nil
}

func (s *Service) InterbankUserDisplay(ctx context.Context, kind string, id int64) (InterbankUserDisplayResponse, error) {
	switch ActorKind(kind) {
	case ActorClient:
		client, err := s.repo.ClientByID(ctx, id)
		if err != nil {
			return InterbankUserDisplayResponse{}, err
		}
		return InterbankUserDisplayResponse{FirstName: client.Ime, LastName: client.Prezime, FullName: client.Ime + " " + client.Prezime}, nil
	case ActorEmployee:
		employee, err := s.repo.EmployeeByID(ctx, id)
		if err != nil {
			return InterbankUserDisplayResponse{}, err
		}
		return InterbankUserDisplayResponse{FirstName: employee.Ime, LastName: employee.Prezime, FullName: employee.Ime + " " + employee.Prezime}, nil
	default:
		return InterbankUserDisplayResponse{}, ErrBadRequest
	}
}

func handleServiceError(err error) (int, string, string) {
	switch {
	case errors.Is(err, ErrBadRequest):
		return 400, "BAD_REQUEST", "Invalid request"
	case errors.Is(err, ErrInvalidLogin):
		return 401, "INVALID_CREDENTIALS", "Invalid credentials"
	case errors.Is(err, ErrInactiveAccount):
		return 403, "INACTIVE_ACCOUNT", "Account is not active"
	case errors.Is(err, ErrLockedAccount):
		return 423, "ACCOUNT_LOCKED", "Account is temporarily locked"
	case errors.Is(err, ErrForbidden):
		return 403, "FORBIDDEN", "Forbidden"
	case errors.Is(err, ErrInvalidToken):
		return 400, "INVALID_TOKEN", "Invalid or expired token"
	case errors.Is(err, ErrNotFound):
		return 404, "NOT_FOUND", "Resource not found"
	case errors.Is(err, ErrDuplicate):
		return 409, "DUPLICATE_RESOURCE", "Resource already exists"
	case errors.Is(err, ErrUnsupportedJMBG):
		return 501, "JMBG_LOOKUP_UNAVAILABLE", "JMBG lookup requires encrypted lookup support"
	default:
		return 500, "INTERNAL_ERROR", "Internal server error"
	}
}

func normalizePage(query *SearchQuery) {
	if query.Page < 0 {
		query.Page = 0
	}
	if query.Size <= 0 {
		query.Size = 10
	}
	if query.Size > 100 {
		query.Size = 100
	}
}

func page[T any](content []T, total, current, size int) PageResponse[T] {
	totalPages := 0
	if size > 0 {
		totalPages = (total + size - 1) / size
	}
	last := totalPages == 0 || current >= totalPages-1
	return PageResponse[T]{
		Content:          content,
		TotalElements:    total,
		TotalPages:       totalPages,
		CurrentPage:      current,
		Number:           current,
		Size:             size,
		NumberOfElements: len(content),
		First:            current == 0,
		Last:             last,
		Empty:            len(content) == 0,
	}
}

func clientInfoDTO(client Client) ClientInfoResponse {
	return ClientInfoResponse{
		ID:            client.ID,
		Ime:           client.Ime,
		Prezime:       client.Prezime,
		Name:          client.Ime,
		LastName:      client.Prezime,
		Email:         client.Email,
		JMBG:          client.JMBG,
		BrojTelefona:  client.BrojTelefona,
		PhoneNumber:   client.BrojTelefona,
		Adresa:        client.Adresa,
		Address:       client.Adresa,
		Pol:           client.Pol,
		Gender:        client.Pol,
		DatumRodjenja: client.DatumRodjenja,
		DateOfBirth:   client.DatumRodjenja,
		Role:          client.Role,
		Aktivan:       client.Aktivan,
		Active:        client.Aktivan,
	}
}

func isClientRole(role string) bool {
	return role == "CLIENT_BASIC" || role == "CLIENT_TRADING"
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func containsPermission(permissions []string, needle string) bool {
	for _, permission := range permissions {
		if permission == needle {
			return true
		}
	}
	return false
}

func confirmationToken(req ActivateRequest) string {
	if req.ConfirmationToken != "" {
		return req.ConfirmationToken
	}
	return req.Token
}

func (s *Service) publishEmail(ctx context.Context, routingKey, emailType, username, email, link string) error {
	vars := map[string]string{"name": username, "username": username}
	if link != "" {
		vars["activationLink"] = link
		vars["resetLink"] = link
	}
	return s.pub.PublishEmail(ctx, routingKey, platform.EmailNotification{
		UserEmail:         email,
		Username:          username,
		EmailType:         emailType,
		TemplateVariables: vars,
	})
}
