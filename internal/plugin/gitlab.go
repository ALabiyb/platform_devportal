// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// gitlab.go implements GitProvider against the GitLab / Gitea REST API.
//
// Authentication: PRIVATE-TOKEN header (Personal Access Token, scope: api).
//
// All methods are idempotent — safe to call multiple times without side effects:
//   EnsureRepo             → returns existing repo if it already exists
//   CommitFiles            → action:"upsert" creates or updates each file
//   EnsureWebhook          → skips creation if a matching URL already exists
//   EnsureProtectedBranch  → ignores "already protected" errors from the API

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// GitLabAdapter implements GitProvider using the GitLab REST API v4.
// It also works with Gitea (which mirrors the GitLab API surface for
// most of the endpoints used here).
type GitLabAdapter struct {
	client  *http.Client
	baseURL string // e.g. "https://gitlab.example.com/api/v4"
	token   string // Personal Access Token (scope: api)
}

// NewGitLabAdapter constructs a GitLabAdapter from config and a shared HTTP client.
// The HTTP client should be the one built in main.go (TLS skip configured there).
func NewGitLabAdapter(cfg *config.Config, httpClient *http.Client) *GitLabAdapter {
	return &GitLabAdapter{
		client:  httpClient,
		baseURL: strings.TrimRight(cfg.GitLabURL, "/") + "/api/v4",
		token:   cfg.GitLabToken,
	}
}

// ── GitProvider implementation ────────────────────────────────────────────────

// EnsureRepo creates the repository if it does not exist, or returns the
// existing repo metadata if it does. Safe to call on every provisioning run.
func (g *GitLabAdapter) EnsureRepo(ctx context.Context, input CreateRepoInput) (*RepoResult, error) {
	fullPath := input.NamespacePath + "/" + input.Name

	// Check if the project already exists.
	existing, err := g.getProject(ctx, fullPath)
	if err == nil {
		slog.Info("gitlab: repo already exists", "path", fullPath)
		return existing, nil
	}

	// Look up the namespace ID so we can create the project inside the group.
	nsID, err := g.getNamespaceID(ctx, input.NamespacePath)
	if err != nil {
		return nil, fmt.Errorf("gitlab.EnsureRepo: resolve namespace %q: %w", input.NamespacePath, err)
	}

	visibility := input.Visibility
	if visibility == "" {
		visibility = "private"
	}
	branch := input.DefaultBranch
	if branch == "" {
		branch = "main"
	}

	payload := map[string]any{
		"name":                   input.Name,
		"path":                   input.Name,
		"description":            input.Description,
		"namespace_id":           nsID,
		"visibility":             visibility,
		"default_branch":         branch,
		"initialize_with_readme": false, // we commit our own files
		"auto_devops_enabled":    false,
	}

	var created glProject
	if err := g.do(ctx, http.MethodPost, "/projects", payload, &created); err != nil {
		return nil, fmt.Errorf("gitlab.EnsureRepo: create project: %w", err)
	}

	slog.Info("gitlab: repo created", "path", fullPath, "id", created.ID)
	return toRepoResult(created), nil
}

// CommitFiles creates or updates files in a single atomic commit using the
// GitLab batch commits API. One HTTP round trip regardless of file count.
//
// CommitFile.Action controls behaviour per file:
//   "create"  — file must not exist (default)
//   "update"  — file must exist
//   "upsert"  — try create, fall back to update if already present (safest for re-runs)
func (g *GitLabAdapter) CommitFiles(ctx context.Context, input CommitFilesInput) error {
	// Resolve project to its numeric ID (commits API requires :id not :path).
	project, err := g.getProject(ctx, input.RepoPath)
	if err != nil {
		return fmt.Errorf("gitlab.CommitFiles: resolve project: %w", err)
	}

	type glAction struct {
		Action   string `json:"action"`
		FilePath string `json:"file_path"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"` // "text" (default) or "base64"
	}

	actions := make([]glAction, 0, len(input.Files))
	for _, f := range input.Files {
		action := f.Action
		if action == "" || action == "upsert" {
			action = "create" // will retry as "update" below if needed
		}
		actions = append(actions, glAction{
			Action:   action,
			FilePath: f.Path,
			Content:  f.Content,
			Encoding: "text",
		})
	}

	payload := map[string]any{
		"branch":         input.Branch,
		"commit_message": input.Message,
		"author_name":    input.AuthorName,
		"author_email":   input.AuthorEmail,
		"actions":        actions,
	}

	endpoint := fmt.Sprintf("/projects/%d/repository/commits", project.ID)
	err = g.do(ctx, http.MethodPost, endpoint, payload, nil)
	if err == nil {
		slog.Info("gitlab: files committed", "repo", input.RepoPath, "files", len(input.Files))
		return nil
	}

	// If any file already existed and action was "upsert", retry everything as "update".
	if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "A file with this name already exists") {
		for i := range actions {
			if input.Files[i].Action == "upsert" || input.Files[i].Action == "" {
				actions[i].Action = "update"
			}
		}
		payload["actions"] = actions
		if retryErr := g.do(ctx, http.MethodPost, endpoint, payload, nil); retryErr != nil {
			return fmt.Errorf("gitlab.CommitFiles: upsert retry: %w", retryErr)
		}
		slog.Info("gitlab: files updated (upsert)", "repo", input.RepoPath)
		return nil
	}

	return fmt.Errorf("gitlab.CommitFiles: %w", err)
}

// EnsureWebhook registers a Jenkins webhook on the repository.
// If a webhook with the same URL is already registered, it is left unchanged.
func (g *GitLabAdapter) EnsureWebhook(ctx context.Context, input WebhookInput) error {
	project, err := g.getProject(ctx, input.RepoPath)
	if err != nil {
		return fmt.Errorf("gitlab.EnsureWebhook: resolve project: %w", err)
	}

	hooksURL := fmt.Sprintf("/projects/%d/hooks", project.ID)

	// List existing hooks and check for a matching URL.
	var hooks []struct {
		ID  int    `json:"id"`
		URL string `json:"url"`
	}
	if err := g.do(ctx, http.MethodGet, hooksURL, nil, &hooks); err != nil {
		return fmt.Errorf("gitlab.EnsureWebhook: list hooks: %w", err)
	}
	for _, h := range hooks {
		if h.URL == input.URL {
			slog.Info("gitlab: webhook already registered", "repo", input.RepoPath, "url", input.URL)
			return nil
		}
	}

	payload := map[string]any{
		"url":                    input.URL,
		"push_events":            input.PushEvents,
		"merge_requests_events":  input.MREvents,
		"tag_push_events":        input.TagEvents,
		"enable_ssl_verification": input.SSLVerify,
	}
	if input.SecretToken != "" {
		payload["token"] = input.SecretToken
	}

	if err := g.do(ctx, http.MethodPost, hooksURL, payload, nil); err != nil {
		return fmt.Errorf("gitlab.EnsureWebhook: create hook: %w", err)
	}

	slog.Info("gitlab: webhook registered", "repo", input.RepoPath, "url", input.URL)
	return nil
}

// EnsureProtectedBranch protects the given branch against force-push and
// requires merge requests for all changes. Idempotent — ignores "already
// protected" errors from the API.
//
// Access levels: 0=No access, 30=Developer, 40=Maintainer.
// We default to Maintainer push (40) and Developer merge (30) which matches
// the standard GitLab workflow: devs open MRs, maintainers merge to main.
func (g *GitLabAdapter) EnsureProtectedBranch(ctx context.Context, repoPath, branch string) error {
	project, err := g.getProject(ctx, repoPath)
	if err != nil {
		return fmt.Errorf("gitlab.EnsureProtectedBranch: resolve project: %w", err)
	}

	payload := map[string]any{
		"name":                branch,
		"push_access_level":   40, // Maintainer
		"merge_access_level":  30, // Developer
		"allow_force_push":    false,
	}

	endpoint := fmt.Sprintf("/projects/%d/protected_branches", project.ID)
	err = g.do(ctx, http.MethodPost, endpoint, payload, nil)
	if err != nil {
		// 409 Conflict means "already protected" — treat as success.
		if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "already exists") {
			slog.Info("gitlab: branch already protected", "repo", repoPath, "branch", branch)
			return nil
		}
		return fmt.Errorf("gitlab.EnsureProtectedBranch: %w", err)
	}

	slog.Info("gitlab: branch protected", "repo", repoPath, "branch", branch)
	return nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// glProject mirrors the subset of GitLab project fields we use.
type glProject struct {
	ID            int    `json:"id"`
	HTTPURLToRepo string `json:"http_url_to_repo"`
	SSHURLToRepo  string `json:"ssh_url_to_repo"`
	WebURL        string `json:"web_url"`
	DefaultBranch string `json:"default_branch"`
}

// getProject fetches a project by its full path (namespace/name).
func (g *GitLabAdapter) getProject(ctx context.Context, fullPath string) (*RepoResult, error) {
	var p glProject
	if err := g.do(ctx, http.MethodGet, "/projects/"+url.PathEscape(fullPath), nil, &p); err != nil {
		return nil, err
	}
	return toRepoResult(p), nil
}

// getNamespaceID resolves a GitLab namespace (group or user) by its path and
// returns its numeric ID. The ID is required when creating a project inside a group.
func (g *GitLabAdapter) getNamespaceID(ctx context.Context, namespacePath string) (int, error) {
	var ns struct {
		ID int `json:"id"`
	}
	if err := g.do(ctx, http.MethodGet, "/namespaces/"+url.PathEscape(namespacePath), nil, &ns); err != nil {
		return 0, err
	}
	return ns.ID, nil
}

// toRepoResult converts an internal glProject to the public RepoResult type.
func toRepoResult(p glProject) *RepoResult {
	return &RepoResult{
		ID:            p.ID,
		HTTPURL:       p.HTTPURLToRepo,
		SSHURL:        p.SSHURLToRepo,
		WebURL:        p.WebURL,
		DefaultBranch: p.DefaultBranch,
	}
}

// toRepoResultFromProject is an alias used when glProject is a value not pointer.
func toRepoResultFromProject(p glProject) *RepoResult { return toRepoResult(p) }

// do executes an HTTP request against the GitLab API.
// It sets the PRIVATE-TOKEN header, marshals the body as JSON (when non-nil),
// and unmarshals the response into out (when non-nil).
// Returns a descriptive error for non-2xx responses including the HTTP status.
func (g *GitLabAdapter) do(ctx context.Context, method, path string, body, out any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, g.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", g.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	if out != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
