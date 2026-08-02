// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// users.go contains HTTP handlers for the /api/v1/users routes.
//
// All routes are admin-only (gated in handler.go with RequireRole(RoleAdmin)).
// User creation via POST /api/v1/users is meaningful only in local auth mode —
// in OIDC mode, Keycloak manages user accounts directly.
//
// Full implementations land in Day 15 (credentials + audit log + user mgmt UI).

package handler

import "net/http"

// ListUsers returns all users in the org.
// Response omits password_hash — it is never returned to the client.
// Implemented: Day 15
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	stub(w, "list users", "Day 15")
}

// CreateUser creates a new local-auth user account.
// Only meaningful when AUTH_MODE=local. In OIDC mode, Keycloak manages accounts.
// Body: {"email":"…","display_name":"…","password":"…","role":"developer"}
// Implemented: Day 15
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	stub(w, "create local user account", "Day 15")
}

// DeactivateUser sets is_active=false, preventing login without deleting audit history.
// DELETE /api/v1/users/{userID}
// Implemented: Day 15
func (h *Handler) DeactivateUser(w http.ResponseWriter, r *http.Request) {
	stub(w, "deactivate user account", "Day 15")
}

// UpdateUserRole changes the RBAC role for a user.
// POST /api/v1/users/{userID}/role
// Body: {"role":"admin"}
// Implemented: Day 15
func (h *Handler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	stub(w, "update user role", "Day 15")
}
