// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// setup.go exposes GET /setup/status — a public endpoint the landing page
// calls before rendering its CTAs.
//
// When needs_setup is true (zero users in the DB) the landing page shows the
// "Create admin account" button. Once the first admin exists the button is
// hidden permanently — the register endpoint also enforces this server-side.

package handler

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	org, err := h.db.GetOrgBySlug(r.Context(), h.cfg.OrgSlug)
	if err != nil {
		// No org yet → system definitely needs setup.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"needs_setup": true})
		return
	}

	count, err := h.db.CountUsers(r.Context(), org.ID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"needs_setup": count == 0})
}
