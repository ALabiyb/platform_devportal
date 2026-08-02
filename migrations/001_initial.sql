-- ---------------------------------------------------------------------------
-- Author: Labiyb M. Said — DevSecOps Engineer
-- Contact: saidlabiybm@gmail.com
-- ---------------------------------------------------------------------------
-- Migration 001 — Initial Schema
--
-- Creates all 10 tables for devportal's own PostgreSQL database.
-- Tables are ordered by dependency: referenced tables are created first.
--
-- Run once on first deploy:
--   make migrate
-- Or automatically via docker-compose on first container start
-- (PostgreSQL runs every *.sql in /docker-entrypoint-initdb.d on init).

-- pgcrypto provides gen_random_uuid() for UUID primary keys.
-- This avoids needing the uuid-ossp extension or application-layer UUID generation.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ── organizations ─────────────────────────────────────────────────────────────
-- Top-level tenant boundary. In v0.1 there is a single org.
-- Multi-org support is the foundation for the future SaaS phase.
CREATE TABLE organizations (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    slug       TEXT        NOT NULL UNIQUE, -- URL-safe name used in API paths, e.g. "nexbridge"
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── users ─────────────────────────────────────────────────────────────────────
-- Every person who has logged into devportal at least once.
-- devportal does NOT manage passwords — Keycloak / GitLab owns all credentials.
-- We only store what the identity provider told us at login.
CREATE TABLE users (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email        TEXT        NOT NULL,
    display_name TEXT        NOT NULL,
    -- provider identifies the auth backend that created this user record.
    -- "oidc" = Keycloak · "gitlab" = GitLab OAuth fallback
    provider     TEXT        NOT NULL,
    -- provider_id is the stable unique identifier from the identity provider
    -- (Keycloak "sub" claim or GitLab user ID). Used to match returning users.
    provider_id  TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_id)      -- one devportal account per provider identity
);

-- ── sessions ──────────────────────────────────────────────────────────────────
-- DB-backed sessions survive pod restarts and can be explicitly revoked by admins.
-- Unlike pipeline-init's in-memory map, these persist across Kubernetes rollouts.
CREATE TABLE sessions (
    id         TEXT        PRIMARY KEY,  -- crypto/rand 256-bit hex string
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- role is cached at login from the identity provider's group claim.
    -- Possible values: "admin" | "developer" | "viewer"
    role       TEXT        NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- expires_at is extended on each request (sliding 24-hour window).
    expires_at TIMESTAMPTZ NOT NULL,
    last_seen  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index for the cleanup goroutine that deletes expired sessions every hour.
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- ── teams ─────────────────────────────────────────────────────────────────────
-- A team groups developers within an org (e.g. "backend", "mobile", "platform").
-- Projects belong to a team, so team members can see their own team's projects.
CREATE TABLE teams (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    slug       TEXT        NOT NULL,    -- URL-safe name, e.g. "backend-team"
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, slug)               -- slugs are unique within an org
);

-- ── team_members ──────────────────────────────────────────────────────────────
-- Many-to-many: a user can be a member of multiple teams, in different roles.
CREATE TABLE team_members (
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- role within the team: "lead" | "member"
    -- The org-level RBAC role (admin/developer/viewer) is separate and comes from Keycloak.
    role    TEXT NOT NULL DEFAULT 'member',
    PRIMARY KEY (team_id, user_id)
);

-- ── workspace_credentials ─────────────────────────────────────────────────────
-- Stores encrypted service credentials (GitLab token, Jenkins token, Harbor password, etc.)
-- at the org level so all teams can use them for provisioning.
-- encrypted_value is the AES-256-GCM ciphertext — the plaintext never touches the DB.
CREATE TABLE workspace_credentials (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    -- provider_type identifies what the credential is for.
    -- e.g. "gitlab" | "jenkins" | "harbor" | "defectdojo" | "argocd"
    provider_type   TEXT        NOT NULL,
    label           TEXT        NOT NULL,    -- human-readable name, e.g. "GitLab service account"
    encrypted_value BYTEA       NOT NULL,    -- AES-256-GCM ciphertext (never store plaintext)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── projects ──────────────────────────────────────────────────────────────────
-- One row per developer project that devportal has provisioned (or is provisioning).
-- This is the central table — environments, provisioning steps, and audit events
-- all reference a project.
CREATE TABLE projects (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id                 UUID        NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
    name                    TEXT        NOT NULL,
    slug                    TEXT        NOT NULL,           -- URL-safe name, e.g. "payment-service"
    -- git_repo_url is the developer's application source repository.
    git_repo_url            TEXT        NOT NULL,
    -- harbor_project is the Harbor image registry project where built images land.
    harbor_project          TEXT        NOT NULL,
    -- jenkins_folder is the Jenkins folder (team-level grouping) that contains this job.
    jenkins_folder          TEXT        NOT NULL,
    -- build_tool controls which Jenkinsfile / Dockerfile template is used.
    -- Values: "maven" | "gradle" | "nodejs-express" | "nextjs" | "go" |
    --         "python-fastapi" | "dotnet" | "flutter-web" | "auto"
    build_tool              TEXT        NOT NULL DEFAULT 'auto',
    notification_email      TEXT        NOT NULL,
    -- defectdojo_product_id is set after the DefectDojo product is created during provisioning.
    defectdojo_product_id   INT,
    -- status tracks the overall provisioning state of the project.
    -- Values: "provisioning" | "active" | "failed" | "archived"
    status                  TEXT        NOT NULL DEFAULT 'provisioning',
    -- generated_jenkinsfile stores the exact Jenkinsfile text that was committed,
    -- so developers can view and re-download it from devportal without re-generating.
    generated_jenkinsfile   TEXT,
    -- generated_manifest_paths is a JSON object mapping env name to committed file paths.
    -- e.g. {"dev": "projects/payment-service/dev/", "uat": "projects/payment-service/uat/"}
    generated_manifest_paths JSONB,
    -- manifest_repo_url is the GitLab repo where K8s manifests are committed.
    manifest_repo_url       TEXT,
    -- app_repo_url mirrors git_repo_url — kept for denormalization so queries are simpler.
    app_repo_url            TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by              UUID        REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE (team_id, slug)
);

-- ── environments ──────────────────────────────────────────────────────────────
-- One row per environment (dev, uat, prod) for each project.
-- Each environment gets its own K8s namespace, ArgoCD Application, and database.
CREATE TABLE environments (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    -- name is the environment tier: "dev" | "uat" | "prod"
    name           TEXT        NOT NULL,
    -- namespace is the K8s namespace, e.g. "payment-service-dev"
    namespace      TEXT        NOT NULL,
    -- ingress_url is the public URL for this environment, e.g. "https://payment-service-dev.apps.example.com"
    ingress_url    TEXT,
    -- argocd_app_name is the ArgoCD Application resource name, e.g. "payment-service-dev"
    argocd_app_name TEXT,
    -- db_name / db_username are the credentials devportal provisioned in App PostgreSQL.
    db_name        TEXT,
    db_username    TEXT,
    -- manifest_path is the directory in the manifest repo for this environment's YAMLs.
    manifest_path  TEXT,
    -- status mirrors the project status but per-environment.
    -- Values: "provisioning" | "active" | "failed"
    status         TEXT        NOT NULL DEFAULT 'provisioning',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)           -- each project has at most one "dev", one "uat", one "prod"
);

-- ── provisioning_steps ────────────────────────────────────────────────────────
-- Tracks the 15 steps of the provisioning orchestrator per project.
-- The SSE hub streams rows from this table to the browser in real time
-- so developers see live progress as each step completes.
CREATE TABLE provisioning_steps (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    -- step_index is the ordered position of this step, 1-based.
    step_index  INT         NOT NULL,
    label       TEXT        NOT NULL,   -- human-readable name shown in the UI
    -- status: "pending" | "running" | "done" | "failed"
    status      TEXT        NOT NULL DEFAULT 'pending',
    -- detail holds the output message or error from the step.
    detail      TEXT,
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    UNIQUE (project_id, step_index)     -- each step index appears once per project
);

-- ── audit_events ──────────────────────────────────────────────────────────────
-- Immutable log of every significant action taken in devportal.
-- Used by admins to answer "who did what and when" without grepping logs.
-- Rows are never updated or deleted.
CREATE TABLE audit_events (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id       UUID        REFERENCES users(id) ON DELETE SET NULL,
    -- action is a verb describing what happened, e.g. "project.create", "credential.delete"
    action        TEXT        NOT NULL,
    -- resource_type identifies the entity acted on, e.g. "project" | "team" | "credential"
    resource_type TEXT        NOT NULL,
    resource_id   UUID,                 -- references the affected row in its own table
    -- detail is a free-form JSON payload with context specific to this action.
    detail        JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index for the audit log UI which filters by org and sorts by time.
CREATE INDEX idx_audit_events_org_created ON audit_events(org_id, created_at DESC);
