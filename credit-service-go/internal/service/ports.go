package service

import (
	"context"
	"time"

	"Banka1Back/credit-service-go/internal/client"
	"Banka1Back/credit-service-go/internal/model"

	"github.com/shopspring/decimal"
)

// LoanRequestRepository is the persistence port for loan request operations.
type LoanRequestRepository interface {
	Save(ctx context.Context, req model.LoanRequest) (model.LoanRequest, error)
	FindAll(ctx context.Context, loanType *model.LoanType, accountNumber *string, page int, size int) ([]model.LoanRequest, int, error)
	FindByID(ctx context.Context, id int64) (model.LoanRequest, error)
	UpdateStatusIfPending(ctx context.Context, id int64, status model.Status) (bool, error)
	ApproveWithLoanAndInstallment(ctx context.Context, requestID int64, loan model.Loan, installment model.Installment) (model.Loan, error)
}

// LoanRepository is the persistence port for loan operations.
type LoanRepository interface {
	FindByClientID(ctx context.Context, clientID int64, page int, size int) ([]model.Loan, int, error)
	FindByID(ctx context.Context, id int64) (model.Loan, error)
	FindAllWithFilters(ctx context.Context, loanType *model.LoanType, accountNumber *string, status *model.Status, page int, size int) ([]model.Loan, int, error)
	MarkOverdue(ctx context.Context, loanID int64) error
	UpdateAfterInstallmentPayment(ctx context.Context, loanID int64, remainingDebt decimal.Decimal, installmentCount int, nextInstallmentDate time.Time, status model.Status) error
}

// InstallmentRepository is the persistence port for installment operations.
type InstallmentRepository interface {
	FindByLoanID(ctx context.Context, loanID int64) ([]model.Installment, error)
	FindDueUnpaid(ctx context.Context) ([]model.Installment, error)
	MarkRetryOrOverdue(ctx context.Context, installment model.Installment) error
	MarkPaid(ctx context.Context, installmentID int64) error
	Create(ctx context.Context, installment model.Installment) error
}

// AccountGateway is the port for account service operations.
type AccountGateway interface {
	GetDetails(accountNumber string) (client.AccountDetailsResponse, error)
	TransactionFromBank(toBankNumber string, amount decimal.Decimal) error
	TransactionToBank(fromBankNumber string, amount decimal.Decimal) error
}

// ExchangeGateway is the port for currency exchange operations.
type ExchangeGateway interface {
	Calculate(fromCurrency string, toCurrency string, amount decimal.Decimal) (client.ConversionResponse, error)
}

// ClientGateway is the port for client service operations.
type ClientGateway interface {
	AddMarginPermission(clientID int64) error
}

// NotificationPublisher is the port for publishing notification events.
type NotificationPublisher interface {
	PublishJSON(ctx context.Context, routingKey string, payload any) error
}
