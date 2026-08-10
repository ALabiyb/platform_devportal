-- ---------------------------------------------------------------------------
-- Author: Labiyb M. Said — DevSecOps Engineer
-- Contact: saidlabiybm@gmail.com
-- ---------------------------------------------------------------------------
-- Migration 005 — Named unique constraints and cascade indexes
--
-- Replaces the anonymous UNIQUE(team_id, slug) from 001 with named partial
-- unique indexes so application-scoped services don't collide with team-scoped
-- projects, and vice versa. Safe to run even if the constraints already exist.
-- ---------------------------------------------------------------------------

-- Drop the old anonymous constraint if it exists (created in 001_initial.sql).
-- We replace it with two partial unique indexes below.
DO $$
BEGIN
    -- Remove the old unconditional unique constraint if present.
    -- This is safe: the two partial indexes below enforce the same invariant
    -- but correctly allow the same slug across different teams/applications.
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'projects_team_id_slug_key'
          AND conrelid = 'projects'::regclass
    ) THEN
        ALTER TABLE projects DROP CONSTRAINT projects_team_id_slug_key;
    END IF;
END$$;

-- Slug must be unique within a team (for team-owned services).
CREATE UNIQUE INDEX IF NOT EXISTS uq_projects_team_slug
    ON projects (team_id, slug)
    WHERE application_id IS NULL;

-- Slug must be unique within an application (for application-owned services).
CREATE UNIQUE INDEX IF NOT EXISTS uq_projects_app_slug
    ON projects (application_id, slug)
    WHERE application_id IS NOT NULL;
