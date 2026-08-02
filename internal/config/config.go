// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// Package config loads all runtime configuration from environment variables.
//
// Design rules:
//   - Every value has a default where safe; only secrets MUST be set explicitly.
//   - mustEnv() exits immediately with a clear error if a required var is missing,
//     so the service fails at startup rather than crashing mid-request.
//   - Config is populated once by Load() and then read-only for the process lifetime.
//     Always pass *Config — never individual fields — so adding fields later
//     does not require changing every call site.
package config

import (
	"log/slog"
	"os"
)

// Config holds every external-service URL and credential devportal needs.
type Config struct {

	// ── HTTP Server ──────────────────────────────────────────────────────────
	// HTTPAddr is the TCP address the HTTP server binds to.
	// Env: HTTP_ADDR · Default: :8080
	HTTPAddr string

	// ── DevPortal Database ───────────────────────────────────────────────────
	// devportal uses its own PostgreSQL 16 instance (Bitnami Helm subchart in
	// production, plain postgres:16-alpine in docker-compose for dev) to store:
	//   - project registry (organizations, teams, projects, environments)
	//   - DB-backed sessions (survives pod restarts, revocable)
	//   - audit events (who did what and when)
	//   - workspace credentials (encrypted with AES-256-GCM)
	//   - provisioning steps (live status for the SSE stream)
	// This database is separate from the PostgreSQL instances devportal
	// provisions for developer applications.
	DBHost     string // Env: DB_HOST      · Default: localhost
	DBPort     string // Env: DB_PORT      · Default: 5432
	DBName     string // Env: DB_NAME      · Default: devportal
	DBUser     string // Env: DB_USER      · Default: devportal
	DBPassword string // Env: DB_PASSWORD  · REQUIRED
	DBSSLMode  string // Env: DB_SSL_MODE  · Default: disable (use "require" in prod)

	// ── Auth Mode ────────────────────────────────────────────────────────────
	// Controls which authentication backend is active.
	//   "oidc"   → Keycloak OIDC (primary — used at SoftNet)
	//   "gitlab" → GitLab OAuth fallback (personal deployments without Keycloak)
	// Env: AUTH_MODE · Default: oidc
	AuthMode string

	// ── OIDC / Keycloak ──────────────────────────────────────────────────────
	// Used when AuthMode == "oidc". Keycloak is the SSO hub for SoftNet —
	// it federates AD/LDAP, GitLab, GitHub, and Google accounts behind one
	// login, and maps Keycloak group membership to devportal RBAC roles
	// via the "groups" claim in the OIDC id_token.
	OIDCIssuerURL    string // Env: OIDC_ISSUER_URL    · e.g. https://keycloak.softnethq.co.tz/realms/softnet
	OIDCClientID     string // Env: OIDC_CLIENT_ID     · Default: devportal
	OIDCClientSecret string // Env: OIDC_CLIENT_SECRET · REQUIRED when mode=oidc
	OIDCRedirectURL  string // Env: OIDC_REDIRECT_URL  · e.g. https://devportal.devops.softnethq.co.tz/auth/callback

	// Keycloak group names that map to devportal roles.
	// Any user not in either group gets "viewer" (least-privilege default).
	OIDCAdminGroup     string // Env: OIDC_ADMIN_GROUP      · Default: devportal-admins
	OIDCDeveloperGroup string // Env: OIDC_DEVELOPER_GROUP  · Default: devportal-developers

	// ── GitLab OAuth Fallback ─────────────────────────────────────────────────
	// Used when AuthMode == "gitlab". No Keycloak required.
	// All GitLab users get the "developer" role by default.
	// Users whose email matches AdminEmail get the "admin" role.
	GitLabURL               string // Env: GITLAB_URL                · Default: http://192.168.15.85
	GitLabOAuthClientID     string // Env: GITLAB_OAUTH_CLIENT_ID    · REQUIRED when mode=gitlab
	GitLabOAuthClientSecret string // Env: GITLAB_OAUTH_CLIENT_SECRET· REQUIRED when mode=gitlab
	GitLabOAuthRedirectURL  string // Env: GITLAB_OAUTH_REDIRECT_URL

	// GitLabToken is the service-account Personal Access Token used for all
	// GitLab API calls: list repos, commit Jenkinsfile/manifests, create webhooks.
	// Required scope: api.
	GitLabToken string // Env: GITLAB_TOKEN · REQUIRED

	// ── Bootstrap Admin ──────────────────────────────────────────────────────
	// Gives admin rights to a specific email regardless of group membership.
	// Used to log in on a fresh install before Keycloak groups are configured.
	// Remove from values.yaml after the first admin is set up in Keycloak.
	AdminEmail string // Env: ADMIN_EMAIL

	// ── Jenkins ──────────────────────────────────────────────────────────────
	JenkinsURL       string // Env: JENKINS_URL        · Default: http://192.168.200.78:8080
	JenkinsPublicURL string // Env: JENKINS_PUBLIC_URL · Default: https://jenkins.devops.softnethq.co.tz
	JenkinsUser      string // Env: JENKINS_USER       · Default: admin
	JenkinsToken     string // Env: JENKINS_TOKEN      · REQUIRED

	// ── Harbor ───────────────────────────────────────────────────────────────
	HarborURL   string // Env: HARBOR_URL    · Default: https://harbor.devops.softnethq.co.tz
	HarborUser  string // Env: HARBOR_USER   · Default: admin
	HarborToken string // Env: HARBOR_TOKEN  · REQUIRED

	// ── DefectDojo ───────────────────────────────────────────────────────────
	DefectDojoURL   string // Env: DEFECTDOJO_URL   · Default: https://defectdojo.devops.softnethq.co.tz
	DefectDojoToken string // Env: DEFECTDOJO_TOKEN · REQUIRED

	// ── ArgoCD ───────────────────────────────────────────────────────────────
	ArgoCDURL      string // Env: ARGOCD_URL      · Default: https://argocd.devops.softnethq.co.tz
	ArgoCDToken    string // Env: ARGOCD_TOKEN    · REQUIRED (generate: argocd account generate-token)
	ArgoCDInsecure bool   // Env: ARGOCD_INSECURE · Default: false

	// ── App PostgreSQL (provisioned FOR developer apps) ───────────────────────
	// devportal's DB provisioner connects to this instance with superuser
	// credentials to run CREATE DATABASE / CREATE USER / GRANT for each new
	// project environment. Completely separate from devportal's own DB above.
	AppDBHost     string // Env: APP_DB_HOST           · Default: postgres.devops.svc.cluster.local
	AppDBPort     string // Env: APP_DB_PORT           · Default: 5432
	AppDBUser     string // Env: APP_DB_ADMIN_USER     · Default: postgres
	AppDBPassword string // Env: APP_DB_ADMIN_PASSWORD · REQUIRED

	// ── Credential Encryption ─────────────────────────────────────────────────
	// AES-256-GCM symmetric key used to encrypt all workspace credentials before
	// storing them in the DB. Must be exactly 32 bytes, base64-encoded.
	// Stored in a K8s Secret and mounted as an env var at runtime.
	// Generate: openssl rand -base64 32
	EncryptionKey string // Env: ENCRYPTION_KEY · REQUIRED

	// ── Jenkinsfile / K8s Manifest Defaults ──────────────────────────────────
	// These values are baked into every generated Jenkinsfile so they never
	// need to be changed per project — only the per-project values (name, repo,
	// Harbor project) change.
	RegistryURL           string // Env: REGISTRY_URL            · Default: harbor.devops.softnethq.co.tz
	RegistryCredentialsID string // Env: REGISTRY_CREDENTIALS_ID · Default: robot-jenkins
	GitCredentialsID      string // Env: GIT_CREDENTIALS_ID      · Default: lsaid
	SharedLibraryURL      string // Env: SHARED_LIBRARY_URL      · Default: http://192.168.15.85/devsecops1/pipeline.git
	DependencyTrackURL    string // Env: DEPENDENCY_TRACK_URL    · Default: https://dependencytrack.devops.softnethq.co.tz
	K8sManifestGroup      string // Env: K8S_MANIFEST_GROUP      · Default: kubernetes-manifest
	IngressBaseDomain     string // Env: INGRESS_BASE_DOMAIN     · Default: k8s.softnethq.co.tz

	// ── Git Bot Commit Author ────────────────────────────────────────────────
	// Used as the commit author when devportal pushes files to GitLab
	// (Jenkinsfile, VERSION, Dockerfile, K8s manifests).
	// When a developer is signed in via OIDC, their own name/email is used instead.
	BotName  string // Env: BOT_NAME  · Default: DevPortal Bot
	BotEmail string // Env: BOT_EMAIL · Default: devportal@softnet.co.tz

	// ── TLS ──────────────────────────────────────────────────────────────────
	// Skips TLS certificate validation for all outbound HTTP calls.
	// Acceptable on the internal LAN where services use self-signed certs.
	// Set TLS_SKIP_VERIFY=false once proper certificates are installed on
	// GitLab, Jenkins, Harbor, DefectDojo, and ArgoCD.
	TLSSkipVerify bool // Env: TLS_SKIP_VERIFY · Default: true
}

// Load reads all configuration from environment variables and returns a
// populated *Config. It logs each non-secret value at INFO level and
// calls os.Exit(1) if any required variable is missing, so the service
// fails at startup with a clear error rather than crashing mid-request.
func Load() *Config {
	cfg := &Config{
		HTTPAddr: env("HTTP_ADDR", ":8080"),

		DBHost:     env("DB_HOST", "localhost"),
		DBPort:     env("DB_PORT", "5432"),
		DBName:     env("DB_NAME", "devportal"),
		DBUser:     env("DB_USER", "devportal"),
		DBPassword: mustEnv("DB_PASSWORD"),
		DBSSLMode:  env("DB_SSL_MODE", "disable"),

		AuthMode: env("AUTH_MODE", "oidc"),

		OIDCIssuerURL:      env("OIDC_ISSUER_URL", ""),
		OIDCClientID:       env("OIDC_CLIENT_ID", "devportal"),
		OIDCClientSecret:   env("OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURL:    env("OIDC_REDIRECT_URL", ""),
		OIDCAdminGroup:     env("OIDC_ADMIN_GROUP", "devportal-admins"),
		OIDCDeveloperGroup: env("OIDC_DEVELOPER_GROUP", "devportal-developers"),

		GitLabURL:               env("GITLAB_URL", "http://192.168.15.85"),
		GitLabOAuthClientID:     env("GITLAB_OAUTH_CLIENT_ID", ""),
		GitLabOAuthClientSecret: env("GITLAB_OAUTH_CLIENT_SECRET", ""),
		GitLabOAuthRedirectURL:  env("GITLAB_OAUTH_REDIRECT_URL", ""),
		GitLabToken:             mustEnv("GITLAB_TOKEN"),

		AdminEmail: env("ADMIN_EMAIL", ""),

		JenkinsURL:       env("JENKINS_URL", "http://192.168.200.78:8080"),
		JenkinsPublicURL: env("JENKINS_PUBLIC_URL", "https://jenkins.devops.softnethq.co.tz"),
		JenkinsUser:      env("JENKINS_USER", "admin"),
		JenkinsToken:     mustEnv("JENKINS_TOKEN"),

		HarborURL:   env("HARBOR_URL", "https://harbor.devops.softnethq.co.tz"),
		HarborUser:  env("HARBOR_USER", "admin"),
		HarborToken: mustEnv("HARBOR_TOKEN"),

		DefectDojoURL:   env("DEFECTDOJO_URL", "https://defectdojo.devops.softnethq.co.tz"),
		DefectDojoToken: mustEnv("DEFECTDOJO_TOKEN"),

		ArgoCDURL:      env("ARGOCD_URL", "https://argocd.devops.softnethq.co.tz"),
		ArgoCDToken:    mustEnv("ARGOCD_TOKEN"),
		ArgoCDInsecure: env("ARGOCD_INSECURE", "false") != "false",

		AppDBHost:     env("APP_DB_HOST", "postgres.devops.svc.cluster.local"),
		AppDBPort:     env("APP_DB_PORT", "5432"),
		AppDBUser:     env("APP_DB_ADMIN_USER", "postgres"),
		AppDBPassword: mustEnv("APP_DB_ADMIN_PASSWORD"),

		EncryptionKey: mustEnv("ENCRYPTION_KEY"),

		RegistryURL:           env("REGISTRY_URL", "harbor.devops.softnethq.co.tz"),
		RegistryCredentialsID: env("REGISTRY_CREDENTIALS_ID", "robot-jenkins"),
		GitCredentialsID:      env("GIT_CREDENTIALS_ID", "lsaid"),
		SharedLibraryURL:      env("SHARED_LIBRARY_URL", "http://192.168.15.85/devsecops1/pipeline.git"),
		DependencyTrackURL:    env("DEPENDENCY_TRACK_URL", "https://dependencytrack.devops.softnethq.co.tz"),
		K8sManifestGroup:      env("K8S_MANIFEST_GROUP", "kubernetes-manifest"),
		IngressBaseDomain:     env("INGRESS_BASE_DOMAIN", "k8s.softnethq.co.tz"),

		BotName:  env("BOT_NAME", "DevPortal Bot"),
		BotEmail: env("BOT_EMAIL", "devportal@softnet.co.tz"),

		TLSSkipVerify: env("TLS_SKIP_VERIFY", "true") != "false",
	}

	// Log the active configuration at startup (secrets are omitted).
	slog.Info("config loaded",
		"auth_mode", cfg.AuthMode,
		"http_addr", cfg.HTTPAddr,
		"db_host", cfg.DBHost,
		"db_name", cfg.DBName,
		"jenkins_url", cfg.JenkinsURL,
		"harbor_url", cfg.HarborURL,
		"defectdojo_url", cfg.DefectDojoURL,
		"argocd_url", cfg.ArgoCDURL,
		"tls_skip_verify", cfg.TLSSkipVerify,
	)

	return cfg
}

// env reads the environment variable named by key.
// Returns fallback when the variable is unset or empty.
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// mustEnv reads the environment variable named by key.
// Exits with a clear error message if the variable is unset or empty,
// because there is no safe default for secrets and required credentials.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable not set — devportal cannot start", "var", key)
		os.Exit(1)
	}
	return v
}
