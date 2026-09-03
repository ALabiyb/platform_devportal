// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com

package handler

import (
	"net/http"

	authpkg "github.com/ALabiyb/platform_devportal/internal/auth"
)

// GetDashboard returns aggregated stats, cluster summaries, and recent activity
// for the authenticated user's org. Used by the Platform Overview page.
func (h *Handler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	user, _ := authpkg.UserFromContext(r.Context())
	data, err := h.db.GetDashboardData(r.Context(), user.OrgID)
	if err != nil {
		jsonError(w, "dashboard: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, data)
}
