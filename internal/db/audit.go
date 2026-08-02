// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// audit.go handles the append-only audit event log.
//
// Every significant action in devportal writes a row here so admins can
// answer "who did what and when" without grepping application logs.
// Audit rows are NEVER updated or deleted — insert-only by design.
//
// The handlers call InsertAuditEvent as a fire-and-forget background write
// using a separate context so a slow DB write does not block the HTTP response.

package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// InsertAuditEvent writes one immutable audit record.
// detail can be any JSON-serializable value (struct, map, nil).
func (db *DB) InsertAuditEvent(ctx context.Context, e AuditEvent) error {
	const q = `
		INSERT INTO audit_events (org_id, user_id, action, resource_type, resource_id, detail)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := db.pool.Exec(ctx, q,
		e.OrgID, e.UserID, e.Action, e.ResourceType, e.ResourceID, e.Detail,
	)
	if err != nil {
		return fmt.Errorf("db.InsertAuditEvent: %w", err)
	}
	return nil
}

// AuditEventDetail is a helper that serialises any value to JSON bytes
// so callers do not need to import encoding/json when building audit events.
func AuditEventDetail(v any) []byte {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v) // marshal errors are impossible for well-formed Go values
	return b
}

// ListAuditEvents returns the most recent audit events for an org,
// newest first. Used by the admin audit log UI (Day 15).
func (db *DB) ListAuditEvents(ctx context.Context, orgID uuid.UUID, limit int) ([]AuditEvent, error) {
	const q = `
		SELECT id, org_id, user_id, action, resource_type, resource_id, detail, created_at
		FROM audit_events
		WHERE org_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := db.pool.Query(ctx, q, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("db.ListAuditEvents: query: %w", err)
	}
	events, err := pgx.CollectRows(rows, pgx.RowToStructByName[AuditEvent])
	if err != nil {
		return nil, fmt.Errorf("db.ListAuditEvents: scan: %w", err)
	}
	return events, nil
}
