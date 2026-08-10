// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// clusters.go — CRUD for the clusters and cluster_platform_services tables.
//
// Platform engineers register one cluster per environment tier (dev/uat/prod)
// via the admin UI. The orchestrator reads these rows at provision time to
// know which ArgoCD instance to call and which platform services are available.

package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ── Clusters ──────────────────────────────────────────────────────────────────

// CreateCluster inserts a new cluster record and returns the created row.
func (d *DB) CreateCluster(ctx context.Context, c Cluster) (*Cluster, error) {
	var out Cluster
	err := d.pool.QueryRow(ctx, `
		INSERT INTO clusters
		    (org_id, name, display_name, environment, api_endpoint, argocd_url,
		     kubeconfig_credential_id, argocd_credential_id, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, org_id, name, display_name, environment, api_endpoint,
		          argocd_url, kubeconfig_credential_id, argocd_credential_id,
		          status, created_at, created_by`,
		c.OrgID, c.Name, c.DisplayName, c.Environment,
		c.APIEndpoint, c.ArgoCDURL,
		c.KubeconfigCredentialID, c.ArgoCDCredentialID,
		c.Status, c.CreatedBy,
	).Scan(
		&out.ID, &out.OrgID, &out.Name, &out.DisplayName, &out.Environment,
		&out.APIEndpoint, &out.ArgoCDURL,
		&out.KubeconfigCredentialID, &out.ArgoCDCredentialID,
		&out.Status, &out.CreatedAt, &out.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("clusters: create: %w", err)
	}
	return &out, nil
}

// GetClusterByEnvironment returns the active cluster for the given org + tier.
// Returns pgx.ErrNoRows when no cluster is registered for that environment.
func (d *DB) GetClusterByEnvironment(ctx context.Context, orgID uuid.UUID, environment string) (*Cluster, error) {
	var out Cluster
	err := d.pool.QueryRow(ctx, `
		SELECT id, org_id, name, display_name, environment, api_endpoint,
		       argocd_url, kubeconfig_credential_id, argocd_credential_id,
		       status, created_at, created_by
		FROM   clusters
		WHERE  org_id = $1 AND environment = $2 AND status = 'active'`,
		orgID, environment,
	).Scan(
		&out.ID, &out.OrgID, &out.Name, &out.DisplayName, &out.Environment,
		&out.APIEndpoint, &out.ArgoCDURL,
		&out.KubeconfigCredentialID, &out.ArgoCDCredentialID,
		&out.Status, &out.CreatedAt, &out.CreatedBy,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, pgx.ErrNoRows
		}
		return nil, fmt.Errorf("clusters: get by env: %w", err)
	}
	return &out, nil
}

// ListClusters returns all clusters for an org ordered by environment.
func (d *DB) ListClusters(ctx context.Context, orgID uuid.UUID) ([]Cluster, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, org_id, name, display_name, environment, api_endpoint,
		       argocd_url, kubeconfig_credential_id, argocd_credential_id,
		       status, created_at, created_by
		FROM   clusters
		WHERE  org_id = $1
		ORDER  BY environment`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("clusters: list: %w", err)
	}
	defer rows.Close()

	var clusters []Cluster
	for rows.Next() {
		var c Cluster
		if err := rows.Scan(
			&c.ID, &c.OrgID, &c.Name, &c.DisplayName, &c.Environment,
			&c.APIEndpoint, &c.ArgoCDURL,
			&c.KubeconfigCredentialID, &c.ArgoCDCredentialID,
			&c.Status, &c.CreatedAt, &c.CreatedBy,
		); err != nil {
			return nil, fmt.Errorf("clusters: scan: %w", err)
		}
		clusters = append(clusters, c)
	}
	return clusters, rows.Err()
}

// UpdateCluster updates mutable fields on an existing cluster.
func (d *DB) UpdateCluster(ctx context.Context, id uuid.UUID, c Cluster) (*Cluster, error) {
	var out Cluster
	err := d.pool.QueryRow(ctx, `
		UPDATE clusters
		SET    name                     = $2,
		       display_name             = $3,
		       api_endpoint             = $4,
		       argocd_url               = $5,
		       kubeconfig_credential_id = $6,
		       argocd_credential_id     = $7,
		       status                   = $8
		WHERE  id = $1
		RETURNING id, org_id, name, display_name, environment, api_endpoint,
		          argocd_url, kubeconfig_credential_id, argocd_credential_id,
		          status, created_at, created_by`,
		id, c.Name, c.DisplayName, c.APIEndpoint, c.ArgoCDURL,
		c.KubeconfigCredentialID, c.ArgoCDCredentialID, c.Status,
	).Scan(
		&out.ID, &out.OrgID, &out.Name, &out.DisplayName, &out.Environment,
		&out.APIEndpoint, &out.ArgoCDURL,
		&out.KubeconfigCredentialID, &out.ArgoCDCredentialID,
		&out.Status, &out.CreatedAt, &out.CreatedBy,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, pgx.ErrNoRows
		}
		return nil, fmt.Errorf("clusters: update: %w", err)
	}
	return &out, nil
}

// ── Cluster Platform Services ─────────────────────────────────────────────────

// UpsertClusterPlatformService inserts or updates the config for one service
// type on a cluster. ON CONFLICT updates config, enabled, updated_at/by.
func (d *DB) UpsertClusterPlatformService(ctx context.Context, svc ClusterPlatformService, updatedByID uuid.UUID) (*ClusterPlatformService, error) {
	raw, err := json.Marshal(svc.Config)
	if err != nil {
		return nil, fmt.Errorf("cluster_platform_services: marshal config: %w", err)
	}
	var out ClusterPlatformService
	var rawOut []byte
	err = d.pool.QueryRow(ctx, `
		INSERT INTO cluster_platform_services
		    (cluster_id, service_type, enabled, config, updated_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (cluster_id, service_type) DO UPDATE
		    SET enabled    = EXCLUDED.enabled,
		        config     = EXCLUDED.config,
		        updated_at = now(),
		        updated_by = EXCLUDED.updated_by
		RETURNING id, cluster_id, service_type, enabled, config, updated_at, updated_by`,
		svc.ClusterID, svc.ServiceType, svc.Enabled, raw, updatedByID,
	).Scan(
		&out.ID, &out.ClusterID, &out.ServiceType, &out.Enabled,
		&rawOut, &out.UpdatedAt, &out.UpdatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("cluster_platform_services: upsert: %w", err)
	}
	out.Config = json.RawMessage(rawOut)
	return &out, nil
}

// GetClusterPlatformService returns config for one service type on a cluster.
func (d *DB) GetClusterPlatformService(ctx context.Context, clusterID uuid.UUID, serviceType string) (*ClusterPlatformService, error) {
	var out ClusterPlatformService
	var rawOut []byte
	err := d.pool.QueryRow(ctx, `
		SELECT id, cluster_id, service_type, enabled, config, updated_at, updated_by
		FROM   cluster_platform_services
		WHERE  cluster_id = $1 AND service_type = $2`,
		clusterID, serviceType,
	).Scan(
		&out.ID, &out.ClusterID, &out.ServiceType, &out.Enabled,
		&rawOut, &out.UpdatedAt, &out.UpdatedBy,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, pgx.ErrNoRows
		}
		return nil, fmt.Errorf("cluster_platform_services: get: %w", err)
	}
	out.Config = json.RawMessage(rawOut)
	return &out, nil
}

// ListClusterPlatformServices returns all service configs for a cluster.
func (d *DB) ListClusterPlatformServices(ctx context.Context, clusterID uuid.UUID) ([]ClusterPlatformService, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, cluster_id, service_type, enabled, config, updated_at, updated_by
		FROM   cluster_platform_services
		WHERE  cluster_id = $1
		ORDER  BY service_type`,
		clusterID,
	)
	if err != nil {
		return nil, fmt.Errorf("cluster_platform_services: list: %w", err)
	}
	defer rows.Close()

	var svcs []ClusterPlatformService
	for rows.Next() {
		var s ClusterPlatformService
		var rawOut []byte
		if err := rows.Scan(
			&s.ID, &s.ClusterID, &s.ServiceType, &s.Enabled,
			&rawOut, &s.UpdatedAt, &s.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("cluster_platform_services: scan: %w", err)
		}
		s.Config = json.RawMessage(rawOut)
		svcs = append(svcs, s)
	}
	return svcs, rows.Err()
}

// ListEnabledClusterPlatformServices returns only enabled service configs for a cluster.
// Called by the provisioner to know which infra services are available.
func (d *DB) ListEnabledClusterPlatformServices(ctx context.Context, clusterID uuid.UUID) ([]ClusterPlatformService, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, cluster_id, service_type, enabled, config, updated_at, updated_by
		FROM   cluster_platform_services
		WHERE  cluster_id = $1 AND enabled = true
		ORDER  BY service_type`,
		clusterID,
	)
	if err != nil {
		return nil, fmt.Errorf("cluster_platform_services: list enabled: %w", err)
	}
	defer rows.Close()

	var svcs []ClusterPlatformService
	for rows.Next() {
		var s ClusterPlatformService
		var rawOut []byte
		if err := rows.Scan(
			&s.ID, &s.ClusterID, &s.ServiceType, &s.Enabled,
			&rawOut, &s.UpdatedAt, &s.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("cluster_platform_services: scan enabled: %w", err)
		}
		s.Config = json.RawMessage(rawOut)
		svcs = append(svcs, s)
	}
	return svcs, rows.Err()
}

// SetEnvironmentCluster links an environment row to a registered cluster.
// Called by the orchestrator after ArgoCD App creation.
func (d *DB) SetEnvironmentCluster(ctx context.Context, envID, clusterID uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE environments SET cluster_id = $2 WHERE id = $1`,
		envID, clusterID,
	)
	if err != nil {
		return fmt.Errorf("environments: set cluster: %w", err)
	}
	return nil
}

// suppress unused import warning for time (used for future timestamp columns)
var _ = time.Now
