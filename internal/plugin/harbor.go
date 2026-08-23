// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// harbor.go implements RegistryProvider against the Harbor v2 REST API.
//
// Auth: HTTP Basic (HARBOR_USER + HARBOR_TOKEN).
// Idempotency:
//
//	EnsureProject      — HEAD-checks; creates with security metadata if missing,
//	                     then applies HarborProjectConfig (auto-scan, retention).
//	EnsureRobotAccount — POST; if 409 then list→delete→re-create for fresh credentials.

package plugin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// HarborAdapter implements RegistryProvider against the Harbor v2 REST API.
type HarborAdapter struct {
	client  *http.Client
	baseURL string // e.g. "https://harbor.example.com"
	auth    string // precomputed "Basic base64(user:token)" header value
}

// NewHarborAdapter constructs a HarborAdapter from config.
func NewHarborAdapter(cfg *config.Config, httpClient *http.Client) *HarborAdapter {
	creds := base64.StdEncoding.EncodeToString([]byte(cfg.HarborUser + ":" + cfg.HarborToken))
	return &HarborAdapter{
		client:  httpClient,
		baseURL: strings.TrimRight(cfg.HarborURL, "/"),
		auth:    "Basic " + creds,
	}
}

// harborProject extracts the Harbor project name from a path like "app/service".
// Harbor projects are the first path component and must be lowercase with no slashes.
// e.g. "restaurant-pos/api-gateway" → "restaurant-pos"
func harborProject(projectPath string) string {
	name := strings.ToLower(projectPath)
	if idx := strings.Index(name, "/"); idx != -1 {
		name = name[:idx]
	}
	return name
}

// EnsureProject creates a Harbor project if it does not already exist, then
// applies the HarborProjectConfig (auto-scan, retention policy). Idempotent.
func (h *HarborAdapter) EnsureProject(ctx context.Context, projectName string, cfg HarborProjectConfig) error {
	projectName = harborProject(projectName)

	// Metadata to apply — Harbor metadata values are strings, not booleans.
	metadata := map[string]string{}
	if cfg.AutoScanOnPush {
		metadata["auto_scan"] = "true"
	}

	// HEAD /api/v2.0/projects?project_name=<name> → 200 = exists, 404 = not found.
	checkURL := fmt.Sprintf("%s/api/v2.0/projects?project_name=%s", h.baseURL, url.QueryEscape(projectName))
	resp, err := h.do(ctx, http.MethodHead, checkURL, "", nil)
	if err != nil {
		return fmt.Errorf("harbor EnsureProject check: %w", err)
	}
	drain(resp.Body)

	if resp.StatusCode == http.StatusOK {
		// Project exists — apply config updates without creating.
		if cfg.AutoScanOnPush {
			_ = h.updateProjectMetadata(ctx, projectName, metadata)
		}
	} else {
		// Project does not exist — create it with metadata in one request.
		body, _ := json.Marshal(map[string]any{
			"project_name": projectName,
			"public":       false,
			"metadata":     metadata,
		})
		resp2, err := h.do(ctx, http.MethodPost, h.baseURL+"/api/v2.0/projects", "application/json", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("harbor EnsureProject create: %w", err)
		}
		defer resp2.Body.Close()
		if resp2.StatusCode != http.StatusCreated && resp2.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp2.Body)
			return fmt.Errorf("harbor EnsureProject: unexpected status %d: %s", resp2.StatusCode, truncate(b, 256))
		}
	}

	// Apply retention policy (best-effort — failure does not abort provisioning).
	if cfg.RetainCount > 0 {
		projectID, err := h.getProjectID(ctx, projectName)
		if err != nil {
			return nil // non-fatal
		}
		_ = h.ensureRetentionPolicy(ctx, projectID, cfg.RetainCount)
	}
	return nil
}

// getProjectID returns the numeric ID of a Harbor project by name.
// Needed for the retention policy API which uses the numeric project ref.
func (h *HarborAdapter) getProjectID(ctx context.Context, projectName string) (int, error) {
	listURL := fmt.Sprintf("%s/api/v2.0/projects?project_name=%s", h.baseURL, url.QueryEscape(projectName))
	resp, err := h.do(ctx, http.MethodGet, listURL, "", nil)
	if err != nil {
		return 0, fmt.Errorf("harbor getProjectID: %w", err)
	}
	defer resp.Body.Close()

	var projects []struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil || len(projects) == 0 {
		return 0, fmt.Errorf("harbor getProjectID: project not found after creation")
	}
	return projects[0].ID, nil
}

// updateProjectMetadata applies metadata changes to an existing Harbor project.
func (h *HarborAdapter) updateProjectMetadata(ctx context.Context, projectName string, metadata map[string]string) error {
	body, _ := json.Marshal(map[string]any{"metadata": metadata})
	patchURL := fmt.Sprintf("%s/api/v2.0/projects/%s", h.baseURL, url.PathEscape(projectName))
	resp, err := h.do(ctx, http.MethodPut, patchURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("harbor updateProjectMetadata: %w", err)
	}
	drain(resp.Body)
	return nil
}

// ensureRetentionPolicy creates a "keep last N images" retention rule for the
// project. Each call creates a new policy — Harbor allows multiple per project.
// Best-effort: caller ignores the return value.
func (h *HarborAdapter) ensureRetentionPolicy(ctx context.Context, projectID, retainCount int) error {
	body, _ := json.Marshal(map[string]any{
		"algorithm": "or",
		"rules": []map[string]any{{
			"disabled": false,
			"action":   "retain",
			"template": "latestPushedK",
			"params":   map[string]any{"latestPushedK": retainCount},
			"scope_selectors": map[string]any{
				"repository": []map[string]any{{
					"kind":       "doublestar",
					"decoration": "repoMatches",
					"pattern":    "**",
				}},
			},
			"tag_selectors": []map[string]any{{
				"kind":       "doublestar",
				"decoration": "matches",
				"pattern":    "**",
			}},
		}},
		"scope": map[string]any{
			"level": "project",
			"ref":   projectID,
		},
		"trigger": map[string]any{
			"kind":     "Schedule",
			"settings": map[string]any{"cron": "0 0 0 * * *"},
		},
	})

	resp, err := h.do(ctx, http.MethodPost, h.baseURL+"/api/v2.0/retentions", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("harbor ensureRetentionPolicy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("harbor ensureRetentionPolicy: status %d: %s", resp.StatusCode, truncate(b, 256))
	}

	// Extract retention ID from Location header: /api/v2.0/retentions/{id}
	loc := resp.Header.Get("Location")
	if loc == "" {
		return nil
	}
	parts := strings.Split(strings.TrimRight(loc, "/"), "/")
	retentionID, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || retentionID == 0 {
		return nil
	}

	// Link retention policy to project via PUT /api/v2.0/projects/{id}
	linkBody, _ := json.Marshal(map[string]any{"retention_id": retentionID})
	linkURL := fmt.Sprintf("%s/api/v2.0/projects/%s", h.baseURL, strconv.Itoa(projectID))
	linkResp, err := h.do(ctx, http.MethodPut, linkURL, "application/json", bytes.NewReader(linkBody))
	if err != nil {
		return nil // best-effort
	}
	drain(linkResp.Body)
	return nil
}

// EnsureRobotAccount creates a robot account with push+pull access to the project.
// Returns credentials (name + secret) for Jenkins to use when pushing images.
// If the robot already exists it is deleted and re-created to return fresh credentials.
//
// Uses the Harbor v2 system robots API (POST /api/v2.0/robots) instead of the
// project-level endpoint (/api/v2.0/projects/{id}/robots). The system API requires
// the "Robot Account" permission at system level on the calling robot account.
// Set level=project with a permissions array to scope the new robot to one project.
func (h *HarborAdapter) EnsureRobotAccount(ctx context.Context, projectName, robotName string) (*RobotCredential, error) {
	projectName = harborProject(projectName)

	robotBody := map[string]any{
		"name":        robotName,
		"description": "Jenkins push/pull access — managed by DevPortal",
		"duration":    -1, // never expire
		"level":       "project",
		"permissions": []map[string]any{
			{
				"kind":      "project",
				"namespace": projectName,
				"access": []map[string]string{
					{"resource": "repository", "action": "push"},
					{"resource": "repository", "action": "pull"},
				},
			},
		},
	}

	cred, err := h.createRobot(ctx, robotBody)
	if err == nil {
		return cred, nil
	}
	if !isConflict(err) {
		return nil, err
	}

	// 409 Conflict — robot already exists. Credentials are not re-fetchable so
	// returning nil here is safe: the robot has the right permissions already.
	return nil, nil
}

// SyncProjectMembers reconciles the Harbor project member list to the given
// email set. Harbor usernames for Keycloak OIDC users match the email prefix
// before '@' — users not found in Harbor are silently skipped.
func (h *HarborAdapter) SyncProjectMembers(ctx context.Context, projectName string, emails []string) error {
	projectName = harborProject(projectName)

	// Fetch current members.
	listURL := fmt.Sprintf("%s/api/v2.0/projects/%s/members", h.baseURL, url.PathEscape(projectName))
	resp, err := h.do(ctx, http.MethodGet, listURL, "", nil)
	if err != nil {
		return fmt.Errorf("harbor SyncProjectMembers list: %w", err)
	}
	defer resp.Body.Close()

	var members []struct {
		ID       int    `json:"id"`
		EntityID int    `json:"entity_id"`
		Username string `json:"entity_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
		return fmt.Errorf("harbor SyncProjectMembers decode: %w", err)
	}

	// Build wanted set using email prefix as Harbor username.
	wanted := make(map[string]bool, len(emails))
	for _, e := range emails {
		username := e
		if idx := strings.Index(e, "@"); idx > 0 {
			username = e[:idx]
		}
		wanted[username] = true
	}

	// Build current state keyed by Harbor username.
	currentByName := make(map[string]int) // username → member entity ID for DELETE
	for _, m := range members {
		currentByName[m.Username] = m.ID
	}

	// Add missing members (Harbor role 2 = Developer).
	for _, email := range emails {
		username := email
		if idx := strings.Index(email, "@"); idx > 0 {
			username = email[:idx]
		}
		if _, exists := currentByName[username]; exists {
			continue
		}
		addBody, _ := json.Marshal(map[string]any{
			"role_id":     2,
			"member_user": map[string]string{"username": username},
		})
		addResp, err := h.do(ctx, http.MethodPost, listURL, "application/json", bytes.NewReader(addBody))
		if err == nil {
			drain(addResp.Body)
		}
	}

	// Remove stale members.
	for username, memberID := range currentByName {
		if !wanted[username] {
			delURL := fmt.Sprintf("%s/api/v2.0/projects/%s/members/%d",
				h.baseURL, url.PathEscape(projectName), memberID)
			delResp, err := h.do(ctx, http.MethodDelete, delURL, "", nil)
			if err == nil {
				drain(delResp.Body)
			}
		}
	}
	return nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

type harborConflictError struct{}

func (harborConflictError) Error() string { return "harbor: 409 conflict" }

func isConflict(err error) bool {
	_, ok := err.(harborConflictError)
	return ok
}

func (h *HarborAdapter) createRobot(ctx context.Context, robotBody map[string]any) (*RobotCredential, error) {
	body, _ := json.Marshal(robotBody)
	createURL := h.baseURL + "/api/v2.0/robots"
	resp, err := h.do(ctx, http.MethodPost, createURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("harbor createRobot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		drain(resp.Body)
		return nil, harborConflictError{}
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("harbor createRobot: unexpected status %d: %s", resp.StatusCode, truncate(b, 256))
	}

	var result struct {
		Name   string `json:"name"`
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("harbor createRobot decode: %w", err)
	}
	return &RobotCredential{Name: result.Name, Secret: result.Secret}, nil
}

func (h *HarborAdapter) do(ctx context.Context, method, rawURL, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", h.auth)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return h.client.Do(req)
}
