// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// users.go contains query helpers for the organizations and users tables.
//
// UpsertUser is the most important function here — it is called on every
// successful login to create a new user record or update the display name
// and email of a returning user. The (provider, provider_id) pair is the
// stable unique key from the identity provider (Keycloak sub / GitLab ID).

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
	// Try to fetch the org first — the common path on every login after bootstrap.
	org, err := db.GetOrgBySlug(ctx, slug)
	if err == nil {
		return org, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("db.EnsureOrg: lookup: %w", err)
	}

	// Org does not exist — insert it.
	// ON CONFLICT DO NOTHING handles the race where two goroutines both try
	// to create the same org simultaneously (only one row will be inserted).
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
		// If the INSERT was a no-op (conflict), fall back to a SELECT.
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

// UpsertUser creates a new user record or updates the display name and email
// of a returning user. The (provider, provider_id) pair is the stable key
// from the identity provider — it never changes even if the user renames
// their account in Keycloak or GitLab.
func (db *DB) UpsertUser(ctx context.Context, u User) (*User, error) {
	const q = `
		INSERT INTO users (org_id, email, display_name, provider, provider_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (provider, provider_id) DO UPDATE
			SET email        = EXCLUDED.email,
			    display_name = EXCLUDED.display_name
		RETURNING id, org_id, email, display_name, provider, provider_id, created_at
	`
	rows, err := db.pool.Query(ctx, q,
		u.OrgID, u.Email, u.DisplayName, u.Provider, u.ProviderID,
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
// Returns pgx.ErrNoRows if no user has that ID.
func (db *DB) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	const q = `
		SELECT id, org_id, email, display_name, provider, provider_id, created_at
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
// Used during auth callback to find an existing user before calling UpsertUser.
// Returns pgx.ErrNoRows if the user has never logged in before.
func (db *DB) GetUserByProvider(ctx context.Context, provider, providerID string) (*User, error) {
	const q = `
		SELECT id, org_id, email, display_name, provider, provider_id, created_at
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
