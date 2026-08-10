-- ---------------------------------------------------------------------------
-- Author: Labiyb M. Said — DevSecOps Engineer
-- Contact: saidlabiybm@gmail.com
-- ---------------------------------------------------------------------------
-- Migration 003: Applications and application membership (least-privilege model)
--
-- An Application is a business initiative (e.g. "Restaurant POS System").
-- It owns a GitLab group and controls who can create services under it.
-- Team membership is just a department label — access is per-application.
-- ---------------------------------------------------------------------------

-- ── applications ─────────────────────────────────────────────────────────────
CREATE TABLE applications (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name          TEXT        NOT NULL,
    slug          TEXT        NOT NULL,
    description   TEXT        NOT NULL DEFAULT '',
    git_namespace TEXT        NOT NULL,   -- top-level GitLab group, e.g. "restaurant-pos-system"
    status        TEXT        NOT NULL DEFAULT 'active',  -- 'active' | 'archived'
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by    UUID        REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE (org_id, slug),
    UNIQUE (org_id, git_namespace)
);

-- ── application_members ───────────────────────────────────────────────────────
-- Controls who can see and create services in an application.
-- Only members (or admins) can access an application — not the whole team.
CREATE TABLE application_members (
    application_id UUID        NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    user_id        UUID        NOT NULL REFERENCES users(id)         ON DELETE CASCADE,
    role           TEXT        NOT NULL DEFAULT 'developer',  -- 'lead' | 'developer'
    added_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    added_by       UUID        REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (application_id, user_id)
);

-- ── projects: link to application ─────────────────────────────────────────────
-- Nullable so existing services without an application still work.
ALTER TABLE projects ADD COLUMN IF NOT EXISTS application_id UUID REFERENCES applications(id) ON DELETE SET NULL;

-- Index for fast "list services by application" queries.
CREATE INDEX IF NOT EXISTS idx_projects_application_id ON projects (application_id);
CREATE INDEX IF NOT EXISTS idx_application_members_user_id ON application_members (user_id);
