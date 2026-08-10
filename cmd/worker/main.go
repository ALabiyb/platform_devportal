// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// worker is the standalone provisioning worker binary for DevPortal.
//
// Run this as a separate Kubernetes Deployment (scaled independently from the
// API server) when provisioning load requires horizontal scaling.
//
// Default mode (development): the worker is embedded inside cmd/devportal and
// does not need to run separately. See internal/worker/worker.go for the
// design rationale.
//
// Startup sequence:
//  1. JSON structured logging
//  2. Config loaded from environment variables
//  3. PostgreSQL connection pool opened and verified
//  4. Plugin adapters constructed (same as the API server)
//  5. Provisioner orchestrator constructed
//  6. Worker pool started — polls provisioning_jobs until SIGTERM
package main

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ALabiyb/platform_devportal/internal/config"
	"github.com/ALabiyb/platform_devportal/internal/db"
	"github.com/ALabiyb/platform_devportal/internal/plugin"
	"github.com/ALabiyb/platform_devportal/internal/provisioner"
	tmpl "github.com/ALabiyb/platform_devportal/internal/template"
	"github.com/ALabiyb/platform_devportal/internal/worker"
)

var version = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	slog.Info("devportal-worker starting", "version", version)

	cfg := config.Load()

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel()

	database, err := db.Open(dbCtx, cfg)
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	slog.Info("database connected", "host", cfg.DBHost, "name", cfg.DBName)

	httpClient := newHTTPClient(cfg.TLSSkipVerify)

	gitAdapter := buildGitAdapter(cfg, httpClient)
	ciAdapter := plugin.NewJenkinsAdapter(cfg, httpClient)
	registryAdapter := plugin.NewHarborAdapter(cfg, httpClient)
	securityAdapter := plugin.NewDefectDojoAdapter(cfg, httpClient)
	gitopsAdapter := plugin.NewArgoCDAdapter(cfg, httpClient)
	dbAdapter := plugin.NewPostgresDBAdapter(cfg)

	templates := tmpl.New(cfg)

	// Standalone worker uses its own in-memory hub. SSE streaming is only
	// available when the worker is embedded in the API server process (the
	// hub is shared in-memory). In standalone mode, step progress is still
	// written to the DB and can be polled via GET /api/v1/projects/{id}/steps.
	hub := provisioner.NewHub()

	orch := provisioner.New(
		database, hub,
		gitAdapter, ciAdapter, registryAdapter,
		securityAdapter, gitopsAdapter, dbAdapter,
		cfg, templates,
	)

	w := worker.NewWithHostname(database, orch, cfg.WorkerConcurrency)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// RecoverStalledJobs once at startup in case the previous worker pod crashed.
	n, err := database.RecoverStalledJobs(ctx, 15*time.Minute)
	if err != nil {
		slog.Warn("startup stalled job recovery failed", "err", err)
	} else if n > 0 {
		slog.Info("recovered stalled jobs at startup", "count", n)
	}

	// Run blocks until ctx is cancelled (SIGINT / SIGTERM).
	w.Run(ctx)

	database.Close()
	slog.Info("devportal-worker stopped cleanly")
}

func buildGitAdapter(cfg *config.Config, httpClient *http.Client) plugin.GitProvider {
	switch strings.ToLower(cfg.GitProvider) {
	case "gitlab":
		slog.Info("git provider: gitlab", "url", cfg.GitLabURL)
		return plugin.NewGitLabAdapter(cfg, httpClient)
	default:
		slog.Info("git provider: gitea", "url", cfg.GiteaURL)
		return plugin.NewGiteaAdapter(cfg, httpClient)
	}
}

func newHTTPClient(skipVerify bool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if skipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}
}
