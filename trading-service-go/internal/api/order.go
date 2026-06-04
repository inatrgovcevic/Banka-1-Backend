package api

import "github.com/shopspring/decimal"

// Order DTOs mirror order-service com.banka1.order.dto.* exactly so the existing
// frontend keeps working. Enum fields (orderType/status/direction/listingType)
// serialize as their Java enum name strings. Nullable Java fields are pointers so
// they marshal to JSON null, matching Jackson.

// CreateBuyOrderRequest ↔ POST /orders/buy body (CreateBuyOrderRequest).
// All numeric fields are pointers so the service can reproduce Java's null checks
// and bean-validation (@NotNull/@Positive) precisely.
type CreateBuyOrderRequest struct {
	ListingID   *int64           `json:"listingId"`
	Quantity    *int             `json:"quantity"`
	LimitValue  *decimal.Decimal `json:"limitValue"`
	StopValue   *decimal.Decimal `json:"stopValue"`
	AllOrNone   *bool            `json:"allOrNone"`
	Margin      *bool            `json:"margin"`
	AccountID   *int64           `json:"accountId"`
	PurchaseFor *string          `json:"purchaseFor"`
	FundID      *int64           `json:"fundId"`
}

// CreateSellOrderRequest ↔ POST /orders/sell body (CreateSellOrderRequest).
// accountId is @NotNull for sells (validated in the service).
type CreateSellOrderRequest struct {
	ListingID  *int64           `json:"listingId"`
	Quantity   *int             `json:"quantity"`
	LimitValue *decimal.Decimal `json:"limitValue"`
	StopValue  *decimal.Decimal `json:"stopValue"`
	AllOrNone  *bool            `json:"allOrNone"`
	Margin     *bool            `json:"margin"`
	AccountID  *int64           `json:"accountId"`
}

// PartialCancelOrderRequest ↔ optional body of PUT /orders/{id}/cancel.
type PartialCancelOrderRequest struct {
	Quantity *int `json:"quantity"`
}

// CreateRecurringOrderRequest ↔ POST /recurring-orders body (Celina 3.6 standing
// orders). nextRun arrives as a Jackson LocalDateTime string
// ("2026-06-10T09:00:00"); the handler parses + @Future-validates it.
type CreateRecurringOrderRequest struct {
	ListingID *int64           `json:"listingId"`
	Direction *string          `json:"direction"`
	Mode      *string          `json:"mode"`
	Value     *decimal.Decimal `json:"value"`
	AccountID *int64           `json:"accountId"`
	Cadence   *string          `json:"cadence"`
	NextRun   *string          `json:"nextRun"`
}

// RecurringOrderDto ↔ order-service RecurringOrderDto (all entity fields).
type RecurringOrderDto struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"userId"`
	ListingID int64           `json:"listingId"`
	Direction string          `json:"direction"`
	Mode      string          `json:"mode"`
	Value     decimal.Decimal `json:"value"`
	AccountID int64           `json:"accountId"`
	Cadence   string          `json:"cadence"`
	NextRun   LocalDateTime   `json:"nextRun"`
	Active    bool            `json:"active"`
	CreatedAt LocalDateTime   `json:"createdAt"`
}

// RecurringOrderSkippedNotification ↔ order-service RecurringOrderSkippedNotification
// — the order.recurring_skipped payload. clientId drives the FCM token lookup:
// the owner's user id for client owners, null for actuaries (no device push).
// username/userEmail stay null (the push-only notification path needs neither).
type RecurringOrderSkippedNotification struct {
	Username          *string           `json:"username"`
	UserEmail         *string           `json:"userEmail"`
	ClientID          *int64            `json:"clientId"`
	TemplateVariables map[string]string `json:"templateVariables"`
}

// OrderResponse ↔ order-service OrderResponse. Nullable fields (approvedBy,
// limitValue, stopValue, the enrichment fields) are pointers. Celina 3 changed
// the timestamps to UTC Instants (trailing Z) and added the mobile My Orders
// enrichment fields (ticker/securityName/listingType/executionPrice) plus
// createdAt/executedAt.
type OrderResponse struct {
	ID                int64            `json:"id"`
	UserID            int64            `json:"userId"`
	ListingID         int64            `json:"listingId"`
	OrderType         string           `json:"orderType"`
	Quantity          int              `json:"quantity"`
	ContractSize      int              `json:"contractSize"`
	PricePerUnit      decimal.Decimal  `json:"pricePerUnit"`
	LimitValue        *decimal.Decimal `json:"limitValue"`
	StopValue         *decimal.Decimal `json:"stopValue"`
	Direction         string           `json:"direction"`
	Status            string           `json:"status"`
	ApprovedBy        *int64           `json:"approvedBy"`
	IsDone            bool             `json:"isDone"`
	LastModification  UTCInstant       `json:"lastModification"`
	RemainingPortions int              `json:"remainingPortions"`
	AfterHours        bool             `json:"afterHours"`
	ExchangeClosed    bool             `json:"exchangeClosed"`
	AllOrNone         bool             `json:"allOrNone"`
	Margin            bool             `json:"margin"`
	AccountID         int64            `json:"accountId"`
	ApproximatePrice  decimal.Decimal  `json:"approximatePrice"`
	Fee               decimal.Decimal  `json:"fee"`
	Ticker            *string          `json:"ticker"`
	SecurityName      *string          `json:"securityName"`
	ListingType       *string          `json:"listingType"`
	ExecutionPrice    *decimal.Decimal `json:"executionPrice"`
	CreatedAt         UTCInstant       `json:"createdAt"`
	ExecutedAt        UTCInstant       `json:"executedAt"`
}

// OrderOverviewResponse ↔ order-service OrderOverviewResponse (supervisor portal
// row). agentName and listingType are nullable.
type OrderOverviewResponse struct {
	OrderID           int64           `json:"orderId"`
	AgentName         *string         `json:"agentName"`
	OrderType         string          `json:"orderType"`
	ListingType       *string         `json:"listingType"`
	Quantity          int             `json:"quantity"`
	ContractSize      int             `json:"contractSize"`
	PricePerUnit      decimal.Decimal `json:"pricePerUnit"`
	Direction         string          `json:"direction"`
	RemainingPortions int             `json:"remainingPortions"`
	Status            string          `json:"status"`
}

// OrderNotificationPayload ↔ order-service OrderNotificationPayload — the JSON
// published to employee.events with routing keys order.approved / order.declined
// (supervisor decisions) and order.created / order.done / order.partial_fill /
// order.auto_cancelled (lifecycle events). notification-service consumes this
// shape, so field names are load-bearing.
//
// ClientID is the order owner's id (== UserID); notification-service keys the FCM
// device-token lookup on it, so the mobile push reaches client-placed orders.
// Agent/actuary orders set it too but simply have no registered device.
type OrderNotificationPayload struct {
	OrderID           int64             `json:"orderId"`
	Status            string            `json:"status"`
	UserID            int64             `json:"userId"`
	ClientID          int64             `json:"clientId"`
	SupervisorID      int64             `json:"supervisorId"`
	ListingID         int64             `json:"listingId"`
	OrderType         string            `json:"orderType"`
	Direction         string            `json:"direction"`
	Username          *string           `json:"username"`
	UserEmail         *string           `json:"userEmail"`
	TemplateVariables map[string]string `json:"templateVariables"`
}
