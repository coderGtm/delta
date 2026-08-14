// Package httpapi provides shared HTTP plumbing: response helpers,
// pagination, request context values, and middleware.
package httpapi

import (
	"context"
	"net/http"
)

type ctxKey int

const (
	subjectKey ctxKey = iota
	requestIDKey
)

// Subject is implemented by the authenticated user model (db.User) and
// anything else that needs to be attached to the request context.
type Subject interface {
	SubjectID() string
}

// WithSubject returns a copy of r with the given subject attached to its
// context.
func WithSubject(r *http.Request, s Subject) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), subjectKey, s))
}

// SubjectFrom returns the subject attached to r's context, or nil if there is
// none.
func SubjectFrom(r *http.Request) Subject {
	if s, ok := r.Context().Value(subjectKey).(Subject); ok {
		return s
	}
	return nil
}

// SubjectID returns the authenticated subject's ID, or "" when no subject is
// attached.
func SubjectID(r *http.Request) string {
	if s := SubjectFrom(r); s != nil {
		return s.SubjectID()
	}
	return ""
}
