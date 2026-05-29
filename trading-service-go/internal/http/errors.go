package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"banka1/trading-service-go/internal/api"

	gpauth "banka1/go-platform/auth"
	"banka1/go-platform/httpx"
)

// parsePathID reads the {id} path value as int64. A non-numeric id mirrors
// Spring's MethodArgumentTypeMismatchException -> 400 (order ApiErrorResponse).
func parsePathID(r *http.Request) (int64, error) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, api.NewOrderError(http.StatusBadRequest,
			"Invalid value '"+raw+"' for parameter 'id', expected type: Long.")
	}
	return id, nil
}

// writeDomainError renders a service-layer error as the exact body the
// consolidated Java JVM would produce for the /portfolio and /actuaries routes:
// an *api.DomainError carries the status, message, and which handler shape to use
// (order ApiErrorResponse vs OTC handler). Any other error maps to the
// order-service catch-all: 500 "Unexpected server error".
func writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	var de *api.DomainError
	if errors.As(err, &de) {
		if de.Shape == api.ShapeOtc {
			httpx.JSON(w, de.Status, api.OtcErrorResponse{
				Status:    de.Status,
				Error:     api.ReasonPhrase(de.Status),
				Message:   de.Message,
				Timestamp: time.Now().Format("2006-01-02T15:04:05.999999999"),
			})
			return
		}
		httpx.JSON(w, de.Status, api.ApiErrorResponse{
			Timestamp:   time.Now().UTC(),
			Status:      de.Status,
			Error:       api.ReasonPhrase(de.Status),
			Message:     de.Message,
			Path:        r.URL.Path,
			FieldErrors: de.FieldErrors,
		})
		return
	}
	httpx.JSON(w, http.StatusInternalServerError, api.ApiErrorResponse{
		Timestamp: time.Now().UTC(),
		Status:    http.StatusInternalServerError,
		Error:     api.ReasonPhrase(http.StatusInternalServerError),
		Message:   "Unexpected server error",
		Path:      r.URL.Path,
	})
}

// orderSecured wraps an order-module handler with JWT auth + a role gate that
// emits the order-service 403 body ("Access denied"), matching how the live JVM
// maps an @PreAuthorize denial (AuthorizationDeniedException ->
// OrderServiceExceptionHandler). Token-parse failures (401) come from the shared
// Middleware (httpx body) — the same inherited behavior as P1.
func orderSecured(jwtService *gpauth.Service, roles []string, h http.HandlerFunc) http.Handler {
	return jwtService.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := gpauth.PrincipalFromContext(r.Context())
		if !ok {
			writeDomainError(w, r, api.NewOrderError(http.StatusUnauthorized, "Unauthorized"))
			return
		}
		if !principal.HasAnyRole(roles...) {
			writeDomainError(w, r, api.NewOrderError(http.StatusForbidden, "Access denied"))
			return
		}
		h(w, r)
	}))
}
