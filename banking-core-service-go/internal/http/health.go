package http

import (
	"context"
	"net/http"
	"time"
)

func (h *Handler) liveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "UP"})
}

func (h *Handler) readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if h.services == nil || h.services.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "DOWN"})
		return
	}
	if err := h.services.DB.PingContext(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "DOWN"})
		return
	}
	if h.services.Rabbit != nil {
		if err := h.services.Rabbit.Check(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "DOWN"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "UP"})
}
