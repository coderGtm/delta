package metrics

import (
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// uuidPathRe matches UUID path segments so the request metric label stays
// bounded even though route parameters are replaced by concrete values in the
// URL.
var uuidPathRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// statusWriter records the first response status code written by the wrapped
// handler.
type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wrote {
		w.status = code
		w.wrote = true
	}
	w.ResponseWriter.WriteHeader(code)
}

// HTTPMiddleware meters every request handled by next with an HTTP request
// counter and a duration histogram, labeled by method, normalized path (UUID
// segments become {id}), and status code.
func (r *Registry) HTTPMiddleware(next http.Handler) http.Handler {
	cv := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "http_requests_total", Help: "HTTP requests by method, path, and status."},
		[]string{"method", "path", "status"},
	)
	hv := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP request duration in seconds."},
		[]string{"method", "path"},
	)
	r.reg.MustRegister(cv, hv)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		rec := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, req)
		path := uuidPathRe.ReplaceAllString(req.URL.Path, "{id}")
		cv.WithLabelValues(req.Method, path, strconv.Itoa(rec.status)).Inc()
		hv.WithLabelValues(req.Method, path).Observe(time.Since(start).Seconds())
	})
}

// RegisterPoolStats exposes the database connection pool totals as gauges so
// dashboards can show live connection counts.
func (r *Registry) RegisterPoolStats(stats func() (total, idle float64)) {
	r.reg.MustRegister(
		prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{Name: "delta_db_connections_total", Help: "Total database pool connections."},
			func() float64 { total, _ := stats(); return total },
		),
		prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{Name: "delta_db_connections_idle", Help: "Idle database pool connections."},
			func() float64 { _, idle := stats(); return idle },
		),
	)
}
