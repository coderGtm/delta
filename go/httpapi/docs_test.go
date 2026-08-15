package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coderGtm/delta/go/config"
)

// TestDocsOpenAPI verifies the specification is served as YAML and contains
// the expected top-level keys and an auth path.
func TestDocsOpenAPI(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/docs/openapi.yaml", nil)
	DocsHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/yaml") {
		t.Fatalf("Content-Type = %q, want prefix application/yaml", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "paths:") {
		t.Fatalf("spec body missing %q key", "paths:")
	}
	if !strings.Contains(body, "/api/v1/auth/login") {
		t.Fatalf("spec body missing /api/v1/auth/login path")
	}
}

// TestDocsIndex verifies the Swagger UI index page is served and its bundle
// configuration points at the embedded specification.
func TestDocsIndex(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/docs/", nil)
	DocsHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "SwaggerUIBundle") {
		t.Fatalf("index body missing SwaggerUIBundle")
	}
	if !strings.Contains(body, `url: "/docs/openapi.yaml"`) {
		t.Fatalf("index body missing url: /docs/openapi.yaml config")
	}
}

// TestDocsSwaggerAsset verifies a Swagger UI static asset is served.
func TestDocsSwaggerAsset(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/docs/swagger-ui.css", nil)
	DocsHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// TestDocsRoutesRegistered verifies the docs endpoints are wired into the
// application router alongside the health endpoints.
func TestDocsRoutesRegistered(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewRouter(logger, config.Config{TrustProxyHeaders: true},
		func(ctx context.Context) error { return nil }, nil,
		func(h http.Handler) http.Handler { return h },
		func(h http.Handler) http.Handler { return h },
		http.NewServeMux())

	for _, path := range []string{"/docs/openapi.yaml", "/docs/", "/docs/swagger-ui.css"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", path, rec.Code)
		}
	}
}
