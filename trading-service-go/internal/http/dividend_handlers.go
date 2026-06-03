package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/dividend"

	gpauth "banka1/go-platform/auth"
	"banka1/go-platform/httpx"
)

// Dividend payout endpoints (WP-14 Celina 3.7) — mirror DividendController.

// MyDividends ↔ GET /dividends?listingId={optional}: the authenticated user's
// dividend payout history, newest first, always scoped to the JWT id claim.
func (h *Handlers) MyDividends(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	repo := h.app.DividendPayout.Repo()

	var (
		payouts []dividend.Payout
		err     error
	)
	if raw := strings.TrimSpace(r.URL.Query().Get("listingId")); raw != "" {
		listingID, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil {
			writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest,
				"Invalid value '"+raw+"' for parameter 'listingId', expected type: Long."))
			return
		}
		payouts, err = repo.FindByUserIDAndListingID(r.Context(), repo.Pool(), principal.ID, listingID)
	} else {
		payouts, err = repo.FindByUserID(r.Context(), repo.Pool(), principal.ID)
	}
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	out := make([]api.DividendPayoutDto, 0, len(payouts))
	for i := range payouts {
		p := &payouts[i]
		out = append(out, api.DividendPayoutDto{
			ID:           p.ID,
			UserID:       p.UserID,
			StockTicker:  p.StockTicker,
			ListingID:    p.ListingID,
			Quantity:     p.Quantity,
			GrossAmount:  p.GrossAmount,
			Currency:     p.Currency,
			TaxAmountRsd: p.TaxAmountRsd,
			NetAmount:    p.NetAmount,
			AccountID:    p.AccountID,
			PaymentDate:  api.NewLocalDate(p.PaymentDate),
			ForBank:      p.ForBank,
		})
	}
	httpx.JSON(w, http.StatusOK, out)
}

// DividendTrigger ↔ POST /dividends/trigger?asOf={optional ISO date} (ADMIN):
// manual payout run for E2E verification without waiting for the quarterly cron.
func (h *Handlers) DividendTrigger(w http.ResponseWriter, r *http.Request) {
	asOf := time.Now()
	if raw := strings.TrimSpace(r.URL.Query().Get("asOf")); raw != "" {
		parsed, err := time.ParseInLocation("2006-01-02", raw, time.UTC)
		if err != nil {
			writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest,
				"Invalid value '"+raw+"' for parameter 'asOf', expected type: LocalDate."))
			return
		}
		asOf = parsed
	} else {
		asOf = time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)
	}
	paid := h.app.DividendPayout.Distribute(r.Context(), asOf)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"paid": paid,
		"asOf": asOf.Format("2006-01-02"),
	})
}
