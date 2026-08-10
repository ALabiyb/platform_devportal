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
	if input.StartBranch != "" {
		payload["start_branch"] = input.StartBranch
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

// EnsureGroup creates a GitLab group at root (parentPath="") or as subgroup.
// Returns the numeric group ID. Idempotent — returns existing ID if found.
func (g *GitLabAdapter) EnsureGroup(ctx context.Context, slug, parentPath string) (int, error) {
	path := slug
	if parentPath != "" {
		path = parentPath + "/" + slug
	}
	return g.getNamespaceID(ctx, path)
}

// AddGroupMember looks up the GitLab user by email and adds them to the group.
// accessLevel: 30=Developer, 40=Maintainer. Silently succeeds if user not in GitLab.
func (g *GitLabAdapter) AddGroupMember(ctx context.Context, namespacePath, email string, accessLevel int) error {
	userID, err := g.findUserByEmail(ctx, email)
	if err != nil {
		slog.Warn("gitlab: user not found, skipping group member add", "email", email)
		return nil
	}

	var ns struct{ ID int `json:"id"` }
	if err := g.do(ctx, http.MethodGet, "/namespaces/"+url.PathEscape(namespacePath), nil, &ns); err != nil {
		return fmt.Errorf("gitlab.AddGroupMember: resolve group: %w", err)
	}

	payload := map[string]any{"user_id": userID, "access_level": accessLevel}
	endpoint := fmt.Sprintf("/groups/%d/members", ns.ID)
	if err := g.do(ctx, http.MethodPost, endpoint, payload, nil); err != nil {
		if strings.Contains(err.Error(), "already a member") || strings.Contains(err.Error(), "409") {
			slog.Info("gitlab: user already group member", "email", email, "group", namespacePath)
			return nil
		}
		return fmt.Errorf("gitlab.AddGroupMember: %w", err)
	}
	slog.Info("gitlab: group member added", "email", email, "group", namespacePath)
	return nil
}

// RemoveGroupMember looks up the GitLab user by email and removes them from the group.
func (g *GitLabAdapter) RemoveGroupMember(ctx context.Context, namespacePath, email string) error {
	userID, err := g.findUserByEmail(ctx, email)
	if err != nil {
		slog.Warn("gitlab: user not found, skipping group member remove", "email", email)
		return nil
	}

	var ns struct{ ID int `json:"id"` }
	if err := g.do(ctx, http.MethodGet, "/namespaces/"+url.PathEscape(namespacePath), nil, &ns); err != nil {
		return fmt.Errorf("gitlab.RemoveGroupMember: resolve group: %w", err)
	}

	endpoint := fmt.Sprintf("/groups/%d/members/%d", ns.ID, userID)
	if err := g.do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil {
		if strings.Contains(err.Error(), "404") {
			return nil // already not a member
		}
		return fmt.Errorf("gitlab.RemoveGroupMember: %w", err)
	}
	slog.Info("gitlab: group member removed", "email", email, "group", namespacePath)
	return nil
}

// findUserByEmail searches GitLab for a user by email and returns their numeric ID.
func (g *GitLabAdapter) findUserByEmail(ctx context.Context, email string) (int, error) {
	var users []struct {
		ID    int    `json:"id"`
		Email string `json:"email"`
	}
	if err := g.do(ctx, http.MethodGet, "/users?search="+url.QueryEscape(email), nil, &users); err != nil {
		return 0, fmt.Errorf("search user by email: %w", err)
	}
	for _, u := range users {
		if strings.EqualFold(u.Email, email) {
			return u.ID, nil
		}
	}
	return 0, fmt.Errorf("no GitLab user found with email %q", email)
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
// returns its numeric ID. If the namespace is a group that does not exist yet,
// it is created automatically — idempotent on re-runs.
func (g *GitLabAdapter) getNamespaceID(ctx context.Context, namespacePath string) (int, error) {
	var ns struct {
		ID int `json:"id"`
	}
	err := g.do(ctx, http.MethodGet, "/namespaces/"+url.PathEscape(namespacePath), nil, &ns)
	if err == nil {
		return ns.ID, nil
	}

	// Namespace not found — try to create it as a group.
	// Only single-level paths are auto-created; subgroups require the parent to exist first.
	if !strings.Contains(err.Error(), "404") {
		return 0, fmt.Errorf("resolve namespace: %w", err)
	}

	slog.Info("gitlab: namespace not found, creating group", "path", namespacePath)

	parts := strings.SplitN(namespacePath, "/", 2)
	groupPath := parts[0]
	groupName := groupPath

	payload := map[string]any{
		"name":       groupName,
		"path":       groupPath,
		"visibility": "private",
	}

	// If it's a subgroup (e.g. "nexbridge/backend"), resolve the parent first.
	if len(parts) == 2 {
		parentID, err := g.getNamespaceID(ctx, parts[0])
		if err != nil {
			return 0, fmt.Errorf("resolve parent namespace %q: %w", parts[0], err)
		}
		groupName = parts[1]
		groupPath = parts[1]
		payload["name"] = groupName
		payload["path"] = groupPath
		payload["parent_id"] = parentID
	}

	var created struct {
		ID int `json:"id"`
	}
	if err := g.do(ctx, http.MethodPost, "/groups", payload, &created); err != nil {
		// Another process may have created it concurrently — re-fetch.
		if strings.Contains(err.Error(), "already been taken") || strings.Contains(err.Error(), "already exists") {
			if err2 := g.do(ctx, http.MethodGet, "/namespaces/"+url.PathEscape(namespacePath), nil, &ns); err2 == nil {
				return ns.ID, nil
			}
		}
		return 0, fmt.Errorf("create group %q: %w", namespacePath, err)
	}

	slog.Info("gitlab: group created", "path", namespacePath, "id", created.ID)
	return created.ID, nil
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
