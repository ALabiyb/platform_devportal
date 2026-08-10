-- ---------------------------------------------------------------------------
-- Author: Labiyb M. Said — DevSecOps Engineer
-- Migration 004: dynamic pipeline templates + per-service Jenkins env vars
-- ---------------------------------------------------------------------------

-- Per-build-tool Jenkinsfile and Dockerfile templates, editable at runtime.
CREATE TABLE IF NOT EXISTS pipeline_templates (
    build_tool   TEXT PRIMARY KEY,
    jenkinsfile  TEXT NOT NULL,
    dockerfile   TEXT NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by   UUID REFERENCES users(id) ON DELETE SET NULL
);

GRANT SELECT, INSERT, UPDATE ON pipeline_templates TO devportal;

-- Per-service Jenkins env vars collected at service creation time.
ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS app_timezone       TEXT NOT NULL DEFAULT 'Africa/Dar_es_Salaam',
    ADD COLUMN IF NOT EXISTS staging_url        TEXT,
    ADD COLUMN IF NOT EXISTS k8s_manifest_paths TEXT NOT NULL DEFAULT '04-deployment.yaml';
