// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// models.go defines Go structs that mirror every table in the devportal
// PostgreSQL schema (migrations/001_initial.sql).
//
// Naming rules:
//   - Struct names match table names in PascalCase (organizations → Organization).
//   - Field names match column names in PascalCase (created_at → CreatedAt).
//   - `db` struct tags match the exact PostgreSQL column names so pgx can
//     map query results to structs using pgx.RowToStructByName.
//   - Nullable columns use pointer types (*string, *int, *uuid.UUID, *time.Time)
//     so a NULL in the DB becomes nil in Go — no sentinel zero values needed.

package db

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Application is a business initiative that groups related microservices.
// Access is controlled per-application via application_members — team
// membership alone does not grant access (least-privilege model).
type Application struct {
	ID           uuid.UUID  `db:"id"            json:"id"`
	OrgID        uuid.UUID  `db:"org_id"        json:"org_id"`
	Name         string     `db:"name"          json:"name"`
	Slug         string     `db:"slug"          json:"slug"`
	Description  string     `db:"description"   json:"description"`
	GitNamespace string     `db:"git_namespace" json:"git_namespace"`
	Status       string     `db:"status"        json:"status"`
	CreatedAt    time.Time  `db:"created_at"    json:"created_at"`
	CreatedBy    *uuid.UUID `db:"created_by"    json:"created_by,omitempty"`
}

// ApplicationMember links a user to an application with a role.
// Only members can see or create services within the application.
type ApplicationMember struct {
	ApplicationID uuid.UUID  `db:"application_id" json:"application_id"`
	UserID        uuid.UUID  `db:"user_id"        json:"user_id"`
	Role          string     `db:"role"           json:"role"` // "lead" | "developer"
	AddedAt       time.Time  `db:"added_at"       json:"added_at"`
	AddedBy       *uuid.UUID `db:"added_by"       json:"added_by,omitempty"`
}

// Organization is the top-level tenant boundary.
// In v0.1 there is a single org. Multi-org support enables the future SaaS phase.
type Organization struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	Slug      string    `db:"slug"` // URL-safe identifier used in API paths, e.g. "nexbridge"
	CreatedAt time.Time `db:"created_at"`
}

// User is every person who has authenticated into devportal at least once.
// For OIDC users, devportal does NOT manage passwords — Keycloak owns credentials.
// For local-auth users, password_hash holds the bcrypt hash.
type User struct {
	ID          uuid.UUID `db:"id"`
	OrgID       uuid.UUID `db:"org_id"`
	Email       string    `db:"email"`
	DisplayName string    `db:"display_name"`
	// Provider: "oidc" (Keycloak) or "local" (built-in password auth).
	Provider string `db:"provider"`
	// ProviderID: Keycloak "sub" claim for OIDC, or email for local users.
	ProviderID   string  `db:"provider_id"`
	Role         string  `db:"role"`         // "admin" | "developer" | "viewer"
	PasswordHash *string `db:"password_hash"` // nil for OIDC users
	IsActive     bool    `db:"is_active"`
	CreatedAt    time.Time `db:"created_at"`
}

// Session is a DB-backed login session.
// Unlike pipeline-init's in-memory sessions, these survive pod restarts
// and can be explicitly revoked by an admin.
type Session struct {
	ID        string    `db:"id"`         // crypto/rand 256-bit hex string
	UserID    uuid.UUID `db:"user_id"`
	Role      string    `db:"role"`       // "admin" | "developer" | "viewer"
	IPAddress *string   `db:"ip_address"` // nullable — not always available
	UserAgent *string   `db:"user_agent"` // nullable
	CreatedAt time.Time `db:"created_at"`
	ExpiresAt time.Time `db:"expires_at"` // extended on each request (sliding 24h window)
	LastSeen  time.Time `db:"last_seen"`
}

// Team groups developers within an org (e.g. "backend", "mobile", "platform").
// Projects belong to a team so members can see their own team's projects.
type Team struct {
	ID        uuid.UUID `db:"id"         json:"id"`
	OrgID     uuid.UUID `db:"org_id"     json:"org_id"`
	Name      string    `db:"name"       json:"name"`
	Slug      string    `db:"slug"       json:"slug"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// TeamMember links a user to a team with a role.
type TeamMember struct {
	TeamID uuid.UUID `db:"team_id" json:"team_id"`
	UserID uuid.UUID `db:"user_id" json:"user_id"`
	Role   string    `db:"role"    json:"role"`
}

// TeamMemberDetail is a join of team_members + users for API responses.
type TeamMemberDetail struct {
	TeamID      uuid.UUID `db:"team_id"      json:"team_id"`
	UserID      uuid.UUID `db:"user_id"      json:"user_id"`
	MemberRole  string    `db:"member_role"  json:"member_role"`
	Role        string    `db:"org_role"     json:"role"`
	Email       string    `db:"email"        json:"email"`
	DisplayName string    `db:"display_name" json:"display_name"`
}

// WorkspaceCredential stores an encrypted service credential at the org level.
// The plaintext never touches the database — only the AES-256-GCM ciphertext.
type WorkspaceCredential struct {
	ID             uuid.UUID `db:"id"`
	OrgID          uuid.UUID `db:"org_id"`
	ProviderType   string    `db:"provider_type"`   // "gitlab" | "jenkins" | "harbor" | "defectdojo" | "argocd"
	Label          string    `db:"label"`            // human-readable name, e.g. "GitLab service account"
	EncryptedValue []byte    `db:"encrypted_value"`  // AES-256-GCM ciphertext
	CreatedAt      time.Time `db:"created_at"`
}

// Project is the central entity — one row per developer project.
// Environments, provisioning steps, and audit events all reference a project.
type Project struct {
	ID                   uuid.UUID  `db:"id"                    json:"id"`
	TeamID               *uuid.UUID `db:"team_id"               json:"team_id,omitempty"`
	Name                 string     `db:"name"                  json:"name"`
	Slug                 string     `db:"slug"                  json:"slug"`
	GitRepoURL           string     `db:"git_repo_url"          json:"git_repo_url"`
	HarborProject        string     `db:"harbor_project"        json:"harbor_project"`
	JenkinsFolder        string     `db:"jenkins_folder"        json:"jenkins_folder"`
	BuildTool            string     `db:"build_tool"            json:"build_tool"`
	NotificationEmail    string     `db:"notification_email"    json:"notification_email"`
	DefectDojoProductID    *int       `db:"defectdojo_product_id"    json:"defectdojo_product_id,omitempty"`
	DefectDojoEngagementID *int       `db:"defectdojo_engagement_id" json:"defectdojo_engagement_id,omitempty"`
	Status               string     `db:"status"                json:"status"`
	GeneratedJenkinsfile *string    `db:"generated_jenkinsfile" json:"-"`
	ManifestRepoURL      *string    `db:"manifest_repo_url"     json:"manifest_repo_url,omitempty"`
	AppRepoURL           *string    `db:"app_repo_url"          json:"app_repo_url,omitempty"`
	AppTimezone          string     `db:"app_timezone"          json:"app_timezone"`
	StagingURL           *string    `db:"staging_url"           json:"staging_url,omitempty"`
	K8sManifestPaths     string     `db:"k8s_manifest_paths"    json:"k8s_manifest_paths"`
	CreatedAt            time.Time  `db:"created_at"            json:"created_at"`
	CreatedBy            *uuid.UUID `db:"created_by"            json:"created_by,omitempty"`
	ApplicationID        *uuid.UUID `db:"application_id"        json:"application_id,omitempty"`
	// Added in migration 007 — used in manifest templates.
	Port       int    `db:"port"        json:"port"`
	HealthPath string `db:"health_path" json:"health_path"`
	// Added in migration 009 — DefectDojo SLA remediation deadlines (days).
	VulnSLACritical int `db:"vuln_sla_critical" json:"vuln_sla_critical"`
	VulnSLAHigh     int `db:"vuln_sla_high"     json:"vuln_sla_high"`
	VulnSLAMedium   int `db:"vuln_sla_medium"   json:"vuln_sla_medium"`
	VulnSLALow      int `db:"vuln_sla_low"      json:"vuln_sla_low"`
}

// ProjectMember links a user to a service with a role (lead | developer).
// Controls DefectDojo product membership — only assigned users see that
// service's findings.
type ProjectMember struct {
	ProjectID uuid.UUID  `db:"project_id" json:"project_id"`
	UserID    uuid.UUID  `db:"user_id"    json:"user_id"`
	Role      string     `db:"role"       json:"role"`
	AddedAt   time.Time  `db:"added_at"   json:"added_at"`
	AddedBy   *uuid.UUID `db:"added_by"   json:"added_by,omitempty"`
}

// ProjectMemberDetail is a join of project_members + users for API responses
// and orchestrator use (email is needed to look up the user in DefectDojo).
type ProjectMemberDetail struct {
	ProjectID   uuid.UUID `db:"project_id"   json:"project_id"`
	UserID      uuid.UUID `db:"user_id"      json:"user_id"`
	Email       string    `db:"email"        json:"email"`
	DisplayName string    `db:"display_name" json:"display_name"`
	Role        string    `db:"role"         json:"role"`
}

// Environment is one deployment tier (dev / uat / prod) for a project.
// Each environment gets its own K8s namespace, ArgoCD Application, and database.
type Environment struct {
	ID            uuid.UUID `db:"id"             json:"id"`
	ProjectID     uuid.UUID `db:"project_id"     json:"project_id"`
	Name          string    `db:"name"           json:"name"`
	Namespace     string    `db:"namespace"      json:"namespace"`
	IngressURL    *string   `db:"ingress_url"    json:"ingress_url,omitempty"`
	ArgoCDAppName *string   `db:"argocd_app_name" json:"argocd_app_name,omitempty"`
	DBName        *string   `db:"db_name"        json:"db_name,omitempty"`
	DBUsername    *string   `db:"db_username"    json:"db_username,omitempty"`
	ManifestPath  *string    `db:"manifest_path"  json:"manifest_path,omitempty"`
	Status        string     `db:"status"         json:"status"`
	CreatedAt     time.Time  `db:"created_at"     json:"created_at"`
	// Added in migration 007 — links environment to a registered cluster.
	ClusterID     *uuid.UUID `db:"cluster_id"     json:"cluster_id,omitempty"`
}

// ProvisioningStep tracks one step of the 15-step provisioning orchestrator.
// The SSE hub reads these rows and streams updates to the browser in real time.
type ProvisioningStep struct {
	ID         uuid.UUID  `db:"id"          json:"id"`
	ProjectID  uuid.UUID  `db:"project_id"  json:"project_id"`
	StepIndex  int        `db:"step_index"  json:"step_index"`
	Label      string     `db:"label"       json:"label"`
	Status     string     `db:"status"      json:"status"`
	Detail     *string    `db:"detail"      json:"detail,omitempty"`
	StartedAt  *time.Time `db:"started_at"  json:"started_at,omitempty"`
	FinishedAt *time.Time `db:"finished_at" json:"finished_at,omitempty"`
}

// AuditEvent is an immutable record of a significant action in devportal.
// Rows are never updated or deleted — append-only log.
type AuditEvent struct {
	ID           uuid.UUID  `db:"id"`
	OrgID        uuid.UUID  `db:"org_id"`
	UserID       *uuid.UUID `db:"user_id"`        // nullable — system actions have no user
	Action       string     `db:"action"`         // e.g. "project.create", "credential.delete"
	ResourceType string     `db:"resource_type"`  // e.g. "project" | "team" | "credential"
	ResourceID   *uuid.UUID `db:"resource_id"`    // references the affected row
	Detail       []byte     `db:"detail"`         // JSON payload with action-specific context
	CreatedAt    time.Time  `db:"created_at"`
}

// ── Platform Engineering — added in migration 007 ─────────────────────────────

// Cluster is a registered Kubernetes cluster (one per environment tier per org).
// Credentials are stored encrypted in workspace_credentials and referenced by ID.
type Cluster struct {
	ID          uuid.UUID  `db:"id"           json:"id"`
	OrgID       uuid.UUID  `db:"org_id"       json:"org_id"`
	Name        string     `db:"name"         json:"name"`
	DisplayName string     `db:"display_name" json:"display_name"`
	// Environment is the tier this cluster serves: "dev" | "uat" | "prod"
	Environment             string     `db:"environment"                json:"environment"`
	APIEndpoint             string     `db:"api_endpoint"               json:"api_endpoint"`
	ArgoCDURL               string     `db:"argocd_url"                 json:"argocd_url"`
	KubeconfigCredentialID  *uuid.UUID `db:"kubeconfig_credential_id"   json:"kubeconfig_credential_id,omitempty"`
	ArgoCDCredentialID      *uuid.UUID `db:"argocd_credential_id"       json:"argocd_credential_id,omitempty"`
	Status                  string     `db:"status"                     json:"status"`
	CreatedAt               time.Time  `db:"created_at"                 json:"created_at"`
	CreatedBy               *uuid.UUID `db:"created_by"                 json:"created_by,omitempty"`
}

// ClusterPlatformService holds per-cluster configuration for one shared infra
// service (CNPG, Kafka, MinIO, Redis, RabbitMQ, Vault, Gateway).
// Config is JSONB — each service_type has a different shape (see migration 007).
type ClusterPlatformService struct {
	ID          uuid.UUID       `db:"id"           json:"id"`
	ClusterID   uuid.UUID       `db:"cluster_id"   json:"cluster_id"`
	ServiceType string          `db:"service_type" json:"service_type"`
	Enabled     bool            `db:"enabled"      json:"enabled"`
	Config      json.RawMessage `db:"config"       json:"config"`
	UpdatedAt   time.Time       `db:"updated_at"   json:"updated_at"`
	UpdatedBy   *uuid.UUID      `db:"updated_by"   json:"updated_by,omitempty"`
}

// ManifestTemplate is one of the 15 standard K8s YAML templates.
// Content holds the raw YAML with %%%MARKER%%% substitution tokens.
// Same DB-editable design as PipelineTemplate.
type ManifestTemplate struct {
	Name        string     `db:"name"         json:"name"`
	DisplayName string     `db:"display_name" json:"display_name"`
	// Conditional controls when the template is rendered:
	// "" = always, "cnpg" = only if postgres selected, "prod" = only for prod env, etc.
	Conditional string     `db:"conditional"  json:"conditional"`
	Content     string     `db:"content"      json:"content"`
	UpdatedAt   time.Time  `db:"updated_at"   json:"updated_at"`
	UpdatedBy   *uuid.UUID `db:"updated_by"   json:"updated_by,omitempty"`
}

// EnvironmentProfile holds resource limits and HPA settings for one environment
// tier (dev / uat / prod). DevPortal substitutes these into Deployment templates.
type EnvironmentProfile struct {
	Name         string     `db:"name"          json:"name"`
	CPURequest   string     `db:"cpu_request"   json:"cpu_request"`
	MemRequest   string     `db:"mem_request"   json:"mem_request"`
	CPULimit     string     `db:"cpu_limit"     json:"cpu_limit"`
	MemLimit     string     `db:"mem_limit"     json:"mem_limit"`
	Replicas     int        `db:"replicas"      json:"replicas"`
	StorageClass string     `db:"storage_class" json:"storage_class"`
	HPAEnabled   bool       `db:"hpa_enabled"   json:"hpa_enabled"`
	HPAMin       int        `db:"hpa_min"       json:"hpa_min"`
	HPAMax       int        `db:"hpa_max"       json:"hpa_max"`
	CPUThreshold int        `db:"cpu_threshold" json:"cpu_threshold"`
	MemThreshold int        `db:"mem_threshold" json:"mem_threshold"`
	UpdatedAt    time.Time  `db:"updated_at"    json:"updated_at"`
	UpdatedBy    *uuid.UUID `db:"updated_by"    json:"updated_by,omitempty"`
}

// LanguageProfile holds Kubernetes deployment tuning that varies by build tool
// (probe timing, language-specific env vars). Resource quotas are stored separately
// in environment_profiles and scale per environment tier.
type LanguageProfile struct {
	BuildTool      string            `db:"build_tool"      json:"build_tool"`
	DisplayName    string            `db:"display_name"    json:"display_name"`
	LivenessDelay  int               `db:"liveness_delay"  json:"liveness_delay"`
	ReadinessDelay int               `db:"readiness_delay" json:"readiness_delay"`
	ExtraEnv       map[string]string `db:"extra_env"       json:"extra_env"`
	UpdatedAt      time.Time         `db:"updated_at"      json:"updated_at"`
	UpdatedBy      *uuid.UUID        `db:"updated_by"      json:"updated_by,omitempty"`
}

// ServiceInfraRequirement records one infrastructure selection made by a
// developer at service-registration time (e.g. order-service → postgres).
// One row per service per infra type (UNIQUE project_id, service_type).
type ServiceInfraRequirement struct {
	ID          uuid.UUID       `db:"id"           json:"id"`
	ProjectID   uuid.UUID       `db:"project_id"   json:"project_id"`
	ServiceType string          `db:"service_type" json:"service_type"`
	Config      json.RawMessage `db:"config"       json:"config"`
	Provisioned bool            `db:"provisioned"  json:"provisioned"`
	VaultPath   *string         `db:"vault_path"   json:"vault_path,omitempty"`
	CreatedAt   time.Time       `db:"created_at"   json:"created_at"`
}

// ServiceDependency records a directed communication edge between two services
// in the same application (from_project calls to_project on port).
// Used to generate NetworkPolicy CRs at provision time.
type ServiceDependency struct {
	ID          uuid.UUID `db:"id"           json:"id"`
	FromProject uuid.UUID `db:"from_project" json:"from_project"`
	ToProject   uuid.UUID `db:"to_project"   json:"to_project"`
	Port        int       `db:"port"         json:"port"`
	Description string    `db:"description"  json:"description"`
	CreatedAt   time.Time `db:"created_at"   json:"created_at"`
}
