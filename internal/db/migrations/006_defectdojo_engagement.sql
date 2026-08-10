-- ---------------------------------------------------------------------------
-- Author: Labiyb M. Said — DevSecOps Engineer
-- Contact: saidlabiybm@gmail.com
-- ---------------------------------------------------------------------------
-- Migration 006 — DefectDojo engagement ID per service
--
-- DevPortal creates one DefectDojo engagement per service at provisioning
-- time (Step 11). The returned ID is now stored so subsequent Jenkinsfile
-- renders can embed the correct DEFECTDOJO_ENGAGEMENT_ID.
-- ---------------------------------------------------------------------------

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS defectdojo_engagement_id INTEGER;
