package service

import (
	"context"
	"errors"
	"time"

	"Banka1Back/credit-service-go/internal/auth"
	"Banka1Back/credit-service-go/internal/client"
	"Banka1Back/credit-service-go/internal/dto"
	"Banka1Back/credit-service-go/internal/messaging"
	"Banka1Back/credit-service-go/internal/model"
	"Banka1Back/credit-service-go/internal/store"

	"github.com/shopspring/decimal"
)

type LoanService struct {
	loanRequestStore    *store.LoanRequestStore
	loanStore           *store.LoanStore
	installmentStore    *store.InstallmentStore
	accountClient       *client.AccountClient
	rabbitClient        *messaging.RabbitClient
	exchangeClient      *client.ExchangeClient
	clientServiceClient *client.ClientServiceClient
}

type EmailEvent struct {
	UserEmail string `json:"userEmail"`
	Username  string `json:"username"`
	EmailType string `json:"emailType"`
}

func NewLoanService(
	loanRequestStore *store.LoanRequestStore,
	loanStore *store.LoanStore,
	installmentStore *store.InstallmentStore,
	accountClient *client.AccountClient,
	rabbitClient *messaging.RabbitClient,
	exchangeClient *client.ExchangeClient,
	clientServiceClient *client.ClientServiceClient,
) *LoanService {
	return &LoanService{
		loanRequestStore:    loanRequestStore,
		loanStore:           loanStore,
		installmentStore:    installmentStore,
		accountClient:       accountClient,
		rabbitClient:        rabbitClient,
		exchangeClient:      exchangeClient,
		clientServiceClient: clientServiceClient,
	}
}

func (s *LoanService) Request(ctx context.Context, user auth.User, request dto.LoanRequestDTO) (dto.LoanRequestResponseDTO, error) {
	accountDetails, err := s.accountClient.GetDetails(request.AccountNumber)
	if err != nil {
		return dto.LoanRequestResponseDTO{}, err
	}

	if accountDetails.OwnerID != user.ID {
		return dto.LoanRequestResponseDTO{}, errors.New("nisi vlasnik racuna")
	}

	if accountDetails.Currency != request.Currency {
		return dto.LoanRequestResponseDTO{}, errors.New("valuta racuna ne odgovara valuti kredita")
	}
	loanRequest := model.LoanRequest{
		LoanType:                request.LoanType,
		InterestType:            request.InterestType,
		Amount:                  request.Amount,
		Currency:                request.Currency,
		Purpose:                 request.Purpose,
		MonthlySalary:           request.MonthlySalary,
		EmploymentStatus:        request.EmploymentStatus,
		CurrentEmploymentPeriod: request.CurrentEmploymentPeriod,
		RepaymentPeriod:         request.RepaymentPeriod,
		ContactPhone:            request.ContactPhone,
		AccountNumber:           request.AccountNumber,
		ClientID:                user.ID,
		Status:                  model.StatusPending,
		UserEmail:               accountDetails.Email,
		Username:                accountDetails.Username,
	}

	saved, err := s.loanRequestStore.Save(ctx, loanRequest)
	if err != nil {
		return dto.LoanRequestResponseDTO{}, err
	}

	return dto.LoanRequestResponseDTO{
		ID:        saved.ID,
		CreatedAt: saved.CreatedAt,
	}, nil
}

func (s *LoanService) FindAllLoanRequests(
	ctx context.Context,
	loanType *model.LoanType,
	accountNumber *string,
	page int,
	size int,
) (dto.PageResponse[model.LoanRequest], error) {
	requests, total, err := s.loanRequestStore.FindAll(ctx, loanType, accountNumber, page, size)
	if err != nil {
		return dto.PageResponse[model.LoanRequest]{}, err
	}

	return dto.PageResponse[model.LoanRequest]{
		Content:       requests,
		Page:          page,
		Size:          size,
		TotalElements: total,
	}, nil
}

func (s *LoanService) Confirmation(ctx context.Context, id int64, status model.Status) (string, error) {
	if status != model.StatusApproved && status != model.StatusDeclined {
		return "", errors.New("mozes da saljes samo status approved ili declined")
	}

	loanRequest, err := s.loanRequestStore.FindByID(ctx, id)
	if err != nil {
		return "", err
	}

	if loanRequest.Status != model.StatusPending {
		return "", errors.New("loan request ne postoji ili nije u PENDING statusu")
	}

	if status == model.StatusDeclined {
		updated, err := s.loanRequestStore.UpdateStatusIfPending(ctx, id, status)
		if err != nil {
			return "", err
		}

		if !updated {
			return "", errors.New("loan request ne postoji ili nije u PENDING statusu")
		}

		_ = s.rabbitClient.PublishJSON(ctx, EmailEvent{
			UserEmail: loanRequest.UserEmail,
			Username:  loanRequest.Username,
			EmailType: "loan.request.declined",
		})

		return "ODBIJEN ZAHTEV", nil
	}

	interestAmount := loanRequest.Amount

	if loanRequest.Currency != model.CurrencyRSD {
		conversion, err := s.exchangeClient.Calculate(
			string(loanRequest.Currency),
			"RSD",
			loanRequest.Amount,
		)
		if err != nil {
			return "", err
		}

		interestAmount = conversion.ToAmount
	}

	interest := calculateInterestRate(
		interestAmount,
		loanRequest.LoanType,
		loanRequest.InterestType,
		model.StatusActive,
	)

	one := decimal.NewFromInt(1)
	period := decimal.NewFromInt(int64(loanRequest.RepaymentPeriod))

	stepen := interest.EffectiveInterestRate.Add(one).Pow(period)
	val := interest.EffectiveInterestRate.Mul(stepen).Div(stepen.Sub(one)).Round(10)
	monthlyRate := loanRequest.Amount.Mul(val).Round(4)

	now := time.Now()
	agreementDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	nextInstallmentDate := agreementDate.AddDate(0, 1, 0)

	err = s.accountClient.TransactionFromBank(loanRequest.AccountNumber, loanRequest.Amount)
	if err != nil {
		return "", err
	}

	loan := model.Loan{
		LoanType:              loanRequest.LoanType,
		AccountNumber:         loanRequest.AccountNumber,
		Amount:                loanRequest.Amount,
		RepaymentPeriod:       loanRequest.RepaymentPeriod,
		NominalInterestRate:   interest.NominalInterestRate,
		EffectiveInterestRate: interest.EffectiveInterestRate,
		InterestType:          loanRequest.InterestType,
		AgreementDate:         agreementDate,
		MaturityDate:          agreementDate.AddDate(0, loanRequest.RepaymentPeriod, 0),
		InstallmentAmount:     monthlyRate,
		NextInstallmentDate:   nextInstallmentDate,
		RemainingDebt:         loanRequest.Amount,
		Currency:              loanRequest.Currency,
		Status:                model.StatusActive,
		UserEmail:             loanRequest.UserEmail,
		Username:              loanRequest.Username,
		ClientID:              loanRequest.ClientID,
		InstallmentCount:      0,
	}

	installment := model.Installment{
		InstallmentAmount:     monthlyRate,
		InterestRateAtPayment: loan.EffectiveInterestRate,
		Currency:              loan.Currency,
		ExpectedDueDate:       loan.NextInstallmentDate,
		ActualDueDate:         nil,
		PaymentStatus:         model.PaymentUnpaid,
		Retry:                 0,
	}

	_, err = s.loanRequestStore.ApproveWithLoanAndInstallment(ctx, id, loan, installment)
	if err != nil {
		return "", err
	}

	err = s.clientServiceClient.AddMarginPermission(loanRequest.ClientID)
	if err != nil {
		return "", err
	}

	_ = s.rabbitClient.PublishJSON(ctx, EmailEvent{
		UserEmail: loanRequest.UserEmail,
		Username:  loanRequest.Username,
		EmailType: "loan.request.approved",
	})

	return "ODOBREN ZAHTEV", nil
}

func (s *LoanService) FindClientLoans(
	ctx context.Context,
	user auth.User,
	page int,
	size int,
) (dto.PageResponse[dto.LoanSummaryResponseDTO], error) {
	loans, total, err := s.loanStore.FindByClientID(ctx, user.ID, page, size)
	if err != nil {
		return dto.PageResponse[dto.LoanSummaryResponseDTO]{}, err
	}

	responses := make([]dto.LoanSummaryResponseDTO, 0, len(loans))
	for _, loan := range loans {
		responses = append(responses, dto.LoanSummaryResponseDTO{
			LoanNumber: loan.ID,
			LoanType:   loan.LoanType,
			Amount:     loan.Amount,
			Status:     loan.Status,
		})
	}

	return dto.PageResponse[dto.LoanSummaryResponseDTO]{
		Content:       responses,
		Page:          page,
		Size:          size,
		TotalElements: total,
	}, nil
}

func (s *LoanService) GetLoanInfo(
	ctx context.Context,
	user auth.User,
	loanID int64,
) (dto.LoanInfoResponseDTO, error) {
	loan, err := s.loanStore.FindByID(ctx, loanID)
	if err != nil {
		return dto.LoanInfoResponseDTO{}, err
	}

	if loan.ClientID != user.ID && user.Role != "BASIC" && user.Role != "ADMIN" {
		return dto.LoanInfoResponseDTO{}, errors.New("nemas dozvolu za ovu metodu")
	}

	installments, err := s.installmentStore.FindByLoanID(ctx, loanID)
	if err != nil {
		return dto.LoanInfoResponseDTO{}, err
	}

	installmentResponses := make([]dto.InstallmentResponseDTO, 0, len(installments))
	for _, installment := range installments {
		installmentResponses = append(installmentResponses, dto.InstallmentResponseDTO{
			InstallmentAmount:     installment.InstallmentAmount,
			InterestRateAtPayment: installment.InterestRateAtPayment,
			Currency:              installment.Currency,
			ExpectedDueDate:       installment.ExpectedDueDate,
			ActualDueDate:         installment.ActualDueDate,
			PaymentStatus:         installment.PaymentStatus,
		})
	}

	return dto.LoanInfoResponseDTO{
		Loan: dto.LoanResponseDTO{
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
		},
		Installments: installmentResponses,
	}, nil
}

func (s *LoanService) FindAllLoans(
	ctx context.Context,
	loanType *model.LoanType,
	accountNumber *string,
	status *model.Status,
	page int,
	size int,
) (dto.PageResponse[dto.LoanResponseDTO], error) {
	loans, total, err := s.loanStore.FindAllWithFilters(ctx, loanType, accountNumber, status, page, size)
	if err != nil {
		return dto.PageResponse[dto.LoanResponseDTO]{}, err
	}

	responses := make([]dto.LoanResponseDTO, 0, len(loans))
	for _, loan := range loans {
		responses = append(responses, dto.LoanResponseDTO{
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
		})
	}

	return dto.PageResponse[dto.LoanResponseDTO]{
		Content:       responses,
		Page:          page,
		Size:          size,
		TotalElements: total,
	}, nil
}

func (s *LoanService) ProcessDueInstallments(ctx context.Context) error {
	installments, err := s.installmentStore.FindDueUnpaid(ctx)
	if err != nil {
		return err
	}

	for _, installment := range installments {
		loan, err := s.loanStore.FindByID(ctx, installment.LoanID)
		if err != nil {
			return err
		}

		err = s.accountClient.TransactionToBank(loan.AccountNumber, installment.InstallmentAmount)
		if err != nil {
			err = s.installmentStore.MarkRetryOrOverdue(ctx, installment)
			if err != nil {
				return err
			}

			if installment.Retry > 0 || installment.PaymentStatus == model.PaymentOverdue {
				err = s.loanStore.MarkOverdue(ctx, installment.LoanID)
				if err != nil {
					return err
				}
			}

			continue
		}

		interestPart := loan.RemainingDebt.Mul(installment.InterestRateAtPayment)
		principalPart := installment.InstallmentAmount.Sub(interestPart)
		newRemainingDebt := loan.RemainingDebt.Sub(principalPart)
		newInstallmentCount := loan.InstallmentCount + 1

		err = s.installmentStore.MarkPaid(ctx, installment.ID)
		if err != nil {
			return err
		}

		if newRemainingDebt.GreaterThan(decimal.Zero) && newInstallmentCount < loan.RepaymentPeriod {
			nextDate := time.Now().AddDate(0, 1, 0)

			interest := calculateInterestRate(
				newRemainingDebt,
				loan.LoanType,
				loan.InterestType,
				loan.Status,
			)

			remaining := loan.RepaymentPeriod - newInstallmentCount
			one := decimal.NewFromInt(1)
			period := decimal.NewFromInt(int64(remaining))

			stepen := interest.EffectiveInterestRate.Add(one).Pow(period)
			val := interest.EffectiveInterestRate.Mul(stepen).Div(stepen.Sub(one)).Round(10)
			monthlyRate := newRemainingDebt.Mul(val).Round(4)

			err = s.loanStore.UpdateAfterInstallmentPayment(
				ctx,
				loan.ID,
				newRemainingDebt,
				newInstallmentCount,
				nextDate,
				loan.Status,
			)
			if err != nil {
				return err
			}

			nextInstallment := model.Installment{
				LoanID:                loan.ID,
				InstallmentAmount:     monthlyRate,
				InterestRateAtPayment: interest.EffectiveInterestRate,
				Currency:              loan.Currency,
				ExpectedDueDate:       nextDate,
				ActualDueDate:         nil,
				PaymentStatus:         model.PaymentUnpaid,
				Retry:                 0,
			}

			err = s.installmentStore.Create(ctx, nextInstallment)
			if err != nil {
				return err
			}
		} else {
			err = s.loanStore.UpdateAfterInstallmentPayment(
				ctx,
				loan.ID,
				decimal.Zero,
				newInstallmentCount,
				loan.NextInstallmentDate,
				model.StatusPaidOff,
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
