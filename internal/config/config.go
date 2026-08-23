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
	"strconv"
)

// Config holds every external-service URL and credential devportal needs.
//
// Hard required at startup (mustEnv): DB_PASSWORD, ENCRYPTION_KEY.
// Everything else is optional — unset integration vars only cause errors at
// provisioning time, not at startup. This lets the portal run fully for UI
// and auth work before any external service is connected.
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

	// ── SCM Provider ─────────────────────────────────────────────────────────
	// GIT_PROVIDER selects which adapter is used at runtime.
	// "gitea"  → local Gitea instance (default)
	// "gitlab" → GitLab (cloud or self-hosted)
	// No code change needed to switch — just change .env and restart.
	GitProvider string // Env: GIT_PROVIDER · Default: gitea

	// GitLab — used when GIT_PROVIDER=gitlab
	GitLabURL   string // Env: GITLAB_URL   · REQUIRED when provider=gitlab
	GitLabToken string // Env: GITLAB_TOKEN · REQUIRED when provider=gitlab (PAT scope: api)

	// Gitea — used when GIT_PROVIDER=gitea
	GiteaURL   string // Env: GITEA_URL   · REQUIRED when provider=gitea
	GiteaToken string // Env: GITEA_TOKEN · REQUIRED when provider=gitea (PAT scope: all)

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
	DependencyTrackURL      string // Env: DEPENDENCY_TRACK_URL       · REQUIRED
	DependencyTrackAPIKeyID string // Env: DEPENDENCY_TRACK_API_KEY_ID · Jenkins credential ID holding the team API key
	DependencyTrackAPIKey   string // Env: DEPENDENCY_TRACK_API_KEY   · DT team API key for DevPortal direct calls (project + notification mgmt)
	K8sManifestGroup      string // Env: K8S_MANIFEST_GROUP      · Default: kubernetes-manifest
	IngressBaseDomain     string // Env: INGRESS_BASE_DOMAIN     · REQUIRED  (base domain for app ingress URLs)

	// ── Platform Infra Cluster References ────────────────────────────────────
	// Used by the manifest generator to produce Kubernetes operator CRs
	// (CNPG, Strimzi, RabbitMQ) and NetworkPolicy egress rules at provision time.
	CNPGClusterName          string // Env: CNPG_CLUSTER_NAME          · Default: postgres-cluster
	CNPGClusterNamespace     string // Env: CNPG_CLUSTER_NAMESPACE      · Default: postgres
	KafkaClusterName         string // Env: KAFKA_CLUSTER_NAME          · Default: kafka-cluster
	KafkaClusterNamespace    string // Env: KAFKA_CLUSTER_NAMESPACE     · Default: kafka
	RabbitMQClusterName      string // Env: RABBITMQ_CLUSTER_NAME       · Default: rabbitmq-cluster
	RabbitMQClusterNamespace string // Env: RABBITMQ_CLUSTER_NAMESPACE  · Default: rabbitmq
	RedisNamespace           string // Env: REDIS_NAMESPACE             · Default: redis
	MinIONamespace           string // Env: MINIO_NAMESPACE             · Default: minio

	// ── Vault ────────────────────────────────────────────────────────────────
	// Used at provision time to create a per-service KV path, ACL policy, and
	// Kubernetes auth role. Optional — provisioning continues if unset, but the
	// Vault path will be empty and the platform team must seed secrets manually.
	VaultURL          string // Env: VAULT_URL          · REQUIRED for Vault integration
	VaultToken        string // Env: VAULT_TOKEN        · REQUIRED for Vault integration (root or provisioner token)
	VaultKVMount      string // Env: VAULT_KV_MOUNT     · Default: secret
	VaultK8sAuthMount string // Env: VAULT_K8S_AUTH_MOUNT · Default: kubernetes
	VaultUseVSO       bool   // Env: VAULT_USE_VSO      · Default: false (uses ESO ExternalSecret; true = VSO VaultStaticSecret)

	// ── Git Bot Commit Author ────────────────────────────────────────────────
	// Author on commits made by devportal (Jenkinsfile, manifests, VERSION).
	BotName  string // Env: BOT_NAME  · Default: DevPortal Bot
	BotEmail string // Env: BOT_EMAIL · REQUIRED

	// ── Worker ───────────────────────────────────────────────────────────────
	// WorkerConcurrency caps how many provisioning jobs run simultaneously.
	// Increase this if you have many parallel service registrations.
	WorkerConcurrency int // Env: WORKER_CONCURRENCY · Default: 3

	// ── TLS ──────────────────────────────────────────────────────────────────
	// Skips certificate validation for outbound calls to Jenkins, Harbor, etc.
	// Set TLS_SKIP_VERIFY=false once proper certs are installed.
	TLSSkipVerify bool // Env: TLS_SKIP_VERIFY · Default: true

	// ── Branding ─────────────────────────────────────────────────────────────
	// All optional. Change these per customer without touching code or rebuilding
	// the image — the same Docker image ships to every org; branding is config.
	// Served as JSON at GET /branding.json (public, cached 5 min).
	BrandAppName    string // Env: BRAND_APP_NAME    · Default: DevPortal
	BrandCompany    string // Env: BRAND_COMPANY     · Default: (empty)
	BrandPrimaryHue int    // Env: BRAND_PRIMARY_HUE · Default: 199 (sky blue, 0-360)
	BrandLogoURL    string // Env: BRAND_LOGO_URL    · Default: (empty, shows text logo)
}

// Load reads all configuration from environment variables and returns a
// populated *Config.
//
// Only two variables are hard-required at startup:
//   - DB_PASSWORD — devportal cannot open its database without it
//   - ENCRYPTION_KEY — needed to encrypt/decrypt stored credentials
//
// All integration variables (GitLab, Jenkins, Harbor, etc.) are optional at
// startup. The application starts and serves the UI without them. Errors only
// surface at provisioning time when those integrations are actually called —
// the provisioning step is marked failed and the error is broadcast over SSE.
// This lets you run and develop the portal without every external service wired up.
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

		// OIDC — only needed when AUTH_MODE=oidc
		OIDCIssuerURL:      env("OIDC_ISSUER_URL", ""),
		OIDCClientID:       env("OIDC_CLIENT_ID", "devportal"),
		OIDCClientSecret:   env("OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURL:    env("OIDC_REDIRECT_URL", ""),
		OIDCAdminGroup:     env("OIDC_ADMIN_GROUP", "devportal-admins"),
		OIDCDeveloperGroup: env("OIDC_DEVELOPER_GROUP", "devportal-developers"),

		GitProvider: env("GIT_PROVIDER", "gitea"),

		// GitLab — only needed when GIT_PROVIDER=gitlab
		GitLabURL:   env("GITLAB_URL", ""),
		GitLabToken: env("GITLAB_TOKEN", ""),

		// Gitea — only needed when GIT_PROVIDER=gitea
		GiteaURL:   env("GITEA_URL", ""),
		GiteaToken: env("GITEA_TOKEN", ""),

		OrgName: env("ORG_NAME", "My Organization"),
		OrgSlug: env("ORG_SLUG", "default"),

		AdminEmail: env("ADMIN_EMAIL", ""),

		// Jenkins — only needed when provisioning projects
		JenkinsURL:       env("JENKINS_URL", ""),
		JenkinsPublicURL: env("JENKINS_PUBLIC_URL", ""),
		JenkinsUser:      env("JENKINS_USER", "admin"),
		JenkinsToken:     env("JENKINS_TOKEN", ""),

		// Harbor — only needed when provisioning projects
		HarborURL:   env("HARBOR_URL", ""),
		HarborUser:  env("HARBOR_USER", "admin"),
		HarborToken: env("HARBOR_TOKEN", ""),

		// DefectDojo — only needed when provisioning projects
		DefectDojoURL:   env("DEFECTDOJO_URL", ""),
		DefectDojoToken: env("DEFECTDOJO_TOKEN", ""),

		// ArgoCD — only needed when provisioning projects
		ArgoCDURL:      env("ARGOCD_URL", ""),
		ArgoCDToken:    env("ARGOCD_TOKEN", ""),
		ArgoCDInsecure: env("ARGOCD_INSECURE", "false") != "false",

		// App DB — only needed when provisioning project databases
		AppDBHost:     env("APP_DB_HOST", ""),
		AppDBPort:     env("APP_DB_PORT", "5432"),
		AppDBUser:     env("APP_DB_ADMIN_USER", "postgres"),
		AppDBPassword: env("APP_DB_ADMIN_PASSWORD", ""),

		// REQUIRED: used for encrypting credentials at rest
		EncryptionKey: mustEnv("ENCRYPTION_KEY"),

		// Jenkinsfile / K8s manifest values — only needed when provisioning
		RegistryURL:           env("REGISTRY_URL", ""),
		RegistryCredentialsID: env("REGISTRY_CREDENTIALS_ID", "robot-jenkins"),
		GitCredentialsID:      env("GIT_CREDENTIALS_ID", ""),
		SharedLibraryURL:      env("SHARED_LIBRARY_URL", ""),
		DependencyTrackURL:      env("DEPENDENCY_TRACK_URL", ""),
		DependencyTrackAPIKeyID: env("DEPENDENCY_TRACK_API_KEY_ID", "dependency-track-api-key"),
		DependencyTrackAPIKey:   env("DEPENDENCY_TRACK_API_KEY", ""),
		K8sManifestGroup:      env("K8S_MANIFEST_GROUP", "kubernetes-manifest"),
		IngressBaseDomain:     env("INGRESS_BASE_DOMAIN", ""),

		CNPGClusterName:          env("CNPG_CLUSTER_NAME", "postgres-cluster"),
		CNPGClusterNamespace:     env("CNPG_CLUSTER_NAMESPACE", "postgres"),
		KafkaClusterName:         env("KAFKA_CLUSTER_NAME", "kafka-cluster"),
		KafkaClusterNamespace:    env("KAFKA_CLUSTER_NAMESPACE", "kafka"),
		RabbitMQClusterName:      env("RABBITMQ_CLUSTER_NAME", "rabbitmq-cluster"),
		RabbitMQClusterNamespace: env("RABBITMQ_CLUSTER_NAMESPACE", "rabbitmq"),
		RedisNamespace:           env("REDIS_NAMESPACE", "redis"),
		MinIONamespace:           env("MINIO_NAMESPACE", "minio"),

		// Vault — only needed when provisioning Vault secrets paths
		VaultURL:          env("VAULT_URL", ""),
		VaultToken:        env("VAULT_TOKEN", ""),
		VaultKVMount:      env("VAULT_KV_MOUNT", "secret"),
		VaultK8sAuthMount: env("VAULT_K8S_AUTH_MOUNT", "kubernetes"),
		VaultUseVSO:       env("VAULT_USE_VSO", "false") != "false",

		// Bot — only needed when committing generated files to GitLab
		BotName:  env("BOT_NAME", "DevPortal Bot"),
		BotEmail: env("BOT_EMAIL", "devportal@localhost"),

		WorkerConcurrency: envInt("WORKER_CONCURRENCY", 3),

		TLSSkipVerify: env("TLS_SKIP_VERIFY", "true") != "false",

		BrandAppName:    env("BRAND_APP_NAME", "DevPortal"),
		BrandCompany:    env("BRAND_COMPANY", ""),
		BrandPrimaryHue: envInt("BRAND_PRIMARY_HUE", 199),
		BrandLogoURL:    env("BRAND_LOGO_URL", ""),
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

// envInt reads the environment variable named by key and parses it as an int.
// Returns fallback if the variable is unset, empty, or not a valid integer.
func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
