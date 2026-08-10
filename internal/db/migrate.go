// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// migrate.go — embedded SQL migration runner.
//
// How it works:
//   1. On every startup, Migrate() creates a schema_migrations tracking table
//      if it does not exist.
//   2. If the tracking table is brand-new AND the database already has an
//      organisations table, we assume migrations 001–004 were applied manually
//      before this runner existed, and we record them as done so they are
//      not re-executed.
//   3. Each *.sql file in migrations/ is applied exactly once, in filename
//      order. Applied versions are recorded in schema_migrations so a restart
//      never re-runs them.
//   4. Every migration runs inside a transaction. If it fails the transaction
//      is rolled back, the version is not recorded, and the application exits
//      so the operator sees a clear error rather than a half-applied schema.
//
// Adding a migration:
//   - Create migrations/006_your_change.sql
//   - Rebuild the binary (go build embeds the file automatically)
//   - On next startup the runner applies it and records the version
//
// Safe to run on a fresh database or an existing one.

package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate applies any pending SQL migrations and returns an error if any
// migration fails. Call this once during startup, before serving requests.
func (db *DB) Migrate(ctx context.Context) error {
	// Step 1 — ensure the tracking table exists.
	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT        PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return fmt.Errorf("migrate: create schema_migrations: %w", err)
	}

	// Step 2 — bootstrap: if the organisations table already exists but the
	// tracking table is empty, the schema was set up before this runner existed.
	// Record 001–004 as already applied so they are not re-run.
	var trackingEmpty bool
	if err := db.pool.QueryRow(ctx,
		`SELECT NOT EXISTS (SELECT 1 FROM schema_migrations)`,
	).Scan(&trackingEmpty); err != nil {
		return fmt.Errorf("migrate: check tracking table: %w", err)
	}

	if trackingEmpty {
		var orgTableExists bool
		if err := db.pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = 'organizations'
			)
		`).Scan(&orgTableExists); err != nil {
			return fmt.Errorf("migrate: check schema: %w", err)
		}

		if orgTableExists {
			// Schema predates this runner — mark the known-applied migrations.
			bootstrap := []string{
				"001_initial.sql",
				"002_local_auth.sql",
				"003_applications.sql",
				"004_pipeline_templates.sql",
			}
			for _, v := range bootstrap {
				if _, err := db.pool.Exec(ctx,
					`INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT DO NOTHING`,
					v,
				); err != nil {
					return fmt.Errorf("migrate: bootstrap %s: %w", v, err)
				}
			}
			slog.Info("migrate: bootstrapped existing schema", "versions", bootstrap)
		}
	}

	// Step 3 — collect all migration files, sorted by name.
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("migrate: read migrations dir: %w", err)
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	// Step 4 — apply each migration that has not been recorded yet.
	applied := 0
	for _, filename := range files {
		var already bool
		if err := db.pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`,
			filename,
		).Scan(&already); err != nil {
			return fmt.Errorf("migrate: check %s: %w", filename, err)
		}
		if already {
			continue
		}

		sql, err := migrationFS.ReadFile("migrations/" + filename)
		if err != nil {
			return fmt.Errorf("migrate: read %s: %w", filename, err)
		}

		tx, err := db.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("migrate: begin tx for %s: %w", filename, err)
		}

		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrate: apply %s: %w", filename, err)
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1)`,
			filename,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrate: record %s: %w", filename, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("migrate: commit %s: %w", filename, err)
		}

		slog.Info("migrate: applied", "version", filename)
		applied++
	}

	if applied == 0 {
		slog.Info("migrate: schema up to date", "total", len(files))
	} else {
		slog.Info("migrate: done", "applied", applied, "total", len(files))
	}
	return nil
}
