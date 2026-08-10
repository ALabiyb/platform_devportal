// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// Package worker polls the provisioning_jobs Postgres queue and drives the
// 15-step provisioner orchestrator for each claimed job.
//
// Design:
//   - A fixed-size semaphore (buffered channel) caps concurrent runs at
//     cfg.WorkerConcurrency (default 3). Each goroutine acquires a slot before
//     running and releases it when done, so the pool never exceeds the limit.
//   - SELECT FOR UPDATE SKIP LOCKED in ClaimNextJob ensures that when multiple
//     goroutines poll simultaneously they each claim a different job row.
//   - RecoverStalledJobs runs every 5 minutes to reset any job that a crashed
//     worker left in 'running' state, so provisioning is self-healing.
//   - The worker runs until ctx is cancelled (SIGINT / SIGTERM). In-flight
//     provisioning runs are given up to 30 seconds to finish before the process
//     exits.
//
// Embedding (default): cmd/devportal starts the worker in the same process
// alongside the HTTP server. The provisioner.Hub is shared in memory, so SSE
// step events reach connected browsers without any inter-process bridge.
//
// Scale-out: run cmd/worker as a separate pod. SSE streaming from a standalone
// worker requires a Postgres LISTEN/NOTIFY bridge (future work).
package worker

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ALabiyb/platform_devportal/internal/db"
	"github.com/ALabiyb/platform_devportal/internal/provisioner"
)

const (
	// pollInterval is how long the worker sleeps between queue checks when idle.
	pollInterval = 2 * time.Second

	// stalledThreshold is how long a job can stay 'running' before it is
	// considered stalled and reset to 'pending' for retry.
	stalledThreshold = 15 * time.Minute

	// stalledCheckInterval is how often RecoverStalledJobs is called.
	stalledCheckInterval = 5 * time.Minute
)

// Worker polls the provisioning_jobs queue and drives the orchestrator.
type Worker struct {
	db          *db.DB
	orch        *provisioner.Orchestrator
	workerID    string
	concurrency int
	sem         chan struct{}
}

// New constructs a Worker. workerID should be unique per process (os.Hostname
// is a good default). concurrency caps the number of simultaneous provisioning
// runs; 3 is a safe default for a modest database.
func New(database *db.DB, orch *provisioner.Orchestrator, workerID string, concurrency int) *Worker {
	if concurrency <= 0 {
		concurrency = 3
	}
	return &Worker{
		db:          database,
		orch:        orch,
		workerID:    workerID,
		concurrency: concurrency,
		sem:         make(chan struct{}, concurrency),
	}
}

// NewWithHostname is a convenience constructor that uses os.Hostname() as the
// worker ID. Falls back to "worker-unknown" if the hostname call fails.
func NewWithHostname(database *db.DB, orch *provisioner.Orchestrator, concurrency int) *Worker {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "worker-unknown"
	}
	return New(database, orch, hostname, concurrency)
}

// Run starts the poll-and-dispatch loop. It blocks until ctx is cancelled.
// Call this in a goroutine alongside the HTTP server, or as the sole entry
// point in cmd/worker.
func (w *Worker) Run(ctx context.Context) {
	slog.Info("provisioning worker started",
		"worker_id", w.workerID,
		"concurrency", w.concurrency,
	)

	// Start a separate goroutine to periodically recover stalled jobs.
	go w.runStalledRecovery(ctx)

	for {
		// Respect shutdown signal.
		select {
		case <-ctx.Done():
			// Drain the semaphore — wait for all in-flight runs to finish.
			for i := 0; i < w.concurrency; i++ {
				w.sem <- struct{}{}
			}
			slog.Info("provisioning worker stopped")
			return
		default:
		}

		// Acquire a concurrency slot before polling. This blocks when all
		// slots are occupied, providing natural backpressure.
		select {
		case <-ctx.Done():
			return
		case w.sem <- struct{}{}:
		}

		job, err := w.db.ClaimNextJob(ctx, w.workerID)
		if err != nil {
			// Release the slot immediately — no job to run.
			<-w.sem
			if err == pgx.ErrNoRows {
				// Queue empty — sleep before polling again.
				select {
				case <-ctx.Done():
					return
				case <-time.After(pollInterval):
				}
				continue
			}
			slog.Error("worker: claim job failed", "err", err)
			time.Sleep(pollInterval)
			continue
		}

		slog.Info("worker: claimed job",
			"job_id", job.ID,
			"project_id", job.ProjectID,
			"attempt", job.AttemptCount,
			"worker", job.WorkerID,
		)

		// Dispatch in a goroutine so the poll loop can claim the next job
		// immediately (up to w.concurrency parallel runs).
		go func(j *db.ProvisioningJob) {
			defer func() { <-w.sem }() // always release the slot

			w.dispatch(j)
		}(job)
	}
}

// dispatch reloads the project and environments from the DB, then runs the
// orchestrator. On completion it marks the job done or failed.
func (w *Worker) dispatch(job *db.ProvisioningJob) {
	// Use a background context so provisioning is not cancelled if the HTTP
	// request that enqueued the job has already finished.
	ctx := context.Background()

	project, err := w.db.GetProject(ctx, job.ProjectID)
	if err != nil {
		slog.Error("worker: reload project failed",
			"job_id", job.ID, "project_id", job.ProjectID, "err", err)
		_ = w.db.FailJob(ctx, job.ID, "reload project: "+err.Error())
		return
	}

	envRows, err := w.db.GetEnvironmentsByProject(ctx, job.ProjectID)
	if err != nil {
		slog.Error("worker: reload environments failed",
			"job_id", job.ID, "project_id", job.ProjectID, "err", err)
		_ = w.db.FailJob(ctx, job.ID, "reload environments: "+err.Error())
		return
	}

	envs := make([]*db.Environment, len(envRows))
	for i := range envRows {
		envs[i] = &envRows[i]
	}

	input := provisioner.ProvisionInput{
		Project:         project,
		Environments:    envs,
		GitNamespace:    job.Payload.GitNamespace,
		ApplicationSlug: job.Payload.ApplicationSlug,
	}

	// Provision is a blocking call that updates step rows and broadcasts SSE
	// events. It manages its own error handling and project status internally.
	provErr := w.orch.Provision(ctx, input)
	if provErr != nil {
		slog.Error("worker: provisioning failed",
			"job_id", job.ID, "project_id", job.ProjectID, "err", provErr)
		_ = w.db.FailJob(ctx, job.ID, provErr.Error())
		return
	}

	if err := w.db.CompleteJob(ctx, job.ID); err != nil {
		slog.Error("worker: complete job failed", "job_id", job.ID, "err", err)
	}
	slog.Info("worker: job complete", "job_id", job.ID, "project_id", job.ProjectID)
}

// runStalledRecovery periodically resets jobs that a crashed worker left in
// the 'running' state so they can be retried.
func (w *Worker) runStalledRecovery(ctx context.Context) {
	ticker := time.NewTicker(stalledCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := w.db.RecoverStalledJobs(ctx, stalledThreshold)
			if err != nil {
				slog.Error("worker: stalled job recovery failed", "err", err)
			} else if n > 0 {
				slog.Info("worker: stalled jobs recovered", "count", n)
			}
		}
	}
}
