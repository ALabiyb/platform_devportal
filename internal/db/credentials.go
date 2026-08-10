// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// credentials.go — query helpers for the workspace_credentials table.
//
// Credentials are stored with AES-256-GCM encrypted values; this layer
// never decrypts them — encryption/decryption is handled in the HTTP handler.

package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ListCredentials returns all credentials for the org, newest first.
// The encrypted_value blob is NOT included — callers receive only metadata.
func (db *DB) ListCredentials(ctx context.Context, orgID uuid.UUID) ([]WorkspaceCredential, error) {
	const q = `
		SELECT id, org_id, provider_type, label, encrypted_value, created_at
		FROM workspace_credentials
		WHERE org_id = $1
		ORDER BY created_at DESC
	`
	rows, err := db.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("db.ListCredentials: query: %w", err)
	}
	creds, err := pgx.CollectRows(rows, pgx.RowToStructByName[WorkspaceCredential])
	if err != nil {
		return nil, fmt.Errorf("db.ListCredentials: scan: %w", err)
	}
	return creds, nil
}

// CreateCredential inserts a new encrypted credential and returns the saved row.
// The EncryptedValue field must already be the AES-256-GCM ciphertext.
func (db *DB) CreateCredential(ctx context.Context, c WorkspaceCredential) (*WorkspaceCredential, error) {
	const q = `
		INSERT INTO workspace_credentials (org_id, provider_type, label, encrypted_value)
		VALUES ($1, $2, $3, $4)
		RETURNING id, org_id, provider_type, label, encrypted_value, created_at
	`
	rows, err := db.pool.Query(ctx, q, c.OrgID, c.ProviderType, c.Label, c.EncryptedValue)
	if err != nil {
		return nil, fmt.Errorf("db.CreateCredential: query: %w", err)
	}
	cred, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[WorkspaceCredential])
	if err != nil {
		return nil, fmt.Errorf("db.CreateCredential: scan: %w", err)
	}
	return cred, nil
}

// DeleteCredential removes a credential by ID, scoped to the org so one org
// cannot delete another org's credentials.
// Returns pgx.ErrNoRows if the credential does not exist or belongs to a different org.
func (db *DB) DeleteCredential(ctx context.Context, id, orgID uuid.UUID) error {
	tag, err := db.pool.Exec(ctx,
		`DELETE FROM workspace_credentials WHERE id = $1 AND org_id = $2`,
		id, orgID,
	)
	if err != nil {
		return fmt.Errorf("db.DeleteCredential: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("db.DeleteCredential: %w", pgx.ErrNoRows)
	}
	return nil
}
