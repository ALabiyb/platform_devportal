// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// platform.go — DB methods for manifest_templates, environment_profiles,
// and service_infra_requirements (migration 007).
//
// These three tables complete the Platform Engineering IDP foundation:
//   manifest_templates       — 15 K8s YAML templates, admin-editable at runtime
//   environment_profiles     — CPU/mem/replica limits per tier (dev/uat/prod)
//   service_infra_requirements — what each service selected at registration

package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ── Manifest Templates ────────────────────────────────────────────────────────

// GetManifestTemplate returns one template by name (e.g. "11-deployment").
func (d *DB) GetManifestTemplate(ctx context.Context, name string) (*ManifestTemplate, error) {
	var t ManifestTemplate
	err := d.pool.QueryRow(ctx, `
		SELECT name, display_name, conditional, content, updated_at, updated_by
		FROM   manifest_templates
		WHERE  name = $1`,
		name,
	).Scan(&t.Name, &t.DisplayName, &t.Conditional, &t.Content, &t.UpdatedAt, &t.UpdatedBy)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, pgx.ErrNoRows
		}
		return nil, fmt.Errorf("manifest_templates: get %s: %w", name, err)
	}
	return &t, nil
}

// ListManifestTemplates returns all templates ordered by name (01- first, 15- last).
func (d *DB) ListManifestTemplates(ctx context.Context) ([]ManifestTemplate, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT name, display_name, conditional, content, updated_at, updated_by
		FROM   manifest_templates
		ORDER  BY name`)
	if err != nil {
		return nil, fmt.Errorf("manifest_templates: list: %w", err)
	}
	defer rows.Close()

	var templates []ManifestTemplate
	for rows.Next() {
		var t ManifestTemplate
		if err := rows.Scan(&t.Name, &t.DisplayName, &t.Conditional, &t.Content, &t.UpdatedAt, &t.UpdatedBy); err != nil {
			return nil, fmt.Errorf("manifest_templates: scan: %w", err)
		}
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

// UpsertManifestTemplate inserts or updates a manifest template.
// ON CONFLICT (name) updates content, conditional, updated_at, updated_by.
// Used by admin UI and the startup seed.
func (d *DB) UpsertManifestTemplate(ctx context.Context, t ManifestTemplate) (*ManifestTemplate, error) {
	var out ManifestTemplate
	err := d.pool.QueryRow(ctx, `
		INSERT INTO manifest_templates (name, display_name, conditional, content, updated_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (name) DO UPDATE
		    SET display_name = EXCLUDED.display_name,
		        conditional  = EXCLUDED.conditional,
		        content      = EXCLUDED.content,
		        updated_at   = now(),
		        updated_by   = EXCLUDED.updated_by
		RETURNING name, display_name, conditional, content, updated_at, updated_by`,
		t.Name, t.DisplayName, t.Conditional, t.Content, t.UpdatedBy,
	).Scan(&out.Name, &out.DisplayName, &out.Conditional, &out.Content, &out.UpdatedAt, &out.UpdatedBy)
	if err != nil {
		return nil, fmt.Errorf("manifest_templates: upsert %s: %w", t.Name, err)
	}
	return &out, nil
}

// SeedManifestTemplates inserts default templates without overwriting admin edits.
// Uses ON CONFLICT DO NOTHING — once an admin edits a template it is preserved.
func (d *DB) SeedManifestTemplates(ctx context.Context, templates []ManifestTemplate) error {
	for _, t := range templates {
		_, err := d.pool.Exec(ctx, `
			INSERT INTO manifest_templates (name, display_name, conditional, content)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (name) DO NOTHING`,
			t.Name, t.DisplayName, t.Conditional, t.Content,
		)
		if err != nil {
			return fmt.Errorf("manifest_templates: seed %s: %w", t.Name, err)
		}
	}
	return nil
}

// ── Environment Profiles ──────────────────────────────────────────────────────

// GetEnvironmentProfile returns the resource profile for one environment tier.
func (d *DB) GetEnvironmentProfile(ctx context.Context, name string) (*EnvironmentProfile, error) {
	var p EnvironmentProfile
	err := d.pool.QueryRow(ctx, `
		SELECT name, cpu_request, mem_request, cpu_limit, mem_limit,
		       replicas, storage_class, hpa_enabled, hpa_min, hpa_max,
		       cpu_threshold, mem_threshold, updated_at, updated_by
		FROM   environment_profiles
		WHERE  name = $1`,
		name,
	).Scan(
		&p.Name, &p.CPURequest, &p.MemRequest, &p.CPULimit, &p.MemLimit,
		&p.Replicas, &p.StorageClass, &p.HPAEnabled, &p.HPAMin, &p.HPAMax,
		&p.CPUThreshold, &p.MemThreshold, &p.UpdatedAt, &p.UpdatedBy,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, pgx.ErrNoRows
		}
		return nil, fmt.Errorf("environment_profiles: get %s: %w", name, err)
	}
	return &p, nil
}

// ListEnvironmentProfiles returns all profiles (dev, uat, prod).
func (d *DB) ListEnvironmentProfiles(ctx context.Context) ([]EnvironmentProfile, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT name, cpu_request, mem_request, cpu_limit, mem_limit,
		       replicas, storage_class, hpa_enabled, hpa_min, hpa_max,
		       cpu_threshold, mem_threshold, updated_at, updated_by
		FROM   environment_profiles
		ORDER  BY name`)
	if err != nil {
		return nil, fmt.Errorf("environment_profiles: list: %w", err)
	}
	defer rows.Close()

	var profiles []EnvironmentProfile
	for rows.Next() {
		var p EnvironmentProfile
		if err := rows.Scan(
			&p.Name, &p.CPURequest, &p.MemRequest, &p.CPULimit, &p.MemLimit,
			&p.Replicas, &p.StorageClass, &p.HPAEnabled, &p.HPAMin, &p.HPAMax,
			&p.CPUThreshold, &p.MemThreshold, &p.UpdatedAt, &p.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("environment_profiles: scan: %w", err)
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// UpdateEnvironmentProfile updates the resource settings for one tier.
// Only platform admins call this.
func (d *DB) UpdateEnvironmentProfile(ctx context.Context, p EnvironmentProfile, updatedByID uuid.UUID) (*EnvironmentProfile, error) {
	var out EnvironmentProfile
	err := d.pool.QueryRow(ctx, `
		UPDATE environment_profiles
		SET    cpu_request   = $2,
		       mem_request   = $3,
		       cpu_limit     = $4,
		       mem_limit     = $5,
		       replicas      = $6,
		       storage_class = $7,
		       hpa_enabled   = $8,
		       hpa_min       = $9,
		       hpa_max       = $10,
		       cpu_threshold = $11,
		       mem_threshold = $12,
		       updated_at    = now(),
		       updated_by    = $13
		WHERE  name = $1
		RETURNING name, cpu_request, mem_request, cpu_limit, mem_limit,
		          replicas, storage_class, hpa_enabled, hpa_min, hpa_max,
		          cpu_threshold, mem_threshold, updated_at, updated_by`,
		p.Name, p.CPURequest, p.MemRequest, p.CPULimit, p.MemLimit,
		p.Replicas, p.StorageClass, p.HPAEnabled, p.HPAMin, p.HPAMax,
		p.CPUThreshold, p.MemThreshold, updatedByID,
	).Scan(
		&out.Name, &out.CPURequest, &out.MemRequest, &out.CPULimit, &out.MemLimit,
		&out.Replicas, &out.StorageClass, &out.HPAEnabled, &out.HPAMin, &out.HPAMax,
		&out.CPUThreshold, &out.MemThreshold, &out.UpdatedAt, &out.UpdatedBy,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, pgx.ErrNoRows
		}
		return nil, fmt.Errorf("environment_profiles: update %s: %w", p.Name, err)
	}
	return &out, nil
}

// ── Service Infra Requirements ────────────────────────────────────────────────

// SetServiceInfraRequirement inserts or replaces the infra selection for one
// service + type pair. ON CONFLICT replaces the config (developer changed mind).
func (d *DB) SetServiceInfraRequirement(ctx context.Context, req ServiceInfraRequirement) (*ServiceInfraRequirement, error) {
	raw, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("service_infra_requirements: marshal config: %w", err)
	}
	var out ServiceInfraRequirement
	var rawOut []byte
	err = d.pool.QueryRow(ctx, `
		INSERT INTO service_infra_requirements (project_id, service_type, config)
		VALUES ($1, $2, $3)
		ON CONFLICT (project_id, service_type) DO UPDATE
		    SET config = EXCLUDED.config
		RETURNING id, project_id, service_type, config, provisioned, vault_path, created_at`,
		req.ProjectID, req.ServiceType, raw,
	).Scan(
		&out.ID, &out.ProjectID, &out.ServiceType, &rawOut,
		&out.Provisioned, &out.VaultPath, &out.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("service_infra_requirements: set: %w", err)
	}
	out.Config = json.RawMessage(rawOut)
	return &out, nil
}

// ListServiceInfraRequirements returns all infra selections for a project.
// Called by the provisioner to know which resources to create.
func (d *DB) ListServiceInfraRequirements(ctx context.Context, projectID uuid.UUID) ([]ServiceInfraRequirement, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, project_id, service_type, config, provisioned, vault_path, created_at
		FROM   service_infra_requirements
		WHERE  project_id = $1
		ORDER  BY service_type`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("service_infra_requirements: list: %w", err)
	}
	defer rows.Close()

	var reqs []ServiceInfraRequirement
	for rows.Next() {
		var r ServiceInfraRequirement
		var rawOut []byte
		if err := rows.Scan(
			&r.ID, &r.ProjectID, &r.ServiceType, &rawOut,
			&r.Provisioned, &r.VaultPath, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("service_infra_requirements: scan: %w", err)
		}
		r.Config = json.RawMessage(rawOut)
		reqs = append(reqs, r)
	}
	return reqs, rows.Err()
}

// MarkInfraProvisioned marks one infra requirement as successfully provisioned
// and records the Vault path where credentials were stored.
func (d *DB) MarkInfraProvisioned(ctx context.Context, id uuid.UUID, vaultPath string) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE service_infra_requirements
		SET    provisioned = true,
		       vault_path  = $2
		WHERE  id = $1`,
		id, vaultPath,
	)
	if err != nil {
		return fmt.Errorf("service_infra_requirements: mark provisioned: %w", err)
	}
	return nil
}

// DeleteServiceInfraRequirement removes one infra selection for a project.
// Called when a developer deselects an infra option before provisioning runs.
func (d *DB) DeleteServiceInfraRequirement(ctx context.Context, projectID uuid.UUID, serviceType string) error {
	_, err := d.pool.Exec(ctx, `
		DELETE FROM service_infra_requirements
		WHERE project_id = $1 AND service_type = $2`,
		projectID, serviceType,
	)
	if err != nil {
		return fmt.Errorf("service_infra_requirements: delete: %w", err)
	}
	return nil
}

// ── Service Dependencies ───────────────────────────────────────────────────────

// SetServiceDependencies replaces all dependency edges for fromProject with the
// provided list. Existing edges not in toProjects are deleted; new ones are inserted.
func (d *DB) SetServiceDependencies(ctx context.Context, fromProject uuid.UUID, deps []ServiceDependency) error {
	// Delete all existing edges from this project, then re-insert.
	if _, err := d.pool.Exec(ctx,
		`DELETE FROM service_dependencies WHERE from_project = $1`, fromProject,
	); err != nil {
		return fmt.Errorf("service_dependencies: clear: %w", err)
	}
	for _, dep := range deps {
		if _, err := d.pool.Exec(ctx, `
			INSERT INTO service_dependencies (from_project, to_project, port, description)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (from_project, to_project) DO UPDATE
				SET port = EXCLUDED.port, description = EXCLUDED.description`,
			fromProject, dep.ToProject, dep.Port, dep.Description,
		); err != nil {
			return fmt.Errorf("service_dependencies: insert: %w", err)
		}
	}
	return nil
}

// ListServiceDependencies returns all outbound dependency edges for a project.
func (d *DB) ListServiceDependencies(ctx context.Context, fromProject uuid.UUID) ([]ServiceDependency, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, from_project, to_project, port, description, created_at
		FROM   service_dependencies
		WHERE  from_project = $1
		ORDER  BY created_at`, fromProject,
	)
	if err != nil {
		return nil, fmt.Errorf("service_dependencies: list: %w", err)
	}
	defer rows.Close()
	var out []ServiceDependency
	for rows.Next() {
		var d ServiceDependency
		if err := rows.Scan(&d.ID, &d.FromProject, &d.ToProject, &d.Port, &d.Description, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("service_dependencies: scan: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
