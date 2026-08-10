// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// projects.go contains HTTP handlers for the /api/v1/projects routes.
// All project operations — listing, creating, SSE streaming, and Jenkinsfile
// download — live here. CreateProject triggers the 15-step provisioning
// orchestrator as a background goroutine and returns 201 immediately.

package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	authpkg "github.com/ALabiyb/platform_devportal/internal/auth"
	"github.com/ALabiyb/platform_devportal/internal/db"
	"github.com/ALabiyb/platform_devportal/internal/provisioner"
)

// ── List / Get ────────────────────────────────────────────────────────────────

// ListProjects returns all projects in the authenticated user's org.
func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	user, _ := authpkg.UserFromContext(r.Context())
	projects, err := h.db.ListProjectsByOrg(r.Context(), user.OrgID)
	if err != nil {
		jsonError(w, "list projects: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, projects)
}

// GetProject returns a single project by ID.
func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "projectID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	project, err := h.db.GetProject(r.Context(), id)
	if err != nil {
		jsonError(w, "project not found", http.StatusNotFound)
		return
	}
	jsonOK(w, project)
}

// ListEnvironments returns the environments (dev / uat / prod) for a project.
func (h *Handler) ListEnvironments(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "projectID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	envs, err := h.db.GetEnvironmentsByProject(r.Context(), id)
	if err != nil {
		jsonError(w, "list environments: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, envs)
}

// ListProvisioningSteps returns the ordered list of provisioning steps for a project.
func (h *Handler) ListProvisioningSteps(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "projectID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	steps, err := h.db.GetProvisioningSteps(r.Context(), id)
	if err != nil {
		jsonError(w, "list steps: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, steps)
}

// ── Create ────────────────────────────────────────────────────────────────────

// createProjectRequest is the JSON body for POST /api/v1/projects.
type createProjectRequest struct {
	Name              string    `json:"name"`
	TeamID            uuid.UUID `json:"team_id"`
	BuildTool         string    `json:"build_tool"`
	GitNamespace      string    `json:"git_namespace"`      // GitLab group path, e.g. "nexbridge/backend"
	NotificationEmail string    `json:"notification_email"`
	// Optional per-service Jenkins / pipeline settings
	AppTimezone      string `json:"app_timezone"`       // e.g. "Africa/Dar_es_Salaam"
	StagingURL       string `json:"staging_url"`        // optional; enables DAST if set
	K8sManifestPaths string `json:"k8s_manifest_paths"` // e.g. "04-deployment.yaml"
	// Security SLA — days to remediate CVEs before DefectDojo marks SLA-breached.
	// Defaults: Critical=7, High=30, Medium=90, Low=180.
	VulnSLACritical int `json:"vuln_sla_critical"`
	VulnSLAHigh     int `json:"vuln_sla_high"`
	VulnSLAMedium   int `json:"vuln_sla_medium"`
	VulnSLALow      int `json:"vuln_sla_low"`
	// Members are the DevPortal users assigned to this service.
	// Only assigned members will see this service's findings in DefectDojo.
	Members []projectMemberInput `json:"members"`
}

// projectMemberInput is one entry in the members list of createProjectRequest.
type projectMemberInput struct {
	UserID uuid.UUID `json:"user_id"`
	Role   string    `json:"role"` // "lead" | "developer"
}

// CreateProject validates the form, persists the project + environments +
// provisioning step rows, then launches the orchestrator as a background goroutine.
// Returns 201 immediately so the browser can connect to the SSE stream.
func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	user, _ := authpkg.UserFromContext(r.Context())

	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.GitNamespace == "" || req.TeamID == uuid.Nil {
		jsonError(w, "name, git_namespace, and team_id are required", http.StatusBadRequest)
		return
	}
	if req.BuildTool == "" {
		req.BuildTool = "maven"
	}

	slug := slugify(req.Name)
	harborProject := h.cfg.K8sManifestGroup + "/" + slug // reuse manifest group as Harbor namespace prefix

	// Store team slug as Jenkins folder name
	team, err := h.db.GetTeam(r.Context(), req.TeamID)
	if err != nil {
		jsonError(w, "team not found", http.StatusBadRequest)
		return
	}

	tz := req.AppTimezone
	if tz == "" {
		tz = "Africa/Dar_es_Salaam"
	}
	manifestPaths := req.K8sManifestPaths
	if manifestPaths == "" {
		manifestPaths = "04-deployment.yaml"
	}
	var stagingURL *string
	if req.StagingURL != "" {
		stagingURL = &req.StagingURL
	}

	// Apply SLA defaults (OWASP remediation guidance).
	slaC := req.VulnSLACritical
	if slaC == 0 {
		slaC = 7
	}
	slaH := req.VulnSLAHigh
	if slaH == 0 {
		slaH = 30
	}
	slaM := req.VulnSLAMedium
	if slaM == 0 {
		slaM = 90
	}
	slaL := req.VulnSLALow
	if slaL == 0 {
		slaL = 180
	}

	project, err := h.db.CreateProject(r.Context(), db.Project{
		TeamID:            &req.TeamID,
		Name:              req.Name,
		Slug:              slug,
		GitRepoURL:        "", // filled in after EnsureRepo in step 1
		HarborProject:     harborProject,
		JenkinsFolder:     team.Slug,
		BuildTool:         req.BuildTool,
		NotificationEmail: req.NotificationEmail,
		CreatedBy:         &user.ID,
		AppTimezone:       tz,
		StagingURL:        stagingURL,
		K8sManifestPaths:  manifestPaths,
		VulnSLACritical:   slaC,
		VulnSLAHigh:       slaH,
		VulnSLAMedium:     slaM,
		VulnSLALow:        slaL,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			jsonError(w, `A project named "`+req.Name+`" already exists.`, http.StatusConflict)
			return
		}
		jsonError(w, "create project: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Assign service members so the orchestrator can sync them to DefectDojo in step 10.
	// Errors are non-fatal — provisioning continues without the member assignment.
	for _, m := range req.Members {
		role := m.Role
		if role != "lead" && role != "developer" {
			role = "developer"
		}
		_ = h.db.AddProjectMember(r.Context(), project.ID, m.UserID, role, &user.ID)
	}
	// Always add the creating user as lead if not already in the list.
	_ = h.db.AddProjectMember(r.Context(), project.ID, user.ID, "lead", nil)

	// Insert all 15 step rows upfront so the SSE stream can display them immediately.
	steps := make([]db.ProvisioningStep, len(provisioner.StepLabels))
	for i, label := range provisioner.StepLabels {
		steps[i] = db.ProvisioningStep{
			ProjectID: project.ID,
			StepIndex: i + 1,
			Label:     label,
		}
	}
	if err := h.db.CreateProvisioningSteps(r.Context(), steps); err != nil {
		jsonError(w, "create provisioning steps: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Pre-create environment rows so the SSE and ArgoCD steps can reference them.
	envs := make([]*db.Environment, 0, 3)
	for _, envName := range []string{"dev", "uat", "prod"} {
		env, err := h.db.CreateEnvironment(r.Context(), db.Environment{
			ProjectID: project.ID,
			Name:      envName,
			Namespace: slug + "-" + envName,
		})
		if err != nil {
			jsonError(w, fmt.Sprintf("create %s environment: %s", envName, err), http.StatusInternalServerError)
			return
		}
		envs = append(envs, env)
	}

	// Enqueue the provisioning job. The embedded worker claims and runs the
	// full 15-step orchestrator asynchronously. ApplicationSlug is empty here
	// because CreateProject is the legacy standalone-project endpoint.
	if _, err := h.db.EnqueueProvisioningJob(r.Context(), project.ID, db.JobPayload{
		GitNamespace: req.GitNamespace,
	}); err != nil {
		jsonError(w, "enqueue provisioning: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":         project.ID,
		"name":       project.Name,
		"slug":       project.Slug,
		"status":     project.Status,
		"stream_url": fmt.Sprintf("/api/v1/projects/%s/stream", project.ID),
		"steps_url":  fmt.Sprintf("/api/v1/projects/%s/steps", project.ID),
	})
}

// ── SSE stream ────────────────────────────────────────────────────────────────

// StreamProvisioningProgress upgrades to an SSE stream and broadcasts
// step-status updates to every connected browser tab watching this project.
func (h *Handler) StreamProvisioningProgress(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "projectID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx/traefik response buffering
	w.WriteHeader(http.StatusOK)

	// Send current step snapshot so a late-joining browser sees all steps at once.
	steps, _ := h.db.GetProvisioningSteps(r.Context(), id)
	for _, s := range steps {
		detail := ""
		if s.Detail != nil {
			detail = *s.Detail
		}
		event := provisioner.StepEvent{
			StepIndex: s.StepIndex,
			Label:     s.Label,
			Status:    s.Status,
			Detail:    detail,
		}
		writeSSE(w, event)
	}
	flusher.Flush()

	// Subscribe to live updates from the orchestrator.
	ch, unsubscribe := h.hub.Subscribe(id)
	defer unsubscribe()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(w, event)
			flusher.Flush()
			if event.Done {
				return // provisioning finished — close the stream
			}
		case <-r.Context().Done():
			return // browser disconnected
		}
	}
}

// writeSSE encodes event as JSON and writes one SSE data frame.
func writeSSE(w http.ResponseWriter, event provisioner.StepEvent) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "data: %s\n\n", data)
}

// ── Jenkinsfile download ───────────────────────────────────────────────────────

// DownloadJenkinsfile returns the stored Jenkinsfile text for re-download.
func (h *Handler) DownloadJenkinsfile(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "projectID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	project, err := h.db.GetProject(r.Context(), id)
	if err != nil || project.GeneratedJenkinsfile == nil {
		jsonError(w, "Jenkinsfile not yet generated", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="Jenkinsfile"`)
	fmt.Fprint(w, *project.GeneratedJenkinsfile)
}

// ── shared handler utilities ──────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func parseUUID(r *http.Request, param string) (uuid.UUID, error) {
	raw := chi.URLParam(r, param)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s: %s", param, raw)
	}
	return id, nil
}

// slugify converts a project name to a URL-safe lowercase slug.
func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		if r == ' ' || r == '_' {
			return '-'
		}
		return -1
	}, s)
	// Collapse consecutive hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
