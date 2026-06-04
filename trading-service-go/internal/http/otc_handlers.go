package http

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/otc"

	gpauth "banka1/go-platform/auth"
	"banka1/go-platform/httpx"
	"github.com/shopspring/decimal"
)

// ============================ /otc (authenticated) ========================

// OtcCreateOffer ↔ POST /otc/offers (201). Buyer's initial offer.
func (h *Handlers) OtcCreateOffer(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	var req struct {
		StockTicker    string           `json:"stockTicker"`
		SellerID       *int64           `json:"sellerId"`
		Amount         *int             `json:"amount"`
		PricePerStock  *decimal.Decimal `json:"pricePerStock"`
		Premium        *decimal.Decimal `json:"premium"`
		SettlementDate string           `json:"settlementDate"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	fields := map[string]string{}
	if strings.TrimSpace(req.StockTicker) == "" {
		fields["stockTicker"] = "must not be blank"
	}
	if req.SellerID == nil {
		fields["sellerId"] = "must not be null"
	}
	settle := validateOfferTerms(fields, req.Amount, req.PricePerStock, req.Premium, req.SettlementDate)
	if len(fields) > 0 {
		writeDomainError(w, r, api.NewOrderValidation(fields))
		return
	}
	in := otc.CreateOfferInput{
		StockTicker:    strings.TrimSpace(req.StockTicker),
		SellerID:       *req.SellerID,
		Amount:         *req.Amount,
		PricePerStock:  *req.PricePerStock,
		Premium:        *req.Premium,
		SettlementDate: settle,
	}
	dto, err := h.app.Otc.CreateOffer(r.Context(), principal.ID, in, tokenNameClaim(principal))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, dto)
}

// OtcCounterOffer ↔ POST /otc/offers/{offerId}/counter (200).
func (h *Handlers) OtcCounterOffer(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	offerID, err := parsePathInt64(r, "offerId")
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	var req struct {
		Amount         *int             `json:"amount"`
		PricePerStock  *decimal.Decimal `json:"pricePerStock"`
		Premium        *decimal.Decimal `json:"premium"`
		SettlementDate string           `json:"settlementDate"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	fields := map[string]string{}
	settle := validateOfferTerms(fields, req.Amount, req.PricePerStock, req.Premium, req.SettlementDate)
	if len(fields) > 0 {
		writeDomainError(w, r, api.NewOrderValidation(fields))
		return
	}
	in := otc.CounterOfferInput{
		Amount:         *req.Amount,
		PricePerStock:  *req.PricePerStock,
		Premium:        *req.Premium,
		SettlementDate: settle,
	}
	dto, err := h.app.Otc.CounterOffer(r.Context(), offerID, principal.ID, in, tokenNameClaim(principal))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dto)
}

// OtcAcceptOffer ↔ POST /otc/offers/{offerId}/accept (200).
func (h *Handlers) OtcAcceptOffer(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	offerID, err := parsePathInt64(r, "offerId")
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	dto, err := h.app.Otc.Accept(r.Context(), offerID, principal.ID, tokenNameClaim(principal))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dto)
}

// OtcRejectOffer ↔ POST /otc/offers/{offerId}/reject (200).
func (h *Handlers) OtcRejectOffer(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	offerID, err := parsePathInt64(r, "offerId")
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	dto, err := h.app.Otc.Reject(r.Context(), offerID, principal.ID, tokenNameClaim(principal))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dto)
}

// OtcWithdrawOffer ↔ POST /otc/offers/{offerId}/withdraw (200).
func (h *Handlers) OtcWithdrawOffer(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	offerID, err := parsePathInt64(r, "offerId")
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	dto, err := h.app.Otc.Withdraw(r.Context(), offerID, principal.ID, tokenNameClaim(principal))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dto)
}

// OtcActiveOffers ↔ GET /otc/offers/active (200).
func (h *Handlers) OtcActiveOffers(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	out, err := h.app.Otc.ActiveForUser(r.Context(), principal.ID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// OtcPublicStocks ↔ GET /otc/public-stocks (200). Supervisors see only
// actuary-exposed stocks (role-based view, not a role gate).
func (h *Handlers) OtcPublicStocks(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	supervisorView := strings.EqualFold(principal.Role, "SUPERVISOR")
	out, err := h.app.Otc.GetPublicStocks(r.Context(), principal.ID, supervisorView)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// OtcExerciseContract ↔ POST /otc/contracts/{contractId}/exercise (202).
// Also aliased to POST /options/{contractId}/exercise for SAGA test spec.
// Returns {"correlationId":"<contractId>"} so callers can poll the saga log.
func (h *Handlers) OtcExerciseContract(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	contractID, err := parsePathInt64(r, "contractId")
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	fi := parseFaultInjection(r)
	corrID, err := h.app.Otc.ExerciseContract(r.Context(), contractID, principal.ID, fi)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]string{
		"correlationId": strconv.FormatInt(corrID, 10),
	})
}

// parseFaultInjection extracts X-Saga-* headers into a FaultInjection struct.
// Returns nil when none are present or SAGA_TEST_MODE is not enabled.
func parseFaultInjection(r *http.Request) *otc.FaultInjection {
	if r.Header.Get("X-Saga-Force-Fail") == "" &&
		r.Header.Get("X-Saga-Compensate-Fail") == "" &&
		r.Header.Get("X-Saga-Inject-Delay") == "" {
		return nil
	}
	fi := &otc.FaultInjection{}
	fi.ForceFailStep = r.Header.Get("X-Saga-Force-Fail")
	fi.ForceFailKind = r.Header.Get("X-Saga-Force-Fail-Kind")
	fi.CompensateFailStep = r.Header.Get("X-Saga-Compensate-Fail")
	if n, err := strconv.Atoi(r.Header.Get("X-Saga-Compensate-Fail-Times")); err == nil {
		fi.CompensateFailTimes = n
	}
	// X-Saga-Inject-Delay format: "F3:5000"
	if raw := r.Header.Get("X-Saga-Inject-Delay"); raw != "" {
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) == 2 {
			fi.InjectDelayStep = parts[0]
			if ms, err := strconv.Atoi(parts[1]); err == nil {
				fi.InjectDelayMs = ms
			}
		}
	}
	return fi
}

// OtcMyContracts ↔ GET /otc/contracts/my (200). Optional ?status filter.
func (h *Handlers) OtcMyContracts(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	var statusFilter *string
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		if !isValidContractStatus(raw) {
			writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest,
				"Invalid value '"+raw+"' for parameter 'status'."))
			return
		}
		statusFilter = &raw
	}
	out, err := h.app.Otc.MyContracts(r.Context(), principal.ID, statusFilter)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// OtcMyPositions ↔ GET /otc/my-positions (200).
func (h *Handlers) OtcMyPositions(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	out, err := h.app.Otc.GetMyPositions(r.Context(), principal.ID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// OtcAddPosition ↔ POST /otc/positions (201).
func (h *Handlers) OtcAddPosition(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	var req struct {
		ListingID      *int64 `json:"listingId"`
		PublicQuantity *int   `json:"publicQuantity"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	fields := map[string]string{}
	if req.ListingID == nil {
		fields["listingId"] = "must not be null"
	}
	validatePublicQuantity(fields, req.PublicQuantity)
	if len(fields) > 0 {
		writeDomainError(w, r, api.NewOrderValidation(fields))
		return
	}
	dto, err := h.app.Otc.AddPosition(r.Context(), principal.ID, *req.ListingID, *req.PublicQuantity)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, dto)
}

// OtcUpdatePosition ↔ PUT /otc/positions/{positionId} (200).
func (h *Handlers) OtcUpdatePosition(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	positionID, err := parsePathInt64(r, "positionId")
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	var req struct {
		PublicQuantity *int `json:"publicQuantity"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	fields := map[string]string{}
	validatePublicQuantity(fields, req.PublicQuantity)
	if len(fields) > 0 {
		writeDomainError(w, r, api.NewOrderValidation(fields))
		return
	}
	dto, err := h.app.Otc.UpdatePosition(r.Context(), principal.ID, positionID, *req.PublicQuantity)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dto)
}

// OtcRemovePosition ↔ DELETE /otc/positions/{positionId} (204).
func (h *Handlers) OtcRemovePosition(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	positionID, err := parsePathInt64(r, "positionId")
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	if err := h.app.Otc.RemovePosition(r.Context(), principal.ID, positionID); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// OtcNegotiationHistory ↔ GET /otc/offers/history (200). Filterable by status /
// otherPartyId / dateFrom / dateTo query params.
func (h *Handlers) OtcNegotiationHistory(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	var status *string
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		if !isValidOfferStatus(raw) {
			writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest,
				"Invalid value '"+raw+"' for parameter 'status'."))
			return
		}
		status = &raw
	}
	var otherPartyID *int64
	if raw := strings.TrimSpace(r.URL.Query().Get("otherPartyId")); raw != "" {
		v, perr := strconv.ParseInt(raw, 10, 64)
		if perr != nil {
			writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest,
				"Invalid value '"+raw+"' for parameter 'otherPartyId', expected type: Long."))
			return
		}
		otherPartyID = &v
	}
	dateFrom, err := parseLocalDateParam(r, "dateFrom")
	if err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Invalid dateFrom (expected ISO yyyy-MM-dd)."))
		return
	}
	dateTo, err := parseLocalDateParam(r, "dateTo")
	if err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Invalid dateTo (expected ISO yyyy-MM-dd)."))
		return
	}
	out, err := h.app.Otc.HistoryForUser(r.Context(), principal.ID, status, otherPartyID, dateFrom, dateTo)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// ===================== /stocks/internal (PUBLIC, saga) ====================

// StocksInternalReserve ↔ POST /stocks/internal/reserve (200).
func (h *Handlers) StocksInternalReserve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OwnerID     int64  `json:"ownerId"`
		StockTicker string `json:"stockTicker"`
		Amount      int    `json:"amount"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	resp, err := h.app.OtcReservation.Reserve(r.Context(), req.OwnerID, req.StockTicker, req.Amount, correlationID(r))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// StocksInternalRelease ↔ DELETE /stocks/internal/reservations/{id} (200). id is
// the reservation UUID (string), not numeric.
func (h *Handlers) StocksInternalRelease(w http.ResponseWriter, r *http.Request) {
	reservationID := r.PathValue("id")
	resp, err := h.app.OtcReservation.Release(r.Context(), reservationID, correlationID(r))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// StocksInternalTransfer ↔ POST /stocks/internal/reservations/{id}/transfer (200).
func (h *Handlers) StocksInternalTransfer(w http.ResponseWriter, r *http.Request) {
	reservationID := r.PathValue("id")
	var req struct {
		BuyerID int64 `json:"buyerId"`
	}
	if err := decodeJSONLenient(r, &req); err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
		return
	}
	resp, err := h.app.OtcReservation.TransferOwnership(r.Context(), reservationID, req.BuyerID, correlationID(r))
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// StocksInternalReverse ↔ POST /stocks/internal/ownership-transfers/{id}/reverse
// (200, empty body — matches Java ResponseEntity.ok().build()).
func (h *Handlers) StocksInternalReverse(w http.ResponseWriter, r *http.Request) {
	transferID := r.PathValue("id")
	if err := h.app.OtcReservation.ReverseOwnership(r.Context(), transferID, correlationID(r)); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// =============================== helpers ==================================

// parsePathInt64 reads a named path variable as int64, mirroring Spring's
// MethodArgumentTypeMismatch → 400 (order ApiErrorResponse) for a non-numeric id.
func parsePathInt64(r *http.Request, name string) (int64, error) {
	raw := r.PathValue(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, api.NewOrderError(http.StatusBadRequest,
			"Invalid value '"+raw+"' for parameter '"+name+"', expected type: Long.")
	}
	return id, nil
}

// correlationID reads the X-Correlation-Id header the saga-orchestrator sets,
// defaulting to "no-correlation" (matches the Java InternalStockController default).
func correlationID(r *http.Request) string {
	if id := strings.TrimSpace(r.Header.Get("X-Correlation-Id")); id != "" {
		return id
	}
	return "no-correlation"
}

// tokenNameClaim peeks the "name" claim from the already-verified bearer token
// (the Principal carries the raw JWT; the middleware verified it). OtcController
// uses jwt.getClaim("name") for the offer modifiedBy on create/counter. Returns nil
// when the claim is absent — modifiedBy then renders JSON null, matching Java's
// null claim.
func tokenNameClaim(p gpauth.Principal) *string {
	parts := strings.Split(p.Token, ".")
	if len(parts) != 3 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}
	if v, ok := claims["name"].(string); ok {
		return &v
	}
	return nil
}

// parseLocalDateParam parses an optional ISO yyyy-MM-dd query param. Absent/empty →
// (nil, nil); present but unparseable → (nil, error) so the caller returns 400.
func parseLocalDateParam(r *http.Request, key string) (*time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// validateOfferTerms fills fields with bean-validation-style messages for the four
// shared offer terms and returns the parsed settlementDate. (Exact Hibernate
// default strings are version-sensitive — confirm during the parity sweep, same
// caveat as the P3/P5 create-request validations.)
func validateOfferTerms(fields map[string]string, amount *int, pricePerStock, premium *decimal.Decimal, settlementDate string) time.Time {
	if amount == nil {
		fields["amount"] = "must not be null"
	} else if *amount < 1 {
		fields["amount"] = "must be greater than or equal to 1"
	}
	if pricePerStock == nil {
		fields["pricePerStock"] = "must not be null"
	} else if pricePerStock.Sign() <= 0 {
		fields["pricePerStock"] = "must be greater than 0.00"
	}
	if premium == nil {
		fields["premium"] = "must not be null"
	} else if premium.Sign() < 0 {
		fields["premium"] = "must be greater than or equal to 0.00"
	}
	var settle time.Time
	if strings.TrimSpace(settlementDate) == "" {
		fields["settlementDate"] = "must not be null"
		return settle
	}
	t, err := time.Parse("2006-01-02", strings.TrimSpace(settlementDate))
	if err != nil {
		fields["settlementDate"] = "must be a valid ISO date (yyyy-MM-dd)"
		return settle
	}
	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC)
	if !t.After(today) {
		fields["settlementDate"] = "must be a future date"
	}
	return t
}

func validatePublicQuantity(fields map[string]string, publicQuantity *int) {
	if publicQuantity == nil {
		fields["publicQuantity"] = "must not be null"
	} else if *publicQuantity < 1 {
		fields["publicQuantity"] = "must be greater than or equal to 1"
	}
}

func isValidContractStatus(s string) bool {
	switch s {
	case otc.ContractPendingPremium, otc.ContractActive, otc.ContractExercised, otc.ContractExpired, otc.ContractCanceled:
		return true
	}
	return false
}

func isValidOfferStatus(s string) bool {
	switch s {
	case otc.OfferPendingSeller, otc.OfferPendingBuyer, otc.OfferAccepted, otc.OfferRejected, otc.OfferWithdrawn, otc.OfferExpired:
		return true
	}
	return false
}
