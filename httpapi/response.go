package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// DecodeJSON decodes the request body into dst. A body that is not valid JSON
// yields a 400 Bad Request error.
func DecodeJSON(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return BadRequest("Malformed request body")
	}
	return nil
}

// WriteJSON writes v to w as a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

// WriteError writes an error response. An *APIError is rendered with its
// status, code, and message; any other error is logged and rendered as a 500
// Internal Server Error.
func WriteError(w http.ResponseWriter, err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		WriteJSON(w, apiErr.Status, ErrorResponse{apiErr.Code, apiErr.Message, time.Now().UTC()})
		return
	}
	slog.Error("unhandled error", "err", err)
	WriteJSON(w, http.StatusInternalServerError, ErrorResponse{"INTERNAL_ERROR", "Internal server error", time.Now().UTC()})
}

// PageResponse is the paginated API response wrapper; field declaration order
// defines JSON key order.
type PageResponse[T any] struct {
	Content       []T   `json:"content"`       // Items for the requested page.
	Page          int   `json:"page"`          // Zero-based page number.
	Size          int   `json:"size"`          // Number of items per page.
	TotalElements int64 `json:"totalElements"` // Total number of items across all pages.
	TotalPages    int   `json:"totalPages"`    // Total number of pages.
	First         bool  `json:"first"`         // Whether this is the first page.
	Last          bool  `json:"last"`          // Whether this is the last page.
	Empty         bool  `json:"empty"`         // Whether the page has no items.
}

// NewPageResponse builds a PageResponse from the page content, the total
// element count, and the request's page parameters.
func NewPageResponse[T any](content []T, total int64, p PageParams) *PageResponse[T] {
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(p.Size) - 1) / int64(p.Size))
	}
	return &PageResponse[T]{
		Content:       content,
		Page:          p.Page,
		Size:          p.Size,
		TotalElements: total,
		TotalPages:    totalPages,
		First:         p.Page == 0,
		Last:          int64(p.Page+1)*int64(p.Size) >= total,
		Empty:         len(content) == 0,
	}
}

// WritePage writes a paginated response derived from the page content, the
// total element count, and the request's page parameters.
func WritePage[T any](w http.ResponseWriter, content []T, total int64, p PageParams) {
	WriteJSON(w, http.StatusOK, NewPageResponse(content, total, p))
}
