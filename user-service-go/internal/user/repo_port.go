package user

import (
	"context"
	"time"
)

// UserRepo is the persistence port used by Service.
// Implemented by *Repository.
type UserRepo interface {
	EmployeeByLogin(ctx context.Context, login string) (Employee, error)
	EmployeeByID(ctx context.Context, id int64) (Employee, error)
	FirstActiveEmployeeIDByRoleExcluding(ctx context.Context, role string, excludedID int64) (int64, error)
	EmployeePermissions(ctx context.Context, id int64, role string) []string
	ReplaceEmployeePermissions(ctx context.Context, id int64, permissions []string) error
	ResetEmployeeLoginFailures(ctx context.Context, id int64) error
	RegisterFailedEmployeeLogin(ctx context.Context, employee Employee, maxAttempts int, lockout time.Duration) error
	StoreEmployeeRefreshToken(ctx context.Context, employeeID int64, token string, expiresAt time.Time) error
	EmployeeByRefreshToken(ctx context.Context, token string) (Employee, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	ConfirmationIDByToken(ctx context.Context, table, ownerColumn, token string) (int64, error)
	UpsertEmployeeConfirmation(ctx context.Context, employeeID int64, tokenHash string, expiresAt time.Time) error
	UpsertClientConfirmation(ctx context.Context, clientID int64, tokenHash string, expiresAt time.Time) error
	ActivateEmployeePassword(ctx context.Context, confirmationID int64, tokenHash string, passwordHash string) error
	ActivateClientPassword(ctx context.Context, confirmationID int64, tokenHash string, passwordHash string) error
	SearchEmployees(ctx context.Context, query SearchQuery) ([]Employee, int, error)
	SearchClients(ctx context.Context, query SearchQuery) ([]Client, int, error)
	CreateEmployee(ctx context.Context, req EmployeeCreateRequest, permissions []string) (Employee, error)
	UpdateEmployee(ctx context.Context, id int64, req EmployeeUpdateRequest) (Employee, error)
	SoftDeleteEmployee(ctx context.Context, id int64) error
	ClientByEmail(ctx context.Context, email string) (Client, error)
	ClientByID(ctx context.Context, id int64) (Client, error)
	ClientByPlainJMBG(ctx context.Context, jmbg string) (Client, error)
	ClientPermissions(ctx context.Context, id int64, role string) []string
	OTCTradingClientIDs(ctx context.Context) ([]int64, error)
	CreateClient(ctx context.Context, req ClientCreateRequest, permissions []string) (Client, error)
	UpdateClient(ctx context.Context, id int64, req ClientUpdateRequest) (Client, error)
	AddClientMarginPermission(ctx context.Context, id int64) error
	SoftDeleteClient(ctx context.Context, id int64) error
}
