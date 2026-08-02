// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// credentials.go contains HTTP handlers for the /api/v1/credentials routes.
//
// Workspace credentials are org-level secrets (GitLab PAT, Jenkins token,
// Harbor password, etc.) stored AES-256-GCM encrypted in PostgreSQL.
// The plaintext never touches the DB — only the ciphertext.
//
// All routes are admin-only (gated in handler.go with RequireRole(RoleAdmin)).
// Full implementations land in Day 15 (credentials + audit log UI).

package handler

import "net/http"

// ListCredentials returns all workspace credentials for the org.
// Values are returned as masked placeholders (never decrypted for listing).
// Implemented: Day 15
func (h *Handler) ListCredentials(w http.ResponseWriter, r *http.Request) {
	stub(w, "list workspace credentials", "Day 15")
}

// CreateCredential encrypts the provided plaintext value with AES-256-GCM
// and stores the ciphertext in the workspace_credentials table.
// Implemented: Day 15
func (h *Handler) CreateCredential(w http.ResponseWriter, r *http.Request) {
	stub(w, "create encrypted workspace credential", "Day 15")
}

// DeleteCredential removes a credential from the vault.
// Writes an audit event so the deletion is traceable.
// Implemented: Day 15
func (h *Handler) DeleteCredential(w http.ResponseWriter, r *http.Request) {
	stub(w, "delete workspace credential", "Day 15")
}
