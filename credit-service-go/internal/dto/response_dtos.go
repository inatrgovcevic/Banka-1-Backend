package dto

import (
	"time"

	"Banka1Back/credit-service-go/internal/model"

	"github.com/shopspring/decimal"
)

type AccountDetailsResponseDTO struct {
	OwnerID  int64              `json:"ownerId"`
	Currency model.CurrencyCode `json:"currency"`
	Email    string             `json:"email"`
	Username string             `json:"username"`
}

type AccountSearchResponseDTO struct {
	BrojRacuna           string                     `json:"brojRacuna"`
	Ime                  string                     `json:"ime"`
	Prezime              string                     `json:"prezime"`
	AccountOwnershipType model.AccountOwnershipType `json:"accountOwnershipType"`
	TekuciIliDevizni     string                     `json:"tekuciIliDevizni"`
}

type ConversionResponseDTO struct {
	FromCurrency string          `json:"fromCurrency"`
	ToCurrency   string          `json:"toCurrency"`
	FromAmount   decimal.Decimal `json:"fromAmount"`
	ToAmount     decimal.Decimal `json:"toAmount"`
	Rate         decimal.Decimal `json:"rate"`
	Commission   decimal.Decimal `json:"commission"`
	Date         time.Time       `json:"date"`
}

type ErrorResponseDTO struct {
	ErrorCode        string            `json:"errorCode"`
	ErrorTitle       string            `json:"errorTitle"`
	ErrorDesc        string            `json:"errorDesc"`
	Timestamp        time.Time         `json:"timestamp"`
	ValidationErrors map[string]string `json:"validationErrors,omitempty"`
}

type InfoResponseDTO struct {
	FromCurrencyCode model.CurrencyCode `json:"fromCurrencyCode"`
	ToCurrencyCode   model.CurrencyCode `json:"toCurrencyCode"`
	FromVlasnik      int64              `json:"fromVlasnik"`
	ToVlasnik        int64              `json:"toVlasnik"`
	FromEmail        string             `json:"fromEmail"`
	FromUsername     string             `json:"fromUsername"`
}

type InstallmentResponseDTO struct {
	InstallmentAmount     decimal.Decimal     `json:"installmentAmount"`
	InterestRateAtPayment decimal.Decimal     `json:"interestRateAtPayment"`
	Currency              model.CurrencyCode  `json:"currency"`
	ExpectedDueDate       time.Time           `json:"expectedDueDate"`
	ActualDueDate         *time.Time          `json:"actualDueDate,omitempty"`
	PaymentStatus         model.PaymentStatus `json:"paymentStatus"`
}

type LoanInfoResponseDTO struct {
	Loan         LoanResponseDTO          `json:"loan"`
	Installments []InstallmentResponseDTO `json:"installments"`
}

type LoanRequestResponseDTO struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}

type LoanResponseDTO struct {
	LoanNumber            int64              `json:"loanNumber,omitempty"`
	LoanType              model.LoanType     `json:"loanType,omitempty"`
	AccountNumber         string             `json:"accountNumber,omitempty"`
	Amount                decimal.Decimal    `json:"amount,omitempty"`
	RepaymentMethod       int                `json:"repaymentMethod,omitempty"`
	NominalInterestRate   decimal.Decimal    `json:"nominalInterestRate,omitempty"`
	EffectiveInterestRate decimal.Decimal    `json:"effectiveInterestRate,omitempty"`
	InterestType          model.InterestType `json:"interestType,omitempty"`
	AgreementDate         time.Time          `json:"agreementDate,omitempty"`
	MaturityDate          time.Time          `json:"maturityDate,omitempty"`
	InstallmentAmount     decimal.Decimal    `json:"installmentAmount,omitempty"`
	NextInstallmentDate   time.Time          `json:"nextInstallmentDate,omitempty"`
	RemainingDebt         decimal.Decimal    `json:"remainingDebt,omitempty"`
	Status                model.Status       `json:"status,omitempty"`
}

type UpdatedBalanceResponseDTO struct {
	SenderBalance   decimal.Decimal `json:"senderBalance"`
	ReceiverBalance decimal.Decimal `json:"receiverBalance"`
}

type VerificationStatusResponse struct {
	SessionID int64  `json:"sessionId"`
	Status    string `json:"status"`
}

type LoanSummaryResponseDTO struct {
	LoanNumber int64           `json:"loanNumber"`
	LoanType   model.LoanType  `json:"loanType"`
	Amount     decimal.Decimal `json:"amount"`
	Status     model.Status    `json:"status"`
}

func (v VerificationStatusResponse) IsVerified() bool {
	return v.Status == "VERIFIED"
}

func NewErrorResponse(code string, title string, desc string) ErrorResponseDTO {
	return ErrorResponseDTO{
		ErrorCode:  code,
		ErrorTitle: title,
		ErrorDesc:  desc,
		Timestamp:  time.Now(),
	}
}

func NewValidationErrorResponse(validationErrors map[string]string) ErrorResponseDTO {
	return ErrorResponseDTO{
		ErrorCode:        "ERR_VALIDATION",
		ErrorTitle:       "Neispravni podaci",
		ErrorDesc:        "Molimo Vas proverite unete podatke.",
		ValidationErrors: validationErrors,
		Timestamp:        time.Now(),
	}
}
