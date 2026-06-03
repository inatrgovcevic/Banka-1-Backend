package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"banka1/trading-service-go/internal/api"

	gpauth "banka1/go-platform/auth"
	"banka1/go-platform/httpx"
)

func (h *Handlers) Watchlists(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		rows, err := h.app.DB.Query(r.Context(), `
			SELECT w.id, w.name, COALESCE(json_agg(json_build_object(
				'id', i.id, 'ticker', i.ticker, 'listingType', i.listing_type
			) ORDER BY i.id) FILTER (WHERE i.id IS NOT NULL), '[]'::json) AS items
			  FROM watchlists w
			  LEFT JOIN watchlist_items i ON i.watchlist_id = w.id AND i.deleted = false
			 WHERE w.user_id = $1 AND w.deleted = false
			 GROUP BY w.id, w.name
			 ORDER BY w.id`, principal.ID)
		if err != nil {
			writeDomainError(w, r, err)
			return
		}
		defer rows.Close()
		out := make([]map[string]any, 0)
		for rows.Next() {
			var id int64
			var name string
			var items json.RawMessage
			if err := rows.Scan(&id, &name, &items); err != nil {
				writeDomainError(w, r, err)
				return
			}
			out = append(out, map[string]any{"id": id, "name": name, "items": json.RawMessage(items)})
		}
		httpx.JSON(w, http.StatusOK, out)
	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			Ticker      string `json:"ticker"`
			ListingType string `json:"listingType"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
			return
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = "Default"
		}
		listingType := strings.ToUpper(strings.TrimSpace(req.ListingType))
		if listingType == "" {
			listingType = "STOCK"
		}
		var id int64
		err := h.app.DB.QueryRow(r.Context(), `
			INSERT INTO watchlists(user_id, name) VALUES ($1, $2)
			ON CONFLICT (user_id, name) DO UPDATE SET updated_at = now(), deleted = false
			RETURNING id`, principal.ID, name).Scan(&id)
		if err != nil {
			writeDomainError(w, r, err)
			return
		}
		ticker := strings.ToUpper(strings.TrimSpace(req.Ticker))
		if ticker != "" {
			_, err = h.app.DB.Exec(r.Context(), `
				INSERT INTO watchlist_items(watchlist_id, ticker, listing_type) VALUES ($1, $2, $3)
				ON CONFLICT (watchlist_id, ticker) DO UPDATE SET listing_type = EXCLUDED.listing_type, deleted = false`,
				id, ticker, listingType)
			if err != nil {
				writeDomainError(w, r, err)
				return
			}
		}
		httpx.JSON(w, http.StatusCreated, map[string]any{"id": id, "name": name})
	default:
		writeDomainError(w, r, api.NewOrderError(http.StatusMethodNotAllowed, "Method not allowed"))
	}
}

func (h *Handlers) WatchlistItemDelete(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Invalid watchlist item id"))
		return
	}
	_, err = h.app.DB.Exec(r.Context(), `
		UPDATE watchlist_items i SET deleted = true
		  FROM watchlists w
		 WHERE i.watchlist_id = w.id AND w.user_id = $1 AND i.id = $2`, principal.ID, id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

func (h *Handlers) PriceAlerts(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		rows, err := h.app.DB.Query(r.Context(), `
			SELECT id, ticker, condition, threshold::text, notification_type, active, created_at, triggered_at
			  FROM price_alerts WHERE user_id = $1 ORDER BY id DESC`, principal.ID)
		if err != nil {
			writeDomainError(w, r, err)
			return
		}
		defer rows.Close()
		out := make([]map[string]any, 0)
		for rows.Next() {
			var id int64
			var ticker, condition, threshold, notificationType string
			var active bool
			var createdAt time.Time
			var triggeredAt *time.Time
			if err := rows.Scan(&id, &ticker, &condition, &threshold, &notificationType, &active, &createdAt, &triggeredAt); err != nil {
				writeDomainError(w, r, err)
				return
			}
			out = append(out, map[string]any{"id": id, "ticker": ticker, "condition": condition, "threshold": threshold, "notificationType": notificationType, "active": active, "createdAt": createdAt, "triggeredAt": triggeredAt})
		}
		httpx.JSON(w, http.StatusOK, out)
	case http.MethodPost:
		var req struct {
			Ticker           string `json:"ticker"`
			Condition        string `json:"condition"`
			Threshold        string `json:"threshold"`
			NotificationType string `json:"notificationType"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
			return
		}
		ticker := strings.ToUpper(strings.TrimSpace(req.Ticker))
		condition := strings.ToUpper(strings.TrimSpace(req.Condition))
		if ticker == "" || (condition != "ABOVE" && condition != "BELOW") || strings.TrimSpace(req.Threshold) == "" {
			writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "ticker, condition ABOVE/BELOW and threshold are required"))
			return
		}
		notificationType := strings.ToUpper(strings.TrimSpace(req.NotificationType))
		if notificationType == "" {
			notificationType = "EMAIL"
		}
		var id int64
		err := h.app.DB.QueryRow(r.Context(), `
			INSERT INTO price_alerts(user_id, ticker, condition, threshold, notification_type)
			VALUES ($1, $2, $3, $4::numeric, $5) RETURNING id`,
			principal.ID, ticker, condition, req.Threshold, notificationType).Scan(&id)
		if err != nil {
			writeDomainError(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusCreated, map[string]any{"id": id, "ticker": ticker, "condition": condition, "threshold": req.Threshold, "active": true})
	default:
		writeDomainError(w, r, api.NewOrderError(http.StatusMethodNotAllowed, "Method not allowed"))
	}
}

// (The C3 audit-log stub handler was superseded by the WP-2 / Issue 9 audit
// port — internal/audit + audit_handlers.go: GET /audit serves the Java-parity
// Page contract; GET /audit-log serves the legacy flat view with the
// actorName/targetName enrichment, now derived from the reshaped audit_log
// schema.)

// (The C3 recurring-orders stub handlers were replaced by the real Celina 3.6
// port — typed handlers in order_handlers.go over internal/order/recurring.go,
// routes under /recurring-orders matching the Java RecurringOrderController.
// The /portfolio/dividends stub was likewise superseded by the WP-14 dividend
// port: GET /dividends over internal/dividend, dividend_handlers.go.)
