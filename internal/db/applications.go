// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// applications.go — query helpers for applications and application_members tables.

package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CreateApplication inserts a new application row and returns it.
func (db *DB) CreateApplication(ctx context.Context, a Application) (*Application, error) {
	const q = `
		INSERT INTO applications (org_id, name, slug, description, git_namespace, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, org_id, name, slug, description, git_namespace, status, created_at, created_by
	`
	rows, err := db.pool.Query(ctx, q,
		a.OrgID, a.Name, a.Slug, a.Description, a.GitNamespace, a.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("db.CreateApplication: query: %w", err)
	}
	app, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Application])
	if err != nil {
		return nil, fmt.Errorf("db.CreateApplication: scan: %w", err)
	}
	return app, nil
}

// GetApplication returns an application by ID.
func (db *DB) GetApplication(ctx context.Context, id uuid.UUID) (*Application, error) {
	const q = `
		SELECT id, org_id, name, slug, description, git_namespace, status, created_at, created_by
		FROM applications WHERE id = $1
	`
	rows, err := db.pool.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("db.GetApplication: query: %w", err)
	}
	app, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Application])
	if err != nil {
		return nil, fmt.Errorf("db.GetApplication: %w", err)
	}
	return app, nil
}

// ListApplicationsByOrg returns all applications in the org (admin view).
func (db *DB) ListApplicationsByOrg(ctx context.Context, orgID uuid.UUID) ([]Application, error) {
	const q = `
		SELECT id, org_id, name, slug, description, git_namespace, status, created_at, created_by
		FROM applications
		WHERE org_id = $1 AND status = 'active'
		ORDER BY created_at DESC
	`
	rows, err := db.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("db.ListApplicationsByOrg: query: %w", err)
	}
	apps, err := pgx.CollectRows(rows, pgx.RowToStructByName[Application])
	if err != nil {
		return nil, fmt.Errorf("db.ListApplicationsByOrg: scan: %w", err)
	}
	return apps, nil
}

// ListApplicationsByUser returns applications where the user is a member.
func (db *DB) ListApplicationsByUser(ctx context.Context, userID uuid.UUID) ([]Application, error) {
	const q = `
		SELECT a.id, a.org_id, a.name, a.slug, a.description, a.git_namespace, a.status, a.created_at, a.created_by
		FROM applications a
		JOIN application_members m ON a.id = m.application_id
		WHERE m.user_id = $1 AND a.status = 'active'
		ORDER BY a.created_at DESC
	`
	rows, err := db.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("db.ListApplicationsByUser: query: %w", err)
	}
	apps, err := pgx.CollectRows(rows, pgx.RowToStructByName[Application])
	if err != nil {
		return nil, fmt.Errorf("db.ListApplicationsByUser: scan: %w", err)
	}
	return apps, nil
}

// IsApplicationMember returns true if userID is a member of the application.
func (db *DB) IsApplicationMember(ctx context.Context, appID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := db.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM application_members WHERE application_id=$1 AND user_id=$2)`,
		appID, userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("db.IsApplicationMember: %w", err)
	}
	return exists, nil
}

// ListApplicationMembers returns all members of an application with their user details.
func (db *DB) ListApplicationMembers(ctx context.Context, appID uuid.UUID) ([]ApplicationMemberDetail, error) {
	const q = `
		SELECT m.application_id, m.user_id, m.role, m.added_at, m.added_by,
		       u.email, u.display_name
		FROM application_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.application_id = $1
		ORDER BY m.added_at ASC
	`
	rows, err := db.pool.Query(ctx, q, appID)
	if err != nil {
		return nil, fmt.Errorf("db.ListApplicationMembers: query: %w", err)
	}
	members, err := pgx.CollectRows(rows, pgx.RowToStructByName[ApplicationMemberDetail])
	if err != nil {
		return nil, fmt.Errorf("db.ListApplicationMembers: scan: %w", err)
	}
	return members, nil
}

// AddApplicationMember inserts a member row. Idempotent — updates role if already a member.
func (db *DB) AddApplicationMember(ctx context.Context, appID, userID, addedBy uuid.UUID, role string) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO application_members (application_id, user_id, role, added_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (application_id, user_id) DO UPDATE SET role = EXCLUDED.role
	`, appID, userID, role, addedBy)
	if err != nil {
		return fmt.Errorf("db.AddApplicationMember: %w", err)
	}
	return nil
}

// RemoveApplicationMember deletes the membership row.
func (db *DB) RemoveApplicationMember(ctx context.Context, appID, userID uuid.UUID) error {
	tag, err := db.pool.Exec(ctx,
		`DELETE FROM application_members WHERE application_id=$1 AND user_id=$2`,
		appID, userID,
	)
	if err != nil {
		return fmt.Errorf("db.RemoveApplicationMember: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("db.RemoveApplicationMember: %w", pgx.ErrNoRows)
	}
	return nil
}

// DeleteApplication soft-deletes an application and all its services.
func (db *DB) DeleteApplication(ctx context.Context, id uuid.UUID) error {
	tag, err := db.pool.Exec(ctx,
		`UPDATE applications SET status = 'archived' WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("db.DeleteApplication: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("db.DeleteApplication: %w", pgx.ErrNoRows)
	}
	// Cascade: archive all services that belong to this application.
	if _, err := db.pool.Exec(ctx,
		`UPDATE projects SET status = 'archived' WHERE application_id = $1 AND status != 'archived'`,
		id,
	); err != nil {
		return fmt.Errorf("db.DeleteApplication: cascade services: %w", err)
	}
	return nil
}

// UpdateApplication renames an application and optionally changes its description.
func (db *DB) UpdateApplication(ctx context.Context, id uuid.UUID, name, description string) (*Application, error) {
	const q = `
		UPDATE applications SET name = $1, description = $2 WHERE id = $3
		RETURNING id, org_id, name, slug, description, git_namespace, status, created_at, created_by
	`
	rows, err := db.pool.Query(ctx, q, name, description, id)
	if err != nil {
		return nil, fmt.Errorf("db.UpdateApplication: query: %w", err)
	}
	app, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Application])
	if err != nil {
		return nil, fmt.Errorf("db.UpdateApplication: scan: %w", err)
	}
	return app, nil
}

// ListServicesByApplication returns all projects (services) under an application.
func (db *DB) ListServicesByApplication(ctx context.Context, appID uuid.UUID) ([]Project, error) {
	const q = `
		SELECT id, team_id, name, slug, git_repo_url, harbor_project, jenkins_folder,
		       build_tool, notification_email, defectdojo_product_id, defectdojo_engagement_id,
		       status, generated_jenkinsfile, manifest_repo_url, app_repo_url,
		       created_at, created_by, application_id,
		       app_timezone, staging_url, k8s_manifest_paths,
		       port, health_path,
		       vuln_sla_critical, vuln_sla_high, vuln_sla_medium, vuln_sla_low
		FROM projects
		WHERE application_id = $1
		ORDER BY created_at ASC
	`
	rows, err := db.pool.Query(ctx, q, appID)
	if err != nil {
		return nil, fmt.Errorf("db.ListServicesByApplication: query: %w", err)
	}
	projects, err := pgx.CollectRows(rows, pgx.RowToStructByName[Project])
	if err != nil {
		return nil, fmt.Errorf("db.ListServicesByApplication: scan: %w", err)
	}
	return projects, nil
}

// ApplicationMemberDetail is a join of application_members + users for API responses.
type ApplicationMemberDetail struct {
	ApplicationID uuid.UUID  `db:"application_id" json:"application_id"`
	UserID        uuid.UUID  `db:"user_id"        json:"user_id"`
	Role          string     `db:"role"           json:"role"`
	AddedAt       string     `db:"added_at"       json:"added_at"`
	AddedBy       *uuid.UUID `db:"added_by"       json:"added_by,omitempty"`
	Email         string     `db:"email"          json:"email"`
	DisplayName   string     `db:"display_name"   json:"display_name"`
}
