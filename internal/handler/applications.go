// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// applications.go — HTTP handlers for /api/v1/applications.
//
// Access model (least privilege):
//   - Any authenticated user can create an application (becomes its lead).
//   - Only members (or admins) can read an application and its services.
//   - Only leads (or admins) can add/remove members.
//   - Any member can create services under an application.

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	authpkg "github.com/ALabiyb/platform_devportal/internal/auth"
	"github.com/ALabiyb/platform_devportal/internal/db"
	"github.com/ALabiyb/platform_devportal/internal/provisioner"
)

// ── Application CRUD ─────────────────────────────────────────────────────────

// ListApplications returns the applications the current user is a member of.
// Admins see all applications in the org.
func (h *Handler) ListApplications(w http.ResponseWriter, r *http.Request) {
	user, _ := authpkg.UserFromContext(r.Context())
	var (
		apps []db.Application
		err  error
	)
	if user.Role == authpkg.RoleAdmin {
		apps, err = h.db.ListApplicationsByOrg(r.Context(), user.OrgID)
	} else {
		apps, err = h.db.ListApplicationsByUser(r.Context(), user.ID)
	}
	if err != nil {
		jsonError(w, "list applications: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, apps)
}

// GetApplication returns a single application. Requires membership or admin.
func (h *Handler) GetApplication(w http.ResponseWriter, r *http.Request) {
	appID, err := parseUUID(r, "appID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, _ := authpkg.UserFromContext(r.Context())
	if !h.isMember(r.Context(), user, appID) {
		jsonError(w, "not a member of this application", http.StatusForbidden)
		return
	}
	app, err := h.db.GetApplication(r.Context(), appID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, "application not found", http.StatusNotFound)
			return
		}
		jsonError(w, "get application: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, app)
}

// CreateApplication creates a new application, provisions its GitLab group,
// and adds the creator as its first lead member.
func (h *Handler) CreateApplication(w http.ResponseWriter, r *http.Request) {
	user, _ := authpkg.UserFromContext(r.Context())

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	slug := slugify(req.Name)
	org, err := h.db.EnsureOrg(r.Context(), h.cfg.OrgName, h.cfg.OrgSlug)
	if err != nil {
		jsonError(w, "resolve org: "+err.Error(), http.StatusInternalServerError)
		return
	}

	app, err := h.db.CreateApplication(r.Context(), db.Application{
		OrgID:        org.ID,
		Name:         req.Name,
		Slug:         slug,
		Description:  req.Description,
		GitNamespace: slug,
		CreatedBy:    &user.ID,
	})
	if err != nil {
		jsonError(w, "create application: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create the GitLab group at the root level.
	if h.git != nil {
		if _, err := h.git.EnsureGroup(r.Context(), slug, ""); err != nil {
			jsonError(w, "application saved but GitLab group creation failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Add creator as Maintainer in GitLab.
		_ = h.git.AddGroupMember(r.Context(), slug, user.Email, 40)
	}

	// Make creator the lead in the DB.
	if err := h.db.AddApplicationMember(r.Context(), app.ID, user.ID, user.ID, "lead"); err != nil {
		jsonError(w, "add lead member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(app)
}

// UpdateApplication renames an application / updates its description. Requires admin or lead.
func (h *Handler) UpdateApplication(w http.ResponseWriter, r *http.Request) {
	appID, err := parseUUID(r, "appID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, _ := authpkg.UserFromContext(r.Context())
	if !h.isMember(r.Context(), user, appID) {
		jsonError(w, "not a member of this application", http.StatusForbidden)
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	app, err := h.db.UpdateApplication(r.Context(), appID, req.Name, req.Description)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, "application not found", http.StatusNotFound)
			return
		}
		jsonError(w, "update application: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, app)
}

// DeleteApplication archives an application. Requires admin role.
func (h *Handler) DeleteApplication(w http.ResponseWriter, r *http.Request) {
	appID, err := parseUUID(r, "appID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.db.DeleteApplication(r.Context(), appID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, "application not found", http.StatusNotFound)
			return
		}
		jsonError(w, "delete application: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Membership ────────────────────────────────────────────────────────────────

// ListApplicationMembers returns all members of an application.
func (h *Handler) ListApplicationMembers(w http.ResponseWriter, r *http.Request) {
	appID, err := parseUUID(r, "appID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, _ := authpkg.UserFromContext(r.Context())
	if !h.isMember(r.Context(), user, appID) {
		jsonError(w, "not a member of this application", http.StatusForbidden)
		return
	}
	members, err := h.db.ListApplicationMembers(r.Context(), appID)
	if err != nil {
		jsonError(w, "list members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, members)
}

// AddApplicationMember adds a user to an application. Requires lead or admin.
// Body: {"user_id":"<uuid>","role":"developer"}
func (h *Handler) AddApplicationMember(w http.ResponseWriter, r *http.Request) {
	appID, err := parseUUID(r, "appID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	caller, _ := authpkg.UserFromContext(r.Context())
	if !h.isLead(r.Context(), caller, appID) {
		jsonError(w, "only application leads or admins can manage members", http.StatusForbidden)
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
	targetID, err := uuid.Parse(req.UserID)
	if err != nil {
		jsonError(w, "invalid user_id", http.StatusBadRequest)
		return
	}
	role := req.Role
	if role != "lead" && role != "developer" {
		role = "developer"
	}

	targetUser, err := h.db.GetUserByID(r.Context(), targetID)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}
	app, _ := h.db.GetApplication(r.Context(), appID)

	if err := h.db.AddApplicationMember(r.Context(), appID, targetID, caller.ID, role); err != nil {
		jsonError(w, "add member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Sync to GitLab group.
	if h.git != nil && app != nil {
		level := 30 // Developer
		if role == "lead" {
			level = 40 // Maintainer
		}
		_ = h.git.AddGroupMember(r.Context(), app.GitNamespace, targetUser.Email, level)
	}

	w.WriteHeader(http.StatusNoContent)
}

// RemoveApplicationMember removes a user from an application. Requires lead or admin.
func (h *Handler) RemoveApplicationMember(w http.ResponseWriter, r *http.Request) {
	appID, err := parseUUID(r, "appID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	caller, _ := authpkg.UserFromContext(r.Context())
	if !h.isLead(r.Context(), caller, appID) {
		jsonError(w, "only application leads or admins can manage members", http.StatusForbidden)
		return
	}

	targetID, err := parseUUID(r, "userID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	targetUser, _ := h.db.GetUserByID(r.Context(), targetID)
	app, _ := h.db.GetApplication(r.Context(), appID)

	if err := h.db.RemoveApplicationMember(r.Context(), appID, targetID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, "member not found", http.StatusNotFound)
			return
		}
		jsonError(w, "remove member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Sync removal to GitLab.
	if h.git != nil && app != nil && targetUser != nil {
		_ = h.git.RemoveGroupMember(r.Context(), app.GitNamespace, targetUser.Email)
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── Services (projects under an application) ──────────────────────────────────

// ListServices returns all services (projects) belonging to an application.
func (h *Handler) ListServices(w http.ResponseWriter, r *http.Request) {
	appID, err := parseUUID(r, "appID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, _ := authpkg.UserFromContext(r.Context())
	if !h.isMember(r.Context(), user, appID) {
		jsonError(w, "not a member of this application", http.StatusForbidden)
		return
	}
	services, err := h.db.ListServicesByApplication(r.Context(), appID)
	if err != nil {
		jsonError(w, "list services: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, services)
}

// CreateService creates a microservice under an application and launches the
// 15-step provisioner. GitLab namespace is inherited from the application.
// Body: {"name":"api-gateway","build_tool":"go","notification_email":"..."}
func (h *Handler) CreateService(w http.ResponseWriter, r *http.Request) {
	appID, err := parseUUID(r, "appID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, _ := authpkg.UserFromContext(r.Context())
	if !h.isMember(r.Context(), user, appID) {
		jsonError(w, "not a member of this application", http.StatusForbidden)
		return
	}

	app, err := h.db.GetApplication(r.Context(), appID)
	if err != nil {
		jsonError(w, "application not found", http.StatusNotFound)
		return
	}

	var req struct {
		Name              string `json:"name"`
		BuildTool         string `json:"build_tool"`
		NotificationEmail string `json:"notification_email"`
		AppTimezone       string `json:"app_timezone"`
		StagingURL        string `json:"staging_url"`
		Port              int    `json:"port"`
		HealthPath        string `json:"health_path"`
		VulnSLACritical   int    `json:"vuln_sla_critical"`
		VulnSLAHigh       int    `json:"vuln_sla_high"`
		VulnSLAMedium     int    `json:"vuln_sla_medium"`
		VulnSLALow        int    `json:"vuln_sla_low"`
		Members           []struct {
			UserID uuid.UUID `json:"user_id"`
			Role   string    `json:"role"`
		} `json:"members"`
		// InfraRequirements lists the shared infrastructure this service needs.
		// Each entry maps to a row in service_infra_requirements.
		InfraRequirements []struct {
			ServiceType string          `json:"service_type"`
			Config      json.RawMessage `json:"config"`
		} `json:"infra_requirements"`
		// TalksTo lists IDs of other services in the same application that this
		// service calls. Used to generate NetworkPolicy CRs.
		TalksTo []struct {
			ProjectID uuid.UUID `json:"project_id"`
			Port      int       `json:"port"`
		} `json:"talks_to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.BuildTool == "" {
		req.BuildTool = "auto"
	}

	tz := req.AppTimezone
	if tz == "" {
		tz = "Africa/Dar_es_Salaam"
	}
	port := req.Port
	if port == 0 {
		port = 8080
	}
	healthPath := req.HealthPath
	if healthPath == "" {
		healthPath = "/healthz"
	}
	var stagingURL *string
	if req.StagingURL != "" {
		stagingURL = &req.StagingURL
	}

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

	slug := slugify(req.Name)

	project, err := h.db.CreateProject(r.Context(), db.Project{
		ApplicationID:     &app.ID,
		Name:              req.Name,
		Slug:              slug,
		BuildTool:         req.BuildTool,
		NotificationEmail: req.NotificationEmail,
		HarborProject:     app.Slug + "/" + slug,
		JenkinsFolder:     app.Slug,
		CreatedBy:         &user.ID,
		AppTimezone:       tz,
		StagingURL:        stagingURL,
		Port:              port,
		HealthPath:        healthPath,
		VulnSLACritical:   slaC,
		VulnSLAHigh:       slaH,
		VulnSLAMedium:     slaM,
		VulnSLALow:        slaL,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			jsonError(w, `A service named "`+req.Name+`" already exists in this application.`, http.StatusConflict)
			return
		}
		jsonError(w, "create service: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Save infrastructure requirements.
	for _, ir := range req.InfraRequirements {
		cfg := ir.Config
		if len(cfg) == 0 {
			cfg = json.RawMessage(`{}`)
		}
		_, _ = h.db.SetServiceInfraRequirement(r.Context(), db.ServiceInfraRequirement{
			ProjectID:   project.ID,
			ServiceType: ir.ServiceType,
			Config:      cfg,
		})
	}

	// Save inter-service dependencies.
	if len(req.TalksTo) > 0 {
		deps := make([]db.ServiceDependency, 0, len(req.TalksTo))
		for _, t := range req.TalksTo {
			p := t.Port
			if p == 0 {
				p = 80
			}
			deps = append(deps, db.ServiceDependency{
				FromProject: project.ID,
				ToProject:   t.ProjectID,
				Port:        p,
			})
		}
		_ = h.db.SetServiceDependencies(r.Context(), project.ID, deps)
	}

	// Assign service members so the orchestrator can sync them to DefectDojo in step 10.
	for _, m := range req.Members {
		role := m.Role
		if role != "lead" && role != "developer" {
			role = "developer"
		}
		_ = h.db.AddProjectMember(r.Context(), project.ID, m.UserID, role, &user.ID)
	}
	// Creator is always a lead.
	_ = h.db.AddProjectMember(r.Context(), project.ID, user.ID, "lead", nil)

	// Insert all 15 step rows so the SSE stream can display them immediately.
	steps := make([]db.ProvisioningStep, len(provisioner.StepLabels))
	for i, label := range provisioner.StepLabels {
		steps[i] = db.ProvisioningStep{ProjectID: project.ID, StepIndex: i + 1, Label: label}
	}
	if err := h.db.CreateProvisioningSteps(r.Context(), steps); err != nil {
		jsonError(w, "create steps: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Pre-create dev / uat / prod environment rows.
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

	// Enqueue the provisioning job. The embedded worker (or a standalone
	// cmd/worker pod) will claim it and run the full 15-step orchestrator.
	if _, err := h.db.EnqueueProvisioningJob(r.Context(), project.ID, db.JobPayload{
		GitNamespace:    app.GitNamespace,
		ApplicationSlug: app.Slug,
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

// UpdateService updates editable fields on an existing service.
// Requires application lead or admin. Can be called on any status — most useful
// before retrying a failed provisioning run.
func (h *Handler) UpdateService(w http.ResponseWriter, r *http.Request) {
	appID, err := parseUUID(r, "appID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	serviceID, err := parseUUID(r, "serviceID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, _ := authpkg.UserFromContext(r.Context())
	if !h.isLead(r.Context(), user, appID) {
		jsonError(w, "only application leads or admins can update services", http.StatusForbidden)
		return
	}
	var req struct {
		Name              string `json:"name"`
		BuildTool         string `json:"build_tool"`
		NotificationEmail string `json:"notification_email"`
		AppTimezone       string `json:"app_timezone"`
		StagingURL        string `json:"staging_url"`
		K8sManifestPaths  string `json:"k8s_manifest_paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.BuildTool == "" {
		jsonError(w, "name and build_tool are required", http.StatusBadRequest)
		return
	}
	project, err := h.db.UpdateService(r.Context(), serviceID,
		req.Name, req.BuildTool, req.NotificationEmail, req.AppTimezone, req.StagingURL, req.K8sManifestPaths,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, "service not found", http.StatusNotFound)
			return
		}
		jsonError(w, "update service: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, project)
}

// ReprovisionService resets all provisioning steps back to pending and
// re-runs the 15-step orchestrator for an existing service. Use this to retry
// after a failure or after editing service configuration (e.g. build_tool).
func (h *Handler) ReprovisionService(w http.ResponseWriter, r *http.Request) {
	appID, err := parseUUID(r, "appID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	serviceID, err := parseUUID(r, "serviceID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, _ := authpkg.UserFromContext(r.Context())
	if !h.isLead(r.Context(), user, appID) {
		jsonError(w, "only application leads or admins can reprovision services", http.StatusForbidden)
		return
	}

	project, err := h.db.GetProject(r.Context(), serviceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, "service not found", http.StatusNotFound)
			return
		}
		jsonError(w, "get service: "+err.Error(), http.StatusInternalServerError)
		return
	}

	app, err := h.db.GetApplication(r.Context(), appID)
	if err != nil {
		jsonError(w, "get application: "+err.Error(), http.StatusInternalServerError)
		return
	}

	envs, err := h.db.GetEnvironmentsByProject(r.Context(), serviceID)
	if err != nil {
		jsonError(w, "get environments: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// If there are no step rows (e.g. first create partially failed), seed them now.
	existingSteps, _ := h.db.GetProvisioningSteps(r.Context(), serviceID)
	if len(existingSteps) == 0 {
		steps := make([]db.ProvisioningStep, len(provisioner.StepLabels))
		for i, label := range provisioner.StepLabels {
			steps[i] = db.ProvisioningStep{
				ProjectID: serviceID,
				StepIndex: i + 1,
				Label:     label,
			}
		}
		if err := h.db.CreateProvisioningSteps(r.Context(), steps); err != nil {
			jsonError(w, "create provisioning steps: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		if err := h.db.ResetProvisioningSteps(r.Context(), serviceID); err != nil {
			jsonError(w, "reset steps: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := h.db.UpdateProjectStatus(r.Context(), serviceID, "provisioning"); err != nil {
		jsonError(w, "reset status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure all three environment rows exist (handles partial creation failures).
	existingEnvNames := make(map[string]bool, len(envs))
	for _, e := range envs {
		existingEnvNames[e.Name] = true
	}
	if len(envs) < 3 {
		for _, envName := range []string{"dev", "uat", "prod"} {
			if existingEnvNames[envName] {
				continue
			}
			env, err := h.db.CreateEnvironment(r.Context(), db.Environment{
				ProjectID: serviceID,
				Name:      envName,
				Namespace: project.Slug + "-" + envName,
			})
			if err != nil {
				jsonError(w, "create environment: "+err.Error(), http.StatusInternalServerError)
				return
			}
			envs = append(envs, *env)
		}
	}

	envPtrs := make([]*db.Environment, len(envs))
	for i := range envs {
		envPtrs[i] = &envs[i]
	}

	if _, err := h.db.EnqueueProvisioningJob(r.Context(), serviceID, db.JobPayload{
		GitNamespace:    app.GitNamespace,
		ApplicationSlug: app.Slug,
	}); err != nil {
		jsonError(w, "enqueue reprovisioning: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":         project.ID,
		"status":     "provisioning",
		"stream_url": fmt.Sprintf("/api/v1/projects/%s/stream", project.ID),
	})
}

// DeleteService archives a service (project) under an application.
// Requires application lead or admin. External resources (repo, Jenkins, Harbor)
// are kept intact — only the DevPortal record is archived.
func (h *Handler) DeleteService(w http.ResponseWriter, r *http.Request) {
	appID, err := parseUUID(r, "appID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	serviceID, err := parseUUID(r, "serviceID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, _ := authpkg.UserFromContext(r.Context())
	if !h.isLead(r.Context(), user, appID) {
		jsonError(w, "only application leads or admins can delete services", http.StatusForbidden)
		return
	}
	if err := h.db.ArchiveProject(r.Context(), serviceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, "service not found", http.StatusNotFound)
			return
		}
		jsonError(w, "delete service: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RenameService renames a service. Requires application lead or admin.
func (h *Handler) RenameService(w http.ResponseWriter, r *http.Request) {
	appID, err := parseUUID(r, "appID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	serviceID, err := parseUUID(r, "serviceID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, _ := authpkg.UserFromContext(r.Context())
	if !h.isLead(r.Context(), user, appID) {
		jsonError(w, "only application leads or admins can rename services", http.StatusForbidden)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	project, err := h.db.RenameProject(r.Context(), serviceID, req.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, "service not found", http.StatusNotFound)
			return
		}
		jsonError(w, "rename service: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, project)
}

// ── Access control helpers ────────────────────────────────────────────────────

func (h *Handler) isMember(ctx context.Context, user *db.User, appID uuid.UUID) bool {
	if user.Role == authpkg.RoleAdmin {
		return true
	}
	ok, _ := h.db.IsApplicationMember(ctx, appID, user.ID)
	return ok
}

func (h *Handler) isLead(ctx context.Context, user *db.User, appID uuid.UUID) bool {
	if user.Role == authpkg.RoleAdmin {
		return true
	}
	members, err := h.db.ListApplicationMembers(ctx, appID)
	if err != nil {
		return false
	}
	for _, m := range members {
		if m.UserID == user.ID && m.Role == "lead" {
			return true
		}
	}
	return false
}
