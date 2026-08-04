// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// Package plugin defines the interfaces the provisioning orchestrator uses
// to call external services, and the shared input/output types.
//
// Six provider interfaces:
//   GitProvider      — repository lifecycle, file commits, webhooks
//   CIProvider       — Jenkins folder + multibranch job management
//   RegistryProvider — Harbor image registry project + robot accounts
//   SecurityProvider — DefectDojo product + engagement management
//   GitOpsProvider   — ArgoCD Application per environment
//   DBProvider       — CREATE DATABASE / USER / GRANT for app databases
//
// Design rules:
//   - Interfaces are defined once here; adapters live in separate files.
//   - All methods accept a context.Context as the first argument.
//   - Methods are idempotent: calling them twice must not produce errors.
//     (e.g. EnsureRepo returns the existing repo if it already exists)
//   - Input structs carry only what is needed — no config duplication.
//   - Stub adapters for Day 06-08 return ErrNotImplemented so the
//     provisioner compiles and routes are wired before the full impl lands.

package plugin

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by stub adapters that are not yet wired up.
var ErrNotImplemented = errors.New("plugin: not yet implemented")

// ── Interfaces ────────────────────────────────────────────────────────────────

// GitProvider manages source code repositories, file commits, and webhooks.
// Implemented by GitLabAdapter (Day 05). Gitea support planned for a later day.
type GitProvider interface {
	// EnsureRepo creates the repository if it does not exist, or returns the
	// existing repo metadata if it does. Idempotent.
	EnsureRepo(ctx context.Context, input CreateRepoInput) (*RepoResult, error)

	// CommitFiles creates or updates files in a single atomic commit.
	// Uses the batch commits API — one round trip regardless of file count.
	CommitFiles(ctx context.Context, input CommitFilesInput) error

	// EnsureWebhook registers a webhook on the repository. If a webhook with
	// the same URL already exists, it is left unchanged. Idempotent.
	EnsureWebhook(ctx context.Context, input WebhookInput) error

	// EnsureProtectedBranch protects the given branch (prevents force-push,
	// requires MR). Idempotent — no error if already protected.
	EnsureProtectedBranch(ctx context.Context, repoPath, branch string) error
}

// CIProvider manages CI pipeline jobs and folders.
// Implemented by JenkinsAdapter (Day 06).
type CIProvider interface {
	// EnsureFolder creates the Jenkins folder for a team if it does not exist.
	EnsureFolder(ctx context.Context, folderName string) error

	// CreateJob creates a multibranch pipeline job inside the team folder.
	// Returns the job URL for use in GitLab webhook configuration.
	CreateJob(ctx context.Context, input CreateJobInput) (*JobResult, error)

	// TriggerBranchScan tells Jenkins to scan the repository for branches
	// so the pipeline starts immediately after job creation.
	TriggerBranchScan(ctx context.Context, jobPath string) error
}

// RegistryProvider manages container image registry projects and credentials.
// Implemented by HarborAdapter (Day 07).
type RegistryProvider interface {
	// EnsureProject creates a Harbor project (image namespace) if it does not
	// exist. Idempotent.
	EnsureProject(ctx context.Context, projectName string) error

	// EnsureRobotAccount creates a robot account with push+pull access to the
	// project. Returns the credentials Jenkins uses to push images.
	EnsureRobotAccount(ctx context.Context, projectName, robotName string) (*RobotCredential, error)
}

// SecurityProvider manages security scan products and engagements.
// Implemented by DefectDojoAdapter (Day 07).
type SecurityProvider interface {
	// EnsureProduct creates a DefectDojo product (top-level container for all
	// findings). Returns the product ID for subsequent engagement creation.
	EnsureProduct(ctx context.Context, name, description string) (int, error)

	// CreateEngagement creates a CI/CD engagement under the product.
	// Each project environment gets its own engagement.
	CreateEngagement(ctx context.Context, input CreateEngagementInput) (int, error)
}

// GitOpsProvider manages ArgoCD Applications for GitOps deployment.
// Implemented by ArgoCDAdapter (Day 08).
type GitOpsProvider interface {
	// CreateApplication creates an ArgoCD Application pointing at the manifest
	// repo path for a specific environment. Idempotent.
	CreateApplication(ctx context.Context, input CreateAppInput) error

	// SyncApplication triggers an immediate sync for the Application.
	SyncApplication(ctx context.Context, appName string) error

	// GetApplicationStatus returns the current health and sync status.
	GetApplicationStatus(ctx context.Context, appName string) (*AppStatus, error)
}

// DBProvider provisions databases for developer applications.
// Implemented by PostgresDBAdapter (Day 08).
type DBProvider interface {
	// EnsureDatabase creates the database if it does not exist. Idempotent.
	EnsureDatabase(ctx context.Context, dbName string) error

	// EnsureUser creates the database user if it does not exist. Idempotent.
	EnsureUser(ctx context.Context, username, password string) error

	// GrantPrivileges grants ALL PRIVILEGES on the database to the user.
	GrantPrivileges(ctx context.Context, dbName, username string) error
}

// ── Input / Output types ──────────────────────────────────────────────────────

// CreateRepoInput is the input for GitProvider.EnsureRepo.
type CreateRepoInput struct {
	Name          string // repository name, e.g. "payment-service"
	Description   string
	NamespacePath string // GitLab group path, e.g. "nexbridge/backend"
	Visibility    string // "private" | "internal" | "public" — default "private"
	DefaultBranch string // default "main"
}

// RepoResult is returned by GitProvider.EnsureRepo.
type RepoResult struct {
	ID            int
	HTTPURL       string // clone URL for CI/CD credentials
	SSHURL        string
	WebURL        string // browser URL stored in projects table
	DefaultBranch string
}

// CommitFilesInput is the input for GitProvider.CommitFiles.
type CommitFilesInput struct {
	RepoPath    string       // full project path, e.g. "nexbridge/payment-service"
	Branch      string       // target branch, e.g. "main"
	Message     string       // commit message
	AuthorName  string       // git author name
	AuthorEmail string       // git author email
	Files       []CommitFile // files to create or update
}

// CommitFile is one file in a CommitFilesInput.
type CommitFile struct {
	Path    string // file path in repository, e.g. "Jenkinsfile"
	Content string // file content (UTF-8 text)
	// Action controls whether the file is created or updated.
	// "create" (default): file must not already exist.
	// "update": file must already exist.
	// "upsert": try create, fall back to update if file already exists.
	Action string
}

// WebhookInput is the input for GitProvider.EnsureWebhook.
type WebhookInput struct {
	RepoPath         string   // full project path
	URL              string   // webhook endpoint, e.g. Jenkins multibranch scan URL
	SecretToken      string   // HMAC token Jenkins uses to verify the payload
	PushEvents       bool     // trigger on push
	MREvents         bool     // trigger on merge request open/update
	TagEvents        bool     // trigger on tag creation
	SSLVerify        bool     // set false when Jenkins uses self-signed cert
}

// CreateJobInput is the input for CIProvider.CreateJob.
type CreateJobInput struct {
	FolderName    string // Jenkins folder (team name)
	JobName       string // job name within the folder
	GitURL        string // HTTPS clone URL of the application repo
	CredentialsID string // Jenkins credential ID for git clone
	Description   string
}

// JobResult is returned by CIProvider.CreateJob.
type JobResult struct {
	Path string // folder/jobName — used to build the webhook URL
	URL  string // public Jenkins job URL shown to developers
}

// RobotCredential is returned by RegistryProvider.EnsureRobotAccount.
type RobotCredential struct {
	Name   string // robot account name including "robot$" prefix
	Secret string // robot account secret — store encrypted via Crypto module
}

// CreateEngagementInput is the input for SecurityProvider.CreateEngagement.
type CreateEngagementInput struct {
	ProductID      int
	Name           string // e.g. "payment-service CI/CD"
	Description    string
	StartDate      string // YYYY-MM-DD
	EndDate        string // YYYY-MM-DD (typically 1 year ahead)
	EngagementType string // "CI/CD" | "Interactive"
}

// CreateAppInput is the input for GitOpsProvider.CreateApplication.
type CreateAppInput struct {
	Name           string // ArgoCD Application name, e.g. "payment-service-dev"
	Namespace      string // destination K8s namespace
	RepoURL        string // manifest repo HTTPS URL
	Path           string // directory in manifest repo, e.g. "projects/payment-service/dev"
	TargetRevision string // branch or "HEAD"
	Project        string // ArgoCD project — "default" if not set
	Server         string // K8s cluster URL — "https://kubernetes.default.svc" for in-cluster
	AutoSync       bool   // enable automated sync policy
}

// AppStatus is returned by GitOpsProvider.GetApplicationStatus.
type AppStatus struct {
	Health string // "Healthy" | "Degraded" | "Progressing" | "Missing" | "Unknown"
	Sync   string // "Synced" | "OutOfSync"
}
