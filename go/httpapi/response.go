package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

func WriteError(w http.ResponseWriter, err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		WriteJSON(w, apiErr.Status, ErrorResponse{apiErr.Code, apiErr.Message, time.Now().UTC()})
		return
	}
	slog.Error("unhandled error", "err", err)
	WriteJSON(w, http.StatusInternalServerError, ErrorResponse{"INTERNAL_ERROR", "Internal server error", time.Now().UTC()})
}

// PageResponse mirrors the Java PageResponse record shape; field declaration
// order determines JSON key order.
type PageResponse[T any] struct {
	Content       []T   `json:"content"`
	Page          int   `json:"page"`
	Size          int   `json:"size"`
	TotalElements int64 `json:"totalElements"`
	TotalPages    int   `json:"totalPages"`
	First         bool  `json:"first"`
	Last          bool  `json:"last"`
	Empty         bool  `json:"empty"`
}

func WritePage[T any](w http.ResponseWriter, content []T, total int64, p PageParams) {
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(p.Size) - 1) / int64(p.Size))
	}
	WriteJSON(w, http.StatusOK, PageResponse[T]{
		Content:       content,
		Page:          p.Page,
		Size:          p.Size,
		TotalElements: total,
		TotalPages:    totalPages,
		First:         p.Page == 0,
		Last:          int64(p.Page+1)*int64(p.Size) >= total,
		Empty:         len(content) == 0,
	})
}
