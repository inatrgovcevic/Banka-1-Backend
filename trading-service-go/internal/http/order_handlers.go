package http

import (
	"net/http"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/order"

	gpauth "banka1/go-platform/auth"
	"banka1/go-platform/httpx"
	"github.com/shopspring/decimal"
)

const (
	msgNotNull  = "must not be null"
	msgPositive = "must be greater than 0"
)

func requireNotNullPositiveInt64(fields map[string]string, name string, v *int64) {
	if v == nil {
		fields[name] = msgNotNull
		return
	}
	if *v <= 0 {
		fields[name] = msgPositive
	}
}

func requireNotNullPositiveInt(fields map[string]string, name string, v *int) {
	if v == nil {
		fields[name] = msgNotNull
		return
	}
	if *v <= 0 {
		fields[name] = msgPositive
	}
}

func requirePositiveInt64IfPresent(fields map[string]string, name string, v *int64) {
	if v != nil && *v <= 0 {
		fields[name] = msgPositive
	}
}

func requirePositiveDecimal(fields map[string]string, name string, v *decimal.Decimal) {
	if v != nil && v.Sign() <= 0 {
		fields[name] = msgPositive
	}
}

// authUser builds the order-module AuthUser from the JWT principal. The platform
// principal carries a single role string; the order package treats Roles as a set.
func authUser(r *http.Request) order.AuthUser {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	roles := []string{}
	if principal.Role != "" {
		roles = []string{principal.Role}
	}
	return order.AuthUser{UserID: principal.ID, Roles: roles, Permissions: principal.Permissions}
}

// OrderBuy ↔ POST /orders/buy.
func (h *Handlers) OrderBuy(w http.ResponseWriter, r *http.Request) {
	var req api.CreateBuyOrderRequest
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	if fields := validateCreateBuy(req); fields != nil {
		writeDomainError(w, r, api.NewOrderValidation(fields))
		return
	}
	resp, err := h.app.Order.CreateBuyOrder(r.Context(), authUser(r), req)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// OrderSell ↔ POST /orders/sell.
func (h *Handlers) OrderSell(w http.ResponseWriter, r *http.Request) {
	var req api.CreateSellOrderRequest
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	if fields := validateCreateSell(req); fields != nil {
		writeDomainError(w, r, api.NewOrderValidation(fields))
		return
	}
	resp, err := h.app.Order.CreateSellOrder(r.Context(), authUser(r), req)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// OrderList ↔ GET /orders (supervisor portal overview, Page<OrderOverviewResponse>).
func (h *Handlers) OrderList(w http.ResponseWriter, r *http.Request) {
	statusFilter, ok := order.ParseStatusFilter(r.URL.Query().Get("status"))
	if !ok {
		raw := r.URL.Query().Get("status")
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest,
			"Invalid value '"+raw+"' for parameter 'status', expected type: OrderOverviewStatusFilter."))
		return
	}
	page := queryIntDefault(r, "page", 0)
	size := queryIntDefault(r, "size", 10)
	resp, err := h.app.Order.GetOrders(r.Context(), statusFilter, page, size)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// OrderMyOrders ↔ GET /orders/my-orders (List<OrderResponse>).
func (h *Handlers) OrderMyOrders(w http.ResponseWriter, r *http.Request) {
	resp, err := h.app.Order.GetMyOrders(r.Context(), authUser(r))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// OrderConfirm ↔ POST /orders/{id}/confirm.
func (h *Handlers) OrderConfirm(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	resp, err := h.app.Order.ConfirmOrder(r.Context(), authUser(r), id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// OrderCancel ↔ POST /orders/{id}/cancel (owner cancels the whole remainder).
func (h *Handlers) OrderCancel(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	resp, err := h.app.Order.CancelOrder(r.Context(), authUser(r), id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// OrderCancelSupervisor ↔ PUT /orders/{id}/cancel (supervisor; optional partial
// quantity in the body).
func (h *Handlers) OrderCancelSupervisor(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	var req api.PartialCancelOrderRequest
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	resp, err := h.app.Order.CancelOrderSupervisor(r.Context(), id, req.Quantity)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// OrderApprove ↔ PUT /orders/{id}/approve (supervisor).
func (h *Handlers) OrderApprove(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	resp, err := h.app.Order.ApproveOrder(r.Context(), principal.ID, id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// OrderDecline ↔ PUT /orders/{id}/decline (supervisor).
func (h *Handlers) OrderDecline(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	resp, err := h.app.Order.DeclineOrder(r.Context(), principal.ID, id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// validateCreateBuy reproduces CreateBuyOrderRequest bean validation: listingId
// @NotNull @Positive, quantity @NotNull @Positive, limitValue/stopValue/accountId/
// fundId @Positive. Returns nil when valid.
//
// NOTE: the exact Hibernate Validator default messages ("must not be null",
// "must be greater than 0") are version-sensitive — confirm against the live Java
// service during the P3 parity sweep and adjust if they differ.
func validateCreateBuy(req api.CreateBuyOrderRequest) map[string]string {
	fields := map[string]string{}
	requireNotNullPositiveInt64(fields, "listingId", req.ListingID)
	requireNotNullPositiveInt(fields, "quantity", req.Quantity)
	requirePositiveDecimal(fields, "limitValue", req.LimitValue)
	requirePositiveDecimal(fields, "stopValue", req.StopValue)
	requirePositiveInt64IfPresent(fields, "accountId", req.AccountID)
	requirePositiveInt64IfPresent(fields, "fundId", req.FundID)
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// validateCreateSell reproduces CreateSellOrderRequest bean validation: listingId
// @NotNull @Positive, quantity @NotNull @Positive, accountId @NotNull @Positive,
// limitValue/stopValue @Positive.
func validateCreateSell(req api.CreateSellOrderRequest) map[string]string {
	fields := map[string]string{}
	requireNotNullPositiveInt64(fields, "listingId", req.ListingID)
	requireNotNullPositiveInt(fields, "quantity", req.Quantity)
	requirePositiveDecimal(fields, "limitValue", req.LimitValue)
	requirePositiveDecimal(fields, "stopValue", req.StopValue)
	requireNotNullPositiveInt64(fields, "accountId", req.AccountID)
	if len(fields) == 0 {
		return nil
	}
	return fields
}
