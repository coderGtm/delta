package user

import (
	"net/http"

	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// UserHandler exposes the endpoints for a signed-in user's own account.
type UserHandler struct {
	Deleter    AccountDeleter
	Store      *db.Store
	TrustProxy bool
}

// NewHandler returns a UserHandler wired to the given account deleter, store,
// and proxy-header trust setting.
func NewHandler(deleter AccountDeleter, store *db.Store, trustProxy bool) *UserHandler {
	return &UserHandler{Deleter: deleter, Store: store, TrustProxy: trustProxy}
}

// DeleteMe handles DELETE /api/v1/users/me, soft-deleting the authenticated
// user's account.
func (h *UserHandler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(httpapi.SubjectID(r))
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	u, err := h.Store.Querier().GetUserByID(r.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	if err := h.Deleter.DeleteAccount(r.Context(), &u, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent")); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
