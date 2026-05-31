package api

import "github.com/shopspring/decimal"

// Tax DTOs mirror order-service com.banka1.order.dto.{TaxDebtResponse,
// TaxTrackingRowResponse} exactly. Jackson serializes nulls (Include.ALWAYS), so
// nullable fields use pointers / the LocalDateTime null marshaler.

// TaxDebtResponse ↔ GET /tax/capital-gains/debts (content rows) and
// GET /tax/capital-gains/{userId}.
//
// NOTE: despite the field name, debtRsd carries the per-user tax summed in the
// securities' ORIGINAL currencies — Java's getAllDebts/getUserDebt sum
// TaxChargeEntry.taxAmount with NO FX conversion and NO OTC component. Reproduced
// faithfully (the supervisor "debts" view is a raw cross-currency sum; the RSD
// figures live on the /tax/tracking rows).
type TaxDebtResponse struct {
	UserID  int64           `json:"userId"`
	DebtRsd decimal.Decimal `json:"debtRsd"`
}

// TaxTrackingRowResponse ↔ GET /tax/tracking (content rows). firstName/lastName
// come from client-service / employee-service and may be null. The monetary fields
// are RSD (strict FX conversion); lastTaxCalculationDate is null until a charge
// row exists for the user.
type TaxTrackingRowResponse struct {
	FirstName              *string         `json:"firstName"`
	LastName               *string         `json:"lastName"`
	UserType               string          `json:"userType"`
	TaxDebtRsd             decimal.Decimal `json:"taxDebtRsd"`
	LastTaxCalculationDate LocalDateTime   `json:"lastTaxCalculationDate"`
	CurrentMonthTaxRsd     decimal.Decimal `json:"currentMonthTaxRsd"`
	TotalPaidTaxRsd        decimal.Decimal `json:"totalPaidTaxRsd"`
	Status                 string          `json:"status"`
}

// TaxCollectedPayload ↔ the Map order-service publishes to employee.events with
// routing key tax.collected (OrderNotificationProducer.sendTaxCollected).
// notification-service reads templateVariables; username/userEmail are top-level
// and omitted when the taxpayer could not be resolved. Not part of the parity
// sweep — notifications are side-effects.
type TaxCollectedPayload struct {
	TemplateVariables map[string]string `json:"templateVariables"`
	Username          *string           `json:"username,omitempty"`
	UserEmail         *string           `json:"userEmail,omitempty"`
}
