package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	gpauth "banka1/go-platform/auth"
	"banka1/trading-service-go/internal/audit"
	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/clients"

	"banka1/go-platform/httpx"
)

// TaxCollect ↔ POST /tax/collect. Supervisor manual trigger of the previous-month
// collection. 200 with empty body (ResponseEntity.ok().build()). A strict-FX
// failure during the OTC pass surfaces as a 409 (OTC error shape), exactly as the
// live JVM does.
func (h *Handlers) TaxCollect(w http.ResponseWriter, r *http.Request) {
	if err := h.app.Tax.CollectMonthlyTaxManually(r.Context()); err != nil {
		writeDomainError(w, r, err)
		return
	}
	if h.app.Audit != nil {
		if principal, ok := gpauth.PrincipalFromContext(r.Context()); ok {
			actorID := principal.ID
			actorName := principal.Email
			if actorName == "" {
				actorName = principal.Subject
			}
			if emp, err := h.app.Employees.GetEmployee(clients.WithCallerAuth(r.Context(), r.Header.Get("Authorization")), actorID); err == nil && emp != nil {
				parts := []string{}
				if emp.Ime != nil {
					parts = append(parts, strings.TrimSpace(*emp.Ime))
				}
				if emp.Prezime != nil {
					parts = append(parts, strings.TrimSpace(*emp.Prezime))
				}
				if name := strings.TrimSpace(strings.Join(parts, " ")); name != "" {
					actorName = name
				}
			}
			targetType := "TAX"
			targetID := time.Now().UTC().Format("2006-01")
			details := "Rucni obracun poreza pokrenut."
			ts := time.Now().UnixMilli()
			h.app.Audit.RecordBestEffort(r.Context(), audit.Event{
				ActorID:    &actorID,
				ActorName:  &actorName,
				ActionType: audit.ActionTaxRunManual,
				TargetType: &targetType,
				TargetID:   &targetID,
				Details:    &details,
				Timestamp:  &ts,
			})
		}
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// TaxCollectCurrentMonth ↔ POST /tax/collect/current-month (MTD, idempotent).
func (h *Handlers) TaxCollectCurrentMonth(w http.ResponseWriter, r *http.Request) {
	if err := h.app.Tax.CollectCurrentMonthTax(r.Context()); err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// TaxRunInternal ↔ POST /internal/tax/capital-gains/run (SERVICE-to-service trigger
// of the previous-month collection).
func (h *Handlers) TaxRunInternal(w http.ResponseWriter, r *http.Request) {
	if err := h.app.Tax.CollectMonthlyTax(r.Context()); err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

// TaxDebts ↔ GET /tax/capital-gains/debts (Page<TaxDebtResponse>). page/size
// default 0/10 (TaxController is not @Validated, so @Min/@Max are not enforced).
func (h *Handlers) TaxDebts(w http.ResponseWriter, r *http.Request) {
	page := queryIntDefault(r, "page", 0)
	size := queryIntDefault(r, "size", 10)
	resp, err := h.app.Tax.GetAllDebts(r.Context(), page, size)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// TaxUserDebt ↔ GET /tax/capital-gains/{userId} (TaxDebtResponse). A non-numeric id
// mirrors Spring's MethodArgumentTypeMismatchException -> 400 (order error shape).
func (h *Handlers) TaxUserDebt(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("userId")
	userID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest,
			"Invalid value '"+raw+"' for parameter 'userId', expected type: Long."))
		return
	}
	resp, err := h.app.Tax.GetUserDebt(r.Context(), userID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// TaxTracking ↔ GET /tax/tracking (Page<TaxTrackingRowResponse>). userType/
// firstName/lastName are optional @RequestParam(required=false); absent => nil.
func (h *Handlers) TaxTracking(w http.ResponseWriter, r *http.Request) {
	userType := optionalQueryParam(r, "userType")
	firstName := optionalQueryParam(r, "firstName")
	lastName := optionalQueryParam(r, "lastName")
	page := queryIntDefault(r, "page", 0)
	size := queryIntDefault(r, "size", 10)
	resp, err := h.app.Tax.GetTaxTracking(r.Context(), userType, firstName, lastName, page, size)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}
