// Package platform is the in-service adapter onto banka1/go-platform.
// It exists so handlers in this service can keep their existing API surface
// while the real implementation lives in the shared module.
package platform

import (
	"encoding/json"
	"net/http"

	"banka1/go-platform/httpx"
)

// JSON delegates to go-platform/httpx.JSON.
func JSON(w http.ResponseWriter, status int, payload any) { httpx.JSON(w, status, payload) }

// NoContent delegates to go-platform/httpx.NoContent.
func NoContent(w http.ResponseWriter, status int) { httpx.NoContent(w, status) }

// DecodeJSON intentionally matches Spring/Jackson behavior used by the old
// user-service: unknown fields are ignored so existing frontend payloads with
// extra fields such as "margin" do not fail decoding.
func DecodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

// Error keeps the user-service-go (w, status, code, message) signature.
// Internally builds the standard go-platform ErrorBody, sourcing the
// correlationId from the request that triggered the response when present
// via a context lookup. Because this signature does not take *http.Request
// we fall back to an empty correlation id — callers that want correlation
// in the body should use ErrorWithRequest below.
func Error(w http.ResponseWriter, status int, code, message string) {
	httpx.JSON(w, status, httpx.ErrorBody{
		Code:    code,
		Message: message,
	})
}

// ErrorWithRequest is preferred for new code; it emits the correlationId on
// the response body.
func ErrorWithRequest(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	httpx.Error(w, r, status, code, message)
}
