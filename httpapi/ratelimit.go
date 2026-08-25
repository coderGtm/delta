package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// policy describes a rate limit rule applied to requests matching method and
// path. byUser controls whether the counter key is the authenticated subject's
// ID (falling back to the client IP) instead of the client IP alone.
type policy struct {
	method string
	path   string
	limit  int
	window time.Duration
	byUser bool
}

// window tracks the request count for one counter key within a fixed window.
type window struct {
	endsAt time.Time
	count  int
}

// rateLimitPolicies is the standard policy table for high-risk authentication
// and write-heavy business endpoints.
var rateLimitPolicies = []policy{
	{"POST", "/api/v1/auth/login", 10, time.Minute, false},
	{"POST", "/api/v1/auth/refresh", 30, time.Minute, false},
	{"POST", "/api/v1/auth/logout", 30, time.Minute, false},
	{"POST", "/api/v1/auth/logout-all", 30, time.Minute, true},
	{"POST", "/api/v1/outlets/*/memberships/invite", 20, time.Minute, true},
	{"POST", "/api/v1/outlets/*/attendance", 20, time.Minute, true},
	{"POST", "/api/v1/outlets/*/attendance/manage", 60, time.Minute, true},
	{"PUT", "/api/v1/outlets/*/attendance/*", 60, time.Minute, true},
	{"PUT", "/api/v1/outlets/*/geofence", 20, time.Minute, true},
	{"PUT", "/api/v1/outlets/*/recent-entries-visibility", 20, time.Minute, true},
	{"PUT", "/api/v1/outlets/*/total-time-today-visibility", 20, time.Minute, true},
	{"GET", "/api/v1/outlets/*/reports/salary", 30, time.Minute, true},
	{"GET", "/api/v1/outlets/*/reports/salary.xlsx", 10, time.Minute, true},
}

// RateLimiter provides single-instance in-memory fixed-window rate limiting
// for high-risk endpoints.
type RateLimiter struct {
	mu       sync.Mutex
	windows  map[string]*window
	policies []policy
	trust    bool
}

// NewRateLimiter returns a limiter with the standard policy table. trustProxy
// controls whether client IPs honor forwarded headers.
func NewRateLimiter(trustProxy bool) *RateLimiter {
	return newRateLimiter(trustProxy, rateLimitPolicies)
}

// newRateLimiter is NewRateLimiter with an explicit policy table, used by
// tests to inject short windows.
func newRateLimiter(trustProxy bool, policies []policy) *RateLimiter {
	return &RateLimiter{
		windows:  make(map[string]*window),
		policies: policies,
		trust:    trustProxy,
	}
}

// Middleware enforces the configured rate limits. Requests that match no
// policy pass through untouched; those that exceed their limit receive a 429
// response with a Retry-After header.
func (r *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		p := r.findPolicy(req.Method, req.URL.Path)
		if p == nil {
			next.ServeHTTP(w, req)
			return
		}
		key := ClientIP(req, r.trust)
		if p.byUser {
			if id := SubjectID(req); id != "" {
				key = id
			}
		}
		counterKey := p.method + ":" + p.path + ":" + key

		now := time.Now()
		var over bool
		var retry int
		r.mu.Lock()
		wnd := r.windows[counterKey]
		if wnd == nil || now.After(wnd.endsAt) {
			wnd = &window{endsAt: now.Add(p.window), count: 1}
			r.windows[counterKey] = wnd
		} else {
			wnd.count++
		}
		over = wnd.count > p.limit
		retry = int(time.Until(wnd.endsAt) / time.Second)
		if retry < 1 {
			retry = 1
		}
		r.mu.Unlock()

		if over {
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			WriteError(w, RateLimitExceeded("Too many requests. Please retry later."))
			return
		}
		next.ServeHTTP(w, req)
	})
}

// findPolicy returns the first policy matching the request method and path, or
// nil when none apply.
func (r *RateLimiter) findPolicy(method, path string) *policy {
	for i := range r.policies {
		if r.policies[i].method == method && matchPath(r.policies[i].path, path) {
			return &r.policies[i]
		}
	}
	return nil
}

// matchPath reports whether path matches the pattern, where * matches exactly
// one path segment and the segment counts must be equal.
func matchPath(pattern, path string) bool {
	ps := strings.Split(strings.Trim(pattern, "/"), "/")
	qs := strings.Split(strings.Trim(path, "/"), "/")
	if len(ps) != len(qs) {
		return false
	}
	for i := range ps {
		if ps[i] == "*" {
			continue
		}
		if ps[i] != qs[i] {
			return false
		}
	}
	return true
}
