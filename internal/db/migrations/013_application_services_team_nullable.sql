-- ---------------------------------------------------------------------------
-- Author: Labiyb M. Said — DevSecOps Engineer
-- Contact: saidlabiybm@gmail.com
-- ---------------------------------------------------------------------------
-- Migration 013: Allow application-scoped services to have no team_id.
--
-- Services created under an Application (via the service wizard) belong to the
-- application rather than to any team. The team_id column therefore needs to be
-- nullable for those rows.  Legacy standalone projects (created via the old
-- POST /api/v1/projects endpoint) continue to carry a team_id.

ALTER TABLE projects ALTER COLUMN team_id DROP NOT NULL;
