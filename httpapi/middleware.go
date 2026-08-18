package httpapi

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RequestID ensures every request carries an X-Request-Id, reusing an incoming
// header value or generating a new UUID when absent, and makes the ID available
// to downstream handlers.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, WithRequestID(r, id))
	})
}

// WithRequestID returns a copy of r with the given request ID attached to its
// context.
func WithRequestID(r *http.Request, id string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), requestIDKey, id))
}

// RequestIDFrom returns the request ID attached to r's context, or "" if none
// is present.
func RequestIDFrom(r *http.Request) string {
	if id, ok := r.Context().Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Flush flushes the underlying writer when it supports http.Flusher, so
// streaming responses preserve their flush behavior.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack hands the connection to the caller when the underlying writer
// supports http.Hijacker, so hijacked connections (websockets) still work.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Push proxies an HTTP/2 server push when the underlying writer supports
// http.Pusher.
func (r *statusRecorder) Push(target string, opts *http.PushOptions) error {
	if p, ok := r.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

// RequestLog returns middleware that logs a completion line for each request
// with its status, duration, client IP, request ID, and authenticated user ID.
func RequestLog(logger *slog.Logger, trustProxyHeaders bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			logger.Info(fmt.Sprintf("HTTP %s %s completed", r.Method, r.URL.Path),
				"status", rec.status,
				"durationMs", time.Since(start).Milliseconds(),
				"clientIp", ClientIP(r, trustProxyHeaders),
				"requestId", RequestIDFrom(r),
				"userId", SubjectID(r),
			)
		})
	}
}

// Recoverer returns middleware that recovers panics in downstream handlers and
// responds with a 500 Internal Server Error instead of crashing the process.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "err", rec, "requestId", RequestIDFrom(r))
				WriteError(w, Internal("Internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders returns middleware that sets recommended security response
// headers on every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		h.Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// BodyLimit returns middleware that rejects requests whose declared
// Content-Length exceeds maxBytes and enforces the limit on the body via
// http.MaxBytesReader.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				WriteError(w, PayloadTooLarge("Request body too large"))
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP returns the client IP address. When trustProxyHeaders is true the
// X-Forwarded-For and X-Real-IP headers are honored; otherwise the connection
// peer address is used.
func ClientIP(r *http.Request, trustProxyHeaders bool) string {
	if trustProxyHeaders {
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			if first := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0]); first != "" {
				return first
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
