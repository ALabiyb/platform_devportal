-- ---------------------------------------------------------------------------
-- Author: Labiyb M. Said — DevSecOps Engineer
-- Contact: saidlabiybm@gmail.com
-- ---------------------------------------------------------------------------
-- Migration 009 — Service Security Configuration
--
-- Two additions:
--
--   1. SLA columns on projects — how many days the team has to remediate CVEs
--      of each severity before DefectDojo marks the finding as SLA-breached.
--      Defaults match OWASP remediation guidance:
--        Critical → 7 days, High → 30 days, Medium → 90 days, Low → 180 days
--      DevPortal writes these into DefectDojo SLA Configuration at service
--      creation time (provisioner Step 10).
--
--   2. project_members table — tracks which users are assigned to a service.
--      Used for two things:
--        a. DefectDojo product membership: only assigned users see that
--           service's security findings (provisioner Step 10 calls the API).
--        b. Future: Harbor robot-account scoping per developer.
--
--   The lead sets both at service-creation time; regular members cannot see
--   findings for services they are not assigned to.
-- ---------------------------------------------------------------------------

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS vuln_sla_critical INT NOT NULL DEFAULT 7,
    ADD COLUMN IF NOT EXISTS vuln_sla_high     INT NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS vuln_sla_medium   INT NOT NULL DEFAULT 90,
    ADD COLUMN IF NOT EXISTS vuln_sla_low      INT NOT NULL DEFAULT 180;

CREATE TABLE IF NOT EXISTS project_members (
    project_id UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    -- role: 'lead' | 'developer'
    -- leads can triage/accept findings in DefectDojo; developers are read-only
    role       TEXT        NOT NULL DEFAULT 'developer',
    added_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    added_by   UUID                 REFERENCES users(id),
    PRIMARY KEY (project_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_project_members_project ON project_members(project_id);
CREATE INDEX IF NOT EXISTS idx_project_members_user    ON project_members(user_id);
