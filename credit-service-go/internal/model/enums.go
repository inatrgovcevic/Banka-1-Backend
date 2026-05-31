package model

type AccountOwnershipType string

const (
	AccountOwnershipPersonal AccountOwnershipType = "PERSONAL"
	AccountOwnershipBusiness AccountOwnershipType = "BUSINESS"
)

type EmploymentStatus string

const (
	EmploymentPermanent  EmploymentStatus = "PERMANENT"
	EmploymentTemporary  EmploymentStatus = "TEMPORARY"
	EmploymentUnemployed EmploymentStatus = "UNEMPLOYED"
)

type InterestType string

const (
	InterestFixed    InterestType = "FIXED"
	InterestVariable InterestType = "VARIABLE"
)

type LoanType string

const (
	LoanGotovinski      LoanType = "GOTOVINSKI"
	LoanStambeni        LoanType = "STAMBENI"
	LoanAuto            LoanType = "AUTO"
	LoanRefinansirajuci LoanType = "REFINANSIRAJUCI"
	LoanStudentski      LoanType = "STUDENTSKI"
)

type PaymentStatus string

const (
	PaymentPaid    PaymentStatus = "PAID"
	PaymentUnpaid  PaymentStatus = "UNPAID"
	PaymentOverdue PaymentStatus = "OVERDUE"
)

type Status string

const (
	StatusPending  Status = "PENDING"
	StatusApproved Status = "APPROVED"
	StatusDeclined Status = "DECLINED"
	StatusActive   Status = "ACTIVE"
	StatusOverdue  Status = "OVERDUE"
	StatusPaidOff  Status = "PAID_OFF"
)
