package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, NotFound("Outlet not found: x"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != "NOT_FOUND" || body.Message != "Outlet not found: x" {
		t.Errorf("body = %+v", body)
	}
	if body.Timestamp.IsZero() {
		t.Error("timestamp missing")
	}
}

func TestWriteErrorUnknown(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, errors.New("boom"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var body ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Code != "INTERNAL_ERROR" {
		t.Errorf("code = %q, want INTERNAL_ERROR", body.Code)
	}
}
