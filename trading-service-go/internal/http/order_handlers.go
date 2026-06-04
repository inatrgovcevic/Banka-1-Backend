package http

import (
	"net/http"
	"strings"
	"time"

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

// validListingTypes mirrors the order-service ListingType enum binding for the
// ?listingType filter (invalid value → 400 type-mismatch, like Spring).
var validListingTypes = map[string]bool{
	"STOCK": true, "FUTURES": true, "FOREX": true, "OPTION": true,
}

// OrderMyOrdersPaged ↔ GET /orders/my-orders/paged (Celina 3: filtered, paginated
// mobile My Orders view; Page<OrderResponse>).
func (h *Handlers) OrderMyOrdersPaged(w http.ResponseWriter, r *http.Request) {
	statusFilter, ok := order.ParseStatusFilter(r.URL.Query().Get("status"))
	if !ok {
		raw := r.URL.Query().Get("status")
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest,
			"Invalid value '"+raw+"' for parameter 'status', expected type: OrderOverviewStatusFilter."))
		return
	}
	var listingType *string
	if raw := r.URL.Query().Get("listingType"); raw != "" {
		upper := strings.ToUpper(strings.TrimSpace(raw))
		if !validListingTypes[upper] {
			writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest,
				"Invalid value '"+raw+"' for parameter 'listingType', expected type: ListingType."))
			return
		}
		listingType = &upper
	}
	dateFrom, ok := queryDate(r, "dateFrom")
	if !ok {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest,
			"Invalid value '"+r.URL.Query().Get("dateFrom")+"' for parameter 'dateFrom', expected type: LocalDate."))
		return
	}
	dateTo, ok := queryDate(r, "dateTo")
	if !ok {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest,
			"Invalid value '"+r.URL.Query().Get("dateTo")+"' for parameter 'dateTo', expected type: LocalDate."))
		return
	}
	page := queryIntDefault(r, "page", 0)
	size := queryIntDefault(r, "size", 20)
	if page < 0 || size < 1 || size > 100 {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Request validation failed"))
		return
	}
	resp, err := h.app.Order.GetMyOrdersPaged(r.Context(), authUser(r), statusFilter, listingType, dateFrom, dateTo, page, size)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// queryDate parses an optional ISO date (yyyy-MM-dd) query param. Returns
// (nil, true) when absent, (nil, false) when malformed.
func queryDate(r *http.Request, name string) (*time.Time, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, true
	}
	parsed, err := time.ParseInLocation("2006-01-02", raw, time.UTC)
	if err != nil {
		return nil, false
	}
	return &parsed, true
}

// --- Recurring (standing) orders — Celina 3.6 -------------------------------

// RecurringOrdersList ↔ GET /recurring-orders.
func (h *Handlers) RecurringOrdersList(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	resp, err := h.app.Order.GetRecurringOrders(r.Context(), principal.ID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// RecurringOrderCreate ↔ POST /recurring-orders (201 Created).
func (h *Handlers) RecurringOrderCreate(w http.ResponseWriter, r *http.Request) {
	var req api.CreateRecurringOrderRequest
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}

	// Enum binding: Jackson rejects an unknown enum literal before bean
	// validation runs, surfacing the same malformed-body 400.
	direction, ok := normalizeEnum(req.Direction, order.DirectionBuy, order.DirectionSell)
	if !ok {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	mode, ok := normalizeEnum(req.Mode, order.RecurringModeByQuantity, order.RecurringModeByAmount)
	if !ok {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	cadence, ok := normalizeEnum(req.Cadence, order.CadenceDaily, order.CadenceWeekly, order.CadenceMonthly)
	if !ok {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	var nextRun *time.Time
	if req.NextRun != nil {
		parsed, err := time.ParseInLocation("2006-01-02T15:04:05.999999999", strings.TrimSpace(*req.NextRun), time.UTC)
		if err != nil {
			writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
			return
		}
		nextRun = &parsed
	}

	// Bean validation (@NotNull/@Positive/@Future), mirroring
	// CreateRecurringOrderRequest.
	fields := map[string]string{}
	requireNotNullPositiveInt64(fields, "listingId", req.ListingID)
	if direction == "" {
		fields["direction"] = msgNotNull
	}
	if mode == "" {
		fields["mode"] = msgNotNull
	}
	if req.Value == nil {
		fields["value"] = msgNotNull
	} else if req.Value.Sign() <= 0 {
		fields["value"] = msgPositive
	}
	requireNotNullPositiveInt64(fields, "accountId", req.AccountID)
	if cadence == "" {
		fields["cadence"] = msgNotNull
	}
	if nextRun == nil {
		fields["nextRun"] = msgNotNull
	} else if !nextRun.After(time.Now().UTC()) {
		fields["nextRun"] = "must be a future date"
	}
	if len(fields) > 0 {
		writeDomainError(w, r, api.NewOrderValidation(fields))
		return
	}

	principal, _ := gpauth.PrincipalFromContext(r.Context())
	resp, err := h.app.Order.CreateRecurringOrder(r.Context(), principal.ID,
		*req.ListingID, direction, mode, *req.Value, *req.AccountID, cadence, *nextRun)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

// RecurringOrderPause ↔ PATCH /recurring-orders/{id}/pause.
func (h *Handlers) RecurringOrderPause(w http.ResponseWriter, r *http.Request) {
	h.recurringSetActive(w, r, false)
}

// RecurringOrderResume ↔ PATCH /recurring-orders/{id}/resume.
func (h *Handlers) RecurringOrderResume(w http.ResponseWriter, r *http.Request) {
	h.recurringSetActive(w, r, true)
}

func (h *Handlers) recurringSetActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	var resp api.RecurringOrderDto
	if active {
		resp, err = h.app.Order.ResumeRecurringOrder(r.Context(), principal.ID, id)
	} else {
		resp, err = h.app.Order.PauseRecurringOrder(r.Context(), principal.ID, id)
	}
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// RecurringOrderDelete ↔ DELETE /recurring-orders/{id} (204 No Content).
func (h *Handlers) RecurringOrderDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	if err := h.app.Order.CancelRecurringOrder(r.Context(), principal.ID, id); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// normalizeEnum returns the upper-cased value when it matches one of the allowed
// enum literals; ("", true) for nil/blank (left for @NotNull validation), or
// ("", false) for an unknown literal (Jackson-style malformed body).
func normalizeEnum(raw *string, allowed ...string) (string, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return "", true
	}
	upper := strings.ToUpper(strings.TrimSpace(*raw))
	for _, a := range allowed {
		if upper == a {
			return upper, true
		}
	}
	return "", false
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
