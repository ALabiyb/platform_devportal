// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com

package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type DashboardStats struct {
	ServiceCount        int     `json:"service_count"`
	AppCount            int     `json:"app_count"`
	ClusterCount        int     `json:"cluster_count"`
	ActiveServiceCount  int     `json:"active_service_count"`
	FailedServiceCount  int     `json:"failed_service_count"`
	PipelineSuccessRate float64 `json:"pipeline_success_rate"`
}

type ClusterSummary struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Environment string    `json:"environment"`
	APIEndpoint string    `json:"api_endpoint"`
	Status      string    `json:"status"`
}

type DashActivityEvent struct {
	ID           uuid.UUID       `json:"id"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   *uuid.UUID      `json:"resource_id,omitempty"`
	ActorEmail   *string         `json:"actor_email,omitempty"`
	Detail       json.RawMessage `json:"detail,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

type DashboardData struct {
	Stats    DashboardStats      `json:"stats"`
	Clusters []ClusterSummary    `json:"clusters"`
	Activity []DashActivityEvent `json:"activity"`
}

func (db *DB) GetDashboardData(ctx context.Context, orgID uuid.UUID) (*DashboardData, error) {
	const statsQ = `
		SELECT
			(SELECT COUNT(*) FROM projects p JOIN applications a ON p.application_id = a.id
			 WHERE a.org_id = $1 AND p.status != 'archived')::int   AS service_count,
			(SELECT COUNT(*) FROM applications
			 WHERE org_id = $1 AND status != 'archived')::int        AS app_count,
			(SELECT COUNT(*) FROM clusters
			 WHERE org_id = $1 AND status != 'archived')::int        AS cluster_count,
			(SELECT COUNT(*) FROM projects p JOIN applications a ON p.application_id = a.id
			 WHERE a.org_id = $1 AND p.status = 'active')::int       AS active_svc,
			(SELECT COUNT(*) FROM projects p JOIN applications a ON p.application_id = a.id
			 WHERE a.org_id = $1 AND p.status = 'failed')::int       AS failed_svc
	`
	var stats DashboardStats
	if err := db.pool.QueryRow(ctx, statsQ, orgID).Scan(
		&stats.ServiceCount, &stats.AppCount, &stats.ClusterCount,
		&stats.ActiveServiceCount, &stats.FailedServiceCount,
	); err != nil {
		return nil, fmt.Errorf("db.GetDashboardData stats: %w", err)
	}
	total := stats.ActiveServiceCount + stats.FailedServiceCount
	if total > 0 {
		stats.PipelineSuccessRate = float64(stats.ActiveServiceCount) / float64(total) * 100
	} else {
		stats.PipelineSuccessRate = 100.0
	}

	const clusterQ = `
		SELECT id, name, display_name, environment, api_endpoint, status
		FROM   clusters
		WHERE  org_id = $1 AND status != 'archived'
		ORDER  BY environment, name
		LIMIT  10
	`
	clusterRows, err := db.pool.Query(ctx, clusterQ, orgID)
	if err != nil {
		return nil, fmt.Errorf("db.GetDashboardData clusters: %w", err)
	}
	clusters, err := pgx.CollectRows(clusterRows, func(row pgx.CollectableRow) (ClusterSummary, error) {
		var c ClusterSummary
		return c, row.Scan(&c.ID, &c.Name, &c.DisplayName, &c.Environment, &c.APIEndpoint, &c.Status)
	})
	if err != nil {
		return nil, fmt.Errorf("db.GetDashboardData cluster scan: %w", err)
	}
	if clusters == nil {
		clusters = []ClusterSummary{}
	}

	const actQ = `
		SELECT ae.id, ae.action, ae.resource_type, ae.resource_id,
		       u.email, ae.detail, ae.created_at
		FROM   audit_events ae
		LEFT   JOIN users u ON ae.user_id = u.id
		WHERE  ae.org_id = $1
		ORDER  BY ae.created_at DESC
		LIMIT  10
	`
	actRows, err := db.pool.Query(ctx, actQ, orgID)
	if err != nil {
		return nil, fmt.Errorf("db.GetDashboardData activity: %w", err)
	}
	events, err := pgx.CollectRows(actRows, func(row pgx.CollectableRow) (DashActivityEvent, error) {
		var e DashActivityEvent
		return e, row.Scan(&e.ID, &e.Action, &e.ResourceType, &e.ResourceID, &e.ActorEmail, &e.Detail, &e.CreatedAt)
	})
	if err != nil {
		return nil, fmt.Errorf("db.GetDashboardData activity scan: %w", err)
	}
	if events == nil {
		events = []DashActivityEvent{}
	}

	return &DashboardData{Stats: stats, Clusters: clusters, Activity: events}, nil
}
