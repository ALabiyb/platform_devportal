// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// users.go contains query helpers for the organizations and users tables.
//
// Key functions:
//   - UpsertUser         — called on every OIDC login (create or update)
//   - CreateLocalUser    — called when an admin creates a local-auth account
//   - GetLocalUserByEmail — called on every local login to fetch hash + role
//   - EnsureOrg          — guarantees at least one org exists before first login

package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// EnsureOrg returns an existing organization by slug or creates it if it
// does not exist yet. Used during bootstrap to guarantee at least one org
// exists before any user can log in.
func (db *DB) EnsureOrg(ctx context.Context, name, slug string) (*Organization, error) {
	org, err := db.GetOrgBySlug(ctx, slug)
	if err == nil {
		return org, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("db.EnsureOrg: lookup: %w", err)
	}

	const q = `
		INSERT INTO organizations (name, slug)
		VALUES ($1, $2)
		ON CONFLICT (slug) DO NOTHING
		RETURNING id, name, slug, created_at
	`
	rows, err := db.pool.Query(ctx, q, name, slug)
	if err != nil {
		return nil, fmt.Errorf("db.EnsureOrg: insert: %w", err)
	}
	org, err = pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Organization])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.GetOrgBySlug(ctx, slug)
		}
		return nil, fmt.Errorf("db.EnsureOrg: scan: %w", err)
	}
	return org, nil
}

// GetOrgBySlug returns the organization with the given slug.
// Returns pgx.ErrNoRows if no org exists with that slug.
func (db *DB) GetOrgBySlug(ctx context.Context, slug string) (*Organization, error) {
	const q = `
		SELECT id, name, slug, created_at
		FROM organizations
		WHERE slug = $1
	`
	rows, err := db.pool.Query(ctx, q, slug)
	if err != nil {
		return nil, fmt.Errorf("db.GetOrgBySlug: query: %w", err)
	}
	org, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Organization])
	if err != nil {
		return nil, fmt.Errorf("db.GetOrgBySlug: %w", err)
	}
	return org, nil
}

// UpsertUser creates or updates an OIDC user record on every successful login.
// The (provider, provider_id) pair is the stable key from Keycloak — it never
// changes even if the user renames their account.
// Role is updated on every login so Keycloak group changes take effect immediately.
func (db *DB) UpsertUser(ctx context.Context, u User) (*User, error) {
	const q = `
		INSERT INTO users (org_id, email, display_name, provider, provider_id, role)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (provider, provider_id) DO UPDATE
			SET email        = EXCLUDED.email,
			    display_name = EXCLUDED.display_name,
			    role         = EXCLUDED.role
		RETURNING id, org_id, email, display_name, provider, provider_id,
		          role, password_hash, is_active, created_at
	`
	rows, err := db.pool.Query(ctx, q,
		u.OrgID, u.Email, u.DisplayName, u.Provider, u.ProviderID, u.Role,
	)
	if err != nil {
		return nil, fmt.Errorf("db.UpsertUser: query: %w", err)
	}
	user, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[User])
	if err != nil {
		return nil, fmt.Errorf("db.UpsertUser: scan: %w", err)
	}
	return user, nil
}

// GetUserByID returns the user with the given UUID primary key.
func (db *DB) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	const q = `
		SELECT id, org_id, email, display_name, provider, provider_id,
		       role, password_hash, is_active, created_at
		FROM users
		WHERE id = $1
	`
	rows, err := db.pool.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("db.GetUserByID: query: %w", err)
	}
	user, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[User])
	if err != nil {
		return nil, fmt.Errorf("db.GetUserByID: %w", err)
	}
	return user, nil
}

// GetUserByProvider looks up a user by the (provider, providerID) pair.
// Used during OIDC callback to find existing accounts.
func (db *DB) GetUserByProvider(ctx context.Context, provider, providerID string) (*User, error) {
	const q = `
		SELECT id, org_id, email, display_name, provider, provider_id,
		       role, password_hash, is_active, created_at
		FROM users
		WHERE provider = $1 AND provider_id = $2
	`
	rows, err := db.pool.Query(ctx, q, provider, providerID)
	if err != nil {
		return nil, fmt.Errorf("db.GetUserByProvider: query: %w", err)
	}
	user, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[User])
	if err != nil {
		return nil, fmt.Errorf("db.GetUserByProvider: %w", err)
	}
	return user, nil
}

// GetLocalUserByEmail returns a local-auth user by email.
// Returns pgx.ErrNoRows if no local user exists with that email.
// Includes password_hash so the caller can verify the bcrypt hash.
func (db *DB) GetLocalUserByEmail(ctx context.Context, email string) (*User, error) {
	const q = `
		SELECT id, org_id, email, display_name, provider, provider_id,
		       role, password_hash, is_active, created_at
		FROM users
		WHERE email = $1 AND provider = 'local'
	`
	rows, err := db.pool.Query(ctx, q, email)
	if err != nil {
		return nil, fmt.Errorf("db.GetLocalUserByEmail: query: %w", err)
	}
	user, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[User])
	if err != nil {
		return nil, fmt.Errorf("db.GetLocalUserByEmail: %w", err)
	}
	return user, nil
}

// CreateLocalUser inserts a new local-auth user with a pre-hashed password.
// The hash must be a bcrypt hash — the plaintext must never reach this function.
func (db *DB) CreateLocalUser(ctx context.Context, orgID uuid.UUID, email, displayName, role, passwordHash string) (*User, error) {
	const q = `
		INSERT INTO users (org_id, email, display_name, provider, provider_id, role, password_hash)
		VALUES ($1, $2, $3, 'local', $2, $4, $5)
		ON CONFLICT (provider, provider_id) DO NOTHING
		RETURNING id, org_id, email, display_name, provider, provider_id,
		          role, password_hash, is_active, created_at
	`
	rows, err := db.pool.Query(ctx, q, orgID, email, displayName, role, passwordHash)
	if err != nil {
		return nil, fmt.Errorf("db.CreateLocalUser: query: %w", err)
	}
	user, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("db.CreateLocalUser: email already registered")
		}
		return nil, fmt.Errorf("db.CreateLocalUser: scan: %w", err)
	}
	return user, nil
}

// CountUsers returns the number of users in the org.
// Used by the local-auth bootstrap check: if count == 0, first register = admin.
func (db *DB) CountUsers(ctx context.Context, orgID uuid.UUID) (int64, error) {
	var n int64
	err := db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE org_id = $1`, orgID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("db.CountUsers: %w", err)
	}
	return n, nil
}

// SetUserPasswordHash updates the bcrypt password hash for a local user.
// Used by the admin "reset password" endpoint.
func (db *DB) SetUserPasswordHash(ctx context.Context, userID uuid.UUID, hash string) error {
	tag, err := db.pool.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2 AND provider = 'local'`,
		hash, userID,
	)
	if err != nil {
		return fmt.Errorf("db.SetUserPasswordHash: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("db.SetUserPasswordHash: user not found or not a local account")
	}
	return nil
}

// DeactivateUser sets is_active = false for the given user.
// The user is blocked from logging in but their audit history is preserved.
func (db *DB) DeactivateUser(ctx context.Context, userID uuid.UUID) error {
	tag, err := db.pool.Exec(ctx,
		`UPDATE users SET is_active = FALSE WHERE id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("db.DeactivateUser: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("db.DeactivateUser: user not found")
	}
	return nil
}

// ListUsers returns all users in the given org, ordered by created_at desc.
// Used by the admin user management page.
func (db *DB) ListUsers(ctx context.Context, orgID uuid.UUID) ([]User, error) {
	const q = `
		SELECT id, org_id, email, display_name, provider, provider_id,
		       role, password_hash, is_active, created_at
		FROM users
		WHERE org_id = $1
		ORDER BY created_at DESC
	`
	rows, err := db.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("db.ListUsers: query: %w", err)
	}
	users, err := pgx.CollectRows(rows, pgx.RowToStructByName[User])
	if err != nil {
		return nil, fmt.Errorf("db.ListUsers: scan: %w", err)
	}
	return users, nil
}

// UpdateUserRole sets the RBAC role for a user.
// Admin only — used by the user management API.
func (db *DB) UpdateUserRole(ctx context.Context, userID uuid.UUID, role string) error {
	tag, err := db.pool.Exec(ctx,
		`UPDATE users SET role = $1 WHERE id = $2`,
		role, userID,
	)
	if err != nil {
		return fmt.Errorf("db.UpdateUserRole: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("db.UpdateUserRole: user not found")
	}
	return nil
}
