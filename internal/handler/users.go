// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// users.go — HTTP handlers for /api/v1/users (admin-only).

package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	authpkg "github.com/ALabiyb/platform_devportal/internal/auth"
)

// userResponse is the safe JSON shape returned for a user — never includes password_hash.
type userResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	IsActive    bool   `json:"is_active"`
	Provider    string `json:"provider"`
	CreatedAt   string `json:"created_at"`
}

// ListUsers returns all users in the org.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	user, _ := authpkg.UserFromContext(r.Context())
	users, err := h.db.ListUsers(r.Context(), user.OrgID)
	if err != nil {
		jsonError(w, "list users: "+err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]userResponse, len(users))
	for i, u := range users {
		out[i] = userResponse{
			ID:          u.ID.String(),
			Email:       u.Email,
			DisplayName: u.DisplayName,
			Role:        u.Role,
			IsActive:    u.IsActive,
			Provider:    u.Provider,
			CreatedAt:   u.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	jsonOK(w, out)
}

// CreateUser creates a new local-auth user. Requires local AUTH_MODE.
// Body: {"email":"…","display_name":"…","password":"…","role":"developer"}
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if h.cfg.AuthMode != "local" {
		jsonError(w, "user creation via API is only available in local auth mode", http.StatusBadRequest)
		return
	}

	var req struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
		Role        string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.DisplayName == "" || req.Password == "" {
		jsonError(w, "email, display_name, and password are required", http.StatusBadRequest)
		return
	}
	role := req.Role
	if role == "" {
		role = authpkg.RoleDeveloper
	}

	admin, _ := authpkg.UserFromContext(r.Context())
	org, err := h.db.EnsureOrg(r.Context(), h.cfg.OrgName, h.cfg.OrgSlug)
	if err != nil {
		jsonError(w, "failed to resolve org", http.StatusInternalServerError)
		return
	}
	_ = admin

	localProvider := authpkg.NewLocalProvider(h.db, h.cfg)
	user, err := localProvider.CreateUser(r.Context(), org.ID, req.Email, req.DisplayName, role, req.Password)
	if err != nil {
		jsonError(w, "create user: "+err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(userResponse{
		ID:          user.ID.String(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		IsActive:    user.IsActive,
		Provider:    user.Provider,
		CreatedAt:   user.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// DeactivateUser sets is_active=false for the given user.
// DELETE /api/v1/users/{userID}
func (h *Handler) DeactivateUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "userID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.db.DeactivateUser(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, "user not found", http.StatusNotFound)
			return
		}
		jsonError(w, "deactivate user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateUserRole changes the RBAC role for a user.
// POST /api/v1/users/{userID}/role  — Body: {"role":"admin"}
func (h *Handler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "userID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Role == "" {
		jsonError(w, "role is required", http.StatusBadRequest)
		return
	}
	switch req.Role {
	case authpkg.RoleAdmin, authpkg.RoleDeveloper, authpkg.RoleViewer:
	default:
		jsonError(w, "role must be admin, developer, or viewer", http.StatusBadRequest)
		return
	}
	if err := h.db.UpdateUserRole(r.Context(), id, req.Role); err != nil {
		jsonError(w, "update role: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// chi.URLParam needs the chi import — use the existing parseUUID helper in projects.go.
// This file relies on chi being imported transitively; add the direct import for URLParam.
var _ = chi.URLParam
