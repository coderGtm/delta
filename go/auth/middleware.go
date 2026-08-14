// Package auth implements authentication primitives: the JWT access-token
// signing and verification service, the refresh-token lifecycle of creation,
// validation, rotation, revocation, and cleanup, a Firebase ID-token wrapper,
// the auth business service, and the HTTP middleware and handlers that expose
// them.
package auth

import (
	"net/http"
	"strings"

	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/httpapi"
)

// AttachUser returns middleware that resolves a Bearer access token to the
// matching active user and attaches it to the request context. Requests
// without a valid token, or whose user cannot be loaded, pass through
// unauthenticated so that Require can reject them.
func AttachUser(jwt *JWTService, store *db.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hdr := r.Header.Get("Authorization")
			if !strings.HasPrefix(hdr, "Bearer ") {
				next.ServeHTTP(w, r)
				return
			}
			userID, err := jwt.ParseAccessToken(strings.TrimPrefix(hdr, "Bearer "))
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			user, err := store.Querier().GetUserByID(r.Context(), pgUUID(userID))
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, httpapi.WithSubject(r, user))
		})
	}
}

// Require returns middleware that responds 401 with an empty body and a
// WWW-Authenticate challenge when the request carries no authenticated
// subject.
func Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if httpapi.SubjectFrom(r) == nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
