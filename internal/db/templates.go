// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// templates.go — CRUD for pipeline_templates (Jenkinsfile + Dockerfile per build tool).

package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PipelineTemplate holds the Jenkinsfile template and Dockerfile for one build tool.
// Both are stored as raw text so admins can edit them live from the portal UI.
type PipelineTemplate struct {
	BuildTool  string     `db:"build_tool"  json:"build_tool"`
	Jenkinsfile string    `db:"jenkinsfile" json:"jenkinsfile"`
	Dockerfile  string    `db:"dockerfile"  json:"dockerfile"`
	UpdatedAt   time.Time `db:"updated_at"  json:"updated_at"`
	UpdatedBy   *uuid.UUID `db:"updated_by" json:"updated_by,omitempty"`
}

// GetPipelineTemplate returns the template for a build tool, or pgx.ErrNoRows if absent.
func (db *DB) GetPipelineTemplate(ctx context.Context, buildTool string) (*PipelineTemplate, error) {
	const q = `
		SELECT build_tool, jenkinsfile, dockerfile, updated_at, updated_by
		FROM pipeline_templates WHERE build_tool = $1
	`
	rows, err := db.pool.Query(ctx, q, buildTool)
	if err != nil {
		return nil, fmt.Errorf("db.GetPipelineTemplate: %w", err)
	}
	t, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[PipelineTemplate])
	if err != nil {
		return nil, fmt.Errorf("db.GetPipelineTemplate: %w", err)
	}
	return t, nil
}

// ListPipelineTemplates returns all templates ordered by build_tool.
func (db *DB) ListPipelineTemplates(ctx context.Context) ([]PipelineTemplate, error) {
	const q = `
		SELECT build_tool, jenkinsfile, dockerfile, updated_at, updated_by
		FROM pipeline_templates ORDER BY build_tool ASC
	`
	rows, err := db.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db.ListPipelineTemplates: %w", err)
	}
	templates, err := pgx.CollectRows(rows, pgx.RowToStructByName[PipelineTemplate])
	if err != nil {
		return nil, fmt.Errorf("db.ListPipelineTemplates: %w", err)
	}
	return templates, nil
}

// UpsertPipelineTemplate inserts or replaces a template row.
func (db *DB) UpsertPipelineTemplate(ctx context.Context, buildTool, jenkinsfile, dockerfile string, updatedBy *uuid.UUID) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO pipeline_templates (build_tool, jenkinsfile, dockerfile, updated_at, updated_by)
		VALUES ($1, $2, $3, now(), $4)
		ON CONFLICT (build_tool) DO UPDATE
		    SET jenkinsfile = EXCLUDED.jenkinsfile,
		        dockerfile  = EXCLUDED.dockerfile,
		        updated_at  = now(),
		        updated_by  = EXCLUDED.updated_by
	`, buildTool, jenkinsfile, dockerfile, updatedBy)
	if err != nil {
		return fmt.Errorf("db.UpsertPipelineTemplate: %w", err)
	}
	return nil
}

// SeedPipelineTemplates inserts default templates for build tools that don't yet
// have a row. Existing rows are left untouched — admin edits are preserved.
func (db *DB) SeedPipelineTemplates(ctx context.Context, defaults []PipelineTemplate) error {
	for _, t := range defaults {
		_, err := db.pool.Exec(ctx, `
			INSERT INTO pipeline_templates (build_tool, jenkinsfile, dockerfile)
			VALUES ($1, $2, $3)
			ON CONFLICT (build_tool) DO NOTHING
		`, t.BuildTool, t.Jenkinsfile, t.Dockerfile)
		if err != nil {
			return fmt.Errorf("db.SeedPipelineTemplates[%s]: %w", t.BuildTool, err)
		}
	}
	return nil
}
