// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// harbor.go implements RegistryProvider against the Harbor v2 REST API.
//
// Auth: HTTP Basic (HARBOR_USER + HARBOR_TOKEN).
// Idempotency:
//
//	EnsureProject     — HEAD /api/v2.0/projects?project_name=…; skip POST if 200.
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

// EnsureProject creates a Harbor project (image namespace) if it does not already exist.
// Idempotent — HEAD-checks before creating.
func (h *HarborAdapter) EnsureProject(ctx context.Context, projectName string) error {
	// HEAD /api/v2.0/projects?project_name=<name> → 200 = exists, 404 = not found.
	checkURL := fmt.Sprintf("%s/api/v2.0/projects?project_name=%s", h.baseURL, url.QueryEscape(projectName))
	resp, err := h.do(ctx, http.MethodHead, checkURL, "", nil)
	if err != nil {
		return fmt.Errorf("harbor EnsureProject check: %w", err)
	}
	drain(resp.Body)
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := json.Marshal(map[string]any{
		"project_name": projectName,
		"public":       false,
	})
	resp, err = h.do(ctx, http.MethodPost, h.baseURL+"/api/v2.0/projects", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("harbor EnsureProject create: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("harbor EnsureProject: unexpected status %d: %s", resp.StatusCode, truncate(b, 256))
	}
	return nil
}

// EnsureRobotAccount creates a robot account with push+pull access to the project.
// Returns credentials (name + secret) for Jenkins to use when pushing images.
// If the robot already exists it is deleted and re-created to return fresh credentials.
func (h *HarborAdapter) EnsureRobotAccount(ctx context.Context, projectName, robotName string) (*RobotCredential, error) {
	robotBody := map[string]any{
		"name":        robotName,
		"description": "Jenkins push/pull access — managed by DevPortal",
		"duration":    -1, // never expire
		"access": []map[string]string{
			{"resource": "/project/" + projectName + "/repository", "action": "push"},
			{"resource": "/project/" + projectName + "/repository", "action": "pull"},
		},
	}

	cred, err := h.createRobot(ctx, projectName, robotBody)
	if err == nil {
		return cred, nil
	}
	if !isConflict(err) {
		return nil, err
	}

	// 409 Conflict — robot exists; delete it and re-create for fresh credentials.
	id, err := h.findRobotID(ctx, projectName, robotName)
	if err != nil {
		return nil, err
	}
	if err := h.deleteRobot(ctx, projectName, id); err != nil {
		return nil, err
	}
	return h.createRobot(ctx, projectName, robotBody)
}

// ── internal helpers ──────────────────────────────────────────────────────────

type harborConflictError struct{}

func (harborConflictError) Error() string { return "harbor: 409 conflict" }

func isConflict(err error) bool {
	_, ok := err.(harborConflictError)
	return ok
}

func (h *HarborAdapter) createRobot(ctx context.Context, projectName string, robotBody map[string]any) (*RobotCredential, error) {
	body, _ := json.Marshal(robotBody)
	createURL := fmt.Sprintf("%s/api/v2.0/projects/%s/robots", h.baseURL, url.PathEscape(projectName))
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

func (h *HarborAdapter) findRobotID(ctx context.Context, projectName, robotName string) (int, error) {
	listURL := fmt.Sprintf("%s/api/v2.0/projects/%s/robots", h.baseURL, url.PathEscape(projectName))
	resp, err := h.do(ctx, http.MethodGet, listURL, "", nil)
	if err != nil {
		return 0, fmt.Errorf("harbor findRobotID list: %w", err)
	}
	defer resp.Body.Close()

	var robots []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&robots); err != nil {
		return 0, fmt.Errorf("harbor findRobotID decode: %w", err)
	}
	// Harbor prefixes robot names: "robot$<project>+<name>"
	want := "robot$" + projectName + "+" + robotName
	for _, r := range robots {
		if r.Name == want || r.Name == robotName {
			return r.ID, nil
		}
	}
	return 0, fmt.Errorf("harbor findRobotID: robot %q not found in project %q", robotName, projectName)
}

func (h *HarborAdapter) deleteRobot(ctx context.Context, projectName string, robotID int) error {
	deleteURL := fmt.Sprintf("%s/api/v2.0/projects/%s/robots/%d", h.baseURL, url.PathEscape(projectName), robotID)
	resp, err := h.do(ctx, http.MethodDelete, deleteURL, "", nil)
	if err != nil {
		return fmt.Errorf("harbor deleteRobot: %w", err)
	}
	drain(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("harbor deleteRobot: unexpected status %d", resp.StatusCode)
	}
	return nil
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
