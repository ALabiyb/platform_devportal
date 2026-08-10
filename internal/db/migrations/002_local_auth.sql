-- ---------------------------------------------------------------------------
-- Author: Labiyb M. Said — DevSecOps Engineer
-- Contact: saidlabiybm@gmail.com
-- ---------------------------------------------------------------------------
-- Migration 002 — Local Auth Support
--
-- Adds password_hash to the users table so devportal can manage its own
-- user accounts when AUTH_MODE=local (no external IdP required).
--
-- Design decisions:
--   - password_hash is nullable: OIDC users have no password hash (NULL).
--     Only users created via local auth have a bcrypt hash stored here.
--   - provider = 'local' distinguishes local accounts from OIDC accounts.
--   - role is stored on the user row for local mode so admins can assign
--     roles when creating accounts (OIDC gets role from group claims).
--
-- Run after 001_initial.sql:
--   make migrate

-- password_hash stores the bcrypt hash of the user's password.
-- NULL for OIDC users — they authenticate via Keycloak, not devportal.
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;

-- role stored on the user for local auth (OIDC gets it from claims at login).
-- Defaults to 'viewer' (least privilege). Admin sets role on account creation.
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'viewer';

-- is_active lets admins deactivate accounts without deleting audit history.
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;
