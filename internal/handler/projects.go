// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// projects.go contains HTTP handlers for the /api/v1/projects routes.
//
// Full implementations land in:
//   - Day 09: provisioning orchestrator (CreateProject triggers the 15-step flow)
//   - Day 10: Jenkinsfile + K8s manifest generator (DownloadJenkinsfile)
//
// All handlers are stubs until then — they return 501 with a clear message
// so the router is fully wired on Day 04 and actual logic is dropped in later.

package handler

import "net/http"

// ListProjects returns the list of projects visible to the authenticated user.
// Scoped to the user's teams: developers see their own team's projects,
// admins see all projects across the org.
// Implemented: Day 09 (provisioning orchestrator + project list query)
func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	stub(w, "list projects", "Day 09")
}

// GetProject returns a single project by ID including status and step summary.
// Implemented: Day 09
func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	stub(w, "get project", "Day 09")
}

// CreateProject validates the project form, persists the project row, and
// fires the 15-step provisioning orchestrator as a background goroutine.
// The SSE stream (/projects/{id}/stream) reports live progress to the browser.
// Implemented: Day 09
func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	stub(w, "create project + start provisioning", "Day 09")
}

// ListEnvironments returns the environments (dev / uat / prod) for a project,
// including K8s namespace, ingress URL, ArgoCD app name, and DB credentials.
// Implemented: Day 09
func (h *Handler) ListEnvironments(w http.ResponseWriter, r *http.Request) {
	stub(w, "list project environments", "Day 09")
}

// ListProvisioningSteps returns the ordered list of provisioning steps and
// their current status (pending / running / done / failed) for a project.
// The SSE stream sends these same rows as live events during provisioning.
// Implemented: Day 09
func (h *Handler) ListProvisioningSteps(w http.ResponseWriter, r *http.Request) {
	stub(w, "list provisioning steps", "Day 09")
}

// StreamProvisioningProgress upgrades the connection to an SSE stream and
// broadcasts step-status updates to every connected browser tab watching
// this project's provisioning progress.
// Implemented: Day 09 (SSE broadcast hub)
func (h *Handler) StreamProvisioningProgress(w http.ResponseWriter, r *http.Request) {
	stub(w, "SSE provisioning progress stream", "Day 09")
}

// DownloadJenkinsfile returns the generated Jenkinsfile text stored in the DB
// for re-download after provisioning is complete.
// Implemented: Day 10 (Jenkinsfile generator)
func (h *Handler) DownloadJenkinsfile(w http.ResponseWriter, r *http.Request) {
	stub(w, "download generated Jenkinsfile", "Day 10")
}
