package httpapi

import (
	"fmt"
	"net/http"
	"time"
)

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func BadRequest(msg string) *APIError { return &APIError{http.StatusBadRequest, "BAD_REQUEST", msg} }
func Conflict(msg string) *APIError   { return &APIError{http.StatusConflict, "CONFLICT", msg} }
func Forbidden(msg string) *APIError  { return &APIError{http.StatusForbidden, "FORBIDDEN", msg} }
func NotFound(msg string) *APIError   { return &APIError{http.StatusNotFound, "NOT_FOUND", msg} }
func InvalidToken(msg string) *APIError {
	return &APIError{http.StatusUnauthorized, "INVALID_TOKEN", msg}
}
func Validation(msg string) *APIError {
	return &APIError{http.StatusBadRequest, "VALIDATION_ERROR", msg}
}
func RateLimitExceeded(msg string) *APIError {
	return &APIError{http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", msg}
}
func Internal(msg string) *APIError {
	return &APIError{http.StatusInternalServerError, "INTERNAL_ERROR", msg}
}

type ErrorResponse struct {
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
