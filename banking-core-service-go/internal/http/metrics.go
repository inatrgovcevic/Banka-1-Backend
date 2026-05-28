package http

import (
	"fmt"
	"net/http"
	"runtime"
	"time"
)

var processStartedAt = time.Now()

func (h *Handler) prometheus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "# HELP banking_core_service_up Banking core Go service liveness.\n")
	_, _ = fmt.Fprintf(w, "# TYPE banking_core_service_up gauge\n")
	_, _ = fmt.Fprintf(w, "banking_core_service_up 1\n")
	_, _ = fmt.Fprintf(w, "# HELP process_uptime_seconds Process uptime in seconds.\n")
	_, _ = fmt.Fprintf(w, "# TYPE process_uptime_seconds gauge\n")
	_, _ = fmt.Fprintf(w, "process_uptime_seconds %.0f\n", time.Since(processStartedAt).Seconds())
	_, _ = fmt.Fprintf(w, "# HELP go_goroutines Number of goroutines that currently exist.\n")
	_, _ = fmt.Fprintf(w, "# TYPE go_goroutines gauge\n")
	_, _ = fmt.Fprintf(w, "go_goroutines %d\n", runtime.NumGoroutine())
}
