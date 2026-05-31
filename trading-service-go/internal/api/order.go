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

// OrderResponse ↔ order-service OrderResponse. Nullable fields (approvedBy,
// limitValue, stopValue) are pointers.
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
	LastModification  LocalDateTime    `json:"lastModification"`
	RemainingPortions int              `json:"remainingPortions"`
	AfterHours        bool             `json:"afterHours"`
	ExchangeClosed    bool             `json:"exchangeClosed"`
	AllOrNone         bool             `json:"allOrNone"`
	Margin            bool             `json:"margin"`
	AccountID         int64            `json:"accountId"`
	ApproximatePrice  decimal.Decimal  `json:"approximatePrice"`
	Fee               decimal.Decimal  `json:"fee"`
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
// published to employee.events with routing key order.approved / order.declined.
// notification-service consumes this shape, so field names are load-bearing.
type OrderNotificationPayload struct {
	OrderID           int64             `json:"orderId"`
	Status            string            `json:"status"`
	UserID            int64             `json:"userId"`
	SupervisorID      int64             `json:"supervisorId"`
	ListingID         int64             `json:"listingId"`
	OrderType         string            `json:"orderType"`
	Direction         string            `json:"direction"`
	Username          *string           `json:"username"`
	UserEmail         *string           `json:"userEmail"`
	TemplateVariables map[string]string `json:"templateVariables"`
}
