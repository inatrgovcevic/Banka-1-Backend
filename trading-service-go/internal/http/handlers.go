package http

import (
	"errors"
	"net/http"
	"strconv"

	"banka1/trading-service-go/internal/analytics"

	"banka1/go-platform/httpx"
)

// Handlers holds HTTP handlers; they parse the request, call a domain service
// through *App, and map errors to the shared response contract.
type Handlers struct {
	app *App
}

func NewHandlers(app *App) *Handlers {
	return &Handlers{app: app}
}

func (h *Handlers) AnalyticsLatestRun(w http.ResponseWriter, r *http.Request) {
	resp, err := h.app.Analytics.LatestRun(r.Context())
	if err != nil {
		writeAnalyticsError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *Handlers) AnalyticsClientSegments(w http.ResponseWriter, r *http.Request) {
	resp, err := h.app.Analytics.ClientSegments(r.Context())
	if err != nil {
		writeAnalyticsError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *Handlers) AnalyticsPortfolioRisk(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(r.PathValue("userId"), 10, 64)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "ERR_BAD_REQUEST", "userId must be a number")
		return
	}
	resp, err := h.app.Analytics.PortfolioRisk(r.Context(), userID)
	if err != nil {
		writeAnalyticsError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *Handlers) AnalyticsTopTickers(w http.ResponseWriter, r *http.Request) {
	resp, err := h.app.Analytics.TopTickers(r.Context())
	if err != nil {
		writeAnalyticsError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func writeAnalyticsError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, analytics.ErrNotFound) {
		httpx.Error(w, r, http.StatusNotFound, "ERR_NOT_FOUND", "Resource not found")
		return
	}
	httpx.InternalError(w, r, "Internal server error")
}
