package platform

import (
	"net/http"
	"time"

	"banka1/go-platform/httpx"
)

// StockErrorResponse mirrors the Java stock-service error body shape.
// Kept in-service because it differs from the generic ErrorBody.
type StockErrorResponse struct {
	Timestamp string `json:"timestamp"`
	Status    int    `json:"status"`
	Error     string `json:"error"`
	Message   string `json:"message"`
	Path      string `json:"path"`
}

// ExchangeErrorResponse mirrors the Java exchange-service error body shape.
type ExchangeErrorResponse struct {
	Code             string            `json:"code"`
	Title            string            `json:"title"`
	Message          string            `json:"message"`
	ValidationErrors map[string]string `json:"validationErrors"`
}

// JSON delegates to httpx.JSON to keep one canonical implementation.
func JSON(w http.ResponseWriter, status int, payload any) {
	httpx.JSON(w, status, payload)
}

// Error keeps the existing (w, status, code, message) signature for
// backward compatibility. It writes the generic ErrorBody without the
// correlation id; new code should call httpx.Error(w, r, ...) instead.
func Error(w http.ResponseWriter, status int, code, message string) {
	httpx.JSON(w, status, httpx.ErrorBody{
		Code:      code,
		Message:   message,
		Timestamp: time.Now().UTC(),
	})
}

func StockError(w http.ResponseWriter, r *http.Request, status int, message string) {
	JSON(w, status, StockErrorResponse{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Status:    status,
		Error:     http.StatusText(status),
		Message:   message,
		Path:      r.URL.Path,
	})
}

func ExchangeError(w http.ResponseWriter, status int, code, title, message string, validation map[string]string) {
	if validation == nil {
		validation = map[string]string{}
	}
	JSON(w, status, ExchangeErrorResponse{
		Code:             code,
		Title:            title,
		Message:          message,
		ValidationErrors: validation,
	})
}

func NoContent(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
}
