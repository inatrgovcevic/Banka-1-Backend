package api

import "time"

// --- Error response bodies -------------------------------------------------
//
// The consolidated Java trading-service JVM has TWO global @RestControllerAdvice
// handlers, and which one shapes an error depends on the exception type:
//
//   - trading-service OtcExceptionHandler (@Order HIGHEST_PRECEDENCE, global)
//     intercepts IllegalArgumentException (-> 404) and IllegalStateException
//     (-> 409) thrown ANYWHERE, including the /actuaries validation paths. Body
//     shape: {status, error, message, timestamp}. No path, no fieldErrors.
//   - order-service OrderServiceExceptionHandler (default precedence) handles the
//     order-module exceptions (ResourceNotFound 404, BadRequest 400,
//     ForbiddenOperation 403, BusinessConflict 409, bean-validation 400) and the
//     catch-all 500. Body shape: ApiErrorResponse {timestamp, status, error,
//     message, path, fieldErrors?} with @JsonInclude(NON_EMPTY).
//
// The Go port reproduces both shapes and routes each business error to the same
// shape the live JVM would produce.

// ApiErrorResponse mirrors order-service com.banka1.order.dto.ApiErrorResponse.
// @JsonInclude(NON_EMPTY) drops fieldErrors when empty (omitempty here).
type ApiErrorResponse struct {
	Timestamp   time.Time         `json:"timestamp"`
	Status      int               `json:"status"`
	Error       string            `json:"error"`
	Message     string            `json:"message"`
	Path        string            `json:"path"`
	FieldErrors map[string]string `json:"fieldErrors,omitempty"`
}

// OtcErrorResponse mirrors the LinkedHashMap body of trading-service
// OtcExceptionHandler: {status, error, message, timestamp}.
type OtcErrorResponse struct {
	Status    int    `json:"status"`
	Error     string `json:"error"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// ReasonPhrase maps an HTTP status to Spring's HttpStatus.getReasonPhrase().
func ReasonPhrase(status int) string {
	switch status {
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 409:
		return "Conflict"
	case 500:
		return "Internal Server Error"
	default:
		return ""
	}
}

// --- DomainError -----------------------------------------------------------

// ErrorShape selects which Java handler's body a DomainError reproduces.
type ErrorShape int

const (
	// ShapeOrder => order-service ApiErrorResponse (with path; optional fieldErrors).
	ShapeOrder ErrorShape = iota
	// ShapeOtc => trading-service OtcExceptionHandler body (status/error/message/timestamp).
	ShapeOtc
)

// DomainError is a business error raised by a service layer and translated to an
// HTTP error body by the HTTP layer. It carries the status, message, body shape,
// and optional field errors so the handler can reproduce the exact Java response.
type DomainError struct {
	Status      int
	Message     string
	Shape       ErrorShape
	FieldErrors map[string]string
}

func (e *DomainError) Error() string { return e.Message }

// NewOrderError builds an order-shaped (ApiErrorResponse) error. Use for the
// order-module exceptions: ResourceNotFound (404), BadRequest (400),
// ForbiddenOperation (403), BusinessConflict (409).
func NewOrderError(status int, message string) *DomainError {
	return &DomainError{Status: status, Message: message, Shape: ShapeOrder}
}

// NewOrderValidation builds the 400 bean-validation error (ApiErrorResponse with
// fieldErrors), mirroring MethodArgumentNotValidException handling.
func NewOrderValidation(fields map[string]string) *DomainError {
	return &DomainError{Status: 400, Message: "Request validation failed", Shape: ShapeOrder, FieldErrors: fields}
}

// NewOtcError builds an OTC-shaped error. Use for IllegalArgumentException
// (status 404) and IllegalStateException (status 409) — the trading-service
// OtcExceptionHandler outranks the order handler for these.
func NewOtcError(status int, message string) *DomainError {
	return &DomainError{Status: status, Message: message, Shape: ShapeOtc}
}
