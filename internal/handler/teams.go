// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// teams.go contains HTTP handlers for the /api/v1/teams routes.

package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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

// ListTeamMembers returns all members of a team.
func (h *Handler) ListTeamMembers(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "teamID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	members, err := h.db.ListTeamMembers(r.Context(), id)
	if err != nil {
		jsonError(w, "list team members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, members)
}

// AddTeamMember adds a user to a team. Requires admin role.
func (h *Handler) AddTeamMember(w http.ResponseWriter, r *http.Request) {
	teamID, err := parseUUID(r, "teamID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		jsonError(w, "user_id is required", http.StatusBadRequest)
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		jsonError(w, "invalid user_id", http.StatusBadRequest)
		return
	}
	role := req.Role
	if role != "lead" && role != "member" {
		role = "member"
	}
	if err := h.db.AddTeamMember(r.Context(), teamID, userID, role); err != nil {
		jsonError(w, "add team member: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveTeamMember removes a user from a team. Requires admin role.
func (h *Handler) RemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	teamID, err := parseUUID(r, "teamID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	userID, err := parseUUID(r, "userID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.db.RemoveTeamMember(r.Context(), teamID, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, "member not found", http.StatusNotFound)
			return
		}
		jsonError(w, "remove team member: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateTeam renames a team. Requires admin role.
func (h *Handler) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "teamID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	team, err := h.db.RenameTeam(r.Context(), id, req.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, "team not found", http.StatusNotFound)
			return
		}
		jsonError(w, "rename team: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, team)
}

// DeleteTeam deletes a team. Fails if projects are still assigned. Requires admin.
func (h *Handler) DeleteTeam(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "teamID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.db.DeleteTeam(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, "team not found", http.StatusNotFound)
			return
		}
		jsonError(w, "delete team: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
