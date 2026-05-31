package httpx

import (
	"encoding/json"
	"net/http"
	"time"
)

// ErrorBody is the standard error response across all Go services. The shape
// is intentionally close to the existing Java `ErrorResponseDto` so the
// frontend does not need a special case for Go upstreams.
//
//	{
//	  "code":          "ERR_VALIDATION",
//	  "title":         "Validation error",
//	  "message":       "amount must be greater than 0.",
//	  "correlationId": "abc123...",
//	  "timestamp":     "2026-05-26T14:00:00Z",
//	  "validationErrors": { "amount": "..." }
//	}
type ErrorBody struct {
	Code             string            `json:"code"`
	Title            string            `json:"title,omitempty"`
	Message          string            `json:"message"`
	CorrelationID    string            `json:"correlationId,omitempty"`
	Timestamp        time.Time         `json:"timestamp"`
	ValidationErrors map[string]string `json:"validationErrors,omitempty"`
}

// JSON writes status with payload encoded as JSON. nil payload means a body-less
// response with the chosen status code.
func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

// NoContent writes status with no body. Use for 204.
func NoContent(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
}

// Error writes the standard ErrorBody with the active correlation id.
func Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	JSON(w, status, ErrorBody{
		Code:          code,
		Message:       message,
		CorrelationID: CorrelationFromContext(r.Context()),
		Timestamp:     time.Now().UTC(),
	})
}

// ErrorWithTitle is like Error but also sets the title field.
func ErrorWithTitle(w http.ResponseWriter, r *http.Request, status int, code, title, message string) {
	JSON(w, status, ErrorBody{
		Code:          code,
		Title:         title,
		Message:       message,
		CorrelationID: CorrelationFromContext(r.Context()),
		Timestamp:     time.Now().UTC(),
	})
}

// ValidationError writes a 400 with per-field errors. Mirrors the Java
// GlobalExceptionHandler.buildValidationErrorResponse shape.
func ValidationError(w http.ResponseWriter, r *http.Request, message string, fields map[string]string) {
	JSON(w, http.StatusBadRequest, ErrorBody{
		Code:             "ERR_VALIDATION",
		Title:            "Validation error",
		Message:          message,
		CorrelationID:    CorrelationFromContext(r.Context()),
		Timestamp:        time.Now().UTC(),
		ValidationErrors: fields,
	})
}

// InternalError is the helper RecoverMiddleware uses; surfaces a generic
// message without leaking the panic value.
func InternalError(w http.ResponseWriter, r *http.Request, message string) {
	Error(w, r, http.StatusInternalServerError, "ERR_INTERNAL_SERVER", message)
}

// DecodeJSON reads r.Body into dst, rejecting unknown fields.
func DecodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}
