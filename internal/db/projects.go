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
			jenkins_folder, build_tool, notification_email, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING
			id, team_id, name, slug, git_repo_url, harbor_project,
			jenkins_folder, build_tool, notification_email,
			defectdojo_product_id, status, generated_jenkinsfile,
			manifest_repo_url, app_repo_url, created_at, created_by
	`
	rows, err := db.pool.Query(ctx, q,
		p.TeamID, p.Name, p.Slug, p.GitRepoURL, p.HarborProject,
		p.JenkinsFolder, p.BuildTool, p.NotificationEmail, p.CreatedBy,
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
			defectdojo_product_id, status, generated_jenkinsfile,
			manifest_repo_url, app_repo_url, created_at, created_by
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
			defectdojo_product_id, status, generated_jenkinsfile,
			manifest_repo_url, app_repo_url, created_at, created_by
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

// ── Environments ──────────────────────────────────────────────────────────────

// CreateEnvironment inserts a new environment row (dev / uat / prod) for a project.
func (db *DB) CreateEnvironment(ctx context.Context, e Environment) (*Environment, error) {
	const q = `
		INSERT INTO environments (project_id, name, namespace, status)
		VALUES ($1, $2, $3, 'provisioning')
		RETURNING id, project_id, name, namespace, ingress_url, argocd_app_name,
		          db_name, db_username, manifest_path, status, created_at
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
		       db_name, db_username, manifest_path, status, created_at
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
