package auth

import (
	"net/http"

	"github.com/coderGtm/delta/go/httpapi"
	"github.com/google/uuid"
)

// Handlers exposes the auth endpoints.
type Handlers struct {
	Svc        *Service
	TrustProxy bool
}

// loginRequest is the body of POST /api/v1/auth/login.
type loginRequest struct {
	FirebaseIDToken string `json:"firebaseIdToken"`
}

// refreshRequest is the body of POST /api/v1/auth/refresh.
type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// logoutRequest is the body of POST /api/v1/auth/logout.
type logoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// Login handles POST /api/v1/auth/login.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	if req.FirebaseIDToken == "" {
		httpapi.WriteError(w, httpapi.Validation("must not be blank"))
		return
	}
	if len(req.FirebaseIDToken) > 8192 {
		httpapi.WriteError(w, httpapi.Validation("size must be between 0 and 8192"))
		return
	}
	resp, err := h.Svc.Login(r.Context(), req.FirebaseIDToken, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *Handlers) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	if req.RefreshToken == "" {
		httpapi.WriteError(w, httpapi.Validation("must not be blank"))
		return
	}
	if len(req.RefreshToken) > 512 {
		httpapi.WriteError(w, httpapi.Validation("size must be between 0 and 512"))
		return
	}
	resp, err := h.Svc.Refresh(r.Context(), req.RefreshToken, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

// Logout handles POST /api/v1/auth/logout.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	if req.RefreshToken == "" {
		httpapi.WriteError(w, httpapi.Validation("must not be blank"))
		return
	}
	if len(req.RefreshToken) > 512 {
		httpapi.WriteError(w, httpapi.Validation("size must be between 0 and 512"))
		return
	}
	if err := h.Svc.Logout(r.Context(), req.RefreshToken); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// LogoutAll handles POST /api/v1/auth/logout-all.
func (h *Handlers) LogoutAll(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(httpapi.SubjectID(r))
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	if err := h.Svc.LogoutAll(r.Context(), userID, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent")); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
