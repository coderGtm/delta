package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dto "github.com/prometheus/client_model/go"
)

func TestHTTPMiddlewareRecordsRequests(t *testing.T) {
	reg := NewRegistry()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/outlets/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	handler := reg.HTTPMiddleware(inner)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/outlets/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", nil)
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	mfs, err := reg.reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var requests *dto.MetricFamily
	var durations *dto.MetricFamily
	for _, mf := range mfs {
		switch mf.GetName() {
		case "http_requests_total":
			requests = mf
		case "http_request_duration_seconds":
			durations = mf
		}
	}
	if requests == nil {
		t.Fatal("http_requests_total not exposed")
	}
	found := map[string]bool{}
	for _, m := range requests.GetMetric() {
		var labels []string
		for _, l := range m.GetLabel() {
			labels = append(labels, l.GetName()+"="+l.GetValue())
		}
		found[strings.Join(labels, ",")] = true
	}
	// The UUID segment must be normalized to {id} so cardinality stays bounded.
	if !found["method=GET,path=/api/v1/outlets/{id},status=200"] {
		t.Errorf("expected normalized 200 metric, got %v", found)
	}
	if !found["method=GET,path=/nope,status=404"] {
		t.Errorf("expected 404 metric, got %v", found)
	}
	if durations == nil {
		t.Fatal("http_request_duration_seconds not exposed")
	}
}

func TestRegisterPoolStats(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterPoolStats(func() (float64, float64) { return 3, 2 })

	mfs, err := reg.reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	names := map[string]bool{}
	for _, mf := range mfs {
		names[mf.GetName()] = true
	}
	if !names["delta_db_connections_total"] || !names["delta_db_connections_idle"] {
		t.Errorf("pool gauges missing: %v", names)
	}
}
