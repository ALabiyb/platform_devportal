-- ---------------------------------------------------------------------------
-- Author: Labiyb M. Said — DevSecOps Engineer
-- Contact: saidlabiybm@gmail.com
-- ---------------------------------------------------------------------------
-- Migration 008 — Provisioning Job Queue
--
-- Decouples the HTTP handler from the provisioner orchestrator.
-- The handler enqueues a row here and returns 201 immediately.
-- The worker process (cmd/worker or embedded in cmd/devportal) polls this
-- table with SELECT ... FOR UPDATE SKIP LOCKED, atomically claiming one job
-- at a time per worker goroutine, and runs the full 15-step orchestrator.
--
-- Benefits:
--   - Provisioning survives an API pod restart (job is re-claimed)
--   - Multiple worker goroutines can run concurrently without coordination
--   - Audit trail: every provisioning attempt is recorded with timing + error
--   - No external queue dependency (Redis, RabbitMQ) — Postgres is enough
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS provisioning_jobs (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,

    -- payload carries the context the worker needs beyond the project row itself.
    -- Stored as JSONB so the worker can reload it without parsing a binary blob.
    -- Shape: { "git_namespace": "restaurant-pos", "application_slug": "restaurant-pos" }
    payload       JSONB       NOT NULL DEFAULT '{}',

    -- status lifecycle: pending → running → done | failed
    status        TEXT        NOT NULL DEFAULT 'pending',

    -- attempt_count is incremented each time a worker claims this job.
    -- When attempt_count >= max_attempts the job is permanently failed.
    attempt_count INT         NOT NULL DEFAULT 0,
    max_attempts  INT         NOT NULL DEFAULT 3,

    -- worker_id identifies the host/pod that claimed this job.
    -- Set to the OS hostname at claim time for observability.
    worker_id     TEXT,

    -- error_msg is the last failure reason. Overwritten on each failed attempt.
    error_msg     TEXT,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ
);

-- Index for the hot claim path: find the oldest pending job efficiently.
CREATE INDEX IF NOT EXISTS idx_provisioning_jobs_claim
    ON provisioning_jobs (status, created_at)
    WHERE status = 'pending';

-- Index for the stalled-job recovery query: find running jobs older than N minutes.
CREATE INDEX IF NOT EXISTS idx_provisioning_jobs_running
    ON provisioning_jobs (status, started_at)
    WHERE status = 'running';
