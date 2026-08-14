package httpapi

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
)

// NewRouter registers the health, readiness, and metrics endpoints on mux and
// wraps it in the request middleware chain. The ready func is injected by main;
// it currently pings the database. metricsHandler is exposed under /metrics,
// gated by a bearer token when prometheusToken is non-empty.
func NewRouter(logger *slog.Logger, trustProxyHeaders bool, prometheusToken string, ready func(ctx context.Context) error, metricsHandler http.Handler, mux *http.ServeMux) http.Handler {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "UP"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := ready(r.Context()); err != nil {
			WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "DOWN", "error": err.Error()})
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "UP"})
	})
	if metricsHandler != nil {
		if prometheusToken != "" {
			mux.Handle("GET /metrics", prometheusAuth(prometheusToken, metricsHandler))
		} else {
			mux.Handle("GET /metrics", metricsHandler)
		}
	}
	return Recoverer(SecurityHeaders(BodyLimit(2 << 20)(RequestID(RequestLog(logger, trustProxyHeaders)(mux)))))
}

// prometheusAuth guards the metrics handler, requiring an Authorization header
// with a Bearer token equal to the configured one.
func prometheusAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(auth, "Bearer ")), []byte(token)) != 1 {
			WriteError(w, Forbidden("Forbidden"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
