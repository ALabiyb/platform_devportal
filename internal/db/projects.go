// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// projects.go contains query helpers for projects, environments, and
// provisioning_steps — the three tables that represent the core lifecycle
// of a provisioned developer project.
//
// Provisioning flow (called from the orchestrator in Day 09):
//   CreateProject          → inserts the project row (status: "provisioning")
//   CreateProvisioningSteps → inserts all 15 step rows upfront (status: "pending")
//   UpdateProvisioningStep  → called after each step completes or fails
//   CreateEnvironment       → inserts dev/uat/prod rows as they are set up
//   UpdateProjectStatus     → set to "active" when all steps succeed, "failed" otherwise

package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ── Projects ─────────────────────────────────────────────────────────────────

// CreateProject inserts a new project row with status "provisioning".
// The provisioning orchestrator calls this as its very first step so the
// project exists in the DB before any external API calls are made.
func (db *DB) CreateProject(ctx context.Context, p Project) (*Project, error) {
	const q = `
		INSERT INTO projects (
			team_id, name, slug, git_repo_url, harbor_project,
			jenkins_folder, build_tool, notification_email, created_by, application_id,
			app_timezone, staging_url, k8s_manifest_paths,
			vuln_sla_critical, vuln_sla_high, vuln_sla_medium, vuln_sla_low
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING
			id, team_id, name, slug, git_repo_url, harbor_project,
			jenkins_folder, build_tool, notification_email,
			defectdojo_product_id, defectdojo_engagement_id, status, generated_jenkinsfile,
			manifest_repo_url, app_repo_url, created_at, created_by, application_id,
			app_timezone, staging_url, k8s_manifest_paths,
			port, health_path,
			vuln_sla_critical, vuln_sla_high, vuln_sla_medium, vuln_sla_low
	`
	rows, err := db.pool.Query(ctx, q,
		p.TeamID, p.Name, p.Slug, p.GitRepoURL, p.HarborProject,
		p.JenkinsFolder, p.BuildTool, p.NotificationEmail, p.CreatedBy, p.ApplicationID,
		p.AppTimezone, p.StagingURL, p.K8sManifestPaths,
		p.VulnSLACritical, p.VulnSLAHigh, p.VulnSLAMedium, p.VulnSLALow,
	)
	if err != nil {
		return nil, fmt.Errorf("db.CreateProject: query: %w", err)
	}
	project, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Project])
	if err != nil {
		return nil, fmt.Errorf("db.CreateProject: scan: %w", err)
	}
	return project, nil
}

// GetProject returns the project with the given ID.
func (db *DB) GetProject(ctx context.Context, id uuid.UUID) (*Project, error) {
	const q = `
		SELECT
			id, team_id, name, slug, git_repo_url, harbor_project,
			jenkins_folder, build_tool, notification_email,
			defectdojo_product_id, defectdojo_engagement_id, status, generated_jenkinsfile,
			manifest_repo_url, app_repo_url, created_at, created_by, application_id,
			app_timezone, staging_url, k8s_manifest_paths,
			port, health_path,
			vuln_sla_critical, vuln_sla_high, vuln_sla_medium, vuln_sla_low
		FROM projects
		WHERE id = $1
	`
	rows, err := db.pool.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("db.GetProject: query: %w", err)
	}
	project, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Project])
	if err != nil {
		return nil, fmt.Errorf("db.GetProject: %w", err)
	}
	return project, nil
}

// ListProjectsByTeam returns all projects belonging to a team, ordered newest first.
func (db *DB) ListProjectsByTeam(ctx context.Context, teamID uuid.UUID) ([]Project, error) {
	const q = `
		SELECT
			id, team_id, name, slug, git_repo_url, harbor_project,
			jenkins_folder, build_tool, notification_email,
			defectdojo_product_id, defectdojo_engagement_id, status, generated_jenkinsfile,
			manifest_repo_url, app_repo_url, created_at, created_by, application_id,
			app_timezone, staging_url, k8s_manifest_paths,
			port, health_path,
			vuln_sla_critical, vuln_sla_high, vuln_sla_medium, vuln_sla_low
		FROM projects
		WHERE team_id = $1
		ORDER BY created_at DESC
	`
	rows, err := db.pool.Query(ctx, q, teamID)
	if err != nil {
		return nil, fmt.Errorf("db.ListProjectsByTeam: query: %w", err)
	}
	projects, err := pgx.CollectRows(rows, pgx.RowToStructByName[Project])
	if err != nil {
		return nil, fmt.Errorf("db.ListProjectsByTeam: scan: %w", err)
	}
	return projects, nil
}

// ArchiveProject sets a project's status to "archived" (soft-delete).
// The external resources (repo, Jenkins job, Harbor project) are kept intact.
func (db *DB) ArchiveProject(ctx context.Context, id uuid.UUID) error {
	tag, err := db.pool.Exec(ctx,
		`UPDATE projects SET status = 'archived' WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("db.ArchiveProject: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("db.ArchiveProject: %w", pgx.ErrNoRows)
	}
	return nil
}

// RenameProject updates the display name of a project (slug stays the same).
func (db *DB) RenameProject(ctx context.Context, id uuid.UUID, name string) (*Project, error) {
	const q = `
		UPDATE projects SET name = $1 WHERE id = $2
		RETURNING id, team_id, name, slug, git_repo_url, harbor_project,
		          jenkins_folder, build_tool, notification_email,
		          defectdojo_product_id, defectdojo_engagement_id, status, generated_jenkinsfile,
		          manifest_repo_url, app_repo_url, created_at, created_by, application_id,
		          app_timezone, staging_url, k8s_manifest_paths,
		          port, health_path,
		          vuln_sla_critical, vuln_sla_high, vuln_sla_medium, vuln_sla_low
	`
	rows, err := db.pool.Query(ctx, q, name, id)
	if err != nil {
		return nil, fmt.Errorf("db.RenameProject: query: %w", err)
	}
	project, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Project])
	if err != nil {
		return nil, fmt.Errorf("db.RenameProject: scan: %w", err)
	}
	return project, nil
}

// UpdateService updates the editable fields of a service before or after
// provisioning. Slug is intentionally excluded — it is baked into the Gitea
// repo name and Jenkins job name and cannot change without external cleanup.
func (db *DB) UpdateService(ctx context.Context, id uuid.UUID,
	name, buildTool, notificationEmail, appTimezone, stagingURL, k8sManifestPaths string,
) (*Project, error) {
	const q = `
		UPDATE projects SET
			name                = $1,
			build_tool          = $2,
			notification_email  = $3,
			app_timezone        = $4,
			staging_url         = NULLIF($5, ''),
			k8s_manifest_paths  = $6
		WHERE id = $7
		RETURNING id, team_id, name, slug, git_repo_url, harbor_project,
		          jenkins_folder, build_tool, notification_email,
		          defectdojo_product_id, defectdojo_engagement_id, status, generated_jenkinsfile,
		          manifest_repo_url, app_repo_url, created_at, created_by, application_id,
		          app_timezone, staging_url, k8s_manifest_paths,
		          port, health_path,
		          vuln_sla_critical, vuln_sla_high, vuln_sla_medium, vuln_sla_low
	`
	rows, err := db.pool.Query(ctx, q, name, buildTool, notificationEmail, appTimezone, stagingURL, k8sManifestPaths, id)
	if err != nil {
		return nil, fmt.Errorf("db.UpdateService: query: %w", err)
	}
	p, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Project])
	if err != nil {
		return nil, fmt.Errorf("db.UpdateService: %w", err)
	}
	return p, nil
}

// ResetProvisioningSteps clears all step results so the orchestrator can
// re-run them from scratch. Called before every reprovision attempt.
func (db *DB) ResetProvisioningSteps(ctx context.Context, projectID uuid.UUID) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE provisioning_steps
		SET status = 'pending', detail = '', started_at = NULL, finished_at = NULL
		WHERE project_id = $1
	`, projectID)
	if err != nil {
		return fmt.Errorf("db.ResetProvisioningSteps: %w", err)
	}
	return nil
}

// UpdateProjectStatus sets the overall provisioning status of a project.
// Called by the orchestrator at the end of provisioning: "active" on success,
// "failed" on any unrecoverable error.
func (db *DB) UpdateProjectStatus(ctx context.Context, id uuid.UUID, status string) error {
	const q = `UPDATE projects SET status = $1 WHERE id = $2`
	_, err := db.pool.Exec(ctx, q, status, id)
	if err != nil {
		return fmt.Errorf("db.UpdateProjectStatus: %w", err)
	}
	return nil
}

// SaveGeneratedJenkinsfile stores the Jenkinsfile text that was committed to GitLab
// so developers can view and re-download it from devportal without re-generating.
func (db *DB) SaveGeneratedJenkinsfile(ctx context.Context, id uuid.UUID, content string) error {
	const q = `UPDATE projects SET generated_jenkinsfile = $1 WHERE id = $2`
	_, err := db.pool.Exec(ctx, q, content, id)
	if err != nil {
		return fmt.Errorf("db.SaveGeneratedJenkinsfile: %w", err)
	}
	return nil
}

// SetDefectDojoProductID records the DefectDojo product ID after it has been
// created during provisioning, so future scans can reference it directly.
func (db *DB) SetDefectDojoProductID(ctx context.Context, id uuid.UUID, productID int) error {
	const q = `UPDATE projects SET defectdojo_product_id = $1 WHERE id = $2`
	_, err := db.pool.Exec(ctx, q, productID, id)
	if err != nil {
		return fmt.Errorf("db.SetDefectDojoProductID: %w", err)
	}
	return nil
}

// SetDefectDojoEngagementID records the DefectDojo engagement ID after the
// CI/CD engagement is created in Step 11. This ID is written back into the
// Jenkinsfile so Jenkins can upload scan results to the correct engagement.
func (db *DB) SetDefectDojoEngagementID(ctx context.Context, id uuid.UUID, engagementID int) error {
	const q = `UPDATE projects SET defectdojo_engagement_id = $1 WHERE id = $2`
	_, err := db.pool.Exec(ctx, q, engagementID, id)
	if err != nil {
		return fmt.Errorf("db.SetDefectDojoEngagementID: %w", err)
	}
	return nil
}

// ── Environments ──────────────────────────────────────────────────────────────

// CreateEnvironment inserts a new environment row (dev / uat / prod) for a project.
func (db *DB) CreateEnvironment(ctx context.Context, e Environment) (*Environment, error) {
	const q = `
		INSERT INTO environments (project_id, name, namespace, status)
		VALUES ($1, $2, $3, 'provisioning')
		RETURNING id, project_id, name, namespace, ingress_url, argocd_app_name,
		          db_name, db_username, manifest_path, status, created_at, cluster_id
	`
	rows, err := db.pool.Query(ctx, q, e.ProjectID, e.Name, e.Namespace)
	if err != nil {
		return nil, fmt.Errorf("db.CreateEnvironment: query: %w", err)
	}
	env, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Environment])
	if err != nil {
		return nil, fmt.Errorf("db.CreateEnvironment: scan: %w", err)
	}
	return env, nil
}

// GetEnvironmentsByProject returns all environments for a project ordered by
// the natural promotion order: dev → uat → prod.
func (db *DB) GetEnvironmentsByProject(ctx context.Context, projectID uuid.UUID) ([]Environment, error) {
	const q = `
		SELECT id, project_id, name, namespace, ingress_url, argocd_app_name,
		       db_name, db_username, manifest_path, status, created_at, cluster_id
		FROM environments
		WHERE project_id = $1
		ORDER BY CASE name WHEN 'dev' THEN 1 WHEN 'uat' THEN 2 WHEN 'prod' THEN 3 ELSE 4 END
	`
	rows, err := db.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("db.GetEnvironmentsByProject: query: %w", err)
	}
	envs, err := pgx.CollectRows(rows, pgx.RowToStructByName[Environment])
	if err != nil {
		return nil, fmt.Errorf("db.GetEnvironmentsByProject: scan: %w", err)
	}
	return envs, nil
}

// UpdateEnvironmentStatus sets the status of an environment after provisioning.
func (db *DB) UpdateEnvironmentStatus(ctx context.Context, id uuid.UUID, status string) error {
	const q = `UPDATE environments SET status = $1 WHERE id = $2`
	_, err := db.pool.Exec(ctx, q, status, id)
	if err != nil {
		return fmt.Errorf("db.UpdateEnvironmentStatus: %w", err)
	}
	return nil
}

// UpdateEnvironmentDetails stores the ArgoCD app name, DB credentials, ingress URL,
// and manifest path once they have been provisioned for an environment.
func (db *DB) UpdateEnvironmentDetails(ctx context.Context, id uuid.UUID, e Environment) error {
	const q = `
		UPDATE environments
		SET argocd_app_name = $1,
		    db_name         = $2,
		    db_username     = $3,
		    ingress_url     = $4,
		    manifest_path   = $5
		WHERE id = $6
	`
	_, err := db.pool.Exec(ctx, q,
		e.ArgoCDAppName, e.DBName, e.DBUsername, e.IngressURL, e.ManifestPath, id,
	)
	if err != nil {
		return fmt.Errorf("db.UpdateEnvironmentDetails: %w", err)
	}
	return nil
}

// ── Provisioning Steps ────────────────────────────────────────────────────────

// CreateProvisioningSteps inserts all step rows for a project upfront with
// status "pending". The orchestrator calls this right after CreateProject so
// the SSE stream can show all steps immediately (most greyed-out, one running).
func (db *DB) CreateProvisioningSteps(ctx context.Context, steps []ProvisioningStep) error {
	// Use a batch insert — all rows in one round trip regardless of step count.
	batch := &pgx.Batch{}
	const q = `
		INSERT INTO provisioning_steps (project_id, step_index, label, status)
		VALUES ($1, $2, $3, 'pending')
	`
	for _, s := range steps {
		batch.Queue(q, s.ProjectID, s.StepIndex, s.Label)
	}
	results := db.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range steps {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("db.CreateProvisioningSteps: %w", err)
		}
	}
	return nil
}

// UpdateProvisioningStep updates a single step's status, detail message, and
// timestamps. Called by the orchestrator as each step starts and finishes.
// The SSE hub listens for these changes and broadcasts them to the browser.
func (db *DB) UpdateProvisioningStep(ctx context.Context, projectID uuid.UUID, stepIndex int, status, detail string) error {
	now := time.Now()
	var q string
	var args []any

	switch status {
	case "running":
		// Mark the step as started — record started_at, clear any previous detail.
		q = `
			UPDATE provisioning_steps
			SET status = $1, detail = $2, started_at = $3
			WHERE project_id = $4 AND step_index = $5
		`
		args = []any{status, detail, now, projectID, stepIndex}
	default:
		// "done" or "failed" — record finished_at as well.
		q = `
			UPDATE provisioning_steps
			SET status = $1, detail = $2, finished_at = $3
			WHERE project_id = $4 AND step_index = $5
		`
		args = []any{status, detail, now, projectID, stepIndex}
	}

	_, err := db.pool.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("db.UpdateProvisioningStep: %w", err)
	}
	return nil
}

// GetProvisioningSteps returns all steps for a project ordered by step_index.
// Used by the SSE hub to send the initial snapshot when a browser subscribes.
func (db *DB) GetProvisioningSteps(ctx context.Context, projectID uuid.UUID) ([]ProvisioningStep, error) {
	const q = `
		SELECT id, project_id, step_index, label, status, detail, started_at, finished_at
		FROM provisioning_steps
		WHERE project_id = $1
		ORDER BY step_index ASC
	`
	rows, err := db.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("db.GetProvisioningSteps: query: %w", err)
	}
	steps, err := pgx.CollectRows(rows, pgx.RowToStructByName[ProvisioningStep])
	if err != nil {
		return nil, fmt.Errorf("db.GetProvisioningSteps: scan: %w", err)
	}
	return steps, nil
}

// ── Project Members ───────────────────────────────────────────────────────────

// AddProjectMember assigns a user to a service with a role.
// Upserts on conflict — re-assigning the same user updates their role.
func (db *DB) AddProjectMember(ctx context.Context, projectID, userID uuid.UUID, role string, addedBy *uuid.UUID) error {
	const q = `
		INSERT INTO project_members (project_id, user_id, role, added_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (project_id, user_id) DO UPDATE SET role = EXCLUDED.role
	`
	_, err := db.pool.Exec(ctx, q, projectID, userID, role, addedBy)
	if err != nil {
		return fmt.Errorf("db.AddProjectMember: %w", err)
	}
	return nil
}

// RemoveProjectMember removes a user from a service.
func (db *DB) RemoveProjectMember(ctx context.Context, projectID, userID uuid.UUID) error {
	_, err := db.pool.Exec(ctx,
		`DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`,
		projectID, userID,
	)
	if err != nil {
		return fmt.Errorf("db.RemoveProjectMember: %w", err)
	}
	return nil
}

// ListProjectMemberDetails returns all members of a service with their user
// email and display name — used by the orchestrator to sync membership to
// DefectDojo and by the API to render the member list.
func (db *DB) ListProjectMemberDetails(ctx context.Context, projectID uuid.UUID) ([]ProjectMemberDetail, error) {
	const q = `
		SELECT pm.project_id, pm.user_id, u.email, u.display_name, pm.role
		FROM project_members pm
		JOIN users u ON u.id = pm.user_id
		WHERE pm.project_id = $1
		ORDER BY pm.added_at
	`
	rows, err := db.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("db.ListProjectMemberDetails: query: %w", err)
	}
	members, err := pgx.CollectRows(rows, pgx.RowToStructByName[ProjectMemberDetail])
	if err != nil {
		return nil, fmt.Errorf("db.ListProjectMemberDetails: scan: %w", err)
	}
	return members, nil
}
