// Package metrics provides a Prometheus registry for business counters.
package metrics

import (
	"net/http"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry is a Prometheus registry for business counters, safe for
// concurrent use.
type Registry struct {
	mu       sync.Mutex
	counters map[string]*prometheus.CounterVec
	reg      *prometheus.Registry
}

// NewRegistry returns a Registry preloaded with the Go and process collectors.
func NewRegistry() *Registry {
	r := &Registry{
		counters: make(map[string]*prometheus.CounterVec),
		reg:      prometheus.NewRegistry(),
	}
	r.reg.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return r
}

// Increment records a single increment on the named counter with an even
// sequence of tag key/value pairs. Odd trailing values are dropped. Counters
// are created lazily on first use.
func (r *Registry) Increment(name string, tags ...string) {
	if len(tags)%2 != 0 {
		tags = tags[:len(tags)-1]
	}
	var labelNames []string
	for i := 0; i < len(tags); i += 2 {
		labelNames = append(labelNames, tags[i])
	}
	r.mu.Lock()
	cv := r.registerLocked(name, labelNames)
	r.mu.Unlock()
	cv.WithLabelValues(labelValues(tags)...).Inc()
}

// RegisterCounter eagerly registers the named counter so it is exposed before
// the first increment. labelNames are the counter's label keys; each valueSets
// entry is a complete label-value set whose series is created at zero. Pass no
// value sets for counters with dynamic label values (the vector is registered,
// series appear on first use). This keeps dashboards populated across
// restarts. It is safe to call multiple times with the same name and labels.
func (r *Registry) RegisterCounter(name string, labelNames []string, valueSets ...[]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cv := r.registerLocked(name, labelNames)
	for _, vals := range valueSets {
		cv.WithLabelValues(vals...)
	}
}

func (r *Registry) registerLocked(name string, labelNames []string) *prometheus.CounterVec {
	key := name + "|" + strings.Join(labelNames, ",")
	if cv, ok := r.counters[key]; ok {
		return cv
	}
	cv := prometheus.NewCounterVec(prometheus.CounterOpts{Name: name, Help: name}, labelNames)
	r.reg.MustRegister(cv)
	r.counters[key] = cv
	return cv
}

func labelValues(tags []string) []string {
	var vals []string
	for i := 0; i < len(tags); i += 2 {
		vals = append(vals, tags[i+1])
	}
	return vals
}

// Handler returns an HTTP handler that exposes the registry in the Prometheus
// text format.
func (r *Registry) Handler() http.Handler { return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{}) }
