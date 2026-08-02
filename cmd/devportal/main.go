// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// devportal is an Internal Developer Platform (IDP) for SoftNet.
//
// A developer fills one form and gets a complete, production-ready project
// provisioned end-to-end — automatically and in real time:
//
//	1.  GitLab repo wiring (commits Jenkinsfile, VERSION, Dockerfile)
//	2.  Jenkins multibranch pipeline job creation + webhook
//	3.  Harbor image registry project
//	4.  DefectDojo security product + engagement
//	5.  K8s manifests generated and committed to the manifest repo
//	6.  ArgoCD Application created per environment (dev / uat / prod)
//	7.  PostgreSQL database provisioned per environment
//	8.  SSE broadcast stream so every watcher sees live step progress
//
// This file is the entry point. It:
//   - Sets up structured JSON logging (Loki/Grafana-compatible)
//   - Loads all config from environment variables (fails fast if secrets missing)
//   - Starts the HTTP server with graceful shutdown on SIGINT / SIGTERM
//
// The router, middleware, and all route handlers are wired in Day 04.
// The DB connection pool is opened in Day 02.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// version is set at build time via -ldflags="-X main.version=<git-tag>".
// Falls back to "dev" when built without the flag (e.g. go run).
var version = "dev"

func main() {
	// JSON structured logging so Loki and Grafana can index log fields
	// (level, msg, err, etc.) without any parsing configuration.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	slog.Info("devportal starting", "version", version)

	// Load all configuration from environment variables.
	// mustEnv() inside Load() calls os.Exit(1) for any missing required var,
	// so this is the only place startup can fail before the server starts.
	cfg := config.Load()

	// Build the HTTP handler.
	// Day 04 replaces this minimal mux with the full chi router, RBAC
	// middleware, rate limiter, structured request logger, and all routes.
	mux := buildRouter(cfg)

	// HTTP server with explicit timeouts to prevent slow-client resource exhaustion.
	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: mux,

		// ReadTimeout: maximum time allowed to read the entire incoming request,
		// including body. Prevents a slow sender from tying up a goroutine forever.
		ReadTimeout: 15 * time.Second,

		// WriteTimeout: maximum time to write the entire response.
		// Set to 60s to accommodate SSE provisioning streams, which keep the
		// connection open while broadcasting step-by-step progress to the browser.
		WriteTimeout: 60 * time.Second,

		// IdleTimeout: maximum time an idle keep-alive connection is held open.
		// After this period the connection is closed, freeing the file descriptor.
		IdleTimeout: 120 * time.Second,
	}

	// Run the server in a goroutine so the main goroutine can block on OS signals.
	go func() {
		slog.Info("HTTP server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Block until the OS delivers SIGINT (Ctrl+C) or SIGTERM (kubectl delete pod / docker stop).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutdown signal received — draining in-flight requests", "signal", sig.String())

	// Allow up to 30 seconds for in-flight requests to finish before forcing close.
	// SSE clients will see the connection drop and the frontend reconnects automatically.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown after timeout", "err", err)
		os.Exit(1)
	}

	slog.Info("devportal stopped cleanly")
}

// buildRouter wires together all HTTP routes and middleware.
// Day 04 expands this into the full chi router with RBAC, rate limiting,
// and structured request logging. For now it registers the health endpoint only.
func buildRouter(cfg *config.Config) http.Handler {
	mux := http.NewServeMux()

	// /healthz — always public, never gated by auth.
	// Traefik uses this as a liveness probe; Gatus uses it for uptime monitoring.
	mux.HandleFunc("/healthz", handleHealthz)

	// Suppress the unused-variable warning for cfg until Day 04 uses it.
	_ = fmt.Sprintf("%s", cfg.HTTPAddr)

	return mux
}

// handleHealthz responds with a JSON status payload.
// It intentionally does NOT check downstream services (DB, GitLab, Jenkins)
// so that a probe never fails due to a dependency outage — those checks
// belong in a separate /readyz endpoint added on Day 04.
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","service":"devportal","version":%q}`, version)
}
