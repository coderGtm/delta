package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
)

// swaggerUI holds the vendored Swagger UI static assets embedded into the
// binary.
//
//go:embed all:web/swagger-ui
var swaggerUI embed.FS

// openapiYAML holds the hand-maintained OpenAPI specification for the API.
//
//go:embed openapi.yaml
var openapiYAML []byte

// DocsHandler serves the OpenAPI specification and the embedded Swagger UI.
// The specification is hand-maintained and describes the current behavior of
// the served application, which remains authoritative. It is reachable at
// /docs/openapi.yaml and the UI at /docs/.
func DocsHandler() http.Handler {
	sub, err := fs.Sub(swaggerUI, "web/swagger-ui")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(openapiYAML)
	})
	mux.Handle("GET /docs/", http.StripPrefix("/docs/", http.FileServer(http.FS(sub))))
	return mux
}
