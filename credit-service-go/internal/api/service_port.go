package api

import (
	"context"

	"Banka1Back/credit-service-go/internal/auth"
	"Banka1Back/credit-service-go/internal/dto"
	"Banka1Back/credit-service-go/internal/model"
)

// CreditService is the business-logic port that LoanHandler depends on.
type CreditService interface {
	Request(ctx context.Context, user auth.User, request dto.LoanRequestDTO) (dto.LoanRequestResponseDTO, error)
	FindAllLoanRequests(ctx context.Context, loanType *model.LoanType, accountNumber *string, page int, size int) (dto.PageResponse[model.LoanRequest], error)
	Confirmation(ctx context.Context, id int64, status model.Status) (string, error)
	FindClientLoans(ctx context.Context, user auth.User, page int, size int) (dto.PageResponse[dto.LoanResponseDTO], error)
	GetLoanInfo(ctx context.Context, user auth.User, loanID int64) (dto.LoanInfoResponseDTO, error)
	FindAllLoans(ctx context.Context, loanType *model.LoanType, accountNumber *string, status *model.Status, page int, size int) (dto.PageResponse[dto.LoanResponseDTO], error)
}
