-- ---------------------------------------------------------------------------
-- Author: Labiyb M. Said — DevSecOps Engineer
-- Contact: saidlabiybm@gmail.com
-- ---------------------------------------------------------------------------

-- 009: Add vulnerability SLA columns to projects and create project_members table.

-- SLA columns (days to remediate) with OWASP-recommended defaults.
ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS vuln_sla_critical INT NOT NULL DEFAULT 7,
    ADD COLUMN IF NOT EXISTS vuln_sla_high     INT NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS vuln_sla_medium   INT NOT NULL DEFAULT 90,
    ADD COLUMN IF NOT EXISTS vuln_sla_low      INT NOT NULL DEFAULT 180;

-- project_members: maps DevPortal users to services with a role.
-- Used to sync members to DefectDojo products during provisioning.
CREATE TABLE IF NOT EXISTS project_members (
    project_id  UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    role        TEXT        NOT NULL DEFAULT 'developer', -- 'lead' | 'developer'
    added_by    UUID        REFERENCES users(id) ON DELETE SET NULL,
    added_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members (user_id);
