// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// Package handler wires all HTTP routes, middleware, and handler stubs.
//
// Route layout:
//
//	GET  /healthz                              — liveness + DB ping (public)
//	GET  /auth/login                           — redirect to IdP (public)
//	GET  /auth/callback                        — OAuth2 code exchange (public)
//	POST /auth/logout                          — end session (authenticated)
//	GET  /auth/me                              — current user info (authenticated)
//
//	GET  /api/v1/projects                      — list projects          (viewer+)
//	GET  /api/v1/projects/{projectID}          — get project            (viewer+)
//	POST /api/v1/projects                      — create project         (developer+)
//	GET  /api/v1/projects/{projectID}/environments — list envs          (viewer+)
//	GET  /api/v1/projects/{projectID}/steps    — provisioning steps     (viewer+)
//	GET  /api/v1/projects/{projectID}/stream   — SSE live progress      (viewer+)
//	GET  /api/v1/projects/{projectID}/jenkinsfile — download Jenkinsfile (viewer+)
//
//	GET  /api/v1/teams                         — list teams             (viewer+)
//	GET  /api/v1/teams/{teamID}                — get team               (viewer+)
//	POST /api/v1/teams                         — create team            (admin)
//
//	GET    /api/v1/credentials                 — list credentials       (admin)
//	POST   /api/v1/credentials                 — create credential      (admin)
//	DELETE /api/v1/credentials/{credentialID}  — delete credential      (admin)
//
//	GET  /api/v1/audit                         — audit event log        (admin)

package handler

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/time/rate"

	authpkg "github.com/ALabiyb/platform_devportal/internal/auth"
	"github.com/ALabiyb/platform_devportal/internal/config"
	"github.com/ALabiyb/platform_devportal/internal/db"
	"github.com/ALabiyb/platform_devportal/internal/membersync"
	"github.com/ALabiyb/platform_devportal/internal/plugin"
	"github.com/ALabiyb/platform_devportal/internal/provisioner"
)

// Handler holds every dependency shared across all HTTP handlers.
// Create one with New() and keep it for the process lifetime.
//
// The provisioner orchestrator is intentionally absent here. HTTP handlers
// enqueue a provisioning_jobs row via db.EnqueueProvisioningJob instead of
// calling the orchestrator directly. The embedded worker (or a standalone
// cmd/worker pod) claims and executes jobs asynchronously.
type Handler struct {
	db      *db.DB
	cfg     *config.Config
	auth    *authpkg.Handler
	git     plugin.GitProvider
	hub     *provisioner.Hub
	webFS   fs.FS
	version string
	syncer  *membersync.Service // nil when no platform tools configured

	// per-IP rate limiting
	mu       sync.Mutex
	visitors map[string]*visitor
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// New constructs a Handler and starts the rate-limiter cleanup goroutine.
func New(database *db.DB, cfg *config.Config, authHandler *authpkg.Handler, git plugin.GitProvider, hub *provisioner.Hub, webFS fs.FS, version string, syncer *membersync.Service) *Handler {
	h := &Handler{
		db:       database,
		cfg:      cfg,
		auth:     authHandler,
		git:      git,
		hub:      hub,
		webFS:    webFS,
		version:  version,
		syncer:   syncer,
		visitors: make(map[string]*visitor),
	}
	go h.cleanupVisitors()
	return h
}

// Routes builds the full chi router with all middleware and routes registered.
// Call this once in main() and pass the result to http.Server.Handler.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()

	// ── Global middleware (applied to every request) ───────────────────────
	r.Use(middleware.RealIP)    // trust X-Forwarded-For set by Traefik
	r.Use(middleware.RequestID) // inject X-Request-ID header
	r.Use(h.requestLogger)      // slog structured log line per request
	r.Use(h.rateLimiter)        // per-IP: 30 req/s, burst 60
	r.Use(middleware.Recoverer) // catch panics, return 500 instead of crashing

	// ── Public routes — no authentication required ─────────────────────────
	r.Get("/healthz", h.handleHealthz)
	r.Get("/branding.json", h.handleBranding)
	r.Get("/setup/status", h.handleSetupStatus)

	switch h.cfg.AuthMode {
	case "local":
		// Local mode: form-based login, no external redirect.
		r.Post("/auth/login", h.auth.HandleLocalLogin)
		r.Post("/auth/register", h.auth.HandleLocalRegister) // bootstrap first admin
	default: // "oidc"
		r.Get("/auth/login", h.auth.HandleLogin)
		r.Get("/auth/callback", h.auth.HandleCallback)
	}

	// ── Authenticated routes — valid session cookie required ───────────────
	r.Group(func(r chi.Router) {
		r.Use(authpkg.RequireAuth(h.db))

		r.Post("/auth/logout", h.auth.HandleLogout)
		r.Get("/auth/me", h.auth.HandleMe)

		// ── API v1 ─────────────────────────────────────────────────────────
		r.Route("/api/v1", func(r chi.Router) {

			// dashboard summary
			r.Get("/dashboard", h.GetDashboard)

			// viewer+ — any authenticated user
			r.Get("/projects", h.ListProjects)
			r.Get("/projects/{projectID}", h.GetProject)
			r.Get("/projects/{projectID}/environments", h.ListEnvironments)
			r.Get("/projects/{projectID}/steps", h.ListProvisioningSteps)
			r.Get("/projects/{projectID}/stream", h.StreamProvisioningProgress)
			r.Get("/projects/{projectID}/jenkinsfile", h.DownloadJenkinsfile)
			r.Get("/teams", h.ListTeams)
			r.Get("/teams/{teamID}", h.GetTeam)
			r.Get("/teams/{teamID}/members", h.ListTeamMembers)

			// applications — member-scoped read; leads/admins can update/delete
			r.Put("/applications/{appID}", h.UpdateApplication)
			r.Delete("/applications/{appID}", h.DeleteApplication)

			// developer+ — project creation
			r.Group(func(r chi.Router) {
				r.Use(authpkg.RequireRole(authpkg.RoleDeveloper))
				r.Post("/projects", h.CreateProject)
			})

			// applications — member-scoped access
			r.Get("/applications", h.ListApplications)
			r.Post("/applications", h.CreateApplication)
			r.Get("/applications/{appID}", h.GetApplication)
			r.Get("/applications/{appID}/members", h.ListApplicationMembers)
			r.Get("/applications/{appID}/services", h.ListServices)
			r.Post("/applications/{appID}/services", h.CreateService)
			r.Patch("/applications/{appID}/services/{serviceID}", h.RenameService)
			r.Put("/applications/{appID}/services/{serviceID}", h.UpdateService)
			r.Post("/applications/{appID}/services/{serviceID}/reprovision", h.ReprovisionService)
			r.Delete("/applications/{appID}/services/{serviceID}", h.DeleteService)

			// application lead or admin only
			r.Group(func(r chi.Router) {
				r.Use(authpkg.RequireRole(authpkg.RoleDeveloper))
				r.Post("/applications/{appID}/members", h.AddApplicationMember)
				r.Delete("/applications/{appID}/members/{userID}", h.RemoveApplicationMember)
			})

			// admin only — user management, credential vault, team mgmt, audit log
			r.Group(func(r chi.Router) {
				r.Use(authpkg.RequireRole(authpkg.RoleAdmin))
				r.Get("/users", h.ListUsers)
				r.Post("/users", h.CreateUser)
				r.Delete("/users/{userID}", h.DeactivateUser)
				r.Post("/users/{userID}/role", h.UpdateUserRole)
				r.Post("/teams", h.CreateTeam)
				r.Put("/teams/{teamID}", h.UpdateTeam)
				r.Delete("/teams/{teamID}", h.DeleteTeam)
				r.Post("/teams/{teamID}/members", h.AddTeamMember)
				r.Delete("/teams/{teamID}/members/{userID}", h.RemoveTeamMember)
				r.Get("/credentials", h.ListCredentials)
				r.Post("/credentials", h.CreateCredential)
				r.Delete("/credentials/{credentialID}", h.DeleteCredential)
				r.Get("/audit", h.ListAuditEvents)
				// Pipeline template management
				r.Get("/admin/templates", h.ListTemplates)
				r.Get("/admin/templates/{tool}", h.GetTemplate)
				r.Put("/admin/templates/{tool}", h.UpdateTemplate)

				// Platform Engineering — cluster registry, manifest templates, env profiles
				r.Get("/admin/clusters", h.ListClusters)
				r.Post("/admin/clusters", h.CreateCluster)
				r.Put("/admin/clusters/{clusterID}", h.UpdateCluster)
				r.Get("/admin/clusters/{clusterID}/services", h.ListClusterServices)
				r.Put("/admin/clusters/{clusterID}/services/{serviceType}", h.UpsertClusterService)
				r.Get("/admin/manifest-templates", h.ListManifestTemplates)
				r.Put("/admin/manifest-templates/{name}", h.UpsertManifestTemplate)
				r.Get("/admin/environment-profiles", h.ListEnvironmentProfiles)
				r.Put("/admin/environment-profiles/{name}", h.UpdateEnvironmentProfile)
				r.Get("/admin/language-profiles", h.ListLanguageProfiles)
				r.Put("/admin/language-profiles/{build_tool}", h.UpsertLanguageProfile)
			})
		})
	})

	// ── React SPA — serves the embedded Vite build for all non-API paths ──
	r.Handle("/*", spaHandler(h.webFS))

	return r
}

// handleHealthz responds with a JSON status payload and a DB ping.
// Returns 503 when the database is unreachable so Traefik can route around it.
func (h *Handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if err := h.db.Ping(r.Context()); err != nil {
		dbStatus = "unreachable"
	}
	w.Header().Set("Content-Type", "application/json")
	if dbStatus != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	fmt.Fprintf(w, `{"status":%q,"db":%q,"service":"devportal","version":%q}`,
		dbStatus, dbStatus, h.version,
	)
}

// requestLogger is a chi-compatible middleware that writes one structured slog
// line per request: method, path, HTTP status, latency, request ID, client IP.
func (h *Handler) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("http",
			"method",     r.Method,
			"path",       r.URL.Path,
			"status",     ww.Status(),
			"ms",         time.Since(start).Milliseconds(),
			"req_id",     middleware.GetReqID(r.Context()),
			"ip",         r.RemoteAddr,
			"bytes",      ww.BytesWritten(),
		)
	})
}

// rateLimiter is a chi-compatible middleware enforcing per-IP request rate.
// Allows 10 requests/second with a burst of 30.
// Returns 429 JSON when the limit is exceeded.
func (h *Handler) rateLimiter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !h.getVisitorLimiter(ip).Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "rate limit exceeded",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// getVisitorLimiter returns the rate.Limiter for the given IP, creating one if needed.
func (h *Handler) getVisitorLimiter(ip string) *rate.Limiter {
	h.mu.Lock()
	defer h.mu.Unlock()
	v, ok := h.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(30, 60)}
		h.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupVisitors prunes rate-limiter entries for IPs idle for more than 5 minutes.
// Runs as a background goroutine for the process lifetime.
func (h *Handler) cleanupVisitors() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		h.mu.Lock()
		for ip, v := range h.visitors {
			if time.Since(v.lastSeen) > 5*time.Minute {
				delete(h.visitors, ip)
			}
		}
		h.mu.Unlock()
	}
}

// stub writes a 501 Not Implemented JSON response with the implementing day.
// Used as a placeholder body in handler methods not yet wired up.
func stub(w http.ResponseWriter, feature, implementedDay string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":            "not yet implemented",
		"feature":          feature,
		"implemented_by":   implementedDay,
	})
}
