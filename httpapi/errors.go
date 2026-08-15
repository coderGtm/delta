package httpapi

import (
	"fmt"
	"net/http"
	"time"
)

// APIError is a structured API error carrying an HTTP status, a
// machine-readable code, and a human-readable message.
type APIError struct {
	Status  int    // HTTP status code to send with the response.
	Code    string // Machine-readable error code, e.g. "NOT_FOUND".
	Message string // Human-readable error message.
}

// Error returns a single-line representation of the API error.
func (e *APIError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// BadRequest returns a 400 Bad Request API error with the BAD_REQUEST code.
func BadRequest(msg string) *APIError { return &APIError{http.StatusBadRequest, "BAD_REQUEST", msg} }

// Conflict returns a 409 Conflict API error with the CONFLICT code.
func Conflict(msg string) *APIError { return &APIError{http.StatusConflict, "CONFLICT", msg} }

// Forbidden returns a 403 Forbidden API error with the FORBIDDEN code.
func Forbidden(msg string) *APIError { return &APIError{http.StatusForbidden, "FORBIDDEN", msg} }

// NotFound returns a 404 Not Found API error with the NOT_FOUND code.
func NotFound(msg string) *APIError { return &APIError{http.StatusNotFound, "NOT_FOUND", msg} }

// InvalidToken returns a 401 Unauthorized API error with the INVALID_TOKEN
// code.
func InvalidToken(msg string) *APIError {
	return &APIError{http.StatusUnauthorized, "INVALID_TOKEN", msg}
}

// Validation returns a 400 Bad Request API error with the VALIDATION_ERROR
// code.
func Validation(msg string) *APIError {
	return &APIError{http.StatusBadRequest, "VALIDATION_ERROR", msg}
}

// RateLimitExceeded returns a 429 Too Many Requests API error with the
// RATE_LIMIT_EXCEEDED code.
func RateLimitExceeded(msg string) *APIError {
	return &APIError{http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", msg}
}

// PayloadTooLarge returns a 413 Request Entity Too Large API error with the
// PAYLOAD_TOO_LARGE code.
func PayloadTooLarge(msg string) *APIError {
	return &APIError{http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", msg}
}

// Internal returns a 500 Internal Server Error API error with the
// INTERNAL_ERROR code.
func Internal(msg string) *APIError {
	return &APIError{http.StatusInternalServerError, "INTERNAL_ERROR", msg}
}

// ErrorResponse is the JSON body returned for failed requests.
type ErrorResponse struct {
	Code      string    `json:"code"`      // Machine-readable error code.
	Message   string    `json:"message"`   // Human-readable error message.
	Timestamp time.Time `json:"timestamp"` // Time the error was generated, in UTC.
}
