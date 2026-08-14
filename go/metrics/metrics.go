package metrics

import (
	"net/http"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Registry struct {
	mu       sync.Mutex
	counters map[string]*prometheus.CounterVec
	reg      *prometheus.Registry
}

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
	key := name + "|" + strings.Join(labelNames, ",")
	r.mu.Lock()
	cv, ok := r.counters[key]
	if !ok {
		cv = prometheus.NewCounterVec(prometheus.CounterOpts{Name: name, Help: name}, labelNames)
		r.reg.MustRegister(cv)
		r.counters[key] = cv
	}
	r.mu.Unlock()
	cv.WithLabelValues(labelValues(tags)...).Inc()
}

func labelValues(tags []string) []string {
	var vals []string
	for i := 0; i < len(tags); i += 2 {
		vals = append(vals, tags[i+1])
	}
	return vals
}

func (r *Registry) Handler() http.Handler { return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{}) }
