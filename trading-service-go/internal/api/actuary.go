package api

import "github.com/shopspring/decimal"

// Actuary DTOs — JSON shapes mirror order-service exactly (see
// com.banka1.order.dto.{ActuaryAgentDto,ActuaryProfitDto,BankProfitSummaryDto,
// SetLimitRequestDto,SetNeedApprovalRequestDto,SimpleResponse}).

// ActuaryAgentDto is one row of GET /actuaries/agents (Page content). Employee
// name/email/position come from user-service; limit/usedLimit/needApproval from
// the local actuary_info row. limit is null for unconfigured agents/supervisors.
type ActuaryAgentDto struct {
	EmployeeID   int64            `json:"employeeId"`
	Ime          *string          `json:"ime"`
	Prezime      *string          `json:"prezime"`
	Email        *string          `json:"email"`
	Pozicija     *string          `json:"pozicija"`
	Limit        *decimal.Decimal `json:"limit"`
	UsedLimit    decimal.Decimal  `json:"usedLimit"`
	NeedApproval bool             `json:"needApproval"`
}

// ActuaryProfitDto ↔ one element of GET /actuaries/profit. ime/prezime/pozicija
// are null when the user-service name lookup fails (best-effort enrichment).
type ActuaryProfitDto struct {
	UserID           int64           `json:"userId"`
	TotalCommission  decimal.Decimal `json:"totalCommission"`
	TransactionCount int64           `json:"transactionCount"`
	Ime              *string         `json:"ime"`
	Prezime          *string         `json:"prezime"`
	Pozicija         *string         `json:"pozicija"`
}

// BankProfitSummaryDto ↔ GET /actuaries/profit/bank-summary. from/to echo the
// original (nullable) query params.
type BankProfitSummaryDto struct {
	TotalCommission   decimal.Decimal `json:"totalCommission"`
	TransactionCount  int64           `json:"transactionCount"`
	DistinctActuaries int64           `json:"distinctActuaries"`
	From              LocalDateTime   `json:"from"`
	To                LocalDateTime   `json:"to"`
}

// SetLimitRequest ↔ PUT /actuaries/agents/{id}/limit body. Java validates
// @NotNull @DecimalMin(value="0.0", inclusive=false).
type SetLimitRequest struct {
	Limit *decimal.Decimal `json:"limit"`
}

// SetNeedApprovalRequest ↔ PUT /actuaries/agents/{id}/need-approval body.
// Java validates @NotNull.
type SetNeedApprovalRequest struct {
	NeedApproval *bool `json:"needApproval"`
}

// SimpleResponse ↔ the {status,message} record returned by the actuary mutations.
type SimpleResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// SimpleSuccess mirrors SimpleResponse.success(message).
func SimpleSuccess(message string) SimpleResponse {
	return SimpleResponse{Status: "success", Message: message}
}
