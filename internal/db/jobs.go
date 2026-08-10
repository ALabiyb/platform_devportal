// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// jobs.go — provisioning_jobs table — the Postgres-backed job queue.
//
// Pattern: SELECT ... FOR UPDATE SKIP LOCKED
// When multiple worker goroutines (or worker pods) call ClaimNextJob concurrently,
// each one issues a SELECT FOR UPDATE SKIP LOCKED. Postgres locks the first
// matching row for one session and skips it in all others — no application-level
// locking or external queue needed.

package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// JobPayload carries the context the orchestrator needs that is not stored in
// the projects table itself. Serialised as JSONB in provisioning_jobs.payload.
type JobPayload struct {
	GitNamespace    string `json:"git_namespace"`
	ApplicationSlug string `json:"application_slug"`
}

// ProvisioningJob is a claimed job returned by ClaimNextJob.
// The worker reloads Project and Environments from the DB by ProjectID.
type ProvisioningJob struct {
	ID           uuid.UUID
	ProjectID    uuid.UUID
	Payload      JobPayload
	AttemptCount int
	WorkerID     string
}

// EnqueueProvisioningJob inserts a new pending job for projectID.
// Returns the generated job UUID so callers can log or track it.
func (d *DB) EnqueueProvisioningJob(ctx context.Context, projectID uuid.UUID, payload JobPayload) (uuid.UUID, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, fmt.Errorf("jobs: marshal payload: %w", err)
	}
	var id uuid.UUID
	err = d.pool.QueryRow(ctx, `
		INSERT INTO provisioning_jobs (project_id, payload)
		VALUES ($1, $2)
		RETURNING id`,
		projectID, raw,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("jobs: enqueue: %w", err)
	}
	return id, nil
}

// ClaimNextJob atomically claims the oldest pending job that has not yet
// exhausted its retry budget. Returns pgx.ErrNoRows when the queue is empty.
//
// The CTE uses FOR UPDATE SKIP LOCKED so that concurrent callers each
// claim a different row without blocking each other.
func (d *DB) ClaimNextJob(ctx context.Context, workerID string) (*ProvisioningJob, error) {
	var (
		job     ProvisioningJob
		rawJSON []byte
	)
	err := d.pool.QueryRow(ctx, `
		WITH next AS (
			SELECT id
			FROM   provisioning_jobs
			WHERE  status        = 'pending'
			  AND  attempt_count < max_attempts
			ORDER  BY created_at
			LIMIT  1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE provisioning_jobs j
		SET    status        = 'running',
		       started_at    = now(),
		       attempt_count = attempt_count + 1,
		       worker_id     = $1
		FROM   next
		WHERE  j.id = next.id
		RETURNING j.id, j.project_id, j.payload, j.attempt_count, j.worker_id`,
		workerID,
	).Scan(&job.ID, &job.ProjectID, &rawJSON, &job.AttemptCount, &job.WorkerID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, pgx.ErrNoRows
		}
		return nil, fmt.Errorf("jobs: claim: %w", err)
	}
	if err := json.Unmarshal(rawJSON, &job.Payload); err != nil {
		return nil, fmt.Errorf("jobs: unmarshal payload: %w", err)
	}
	return &job, nil
}

// CompleteJob marks a job as done with the finished timestamp.
func (d *DB) CompleteJob(ctx context.Context, jobID uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE provisioning_jobs
		SET    status      = 'done',
		       finished_at = now(),
		       error_msg   = NULL
		WHERE  id = $1`,
		jobID,
	)
	if err != nil {
		return fmt.Errorf("jobs: complete: %w", err)
	}
	return nil
}

// FailJob records the error message on a job.
// If the job has attempts remaining it is reset to 'pending' so the worker
// can retry. Once max_attempts is reached the job is permanently 'failed'.
func (d *DB) FailJob(ctx context.Context, jobID uuid.UUID, errMsg string) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE provisioning_jobs
		SET    status      = CASE
		                         WHEN attempt_count >= max_attempts THEN 'failed'
		                         ELSE 'pending'
		                     END,
		       finished_at = CASE
		                         WHEN attempt_count >= max_attempts THEN now()
		                         ELSE NULL
		                     END,
		       error_msg   = $2
		WHERE  id = $1`,
		jobID, errMsg,
	)
	if err != nil {
		return fmt.Errorf("jobs: fail: %w", err)
	}
	return nil
}

// RecoverStalledJobs resets any 'running' jobs whose started_at is older than
// stalledAfter back to 'pending'. Called periodically by the worker to handle
// crashes. Returns the number of jobs recovered.
func (d *DB) RecoverStalledJobs(ctx context.Context, stalledAfter time.Duration) (int64, error) {
	cutoff := time.Now().Add(-stalledAfter)
	tag, err := d.pool.Exec(ctx, `
		UPDATE provisioning_jobs
		SET    status     = 'pending',
		       started_at = NULL,
		       worker_id  = NULL
		WHERE  status     = 'running'
		  AND  started_at < $1
		  AND  attempt_count < max_attempts`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("jobs: recover stalled: %w", err)
	}
	return tag.RowsAffected(), nil
}
