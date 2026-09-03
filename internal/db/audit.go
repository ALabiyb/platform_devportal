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

// AuditEventWithActor extends AuditEvent with the actor's email for display.
type AuditEventWithActor struct {
	AuditEvent
	ActorEmail *string `json:"actor_email,omitempty"`
}

// ListAuditEvents returns the most recent audit events for an org,
// newest first, with actor email joined from users.
func (db *DB) ListAuditEvents(ctx context.Context, orgID uuid.UUID, limit int) ([]AuditEventWithActor, error) {
	const q = `
		SELECT ae.id, ae.org_id, ae.user_id, ae.action, ae.resource_type,
		       ae.resource_id, ae.detail, ae.created_at,
		       u.email AS actor_email
		FROM   audit_events ae
		LEFT   JOIN users u ON ae.user_id = u.id
		WHERE  ae.org_id = $1
		ORDER  BY ae.created_at DESC
		LIMIT  $2
	`
	rows, err := db.pool.Query(ctx, q, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("db.ListAuditEvents: query: %w", err)
	}
	defer rows.Close()
	var events []AuditEventWithActor
	for rows.Next() {
		var e AuditEventWithActor
		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.UserID, &e.Action, &e.ResourceType,
			&e.ResourceID, &e.Detail, &e.CreatedAt, &e.ActorEmail,
		); err != nil {
			return nil, fmt.Errorf("db.ListAuditEvents: scan: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
