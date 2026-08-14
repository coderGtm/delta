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

func WithSubject(r *http.Request, s Subject) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), subjectKey, s))
}

func SubjectFrom(r *http.Request) Subject {
	if s, ok := r.Context().Value(subjectKey).(Subject); ok {
		return s
	}
	return nil
}

func SubjectID(r *http.Request) string {
	if s := SubjectFrom(r); s != nil {
		return s.SubjectID()
	}
	return ""
}
