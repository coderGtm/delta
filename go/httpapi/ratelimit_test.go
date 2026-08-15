package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func noContent() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

func TestRateLimitExceeds(t *testing.T) {
	rl := NewRateLimiter(false)
	h := rl.Middleware(noContent())
	statuses := make([]int, 0, 12)
	for i := 0; i < 12; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		statuses = append(statuses, rec.Code)
	}
	for i, s := range statuses {
		want := http.StatusNoContent
		if i >= 10 {
			want = http.StatusTooManyRequests
		}
		if s != want {
			t.Errorf("request %d status = %d, want %d (all: %v)", i+1, s, want, statuses)
		}
	}
}

func TestRateLimitDistinctKeys(t *testing.T) {
	rl := NewRateLimiter(false)
	h := rl.Middleware(noContent())
	for i := 0; i < 12; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		want := http.StatusNoContent
		if i >= 10 {
			want = http.StatusTooManyRequests
		}
		if rec.Code != want {
			t.Fatalf("request %d from 10.0.0.1 status = %d, want %d", i+1, rec.Code, want)
		}
	}
	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("different IP status = %d, want 204", rec.Code)
	}
}

func TestRateLimitRetryAfter(t *testing.T) {
	rl := NewRateLimiter(false)
	h := rl.Middleware(noContent())
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if i < 10 {
			continue
		}
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("status = %d, want 429", rec.Code)
		}
		ra := rec.Header().Get("Retry-After")
		if ra == "" {
			t.Fatal("missing Retry-After header")
		}
		n, err := strconv.Atoi(ra)
		if err != nil || n < 1 {
			t.Fatalf("Retry-After = %q, want integer >= 1", ra)
		}
	}
}

func TestRateLimitWindowReset(t *testing.T) {
	rl := newRateLimiter(false, []policy{{"POST", "/api/v1/auth/login", 10, 50 * time.Millisecond, false}})
	h := rl.Middleware(noContent())
	serve := func() int {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	for i := 0; i < 10; i++ {
		if code := serve(); code != http.StatusNoContent {
			t.Fatalf("request %d status = %d, want 204", i+1, code)
		}
	}
	if code := serve(); code != http.StatusTooManyRequests {
		t.Fatalf("request 11 status = %d, want 429", code)
	}
	time.Sleep(60 * time.Millisecond)
	if code := serve(); code != http.StatusNoContent {
		t.Fatalf("after window reset status = %d, want 204", code)
	}
}

type fakeSubject string

func (f fakeSubject) SubjectID() string { return string(f) }

func TestRateLimitUserKey(t *testing.T) {
	rl := NewRateLimiter(false)
	h := rl.Middleware(noContent())
	invite := func(ip string, subject Subject) int {
		req := httptest.NewRequest("POST", "/api/v1/outlets/abc/memberships/invite", nil)
		req.RemoteAddr = ip
		if subject != nil {
			req = WithSubject(req, subject)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	for i := 0; i < 20; i++ {
		if code := invite(fmt.Sprintf("10.0.0.%d:1234", i+1), fakeSubject("user-1")); code != http.StatusNoContent {
			t.Fatalf("request %d status = %d, want 204", i+1, code)
		}
	}
	if code := invite("10.0.0.99:1234", fakeSubject("user-1")); code != http.StatusTooManyRequests {
		t.Fatalf("21st request same user status = %d, want 429", code)
	}
	if code := invite("10.0.0.77:1234", fakeSubject("user-2")); code != http.StatusNoContent {
		t.Fatalf("different user status = %d, want 204", code)
	}
	if code := invite("10.0.0.55:1234", nil); code != http.StatusNoContent {
		t.Fatalf("unauthenticated IP-keyed request status = %d, want 204", code)
	}
}

func TestRateLimitNoPolicy(t *testing.T) {
	rl := NewRateLimiter(false)
	h := rl.Middleware(noContent())
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/healthz", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("GET /healthz request %d status = %d, want 204", i+1, rec.Code)
		}
	}
	req := httptest.NewRequest("POST", "/api/v1/outlets", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unmatched path status = %d, want 204", rec.Code)
	}
}

func TestMatchPath(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"/api/v1/auth/login", "/api/v1/auth/login", true},
		{"/api/v1/auth/login", "/api/v1/auth/refresh", false},
		{"/api/v1/auth/login", "/api/v1/auth", false},
		{"/api/v1/auth/login/", "/api/v1/auth/login", true},
		{"/api/v1/auth/login", "/api/v1/auth/login/", true},
		{"/api/v1/outlets/*/memberships/invite", "/api/v1/outlets/abc/memberships/invite", true},
		{"/api/v1/outlets/*/memberships/invite", "/api/v1/outlets/abc/def/memberships/invite", false},
		{"/api/v1/outlets/*/attendance/*", "/api/v1/outlets/abc/attendance/entry-1", true},
		{"/api/v1/outlets/*/attendance/*", "/api/v1/outlets/abc/attendance", false},
		{"/api/v1/outlets/*/attendance/*", "/api/v1/outlets/abc/attendance/entry-1/extra", false},
	}
	for _, c := range cases {
		if got := matchPath(c.pattern, c.path); got != c.want {
			t.Errorf("matchPath(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}
