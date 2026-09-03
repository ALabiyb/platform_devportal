-- ---------------------------------------------------------------------------
-- Author: Labiyb M. Said — DevSecOps Engineer
-- Contact: saidlabiybm@gmail.com
-- ---------------------------------------------------------------------------
-- Migration 010 — Service Dependencies
--
-- Records which services communicate with each other within an application.
-- Used by the orchestrator to generate Kubernetes NetworkPolicy CRs that
-- explicitly allow egress from the source service to the target service's port,
-- while denying all other inter-service traffic (default-deny namespace policy).
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS service_dependencies (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    -- from_project is the service that initiates traffic (the caller).
    from_project UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    -- to_project is the service that receives traffic (the callee).
    to_project   UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    -- port is the port on the target service that traffic is allowed to reach.
    port         INT         NOT NULL DEFAULT 80,
    -- description is optional human-readable context (e.g. "REST API calls").
    description  TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- a service may only declare one dependency edge per target service
    UNIQUE (from_project, to_project),
    -- a service cannot depend on itself
    CHECK (from_project <> to_project)
);

CREATE INDEX IF NOT EXISTS idx_service_deps_from ON service_dependencies (from_project);
CREATE INDEX IF NOT EXISTS idx_service_deps_to   ON service_dependencies (to_project);
