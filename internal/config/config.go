// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// Package config loads all runtime configuration from environment variables.
//
// Design rules:
//   - No domain-specific defaults. Every URL that differs per deployment MUST
//     be set via environment variable. Hardcoding a hostname here would mean
//     a code change every time someone deploys to a different environment.
//   - Safe structural defaults only: port numbers, DB names, role names, flags.
//   - mustEnv() exits immediately on missing required vars so the service
//     fails fast at startup rather than crashing mid-request.
//   - Config is populated once by Load() and then read-only for the process
//     lifetime. Always pass *Config — never individual fields.
package config

import (
	"log/slog"
	"os"
)

// Config holds every external-service URL and credential devportal needs.
// All URL fields are intentionally empty by default — set them via environment
// variables. Only structural values (ports, DB names, group names) have defaults.
type Config struct {

	// ── HTTP Server ──────────────────────────────────────────────────────────
	// Env: HTTP_ADDR · Default: :8080
	HTTPAddr string

	// ── DevPortal Database ───────────────────────────────────────────────────
	// devportal's own PostgreSQL instance — separate from app databases.
	// Stores: project registry, sessions, audit log, encrypted credentials,
	// provisioning steps.
	DBHost     string // Env: DB_HOST      · Default: localhost
	DBPort     string // Env: DB_PORT      · Default: 5432
	DBName     string // Env: DB_NAME      · Default: devportal
	DBUser     string // Env: DB_USER      · Default: devportal
	DBPassword string // Env: DB_PASSWORD  · REQUIRED
	DBSSLMode  string // Env: DB_SSL_MODE  · Default: disable

	// ── Auth Mode ────────────────────────────────────────────────────────────
	// "local" → built-in username/password (devportal manages accounts, no IdP)
	// "oidc"  → Keycloak OIDC (recommended for production, SSO, AD/LDAP federation)
	// Env: AUTH_MODE · Default: local
	AuthMode string

	// ── OIDC / Keycloak ──────────────────────────────────────────────────────
	// Used when AuthMode == "oidc". Keycloak federates AD/LDAP, GitLab, GitHub,
	// and Google accounts behind one login and maps group membership to RBAC roles.
	// All URLs are REQUIRED and have no default — they differ per deployment.
	OIDCIssuerURL    string // Env: OIDC_ISSUER_URL    · REQUIRED when mode=oidc  e.g. https://keycloak.example.com/realms/myorg
	OIDCClientID     string // Env: OIDC_CLIENT_ID     · Default: devportal
	OIDCClientSecret string // Env: OIDC_CLIENT_SECRET · REQUIRED when mode=oidc
	OIDCRedirectURL  string // Env: OIDC_REDIRECT_URL  · REQUIRED when mode=oidc  e.g. https://devportal.example.com/auth/callback

	// Keycloak group names mapped to devportal roles.
	// Users not in either group get "viewer" (least-privilege default).
	OIDCAdminGroup     string // Env: OIDC_ADMIN_GROUP      · Default: devportal-admins
	OIDCDeveloperGroup string // Env: OIDC_DEVELOPER_GROUP  · Default: devportal-developers

	// ── GitLab / Gitea ───────────────────────────────────────────────────────
	// API client for Git operations (create repo, commit files, webhooks).
	// Auth is handled separately (local or OIDC) — these are not OAuth fields.
	GitLabURL   string // Env: GITLAB_URL   · REQUIRED
	GitLabToken string // Env: GITLAB_TOKEN · REQUIRED (PAT scope: api)

	// ── Organisation ─────────────────────────────────────────────────────────
	// In v0.1 devportal has a single org created on first login.
	// The slug is used in DB and API paths; the name is shown in the UI.
	OrgName string // Env: ORG_NAME · Default: My Organization
	OrgSlug string // Env: ORG_SLUG · Default: default

	// ── Bootstrap Admin ──────────────────────────────────────────────────────
	// Grants admin rights to this email regardless of group membership.
	// Used on a fresh install before OIDC groups are configured.
	// Remove once groups are set up in Keycloak.
	AdminEmail string // Env: ADMIN_EMAIL

	// ── Jenkins ──────────────────────────────────────────────────────────────
	// JENKINS_URL:        internal API endpoint (used by devportal → Jenkins)
	// JENKINS_PUBLIC_URL: public DNS hostname (used for GitLab webhooks and
	//                     job links shown to developers — must be reachable
	//                     from both the developer's browser and GitLab's server)
	JenkinsURL       string // Env: JENKINS_URL        · REQUIRED
	JenkinsPublicURL string // Env: JENKINS_PUBLIC_URL · REQUIRED
	JenkinsUser      string // Env: JENKINS_USER       · Default: admin
	JenkinsToken     string // Env: JENKINS_TOKEN      · REQUIRED

	// ── Harbor ───────────────────────────────────────────────────────────────
	HarborURL   string // Env: HARBOR_URL   · REQUIRED
	HarborUser  string // Env: HARBOR_USER  · Default: admin
	HarborToken string // Env: HARBOR_TOKEN · REQUIRED

	// ── DefectDojo ───────────────────────────────────────────────────────────
	DefectDojoURL   string // Env: DEFECTDOJO_URL   · REQUIRED
	DefectDojoToken string // Env: DEFECTDOJO_TOKEN · REQUIRED

	// ── ArgoCD ───────────────────────────────────────────────────────────────
	ArgoCDURL      string // Env: ARGOCD_URL      · REQUIRED
	ArgoCDToken    string // Env: ARGOCD_TOKEN    · REQUIRED (argocd account generate-token)
	ArgoCDInsecure bool   // Env: ARGOCD_INSECURE · Default: false

	// ── App PostgreSQL (provisioned FOR developer apps) ───────────────────────
	// devportal runs CREATE DATABASE / CREATE USER / GRANT against this instance.
	// Completely separate from devportal's own database above.
	AppDBHost     string // Env: APP_DB_HOST           · REQUIRED
	AppDBPort     string // Env: APP_DB_PORT           · Default: 5432
	AppDBUser     string // Env: APP_DB_ADMIN_USER     · Default: postgres
	AppDBPassword string // Env: APP_DB_ADMIN_PASSWORD · REQUIRED

	// ── Credential Encryption ─────────────────────────────────────────────────
	// AES-256-GCM key. Must be 32 bytes, base64-encoded.
	// Generate: openssl rand -base64 32
	EncryptionKey string // Env: ENCRYPTION_KEY · REQUIRED

	// ── Jenkinsfile / K8s Manifest values ────────────────────────────────────
	// Baked into every generated Jenkinsfile and K8s manifest.
	// Change once in .env — all future projects pick up the new value.
	RegistryURL           string // Env: REGISTRY_URL            · REQUIRED  (hostname only, no https://)
	RegistryCredentialsID string // Env: REGISTRY_CREDENTIALS_ID · Default: robot-jenkins
	GitCredentialsID      string // Env: GIT_CREDENTIALS_ID      · REQUIRED  (Jenkins credential ID for Git)
	SharedLibraryURL      string // Env: SHARED_LIBRARY_URL      · REQUIRED  (Git URL of Jenkins shared library)
	DependencyTrackURL    string // Env: DEPENDENCY_TRACK_URL    · REQUIRED
	K8sManifestGroup      string // Env: K8S_MANIFEST_GROUP      · Default: kubernetes-manifest
	IngressBaseDomain     string // Env: INGRESS_BASE_DOMAIN     · REQUIRED  (base domain for app ingress URLs)

	// ── Git Bot Commit Author ────────────────────────────────────────────────
	// Author on commits made by devportal (Jenkinsfile, manifests, VERSION).
	BotName  string // Env: BOT_NAME  · Default: DevPortal Bot
	BotEmail string // Env: BOT_EMAIL · REQUIRED

	// ── TLS ──────────────────────────────────────────────────────────────────
	// Skips certificate validation for outbound calls to Jenkins, Harbor, etc.
	// Set TLS_SKIP_VERIFY=false once proper certs are installed.
	TLSSkipVerify bool // Env: TLS_SKIP_VERIFY · Default: true
}

// Load reads all configuration from environment variables and returns a
// populated *Config. Calls os.Exit(1) on any missing required variable.
func Load() *Config {
	cfg := &Config{
		HTTPAddr: env("HTTP_ADDR", ":8080"),

		// DB — safe structural defaults, no domain-specific values
		DBHost:     env("DB_HOST", "localhost"),
		DBPort:     env("DB_PORT", "5432"),
		DBName:     env("DB_NAME", "devportal"),
		DBUser:     env("DB_USER", "devportal"),
		DBPassword: mustEnv("DB_PASSWORD"),
		DBSSLMode:  env("DB_SSL_MODE", "disable"),

		AuthMode: env("AUTH_MODE", "local"),

		// OIDC — no URL defaults, all deployment-specific
		OIDCIssuerURL:      env("OIDC_ISSUER_URL", ""),
		OIDCClientID:       env("OIDC_CLIENT_ID", "devportal"),
		OIDCClientSecret:   env("OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURL:    env("OIDC_REDIRECT_URL", ""),
		OIDCAdminGroup:     env("OIDC_ADMIN_GROUP", "devportal-admins"),
		OIDCDeveloperGroup: env("OIDC_DEVELOPER_GROUP", "devportal-developers"),

		// GitLab — API access only, no OAuth
		GitLabURL:   mustEnv("GITLAB_URL"),
		GitLabToken: mustEnv("GITLAB_TOKEN"),

		OrgName: env("ORG_NAME", "My Organization"),
		OrgSlug: env("ORG_SLUG", "default"),

		AdminEmail: env("ADMIN_EMAIL", ""),

		// Jenkins — no IP or domain defaults
		JenkinsURL:       mustEnv("JENKINS_URL"),
		JenkinsPublicURL: mustEnv("JENKINS_PUBLIC_URL"),
		JenkinsUser:      env("JENKINS_USER", "admin"),
		JenkinsToken:     mustEnv("JENKINS_TOKEN"),

		// Harbor — no domain defaults
		HarborURL:   mustEnv("HARBOR_URL"),
		HarborUser:  env("HARBOR_USER", "admin"),
		HarborToken: mustEnv("HARBOR_TOKEN"),

		// DefectDojo — no domain defaults
		DefectDojoURL:   mustEnv("DEFECTDOJO_URL"),
		DefectDojoToken: mustEnv("DEFECTDOJO_TOKEN"),

		// ArgoCD — no domain defaults
		ArgoCDURL:      mustEnv("ARGOCD_URL"),
		ArgoCDToken:    mustEnv("ARGOCD_TOKEN"),
		ArgoCDInsecure: env("ARGOCD_INSECURE", "false") != "false",

		// App DB — host is deployment-specific, no default
		AppDBHost:     mustEnv("APP_DB_HOST"),
		AppDBPort:     env("APP_DB_PORT", "5432"),
		AppDBUser:     env("APP_DB_ADMIN_USER", "postgres"),
		AppDBPassword: mustEnv("APP_DB_ADMIN_PASSWORD"),

		EncryptionKey: mustEnv("ENCRYPTION_KEY"),

		// Jenkinsfile / manifest defaults — all deployment-specific
		RegistryURL:           mustEnv("REGISTRY_URL"),
		RegistryCredentialsID: env("REGISTRY_CREDENTIALS_ID", "robot-jenkins"),
		GitCredentialsID:      mustEnv("GIT_CREDENTIALS_ID"),
		SharedLibraryURL:      mustEnv("SHARED_LIBRARY_URL"),
		DependencyTrackURL:    mustEnv("DEPENDENCY_TRACK_URL"),
		K8sManifestGroup:      env("K8S_MANIFEST_GROUP", "kubernetes-manifest"),
		IngressBaseDomain:     mustEnv("INGRESS_BASE_DOMAIN"),

		BotName:  env("BOT_NAME", "DevPortal Bot"),
		BotEmail: mustEnv("BOT_EMAIL"),

		TLSSkipVerify: env("TLS_SKIP_VERIFY", "true") != "false",
	}

	slog.Info("config loaded",
		"auth_mode", cfg.AuthMode,
		"http_addr", cfg.HTTPAddr,
		"db_host", cfg.DBHost,
		"db_name", cfg.DBName,
		"gitlab_url", cfg.GitLabURL,
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
// Exits with a clear error if the variable is unset — no guessing at defaults
// for values that are deployment-specific.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable not set — devportal cannot start", "var", key)
		os.Exit(1)
	}
	return v
}
