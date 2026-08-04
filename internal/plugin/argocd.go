// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// argocd.go implements GitOpsProvider against the ArgoCD REST API v1.
//
// Auth: Authorization: Bearer <ARGOCD_TOKEN>
// Idempotency:
//
//	CreateApplication — GET /api/v1/applications/{name}; skip POST if 200.
//	SyncApplication   — always fires; ArgoCD deduplicates concurrent syncs.

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// ArgoCDAdapter implements GitOpsProvider against the ArgoCD REST API.
type ArgoCDAdapter struct {
	client  *http.Client
	baseURL string // e.g. "https://argocd.example.com"
	token   string // ArgoCD API token (argocd account generate-token)
}

// NewArgoCDAdapter constructs an ArgoCDAdapter from config.
func NewArgoCDAdapter(cfg *config.Config, httpClient *http.Client) *ArgoCDAdapter {
	return &ArgoCDAdapter{
		client:  httpClient,
		baseURL: strings.TrimRight(cfg.ArgoCDURL, "/"),
		token:   cfg.ArgoCDToken,
	}
}

// CreateApplication creates an ArgoCD Application pointing at the manifest repo path.
// Idempotent — returns nil if the Application already exists.
func (a *ArgoCDAdapter) CreateApplication(ctx context.Context, input CreateAppInput) error {
	checkURL := fmt.Sprintf("%s/api/v1/applications/%s", a.baseURL, url.PathEscape(input.Name))
	resp, err := a.do(ctx, http.MethodGet, checkURL, "", nil)
	if err != nil {
		return fmt.Errorf("argocd CreateApplication check: %w", err)
	}
	drain(resp.Body)
	if resp.StatusCode == http.StatusOK {
		return nil // application already exists
	}

	project := input.Project
	if project == "" {
		project = "default"
	}
	server := input.Server
	if server == "" {
		server = "https://kubernetes.default.svc"
	}

	app := map[string]any{
		"metadata": map[string]any{
			"name":      input.Name,
			"namespace": "argocd",
		},
		"spec": map[string]any{
			"project": project,
			"source": map[string]any{
				"repoURL":        input.RepoURL,
				"targetRevision": input.TargetRevision,
				"path":           input.Path,
			},
			"destination": map[string]any{
				"server":    server,
				"namespace": input.Namespace,
			},
			"syncPolicy": argoCDSyncPolicy(input.AutoSync),
		},
	}

	body, _ := json.Marshal(app)
	resp2, err := a.do(ctx, http.MethodPost, a.baseURL+"/api/v1/applications", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("argocd CreateApplication create: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("argocd CreateApplication: unexpected status %d: %s", resp2.StatusCode, truncate(b, 256))
	}
	return nil
}

// SyncApplication triggers an immediate sync for the named Application.
func (a *ArgoCDAdapter) SyncApplication(ctx context.Context, appName string) error {
	syncURL := fmt.Sprintf("%s/api/v1/applications/%s/sync", a.baseURL, url.PathEscape(appName))
	resp, err := a.do(ctx, http.MethodPost, syncURL, "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("argocd SyncApplication: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("argocd SyncApplication: unexpected status %d: %s", resp.StatusCode, truncate(b, 256))
	}
	return nil
}

// GetApplicationStatus returns the current health and sync status.
func (a *ArgoCDAdapter) GetApplicationStatus(ctx context.Context, appName string) (*AppStatus, error) {
	statusURL := fmt.Sprintf("%s/api/v1/applications/%s", a.baseURL, url.PathEscape(appName))
	resp, err := a.do(ctx, http.MethodGet, statusURL, "", nil)
	if err != nil {
		return nil, fmt.Errorf("argocd GetApplicationStatus: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("argocd GetApplicationStatus: unexpected status %d: %s", resp.StatusCode, truncate(b, 256))
	}

	var result struct {
		Status struct {
			Health struct{ Status string `json:"status"` } `json:"health"`
			Sync   struct{ Status string `json:"status"` } `json:"sync"`
		} `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("argocd GetApplicationStatus decode: %w", err)
	}
	return &AppStatus{
		Health: result.Status.Health.Status,
		Sync:   result.Status.Sync.Status,
	}, nil
}

func (a *ArgoCDAdapter) do(ctx context.Context, method, rawURL, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return a.client.Do(req)
}

// argoCDSyncPolicy returns the automated sync policy object when autoSync is true,
// or nil (no automated sync — manual sync only) when false.
func argoCDSyncPolicy(autoSync bool) any {
	if !autoSync {
		return nil
	}
	return map[string]any{
		"automated": map[string]any{
			"prune":    true,
			"selfHeal": true,
		},
	}
}
