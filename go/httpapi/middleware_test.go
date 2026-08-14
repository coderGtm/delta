package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })).
		ServeHTTP(rec, req)
	for _, h := range []string{"X-Frame-Options", "Referrer-Policy", "Permissions-Policy",
		"Content-Security-Policy", "Strict-Transport-Security"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("X-Frame-Options = %q", rec.Header().Get("X-Frame-Options"))
	}
}

func TestRequestID(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	var got string
	RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = RequestIDFrom(r) })).
		ServeHTTP(rec, req)
	if got == "" || rec.Header().Get("X-Request-Id") == "" || got != rec.Header().Get("X-Request-Id") {
		t.Fatalf("request id mismatch: ctx=%q header=%q", got, rec.Header().Get("X-Request-Id"))
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
	if got := ClientIP(req, true); got != "203.0.113.5" {
		t.Errorf("trusted = %q", got)
	}
	if got := ClientIP(req, false); got != "10.0.0.1" {
		t.Errorf("untrusted = %q", got)
	}

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "10.0.0.1:1234"
	req2.Header.Set("X-Forwarded-For", "   ")
	req2.Header.Set("X-Real-IP", "203.0.113.9")
	if got := ClientIP(req2, true); got != "203.0.113.9" {
		t.Errorf("blank xff fallthrough = %q, want X-Real-IP", got)
	}

	req3 := httptest.NewRequest("GET", "/", nil)
	req3.RemoteAddr = "10.0.0.1:1234"
	req3.Header.Set("X-Forwarded-For", "  , 203.0.113.7")
	if got := ClientIP(req3, true); got != "10.0.0.1" {
		t.Errorf("empty first xff entry falls to RemoteAddr = %q, want 10.0.0.1", got)
	}
}

func TestBodyLimit(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"x":"`+strings.Repeat("a", 100)+`"}`))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	BodyLimit(32)(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func TestRecoverer(t *testing.T) {
	rec := httptest.NewRecorder()
	Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })).
		ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

type testSubject struct{}

func (testSubject) SubjectID() string { return "user-1" }

func TestContextKeysDoNotCollide(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req = WithSubject(req, testSubject{})
	req = WithRequestID(req, "req-1")
	if got := RequestIDFrom(req); got != "req-1" {
		t.Errorf("RequestIDFrom = %q, want %q", got, "req-1")
	}
	if s := SubjectFrom(req); s == nil || s.SubjectID() != "user-1" {
		t.Errorf("SubjectFrom = %v, want subject user-1", s)
	}

	req2 := httptest.NewRequest("GET", "/", nil)
	req2 = WithRequestID(req2, "req-2")
	req2 = WithSubject(req2, testSubject{})
	if got := RequestIDFrom(req2); got != "req-2" {
		t.Errorf("RequestIDFrom (reverse) = %q, want %q", got, "req-2")
	}
	if s := SubjectFrom(req2); s == nil || s.SubjectID() != "user-1" {
		t.Errorf("SubjectFrom (reverse) = %v, want subject user-1", s)
	}
}
