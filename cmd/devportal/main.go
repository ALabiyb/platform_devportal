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
//  4. Auth provider initialised (OIDC discovery or GitLab OAuth endpoint)
//  5. Session cleanup goroutine started (deletes expired sessions every hour)
//  6. HTTP server started with graceful shutdown on SIGINT / SIGTERM
package main

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	authpkg "github.com/ALabiyb/platform_devportal/internal/auth"
	"github.com/ALabiyb/platform_devportal/internal/config"
	"github.com/ALabiyb/platform_devportal/internal/db"
	"github.com/ALabiyb/platform_devportal/internal/handler"
	"github.com/ALabiyb/platform_devportal/internal/plugin"
	"github.com/ALabiyb/platform_devportal/internal/provisioner"
	tmpl "github.com/ALabiyb/platform_devportal/internal/template"
)

// version is set at build time via -ldflags="-X main.version=<git-tag>".
// Falls back to "dev" when built without the flag (e.g. go run).
var version = "dev"

func main() {
	// JSON structured logging so Loki and Grafana can index log fields
	// without any parsing configuration.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	slog.Info("devportal starting", "version", version)

	// Load all configuration from environment variables.
	// mustEnv() inside Load() calls os.Exit(1) for any missing required var.
	cfg := config.Load()

	// Open the PostgreSQL connection pool.
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel()

	database, err := db.Open(dbCtx, cfg)
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	slog.Info("database connected", "host", cfg.DBHost, "name", cfg.DBName)

	// Shared HTTP client used for all outbound calls (Keycloak, GitLab, Jenkins, etc.).
	// TLS_SKIP_VERIFY=true allows self-signed certs in local dev — set false in prod.
	httpClient := newHTTPClient(cfg.TLSSkipVerify)

	// Initialise the auth handler based on AUTH_MODE.
	// OIDC: performs a discovery request to Keycloak at startup — fails fast.
	// Local: no network call; user accounts are stored in devportal's DB.
	authHandler, err := buildAuthHandler(cfg, httpClient, database)
	if err != nil {
		slog.Error("failed to initialise auth", "mode", cfg.AuthMode, "err", err)
		os.Exit(1)
	}
	slog.Info("auth ready", "mode", cfg.AuthMode)

	// Construct all plugin adapters.
	gitAdapter := plugin.NewGitLabAdapter(cfg, httpClient)
	ciAdapter := plugin.NewJenkinsAdapter(cfg, httpClient)
	registryAdapter := plugin.NewHarborAdapter(cfg, httpClient)
	securityAdapter := plugin.NewDefectDojoAdapter(cfg, httpClient)
	gitopsAdapter := plugin.NewArgoCDAdapter(cfg, httpClient)
	dbAdapter := plugin.NewPostgresDBAdapter(cfg)
	slog.Info("plugin adapters ready")

	// Template generator bakes config values into Jenkinsfiles and K8s manifests.
	templates := tmpl.New(cfg)

	// SSE hub and provisioning orchestrator.
	hub := provisioner.NewHub()
	orch := provisioner.New(
		database, hub,
		gitAdapter, ciAdapter, registryAdapter,
		securityAdapter, gitopsAdapter, dbAdapter,
		cfg, templates,
	)
	slog.Info("provisioner ready")

	// Session cleanup goroutine — deletes expired rows from sessions table once per hour.
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	go runSessionCleanup(cleanupCtx, database)

	// Wire up the full chi router with all middleware and routes.
	h := handler.New(database, cfg, authHandler, orch, hub, version)

	// HTTP server with explicit timeouts.
	// WriteTimeout is omitted — SSE streams need unbounded write time.
	srv := &http.Server{
		Addr:        cfg.HTTPAddr,
		Handler:     h.Routes(),
		ReadTimeout: 15 * time.Second,
		IdleTimeout: 120 * time.Second,
	}

	go func() {
		slog.Info("HTTP server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Block until SIGINT (Ctrl+C) or SIGTERM (kubectl delete pod).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutdown signal received", "signal", sig.String())

	cleanupCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown after timeout", "err", err)
	}

	database.Close()
	slog.Info("devportal stopped cleanly")
}

// buildAuthHandler initialises the correct auth handler based on AUTH_MODE.
// local: no network call — user accounts live in devportal's own DB.
// oidc:  performs OIDC discovery against Keycloak at startup (fails fast on error).
func buildAuthHandler(cfg *config.Config, httpClient *http.Client, database *db.DB) (*authpkg.Handler, error) {
	switch cfg.AuthMode {
	case "local":
		local := authpkg.NewLocalProvider(database, cfg)
		return authpkg.NewHandler(nil, local, database, cfg), nil
	case "oidc":
		provider, err := authpkg.NewOIDCProvider(context.Background(), cfg, httpClient)
		if err != nil {
			return nil, err
		}
		return authpkg.NewHandler(provider, nil, database, cfg), nil
	default:
		slog.Error("unknown AUTH_MODE — must be 'local' or 'oidc'", "mode", cfg.AuthMode)
		os.Exit(1)
		return nil, nil // unreachable
	}
}

// newHTTPClient returns an *http.Client with a 30-second timeout.
// When skipVerify is true, TLS certificate validation is disabled for
// outbound calls — needed when Jenkins/Harbor/ArgoCD use self-signed certs.
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

// runSessionCleanup deletes expired session rows from PostgreSQL once per hour.
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
