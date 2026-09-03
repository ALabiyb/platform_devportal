-- ---------------------------------------------------------------------------
-- Author: Labiyb M. Said — DevSecOps Engineer
-- Contact: saidlabiybm@gmail.com
-- ---------------------------------------------------------------------------

-- Language-specific Kubernetes deployment tuning, editable by the platform team.
-- One row per build tool (matches pipeline_templates.build_tool).
--
-- Why separate from environment_profiles:
--   environment_profiles  → resource quotas that scale per env tier (dev/uat/prod)
--   language_profiles     → probe timing and env vars that are fixed per language
--                           regardless of which environment the service runs in
--
-- The orchestrator reads both at provision time and merges them into ManifestInput.

CREATE TABLE IF NOT EXISTS language_profiles (
    build_tool      TEXT        PRIMARY KEY,
    display_name    TEXT        NOT NULL,
    liveness_delay  INT         NOT NULL DEFAULT 30,   -- initialDelaySeconds for liveness probe
    readiness_delay INT         NOT NULL DEFAULT 10,   -- initialDelaySeconds for readiness probe
    extra_env       JSONB       NOT NULL DEFAULT '{}', -- key→value env vars injected into Deployment
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      UUID        REFERENCES users(id)
);
