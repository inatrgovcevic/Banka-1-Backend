package service_test

import (
	"context"
	"errors"
	"testing"

	"Banka1Back/credit-service-go/internal/auth"
	"Banka1Back/credit-service-go/internal/model"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Confirmation additional edge cases
// ---------------------------------------------------------------------------

func TestConfirmation_FindByIDError_ReturnsError(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	reqRepo.findByIDErr = errors.New("db error")
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	_, err := svc.Confirmation(context.Background(), 1, model.StatusDeclined)
	require.Error(t, err)
}

func TestConfirmation_Decline_UpdateStatusError_ReturnsError(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	reqRepo.findByIDResult = model.LoanRequest{Status: model.StatusPending}
	reqRepo.updateErr = errors.New("update failed")
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	_, err := svc.Confirmation(context.Background(), 1, model.StatusDeclined)
	require.Error(t, err)
}

func TestConfirmation_Approve_ApproveWithLoanAndInstallmentError_ReturnsError(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	reqRepo.findByIDResult = model.LoanRequest{
		Status:          model.StatusPending,
		Currency:        model.CurrencyRSD,
		Amount:          decimal.NewFromInt(50000),
		RepaymentPeriod: 12,
		LoanType:        model.LoanGotovinski,
		InterestType:    model.InterestFixed,
	}
	reqRepo.approveErr = errors.New("approve failed")
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	_, err := svc.Confirmation(context.Background(), 1, model.StatusApproved)
	require.Error(t, err)
}

func TestConfirmation_Approve_ClientGatewayError_ReturnsError(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	reqRepo.findByIDResult = model.LoanRequest{
		Status:          model.StatusPending,
		Currency:        model.CurrencyRSD,
		Amount:          decimal.NewFromInt(50000),
		RepaymentPeriod: 12,
		LoanType:        model.LoanGotovinski,
		InterestType:    model.InterestFixed,
	}
	clientGw.err = errors.New("margin permission failed")
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	_, err := svc.Confirmation(context.Background(), 1, model.StatusApproved)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// GetLoanInfo additional cases
// ---------------------------------------------------------------------------

func TestGetLoanInfo_BasicRole_CanViewAnyLoan(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	loanRepo.findByIDResult = model.Loan{ClientID: 99, LoanType: model.LoanAuto}
	installRepo.findByLoanResult = []model.Installment{}
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	resp, err := svc.GetLoanInfo(context.Background(), auth.User{ID: 1, Role: "BASIC"}, 5)
	require.NoError(t, err)
	require.Equal(t, model.LoanAuto, resp.Loan.LoanType)
}

func TestGetLoanInfo_InstallmentError_ReturnsError(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	loanRepo.findByIDResult = model.Loan{ClientID: 1}
	installRepo.findByLoanErr = errors.New("installment db error")
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	_, err := svc.GetLoanInfo(context.Background(), auth.User{ID: 1}, 5)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// FindAllLoans additional cases
// ---------------------------------------------------------------------------

func TestFindAllLoans_StoreError_ReturnsError(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	loanRepo.findAllErr = errors.New("db error")
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	_, err := svc.FindAllLoans(context.Background(), nil, nil, nil, 0, 10)
	require.Error(t, err)
}

func TestFindAllLoans_WithFilters_ReturnsMappedResults(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	lt := model.LoanGotovinski
	status := model.StatusActive
	loanRepo.findAllResult = []model.Loan{
		{LoanType: model.LoanGotovinski, Status: model.StatusActive},
	}
	loanRepo.findAllTotal = 1
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	resp, err := svc.FindAllLoans(context.Background(), &lt, nil, &status, 0, 10)
	require.NoError(t, err)
	require.Len(t, resp.Content, 1)
	require.Equal(t, model.LoanGotovinski, resp.Content[0].LoanType)
}

// ---------------------------------------------------------------------------
// ProcessDueInstallments error paths
// ---------------------------------------------------------------------------

func TestProcessDueInstallments_FindDueError_ReturnsError(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	installRepo.findDueErr = errors.New("db error")
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	err := svc.ProcessDueInstallments(context.Background())
	require.Error(t, err)
}

func TestProcessDueInstallments_LoanFindError_ReturnsError(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	installRepo.findDueResult = []model.Installment{{LoanID: 10}}
	loanRepo.findByIDErr = errors.New("loan not found")
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	err := svc.ProcessDueInstallments(context.Background())
	require.Error(t, err)
}

func TestProcessDueInstallments_MarkRetryError_ReturnsError(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	installRepo.findDueResult = []model.Installment{{LoanID: 10, Retry: 0}}
	loanRepo.findByIDResult = model.Loan{Status: model.StatusActive}
	account.txToBankErr = errors.New("tx failed")
	installRepo.markRetryErr = errors.New("mark retry failed")
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	err := svc.ProcessDueInstallments(context.Background())
	require.Error(t, err)
}

func TestProcessDueInstallments_MarkOverdueError_ReturnsError(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	installRepo.findDueResult = []model.Installment{
		{LoanID: 10, Retry: 1, PaymentStatus: model.PaymentOverdue},
	}
	loanRepo.findByIDResult = model.Loan{Status: model.StatusActive}
	account.txToBankErr = errors.New("tx failed")
	loanRepo.markOverdueErr = errors.New("mark overdue failed")
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	err := svc.ProcessDueInstallments(context.Background())
	require.Error(t, err)
}

func TestProcessDueInstallments_MarkPaidError_ReturnsError(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	installRepo.findDueResult = []model.Installment{
		{
			LoanID:                10,
			InstallmentAmount:     decimal.NewFromInt(500),
			InterestRateAtPayment: decimal.NewFromFloat(0.01),
			Currency:              model.CurrencyRSD,
		},
	}
	loanRepo.findByIDResult = model.Loan{
		LoanType:         model.LoanGotovinski,
		InterestType:     model.InterestFixed,
		RemainingDebt:    decimal.NewFromInt(10000),
		RepaymentPeriod:  12,
		InstallmentCount: 2,
		Status:           model.StatusActive,
	}
	installRepo.markPaidErr = errors.New("mark paid failed")
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	err := svc.ProcessDueInstallments(context.Background())
	require.Error(t, err)
}

func TestProcessDueInstallments_UpdateAfterPaymentError_ReturnsError(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	installRepo.findDueResult = []model.Installment{
		{
			LoanID:                10,
			InstallmentAmount:     decimal.NewFromInt(500),
			InterestRateAtPayment: decimal.NewFromFloat(0.01),
			Currency:              model.CurrencyRSD,
		},
	}
	loanRepo.findByIDResult = model.Loan{
		LoanType:         model.LoanGotovinski,
		InterestType:     model.InterestFixed,
		RemainingDebt:    decimal.NewFromInt(10000),
		RepaymentPeriod:  12,
		InstallmentCount: 2,
		Status:           model.StatusActive,
	}
	loanRepo.updateAfterErr = errors.New("update after payment failed")
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	err := svc.ProcessDueInstallments(context.Background())
	require.Error(t, err)
}

func TestProcessDueInstallments_CreateNextInstallmentError_ReturnsError(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	installRepo.findDueResult = []model.Installment{
		{
			LoanID:                10,
			InstallmentAmount:     decimal.NewFromInt(500),
			InterestRateAtPayment: decimal.NewFromFloat(0.01),
			Currency:              model.CurrencyRSD,
		},
	}
	loanRepo.findByIDResult = model.Loan{
		LoanType:         model.LoanGotovinski,
		InterestType:     model.InterestFixed,
		RemainingDebt:    decimal.NewFromInt(10000),
		RepaymentPeriod:  12,
		InstallmentCount: 2,
		Status:           model.StatusActive,
	}
	installRepo.createErr = errors.New("create installment failed")
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	err := svc.ProcessDueInstallments(context.Background())
	require.Error(t, err)
}

func TestProcessDueInstallments_PaidOffUpdateError_ReturnsError(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	installRepo.findDueResult = []model.Installment{
		{
			LoanID:                10,
			InstallmentAmount:     decimal.NewFromFloat(1000),
			InterestRateAtPayment: decimal.NewFromFloat(0.01),
			Currency:              model.CurrencyRSD,
		},
	}
	loanRepo.findByIDResult = model.Loan{
		LoanType:         model.LoanGotovinski,
		InterestType:     model.InterestFixed,
		RemainingDebt:    decimal.NewFromFloat(990),
		RepaymentPeriod:  12,
		InstallmentCount: 11,
		Status:           model.StatusActive,
	}
	loanRepo.updateAfterErr = errors.New("update paid off failed")
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	err := svc.ProcessDueInstallments(context.Background())
	require.Error(t, err)
}

func TestProcessDueInstallments_MultipleInstallments_ProcessesAll(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()

	installment1 := model.Installment{
		LoanID:                10,
		InstallmentAmount:     decimal.NewFromInt(500),
		InterestRateAtPayment: decimal.NewFromFloat(0.01),
		Currency:              model.CurrencyRSD,
	}
	installment2 := model.Installment{
		LoanID:                10,
		InstallmentAmount:     decimal.NewFromInt(500),
		InterestRateAtPayment: decimal.NewFromFloat(0.01),
		Currency:              model.CurrencyRSD,
	}
	loanRepo.findByIDResult = model.Loan{
		LoanType:         model.LoanGotovinski,
		InterestType:     model.InterestFixed,
		RemainingDebt:    decimal.NewFromInt(10000),
		RepaymentPeriod:  12,
		InstallmentCount: 2,
		Status:           model.StatusActive,
	}

	installRepo.findDueResult = []model.Installment{installment1, installment2}
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	err := svc.ProcessDueInstallments(context.Background())
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// FindClientLoans additional cases
// ---------------------------------------------------------------------------

func TestFindClientLoans_EmptyResult_ReturnsEmptyPage(t *testing.T) {
	t.Parallel()
	reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier := defaultStubs()
	loanRepo.findByClientResult = []model.Loan{}
	loanRepo.findByClientTotal = 0
	svc := newService(reqRepo, loanRepo, installRepo, account, exchange, clientGw, notifier)

	resp, err := svc.FindClientLoans(context.Background(), auth.User{ID: 1}, 0, 10)
	require.NoError(t, err)
	require.Empty(t, resp.Content)
	require.Equal(t, 0, resp.TotalElements)
}
