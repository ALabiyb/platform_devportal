// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// teams.go contains query helpers for the teams, team_members,
// and organizations tables.

package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ── Organizations ─────────────────────────────────────────────────────────────

// GetOrganization returns the organization with the given ID.
func (db *DB) GetOrganization(ctx context.Context, id uuid.UUID) (*Organization, error) {
	const q = `SELECT id, name, slug, created_at FROM organizations WHERE id = $1`
	rows, err := db.pool.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("db.GetOrganization: query: %w", err)
	}
	org, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Organization])
	if err != nil {
		return nil, fmt.Errorf("db.GetOrganization: %w", err)
	}
	return org, nil
}

// EnsureDefaultOrg creates the default org if it does not exist, or returns the
// existing one. Used at bootstrap when no org exists yet (fresh install).
func (db *DB) EnsureDefaultOrg(ctx context.Context, name, slug string) (*Organization, error) {
	const q = `
		INSERT INTO organizations (name, slug)
		VALUES ($1, $2)
		ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, name, slug, created_at
	`
	rows, err := db.pool.Query(ctx, q, name, slug)
	if err != nil {
		return nil, fmt.Errorf("db.EnsureDefaultOrg: query: %w", err)
	}
	org, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Organization])
	if err != nil {
		return nil, fmt.Errorf("db.EnsureDefaultOrg: scan: %w", err)
	}
	return org, nil
}

// ── Teams ─────────────────────────────────────────────────────────────────────

// GetTeam returns the team with the given ID.
func (db *DB) GetTeam(ctx context.Context, id uuid.UUID) (*Team, error) {
	const q = `SELECT id, org_id, name, slug, created_at FROM teams WHERE id = $1`
	rows, err := db.pool.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("db.GetTeam: query: %w", err)
	}
	team, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Team])
	if err != nil {
		return nil, fmt.Errorf("db.GetTeam: %w", err)
	}
	return team, nil
}

// ListTeamsByOrg returns all teams in the org, ordered alphabetically.
func (db *DB) ListTeamsByOrg(ctx context.Context, orgID uuid.UUID) ([]Team, error) {
	const q = `
		SELECT id, org_id, name, slug, created_at
		FROM teams
		WHERE org_id = $1
		ORDER BY name ASC
	`
	rows, err := db.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("db.ListTeamsByOrg: query: %w", err)
	}
	teams, err := pgx.CollectRows(rows, pgx.RowToStructByName[Team])
	if err != nil {
		return nil, fmt.Errorf("db.ListTeamsByOrg: scan: %w", err)
	}
	return teams, nil
}

// CreateTeam inserts a new team row.
func (db *DB) CreateTeam(ctx context.Context, t Team) (*Team, error) {
	const q = `
		INSERT INTO teams (org_id, name, slug)
		VALUES ($1, $2, $3)
		RETURNING id, org_id, name, slug, created_at
	`
	rows, err := db.pool.Query(ctx, q, t.OrgID, t.Name, t.Slug)
	if err != nil {
		return nil, fmt.Errorf("db.CreateTeam: query: %w", err)
	}
	team, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Team])
	if err != nil {
		return nil, fmt.Errorf("db.CreateTeam: scan: %w", err)
	}
	return team, nil
}

// DeleteTeam removes a team. Fails if any projects are still assigned to the team
// (foreign key: projects.team_id → teams.id ON DELETE RESTRICT).
func (db *DB) DeleteTeam(ctx context.Context, id uuid.UUID) error {
	tag, err := db.pool.Exec(ctx, `DELETE FROM teams WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("db.DeleteTeam: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("db.DeleteTeam: %w", pgx.ErrNoRows)
	}
	return nil
}

// RenameTeam updates the display name of a team (slug stays the same).
func (db *DB) RenameTeam(ctx context.Context, id uuid.UUID, name string) (*Team, error) {
	const q = `
		UPDATE teams SET name = $1 WHERE id = $2
		RETURNING id, org_id, name, slug, created_at
	`
	rows, err := db.pool.Query(ctx, q, name, id)
	if err != nil {
		return nil, fmt.Errorf("db.RenameTeam: query: %w", err)
	}
	team, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Team])
	if err != nil {
		return nil, fmt.Errorf("db.RenameTeam: scan: %w", err)
	}
	return team, nil
}

// ── Team membership ───────────────────────────────────────────────────────────

// ListTeamMembers returns all members of a team with their user details.
func (db *DB) ListTeamMembers(ctx context.Context, teamID uuid.UUID) ([]TeamMemberDetail, error) {
	const q = `
		SELECT m.team_id, m.user_id, m.role AS member_role,
		       u.role AS org_role, u.email, u.display_name
		FROM team_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.team_id = $1
		ORDER BY u.display_name ASC
	`
	rows, err := db.pool.Query(ctx, q, teamID)
	if err != nil {
		return nil, fmt.Errorf("db.ListTeamMembers: query: %w", err)
	}
	members, err := pgx.CollectRows(rows, pgx.RowToStructByName[TeamMemberDetail])
	if err != nil {
		return nil, fmt.Errorf("db.ListTeamMembers: scan: %w", err)
	}
	return members, nil
}

// AddTeamMember inserts or updates a team member row.
func (db *DB) AddTeamMember(ctx context.Context, teamID, userID uuid.UUID, role string) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO team_members (team_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (team_id, user_id) DO UPDATE SET role = EXCLUDED.role
	`, teamID, userID, role)
	if err != nil {
		return fmt.Errorf("db.AddTeamMember: %w", err)
	}
	return nil
}

// RemoveTeamMember deletes a team member row.
func (db *DB) RemoveTeamMember(ctx context.Context, teamID, userID uuid.UUID) error {
	tag, err := db.pool.Exec(ctx,
		`DELETE FROM team_members WHERE team_id=$1 AND user_id=$2`,
		teamID, userID,
	)
	if err != nil {
		return fmt.Errorf("db.RemoveTeamMember: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("db.RemoveTeamMember: %w", pgx.ErrNoRows)
	}
	return nil
}

// ── Projects (org-scoped) ─────────────────────────────────────────────────────

// ListProjectsByOrg returns all projects across all teams in the org,
// ordered newest first. Used by admin ListProjects.
func (db *DB) ListProjectsByOrg(ctx context.Context, orgID uuid.UUID) ([]Project, error) {
	const q = `
		SELECT p.id, p.team_id, p.name, p.slug, p.git_repo_url, p.harbor_project,
		       p.jenkins_folder, p.build_tool, p.notification_email,
		       p.defectdojo_product_id, p.defectdojo_engagement_id, p.status, p.generated_jenkinsfile,
		       p.manifest_repo_url, p.app_repo_url, p.created_at, p.created_by,
		       p.application_id, p.app_timezone, p.staging_url, p.k8s_manifest_paths,
		       p.port, p.health_path,
		       p.vuln_sla_critical, p.vuln_sla_high, p.vuln_sla_medium, p.vuln_sla_low
		FROM projects p
		LEFT JOIN teams t ON p.team_id = t.id
		WHERE p.status != 'archived'
		  AND (t.org_id = $1 OR p.application_id IN (
		    SELECT id FROM applications WHERE org_id = $1
		))
		ORDER BY p.created_at DESC
	`
	rows, err := db.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("db.ListProjectsByOrg: query: %w", err)
	}
	projects, err := pgx.CollectRows(rows, pgx.RowToStructByName[Project])
	if err != nil {
		return nil, fmt.Errorf("db.ListProjectsByOrg: scan: %w", err)
	}
	return projects, nil
}

// UpdateProjectURLs stores the resolved GitLab repo URL and manifest repo URL
// after the source repository is provisioned.
func (db *DB) UpdateProjectURLs(ctx context.Context, id uuid.UUID, appRepoURL, manifestRepoURL string) error {
	const q = `
		UPDATE projects
		SET app_repo_url = $1, git_repo_url = $1, manifest_repo_url = $2
		WHERE id = $3
	`
	_, err := db.pool.Exec(ctx, q, appRepoURL, manifestRepoURL, id)
	if err != nil {
		return fmt.Errorf("db.UpdateProjectURLs: %w", err)
	}
	return nil
}
