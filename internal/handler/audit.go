// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// audit.go — HTTP handler for /api/v1/audit (admin-only).

package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	authpkg "github.com/ALabiyb/platform_devportal/internal/auth"
)

// auditEventResponse is the safe JSON shape for an audit event.
type auditEventResponse struct {
	ID           string          `json:"id"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   *string         `json:"resource_id,omitempty"`
	UserID       *string         `json:"user_id,omitempty"`
	Detail       json.RawMessage `json:"detail,omitempty"`
	CreatedAt    string          `json:"created_at"`
}

// ListAuditEvents returns audit events for the org, newest first.
// Supports ?limit= query param (default 100, max 500).
func (h *Handler) ListAuditEvents(w http.ResponseWriter, r *http.Request) {
	user, _ := authpkg.UserFromContext(r.Context())

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	events, err := h.db.ListAuditEvents(r.Context(), user.OrgID, limit)
	if err != nil {
		jsonError(w, "list audit events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]auditEventResponse, len(events))
	for i, e := range events {
		row := auditEventResponse{
			ID:           e.ID.String(),
			Action:       e.Action,
			ResourceType: e.ResourceType,
			CreatedAt:    e.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if e.ResourceID != nil {
			s := e.ResourceID.String()
			row.ResourceID = &s
		}
		if e.UserID != nil {
			s := e.UserID.String()
			row.UserID = &s
		}
		if len(e.Detail) > 0 {
			row.Detail = json.RawMessage(e.Detail)
		}
		out[i] = row
	}
	jsonOK(w, out)
}
