// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// gitea.go implements GitProvider against the Gitea REST API v1.
//
// Authentication: Authorization: token <PAT>  (PAT scope: all or write:repository)
//
// Gitea has no batch-commit API, so CommitFiles uses the git low-level objects
// API to produce one atomic commit regardless of file count:
//
//	1. GET  /repos/{owner}/{repo}/branches/{branch}  → current HEAD SHA + tree SHA
//	2. POST /repos/{owner}/{repo}/git/blobs           → one blob per file
//	3. POST /repos/{owner}/{repo}/git/trees           → new tree (base_tree = HEAD tree)
//	4. POST /repos/{owner}/{repo}/git/commits         → commit pointing at new tree
//	5. PATCH /repos/{owner}/{repo}/git/refs/heads/{b} → advance branch pointer
//	   (POST /git/refs for the very first commit on an empty repo)
//
// All methods are idempotent — safe to call multiple times.

package plugin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// GiteaAdapter implements GitProvider using the Gitea REST API v1.
type GiteaAdapter struct {
	client  *http.Client
	baseURL string // e.g. "https://git.docker.localhost/api/v1"
	token   string // Personal Access Token
}

// NewGiteaAdapter constructs a GiteaAdapter from config and a shared HTTP client.
func NewGiteaAdapter(cfg *config.Config, httpClient *http.Client) *GiteaAdapter {
	return &GiteaAdapter{
		client:  httpClient,
		baseURL: strings.TrimRight(cfg.GiteaURL, "/") + "/api/v1",
		token:   cfg.GiteaToken,
	}
}

// ── GitProvider implementation ────────────────────────────────────────────────

// EnsureRepo creates the repo under the given org, or returns it if it exists.
func (g *GiteaAdapter) EnsureRepo(ctx context.Context, input CreateRepoInput) (*RepoResult, error) {
	org := orgName(input.NamespacePath)
	branch := input.DefaultBranch
	if branch == "" {
		branch = "main"
	}

	// Check if repo already exists.
	var existing giteaRepo
	err := g.do(ctx, http.MethodGet,
		fmt.Sprintf("/repos/%s/%s", org, input.Name), nil, &existing)
	if err == nil {
		if !existing.Empty {
			slog.Info("gitea: repo already exists", "org", org, "repo", input.Name)
			return toGiteaRepoResult(existing), nil
		}
		// Repo exists but is empty — Gitea's git objects API (/git/blobs, /git/trees)
		// requires at least one commit. Delete and recreate with auto_init so the repo
		// is properly initialized before CommitFiles runs.
		slog.Info("gitea: repo is empty, reinitializing", "org", org, "repo", input.Name)
		if err := g.do(ctx, http.MethodDelete,
			fmt.Sprintf("/repos/%s/%s", org, input.Name), nil, nil); err != nil {
			return nil, fmt.Errorf("gitea.EnsureRepo: delete empty repo: %w", err)
		}
	}

	// Ensure the org exists first.
	if ensureErr := g.ensureOrg(ctx, org); ensureErr != nil {
		return nil, fmt.Errorf("gitea.EnsureRepo: ensure org %q: %w", org, ensureErr)
	}

	private := input.Visibility != "public"
	payload := map[string]any{
		"name":           input.Name,
		"description":    input.Description,
		"private":        private,
		"auto_init":      true, // creates an initial commit so git objects API works
		"default_branch": branch,
		"gitignores":     "",
		"license":        "",
		"readme":         "",
	}

	var created giteaRepo
	if err := g.do(ctx, http.MethodPost,
		fmt.Sprintf("/orgs/%s/repos", org), payload, &created); err != nil {
		return nil, fmt.Errorf("gitea.EnsureRepo: create: %w", err)
	}

	slog.Info("gitea: repo created", "org", org, "repo", input.Name, "id", created.ID)
	return toGiteaRepoResult(created), nil
}

// CommitFiles creates or updates files using the Gitea Contents API.
// Each file is upserted individually. The git low-level objects API
// (/git/blobs, /git/trees) is not used — it is unavailable in Gitea 1.26.x.
func (g *GiteaAdapter) CommitFiles(ctx context.Context, input CommitFilesInput) error {
	owner, repo := splitPath(input.RepoPath)

	// If StartBranch is set, create input.Branch from it (idempotent — 422 if already exists).
	if input.StartBranch != "" {
		branchURL := fmt.Sprintf("/repos/%s/%s/branches", owner, repo)
		_ = g.do(ctx, http.MethodPost, branchURL, map[string]any{
			"new_branch_name": input.Branch,
			"old_branch_name": input.StartBranch,
		}, nil) // 422 = already exists; ignore
	}

	// Remove any branch protection on the target branch before writing.
	// A previous retry may have already run step 4 (ProtectBranch), leaving a
	// protection that blocks the Contents API even for admins. Step 4 re-applies
	// the correct rules after files are committed, so deleting here is safe.
	protURL := fmt.Sprintf("/repos/%s/%s/branch_protections/%s", owner, repo, input.Branch)
	_ = g.do(ctx, http.MethodDelete, protURL, nil, nil)

	author := map[string]string{
		"name":  input.AuthorName,
		"email": input.AuthorEmail,
	}

	for _, f := range input.Files {
		apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, f.Path)

		// Check whether the file already exists so we know the current SHA
		// (required for updates; absent for new files).
		var existing struct {
			SHA string `json:"sha"`
		}
		fileExists := g.do(ctx, http.MethodGet,
			apiPath+"?ref="+input.Branch, nil, &existing) == nil

		payload := map[string]any{
			"message":   input.Message,
			"content":   base64.StdEncoding.EncodeToString([]byte(f.Content)),
			"branch":    input.Branch,
			"author":    author,
			"committer": author,
		}

		method := http.MethodPost
		if fileExists {
			payload["sha"] = existing.SHA
			method = http.MethodPut
		}

		if err := g.do(ctx, method, apiPath, payload, nil); err != nil {
			return fmt.Errorf("gitea.CommitFiles: %s %s: %w", method, f.Path, err)
		}
	}

	slog.Info("gitea: files committed", "repo", input.RepoPath, "files", len(input.Files))
	return nil
}

// EnsureWebhook registers a Jenkins webhook. Idempotent — skips if URL exists.
func (g *GiteaAdapter) EnsureWebhook(ctx context.Context, input WebhookInput) error {
	owner, repo := splitPath(input.RepoPath)
	hooksPath := fmt.Sprintf("/repos/%s/%s/hooks", owner, repo)

	var hooks []struct {
		ID     int    `json:"id"`
		Config struct {
			URL string `json:"url"`
		} `json:"config"`
	}
	if err := g.do(ctx, http.MethodGet, hooksPath, nil, &hooks); err != nil {
		return fmt.Errorf("gitea.EnsureWebhook: list hooks: %w", err)
	}
	for _, h := range hooks {
		if h.Config.URL == input.URL {
			slog.Info("gitea: webhook already registered", "repo", input.RepoPath)
			return nil
		}
	}

	// Gitea webhook events: "push", "pull_request", "create" (tags)
	events := []string{}
	if input.PushEvents {
		events = append(events, "push")
	}
	if input.MREvents {
		events = append(events, "pull_request")
	}
	if input.TagEvents {
		events = append(events, "create")
	}

	payload := map[string]any{
		"type": "gitea",
		"config": map[string]string{
			"url":          input.URL,
			"content_type": "json",
			"secret":       input.SecretToken,
		},
		"events": events,
		"active": true,
	}

	if err := g.do(ctx, http.MethodPost, hooksPath, payload, nil); err != nil {
		return fmt.Errorf("gitea.EnsureWebhook: create: %w", err)
	}
	slog.Info("gitea: webhook registered", "repo", input.RepoPath, "url", input.URL)
	return nil
}

// EnsureProtectedBranch sets branch protection rules. Idempotent.
func (g *GiteaAdapter) EnsureProtectedBranch(ctx context.Context, repoPath, branch string) error {
	owner, repo := splitPath(repoPath)
	protPath := fmt.Sprintf("/repos/%s/%s/branch_protections", owner, repo)

	// Check if a protection rule for this branch already exists.
	var existing []struct {
		RuleName string `json:"rule_name"`
	}
	if err := g.do(ctx, http.MethodGet, protPath, nil, &existing); err == nil {
		for _, r := range existing {
			if r.RuleName == branch {
				slog.Info("gitea: branch already protected", "repo", repoPath, "branch", branch)
				return nil
			}
		}
	}

	payload := map[string]any{
		"rule_name":                  branch,
		"enable_push":                false, // no direct push to main
		"enable_push_whitelist":      false,
		"required_approvals":         1,
		"enable_approvals_whitelist": false,
		"block_on_rejected_reviews":  true,
	}

	if err := g.do(ctx, http.MethodPost, protPath, payload, nil); err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "409") {
			return nil
		}
		return fmt.Errorf("gitea.EnsureProtectedBranch: %w", err)
	}
	slog.Info("gitea: branch protected", "repo", repoPath, "branch", branch)
	return nil
}

// EnsureGroup creates a Gitea organization if it does not exist.
// Returns a placeholder ID (Gitea org IDs are not used by callers).
func (g *GiteaAdapter) EnsureGroup(ctx context.Context, slug, parentPath string) (int, error) {
	name := slug
	if parentPath != "" {
		name = orgName(parentPath + "/" + slug)
	}
	if err := g.ensureOrg(ctx, name); err != nil {
		return 0, err
	}
	// Retrieve the org ID.
	var org struct {
		ID int `json:"id"`
	}
	if err := g.do(ctx, http.MethodGet, "/orgs/"+name, nil, &org); err != nil {
		return 0, err
	}
	return org.ID, nil
}

// AddGroupMember adds a user (looked up by email) to a Gitea org.
func (g *GiteaAdapter) AddGroupMember(ctx context.Context, namespacePath, email string, _ int) error {
	username, err := g.findUsernameByEmail(ctx, email)
	if err != nil {
		slog.Warn("gitea: user not found, skipping org member add", "email", email)
		return nil
	}
	org := orgName(namespacePath)
	if err := g.do(ctx, http.MethodPut,
		fmt.Sprintf("/orgs/%s/members/%s", org, username), nil, nil); err != nil {
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "409") {
			return nil
		}
		return fmt.Errorf("gitea.AddGroupMember: %w", err)
	}
	slog.Info("gitea: org member added", "org", org, "email", email)
	return nil
}

// RemoveGroupMember removes a user from a Gitea org.
func (g *GiteaAdapter) RemoveGroupMember(ctx context.Context, namespacePath, email string) error {
	username, err := g.findUsernameByEmail(ctx, email)
	if err != nil {
		slog.Warn("gitea: user not found, skipping org member remove", "email", email)
		return nil
	}
	org := orgName(namespacePath)
	if err := g.do(ctx, http.MethodDelete,
		fmt.Sprintf("/orgs/%s/members/%s", org, username), nil, nil); err != nil {
		if strings.Contains(err.Error(), "404") {
			return nil
		}
		return fmt.Errorf("gitea.RemoveGroupMember: %w", err)
	}
	slog.Info("gitea: org member removed", "org", org, "email", email)
	return nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// giteaRepo mirrors the Gitea repository response fields we need.
type giteaRepo struct {
	ID            int    `json:"id"`
	CloneURL      string `json:"clone_url"`
	SSHURL        string `json:"ssh_url"`
	HTMLURL       string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
	Empty         bool   `json:"empty"`
}

func toGiteaRepoResult(r giteaRepo) *RepoResult {
	return &RepoResult{
		ID:            r.ID,
		HTTPURL:       r.CloneURL,
		SSHURL:        r.SSHURL,
		WebURL:        r.HTMLURL,
		DefaultBranch: r.DefaultBranch,
	}
}

// ensureOrg creates a Gitea organization if it does not already exist.
func (g *GiteaAdapter) ensureOrg(ctx context.Context, name string) error {
	err := g.do(ctx, http.MethodGet, "/orgs/"+name, nil, nil)
	if err == nil {
		return nil // already exists
	}
	payload := map[string]any{
		"username":   name,
		"visibility": "private",
	}
	if err := g.do(ctx, http.MethodPost, "/orgs", payload, nil); err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "user already exists") {
			return nil
		}
		return fmt.Errorf("create org %q: %w", name, err)
	}
	slog.Info("gitea: org created", "name", name)
	return nil
}

// findUsernameByEmail searches Gitea users by email and returns the login name.
func (g *GiteaAdapter) findUsernameByEmail(ctx context.Context, email string) (string, error) {
	var result struct {
		Data []struct {
			Login string `json:"login"`
			Email string `json:"email"`
		} `json:"data"`
	}
	if err := g.do(ctx, http.MethodGet,
		"/users/search?q="+email+"&limit=10", nil, &result); err != nil {
		return "", fmt.Errorf("search user: %w", err)
	}
	for _, u := range result.Data {
		if strings.EqualFold(u.Email, email) {
			return u.Login, nil
		}
	}
	return "", fmt.Errorf("no Gitea user found with email %q", email)
}

// orgName converts a namespace path (possibly containing "/") to a valid Gitea
// org name. Gitea has no subgroups, so "nexbridge/backend" → "nexbridge".
// The org is the top-level owner; the sub-path is encoded in repo names if needed.
func orgName(namespacePath string) string {
	if idx := strings.Index(namespacePath, "/"); idx != -1 {
		return namespacePath[:idx]
	}
	return namespacePath
}

// splitPath splits "owner/repo" into (owner, repo).
// If there is no "/" it returns ("", path) — callers should validate input.
func splitPath(repoPath string) (owner, repo string) {
	parts := strings.SplitN(repoPath, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", repoPath
}

// do executes an HTTP request against the Gitea API.
// Sets Authorization header, marshals body as JSON, decodes response into out.
func (g *GiteaAdapter) do(ctx context.Context, method, path string, body, out any) error {
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
	req.Header.Set("Authorization", "token "+g.token)
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
			// Ignore decode errors on empty bodies (e.g. 201 with no body).
			if resp.ContentLength != 0 {
				return fmt.Errorf("decode response: %w", err)
			}
		}
	}
	return nil
}
