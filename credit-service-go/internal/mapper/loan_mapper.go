package mapper

import (
	"Banka1Back/credit-service-go/internal/dto"
	"Banka1Back/credit-service-go/internal/model"
)

func LoanToResponseDTO(loan model.Loan) dto.LoanResponseDTO {
	return dto.LoanResponseDTO{
		LoanNumber:            loan.ID,
		LoanType:              loan.LoanType,
		AccountNumber:         loan.AccountNumber,
		Amount:                loan.Amount,
		RepaymentMethod:       loan.RepaymentPeriod,
		NominalInterestRate:   loan.NominalInterestRate,
		EffectiveInterestRate: loan.EffectiveInterestRate,
		InterestType:          loan.InterestType,
		AgreementDate:         loan.AgreementDate,
		MaturityDate:          loan.MaturityDate,
		InstallmentAmount:     loan.InstallmentAmount,
		NextInstallmentDate:   loan.NextInstallmentDate,
		RemainingDebt:         loan.RemainingDebt,
		Status:                loan.Status,
	}
}

func LoanRequestToResponseDTO(request model.LoanRequest) dto.LoanRequestResponseDTO {
	return dto.LoanRequestResponseDTO{
		ID:        request.ID,
		CreatedAt: request.CreatedAt,
	}
}

func InstallmentToResponseDTO(installment model.Installment) dto.InstallmentResponseDTO {
	return dto.InstallmentResponseDTO{
		InstallmentAmount:     installment.InstallmentAmount,
		InterestRateAtPayment: installment.InterestRateAtPayment,
		Currency:              installment.Currency,
		ExpectedDueDate:       installment.ExpectedDueDate,
		ActualDueDate:         installment.ActualDueDate,
		PaymentStatus:         installment.PaymentStatus,
	}
}

func InstallmentsToResponseDTOs(installments []model.Installment) []dto.InstallmentResponseDTO {
	responses := make([]dto.InstallmentResponseDTO, 0, len(installments))

	for _, installment := range installments {
		responses = append(responses, InstallmentToResponseDTO(installment))
	}

	return responses
}

func LoanInfoToResponseDTO(loan model.Loan, installments []model.Installment) dto.LoanInfoResponseDTO {
	return dto.LoanInfoResponseDTO{
		Loan:         LoanToResponseDTO(loan),
		Installments: InstallmentsToResponseDTOs(installments),
	}
}
