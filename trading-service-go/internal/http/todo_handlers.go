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

func (h *Handlers) AuditLog(w http.ResponseWriter, r *http.Request) {
	rows, err := h.app.DB.Query(r.Context(), `
		SELECT id, actor_id, actor_role, action_type, target_type, target_id, old_value, new_value, created_at
		  FROM audit_log
		 ORDER BY created_at DESC
		 LIMIT 100`)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var actorID *int64
		var actorRole, actionType, targetType, targetID, oldValue, newValue *string
		var createdAt time.Time
		if err := rows.Scan(&id, &actorID, &actorRole, &actionType, &targetType, &targetID, &oldValue, &newValue, &createdAt); err != nil {
			writeDomainError(w, r, err)
			return
		}
		out = append(out, map[string]any{"id": id, "actorId": actorID, "actorRole": actorRole, "actionType": actionType, "targetType": targetType, "targetId": targetID, "oldValue": oldValue, "newValue": newValue, "createdAt": createdAt})
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handlers) RecurringOrders(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		rows, err := h.app.DB.Query(r.Context(), `
			SELECT id, ticker, mode, COALESCE(amount::text, ''), quantity, interval, account_number, active, next_run, created_at
			  FROM recurring_orders WHERE user_id = $1 ORDER BY id DESC`, principal.ID)
		if err != nil {
			writeDomainError(w, r, err)
			return
		}
		defer rows.Close()
		out := make([]map[string]any, 0)
		for rows.Next() {
			var id int64
			var ticker, mode, amount, interval string
			var quantity *int64
			var accountNumber *string
			var active bool
			var nextRun, createdAt time.Time
			if err := rows.Scan(&id, &ticker, &mode, &amount, &quantity, &interval, &accountNumber, &active, &nextRun, &createdAt); err != nil {
				writeDomainError(w, r, err)
				return
			}
			out = append(out, map[string]any{"id": id, "ticker": ticker, "mode": mode, "amount": amount, "quantity": quantity, "interval": interval, "accountNumber": accountNumber, "active": active, "nextRun": nextRun, "createdAt": createdAt})
		}
		httpx.JSON(w, http.StatusOK, out)
	case http.MethodPost:
		var req struct {
			Ticker        string `json:"ticker"`
			Mode          string `json:"mode"`
			Amount        string `json:"amount"`
			Quantity      *int64 `json:"quantity"`
			Interval      string `json:"interval"`
			AccountNumber string `json:"accountNumber"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "Malformed JSON request body"))
			return
		}
		ticker := strings.ToUpper(strings.TrimSpace(req.Ticker))
		mode := strings.ToUpper(strings.TrimSpace(req.Mode))
		interval := strings.ToUpper(strings.TrimSpace(req.Interval))
		if interval == "" {
			interval = "MONTHLY"
		}
		if ticker == "" || (mode != "BYAMOUNT" && mode != "BYQUANTITY") {
			writeDomainError(w, r, api.NewOrderError(http.StatusBadRequest, "ticker and mode BYAMOUNT/BYQUANTITY are required"))
			return
		}
		nextRun := time.Now().AddDate(0, 1, 0)
		if interval == "WEEKLY" {
			nextRun = time.Now().AddDate(0, 0, 7)
		} else if interval == "DAILY" {
			nextRun = time.Now().AddDate(0, 0, 1)
		}
		amount := strings.TrimSpace(req.Amount)
		if amount == "" {
			amount = "0"
		}
		var id int64
		err := h.app.DB.QueryRow(r.Context(), `
			INSERT INTO recurring_orders(user_id, ticker, mode, amount, quantity, interval, account_number, next_run)
			VALUES ($1, $2, $3, NULLIF($4, '0')::numeric, $5, $6, NULLIF($7, ''), $8)
			RETURNING id`, principal.ID, ticker, mode, amount, req.Quantity, interval, strings.TrimSpace(req.AccountNumber), nextRun).Scan(&id)
		if err != nil {
			writeDomainError(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusCreated, map[string]any{"id": id, "ticker": ticker, "mode": mode, "interval": interval, "active": true, "nextRun": nextRun})
	default:
		writeDomainError(w, r, api.NewOrderError(http.StatusMethodNotAllowed, "Method not allowed"))
	}
}

func (h *Handlers) RecurringOrderPause(w http.ResponseWriter, r *http.Request) {
	h.setRecurringOrderActive(w, r, false)
}

func (h *Handlers) RecurringOrderResume(w http.ResponseWriter, r *http.Request) {
	h.setRecurringOrderActive(w, r, true)
}

func (h *Handlers) RecurringOrderDelete(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	_, err = h.app.DB.Exec(r.Context(), `DELETE FROM recurring_orders WHERE id = $1 AND user_id = $2`, id, principal.ID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}

func (h *Handlers) setRecurringOrderActive(w http.ResponseWriter, r *http.Request, active bool) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	id, err := parsePathID(r)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	cmd, err := h.app.DB.Exec(r.Context(), `UPDATE recurring_orders SET active = $3, updated_at = now() WHERE id = $1 AND user_id = $2`, id, principal.ID, active)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	if cmd.RowsAffected() == 0 {
		writeDomainError(w, r, api.NewOrderError(http.StatusNotFound, "Recurring order not found"))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": id, "active": active})
}

func (h *Handlers) PortfolioDividends(w http.ResponseWriter, r *http.Request) {
	principal, _ := gpauth.PrincipalFromContext(r.Context())
	rows, err := h.app.DB.Query(r.Context(), `
		SELECT d.id, d.portfolio_id, d.ticker, d.quantity, d.price::text, d.dividend_yield::text, d.amount::text, d.currency, d.paid_at
		  FROM dividend_payouts d WHERE d.user_id = $1
		 UNION ALL
		SELECT 0, p.id, COALESCE(NULLIF(p.listing_type, ''), 'STOCK') || '-' || p.listing_id::text,
		       p.quantity, p.average_purchase_price::text, '0'::text, '0'::text, 'USD', p.last_modified
		  FROM portfolio p
		 WHERE p.user_id = $1 AND NOT EXISTS (SELECT 1 FROM dividend_payouts d WHERE d.user_id = $1)
		 ORDER BY paid_at DESC`, principal.ID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, portfolioID int64
		var ticker, price, dividendYield, amount, currency string
		var quantity int64
		var paidAt time.Time
		if err := rows.Scan(&id, &portfolioID, &ticker, &quantity, &price, &dividendYield, &amount, &currency, &paidAt); err != nil {
			writeDomainError(w, r, err)
			return
		}
		out = append(out, map[string]any{"id": id, "portfolioId": portfolioID, "ticker": ticker, "quantity": quantity, "price": price, "dividendYield": dividendYield, "amount": amount, "currency": currency, "paidAt": paidAt})
	}
	httpx.JSON(w, http.StatusOK, out)
}
