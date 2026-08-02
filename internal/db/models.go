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
	"time"

	"github.com/google/uuid"
)

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
	ID        uuid.UUID `db:"id"`
	OrgID     uuid.UUID `db:"org_id"`
	Name      string    `db:"name"`
	Slug      string    `db:"slug"` // URL-safe, unique within the org
	CreatedAt time.Time `db:"created_at"`
}

// TeamMember links a user to a team with a role.
type TeamMember struct {
	TeamID uuid.UUID `db:"team_id"`
	UserID uuid.UUID `db:"user_id"`
	Role   string    `db:"role"` // "lead" | "member"
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
	ID                    uuid.UUID  `db:"id"`
	TeamID                uuid.UUID  `db:"team_id"`
	Name                  string     `db:"name"`
	Slug                  string     `db:"slug"`             // URL-safe, unique within the team
	GitRepoURL            string     `db:"git_repo_url"`     // developer's source repository
	HarborProject         string     `db:"harbor_project"`   // Harbor image registry project
	JenkinsFolder         string     `db:"jenkins_folder"`   // Jenkins folder containing this job
	BuildTool             string     `db:"build_tool"`       // "maven"|"gradle"|"go"|"nodejs-express"|etc.
	NotificationEmail     string     `db:"notification_email"`
	DefectDojoProductID   *int       `db:"defectdojo_product_id"`    // set after DefectDojo provisioning
	Status                string     `db:"status"`                   // "provisioning"|"active"|"failed"|"archived"
	GeneratedJenkinsfile  *string    `db:"generated_jenkinsfile"`    // stored for re-download
	ManifestRepoURL       *string    `db:"manifest_repo_url"`
	AppRepoURL            *string    `db:"app_repo_url"`
	CreatedAt             time.Time  `db:"created_at"`
	CreatedBy             *uuid.UUID `db:"created_by"`
}

// Environment is one deployment tier (dev / uat / prod) for a project.
// Each environment gets its own K8s namespace, ArgoCD Application, and database.
type Environment struct {
	ID             uuid.UUID `db:"id"`
	ProjectID      uuid.UUID `db:"project_id"`
	Name           string    `db:"name"`             // "dev" | "uat" | "prod"
	Namespace      string    `db:"namespace"`        // K8s namespace, e.g. "payment-service-dev"
	IngressURL     *string   `db:"ingress_url"`      // public URL for this environment
	ArgoCDAppName  *string   `db:"argocd_app_name"`  // ArgoCD Application resource name
	DBName         *string   `db:"db_name"`          // provisioned database name
	DBUsername     *string   `db:"db_username"`      // provisioned database user
	ManifestPath   *string   `db:"manifest_path"`    // directory in manifest repo
	Status         string    `db:"status"`           // "provisioning" | "active" | "failed"
	CreatedAt      time.Time `db:"created_at"`
}

// ProvisioningStep tracks one step of the 15-step provisioning orchestrator.
// The SSE hub reads these rows and streams updates to the browser in real time.
type ProvisioningStep struct {
	ID         uuid.UUID  `db:"id"`
	ProjectID  uuid.UUID  `db:"project_id"`
	StepIndex  int        `db:"step_index"`   // 1-based ordering
	Label      string     `db:"label"`        // human-readable name shown in the UI
	Status     string     `db:"status"`       // "pending" | "running" | "done" | "failed"
	Detail     *string    `db:"detail"`       // output message or error from the step
	StartedAt  *time.Time `db:"started_at"`
	FinishedAt *time.Time `db:"finished_at"`
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
