// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// templates.go — admin endpoints for viewing and editing pipeline templates.
//
// GET  /api/v1/admin/templates           → list all build tools + their templates
// GET  /api/v1/admin/templates/{tool}    → get one template (jenkinsfile + dockerfile)
// PUT  /api/v1/admin/templates/{tool}    → update jenkinsfile and/or dockerfile

package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	authpkg "github.com/ALabiyb/platform_devportal/internal/auth"
)

// ListTemplates returns all pipeline_templates rows ordered by build tool name.
// GET /api/v1/admin/templates
func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.db.ListPipelineTemplates(r.Context())
	if err != nil {
		jsonError(w, "list templates: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, templates)
}

// GetTemplate returns one build tool's Jenkinsfile + Dockerfile.
// GET /api/v1/admin/templates/{tool}
func (h *Handler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	tool := chi.URLParam(r, "tool")
	t, err := h.db.GetPipelineTemplate(r.Context(), tool)
	if err != nil {
		jsonError(w, "template not found", http.StatusNotFound)
		return
	}
	jsonOK(w, t)
}

// UpdateTemplate replaces the Jenkinsfile and/or Dockerfile for a build tool.
// PUT /api/v1/admin/templates/{tool}
func (h *Handler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	user, _ := authpkg.UserFromContext(r.Context())
	tool := chi.URLParam(r, "tool")

	var req struct {
		Jenkinsfile string `json:"jenkinsfile"`
		Dockerfile  string `json:"dockerfile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Jenkinsfile == "" && req.Dockerfile == "" {
		jsonError(w, "jenkinsfile or dockerfile is required", http.StatusBadRequest)
		return
	}

	// Fetch existing values so a partial update (only one field) preserves the other.
	existing, err := h.db.GetPipelineTemplate(r.Context(), tool)
	if err != nil {
		jsonError(w, "template not found", http.StatusNotFound)
		return
	}

	jf := existing.Jenkinsfile
	if req.Jenkinsfile != "" {
		jf = req.Jenkinsfile
	}
	df := existing.Dockerfile
	if req.Dockerfile != "" {
		df = req.Dockerfile
	}

	if err := h.db.UpsertPipelineTemplate(r.Context(), tool, jf, df, &user.ID); err != nil {
		jsonError(w, "update template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]string{"status": "updated", "build_tool": tool})
}
