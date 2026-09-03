-- migration 012: service_kind column on projects
-- Distinguishes backend APIs, frontend apps, and background workers so the
-- orchestrator can generate the correct manifest shape for each.

ALTER TABLE projects
  ADD COLUMN IF NOT EXISTS service_kind TEXT NOT NULL DEFAULT 'backend';

ALTER TABLE projects
  DROP CONSTRAINT IF EXISTS projects_service_kind_check;

ALTER TABLE projects
  ADD CONSTRAINT projects_service_kind_check
  CHECK (service_kind IN ('backend', 'frontend', 'worker'));
