// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// devportal is an Internal Developer Platform (IDP) built by NexBridge Technologies.
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
// Startup sequence:
//  1. JSON structured logging (Loki/Grafana-compatible)
//  2. Config loaded from environment variables (fails fast on missing secrets)
//  3. PostgreSQL connection pool opened and verified with ping
//  4. Session cleanup goroutine started (deletes expired sessions every hour)
//  5. HTTP server started with graceful shutdown on SIGINT / SIGTERM
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
	"github.com/ALabiyb/platform_devportal/internal/db"
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

	// Open the PostgreSQL connection pool.
	// The context here is just for the initial dial — the pool itself lives
	// for the process lifetime and is closed during graceful shutdown.
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel()

	database, err := db.Open(dbCtx, cfg)
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	slog.Info("database connected", "host", cfg.DBHost, "name", cfg.DBName)

	// Start the session cleanup goroutine.
	// Runs every hour and deletes expired session rows so the sessions table
	// does not grow unbounded. Stops when cleanupCancel() is called below.
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	go runSessionCleanup(cleanupCtx, database)

	// Build the HTTP handler.
	// Day 04 replaces this minimal mux with the full chi router, RBAC
	// middleware, rate limiter, structured request logger, and all routes.
	mux := buildRouter(cfg, database)

	// HTTP server with explicit timeouts to prevent slow-client resource exhaustion.
	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: mux,

		// ReadTimeout: maximum time to read the entire incoming request body.
		ReadTimeout: 15 * time.Second,

		// WriteTimeout: 60s to accommodate SSE provisioning streams, which keep
		// the connection open while broadcasting step-by-step progress to the browser.
		WriteTimeout: 60 * time.Second,

		// IdleTimeout: maximum time an idle keep-alive connection is held open.
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

	// Block until the OS delivers SIGINT (Ctrl+C) or SIGTERM (kubectl delete pod).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutdown signal received — draining in-flight requests", "signal", sig.String())

	// Stop the session cleanup goroutine before shutting down the HTTP server.
	cleanupCancel()

	// Allow up to 30 seconds for in-flight requests to finish before forcing close.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown after timeout", "err", err)
	}

	// Close the DB pool after the HTTP server has drained so no in-flight
	// handler loses its DB connection mid-query.
	database.Close()
	slog.Info("devportal stopped cleanly")
}

// buildRouter wires together all HTTP routes and middleware.
// Day 04 expands this into the full chi router with RBAC, rate limiting,
// and structured request logging. For now it registers the health endpoint only.
func buildRouter(cfg *config.Config, database *db.DB) http.Handler {
	mux := http.NewServeMux()

	// /healthz — always public, never gated by auth.
	// Traefik uses this as a liveness probe; Gatus for uptime monitoring.
	mux.HandleFunc("/healthz", handleHealthz(database))

	_ = cfg // used fully in Day 04
	return mux
}

// handleHealthz responds with a JSON status payload including a DB ping.
// Returns 503 if the database is unreachable so load balancers can route away.
func handleHealthz(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		if err := database.Ping(r.Context()); err != nil {
			dbStatus = "unreachable"
		}
		w.Header().Set("Content-Type", "application/json")
		if dbStatus != "ok" {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		fmt.Fprintf(w, `{"status":%q,"db":%q,"service":"devportal","version":%q}`,
			dbStatus, dbStatus, version,
		)
	}
}

// runSessionCleanup deletes expired session rows from PostgreSQL once per hour.
// It runs for the lifetime of the process and exits when ctx is cancelled.
func runSessionCleanup(ctx context.Context, database *db.DB) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := database.DeleteExpiredSessions(ctx)
			if err != nil {
				slog.Error("session cleanup failed", "err", err)
			} else if n > 0 {
				slog.Info("expired sessions deleted", "count", n)
			}
		}
	}
}
