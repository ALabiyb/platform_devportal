// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// teams.go contains HTTP handlers for the /api/v1/teams routes.

package handler

import (
	"encoding/json"
	"net/http"

	authpkg "github.com/ALabiyb/platform_devportal/internal/auth"
	"github.com/ALabiyb/platform_devportal/internal/db"
)

// ListTeams returns all teams in the authenticated user's org.
func (h *Handler) ListTeams(w http.ResponseWriter, r *http.Request) {
	user, _ := authpkg.UserFromContext(r.Context())
	teams, err := h.db.ListTeamsByOrg(r.Context(), user.OrgID)
	if err != nil {
		jsonError(w, "list teams: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, teams)
}

// GetTeam returns a single team by ID.
func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "teamID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	team, err := h.db.GetTeam(r.Context(), id)
	if err != nil {
		jsonError(w, "team not found", http.StatusNotFound)
		return
	}
	jsonOK(w, team)
}

// createTeamRequest is the JSON body for POST /api/v1/teams.
type createTeamRequest struct {
	Name string `json:"name"`
}

// CreateTeam creates a new team within the org. Requires admin role.
func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	user, _ := authpkg.UserFromContext(r.Context())

	var req createTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	team, err := h.db.CreateTeam(r.Context(), db.Team{
		OrgID: user.OrgID,
		Name:  req.Name,
		Slug:  slugify(req.Name),
	})
	if err != nil {
		jsonError(w, "create team: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(team)
}
