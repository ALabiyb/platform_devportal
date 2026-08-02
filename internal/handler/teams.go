// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// teams.go contains HTTP handlers for the /api/v1/teams routes.
//
// Teams group developers within an org. Projects belong to a team.
// Full implementations land in Day 09 alongside the provisioning layer.

package handler

import "net/http"

// ListTeams returns all teams in the authenticated user's org.
// Implemented: Day 09
func (h *Handler) ListTeams(w http.ResponseWriter, r *http.Request) {
	stub(w, "list teams", "Day 09")
}

// GetTeam returns a single team by ID including its members.
// Implemented: Day 09
func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	stub(w, "get team", "Day 09")
}

// CreateTeam creates a new team within the org.
// Requires admin role — only platform engineers can create teams.
// Implemented: Day 09
func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	stub(w, "create team", "Day 09")
}
