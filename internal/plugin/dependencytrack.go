// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// dependencytrack.go implements DependencyTrackProvider against the
// Dependency-Track v4 REST API.
//
// Auth: X-Api-Key header (team API key with PORTFOLIO_MANAGEMENT + SYSTEM_CONFIGURATION).
// Idempotency:
//
//	EnsureProject          — GET by name first; POST only when not found.
//	EnsureEmailNotification — list rules; PATCH existing or PUT new, then scope to project.

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

// DependencyTrackAdapter implements DependencyTrackProvider against the DT v4 REST API.
type DependencyTrackAdapter struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

// NewDependencyTrackAdapter constructs a DependencyTrackAdapter from config.
func NewDependencyTrackAdapter(cfg *config.Config, httpClient *http.Client) *DependencyTrackAdapter {
	return &DependencyTrackAdapter{
		client:  httpClient,
		baseURL: strings.TrimRight(cfg.DependencyTrackURL, "/"),
		apiKey:  cfg.DependencyTrackAPIKey,
	}
}

// EnsureProject creates a Dependency-Track project if it does not exist.
// Returns the project UUID in both cases. Idempotent.
func (d *DependencyTrackAdapter) EnsureProject(ctx context.Context, projectName string) (string, error) {
	if d.baseURL == "" || d.apiKey == "" {
		return "", nil // DT not configured — skip gracefully
	}

	// Search for an existing project by name.
	searchURL := fmt.Sprintf("%s/api/v1/project?name=%s", d.baseURL, url.QueryEscape(projectName))
	resp, err := d.do(ctx, http.MethodGet, searchURL, "", nil)
	if err != nil {
		return "", fmt.Errorf("dt EnsureProject search: %w", err)
	}
	defer resp.Body.Close()

	var projects []struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return "", fmt.Errorf("dt EnsureProject decode search: %w", err)
	}
	for _, p := range projects {
		if p.Name == projectName {
			return p.UUID, nil
		}
	}

	// Not found — create it.
	body, _ := json.Marshal(map[string]any{
		"name":   projectName,
		"active": true,
	})
	resp2, err := d.do(ctx, http.MethodPut, d.baseURL+"/api/v1/project", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("dt EnsureProject create: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusCreated && resp2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp2.Body)
		return "", fmt.Errorf("dt EnsureProject: status %d: %s", resp2.StatusCode, truncate(b, 256))
	}

	var created struct {
		UUID string `json:"uuid"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&created); err != nil {
		return "", fmt.Errorf("dt EnsureProject decode created: %w", err)
	}
	return created.UUID, nil
}

// EnsureEmailNotification creates or updates a notification rule named
// "devportal-<ruleTag>" that sends emails on new vulnerabilities.
// The rule is scoped to the given projectUUID. Idempotent.
func (d *DependencyTrackAdapter) EnsureEmailNotification(ctx context.Context, projectUUID, ruleTag string, emails []string) error {
	if d.baseURL == "" || d.apiKey == "" || len(emails) == 0 {
		return nil
	}

	ruleName := "devportal-" + ruleTag
	destination := strings.Join(emails, ",")
	publisherConfig := fmt.Sprintf(`{"destination":"%s"}`, destination)

	// List existing notification rules.
	resp, err := d.do(ctx, http.MethodGet, d.baseURL+"/api/v1/notification/rule", "", nil)
	if err != nil {
		return fmt.Errorf("dt EnsureEmailNotification list rules: %w", err)
	}
	defer resp.Body.Close()

	var rules []struct {
		UUID            string `json:"uuid"`
		Name            string `json:"name"`
		PublisherConfig string `json:"publisherConfig"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rules); err != nil {
		return fmt.Errorf("dt EnsureEmailNotification decode rules: %w", err)
	}

	// Find existing rule by name.
	var existingUUID string
	for _, r := range rules {
		if r.Name == ruleName {
			existingUUID = r.UUID
			break
		}
	}

	ruleBody := map[string]any{
		"name":            ruleName,
		"enabled":         true,
		"scope":           "PORTFOLIO",
		"level":           "INFORMATIONAL",
		"notifyOn":        []string{"NEW_VULNERABILITY", "NEW_VULNERABLE_DEPENDENCY"},
		"publisherConfig": publisherConfig,
		"publisher": map[string]string{
			"class": "org.dependencytrack.notification.publisher.SendMailPublisher",
		},
	}

	if existingUUID == "" {
		// Create new rule.
		b, _ := json.Marshal(ruleBody)
		createResp, err := d.do(ctx, http.MethodPut, d.baseURL+"/api/v1/notification/rule", "application/json", bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("dt EnsureEmailNotification create: %w", err)
		}
		defer createResp.Body.Close()

		var created struct {
			UUID string `json:"uuid"`
		}
		if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
			return fmt.Errorf("dt EnsureEmailNotification decode created: %w", err)
		}
		existingUUID = created.UUID
	} else {
		// Update publisherConfig on existing rule.
		ruleBody["uuid"] = existingUUID
		b, _ := json.Marshal(ruleBody)
		patchResp, err := d.do(ctx, http.MethodPost, d.baseURL+"/api/v1/notification/rule", "application/json", bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("dt EnsureEmailNotification update: %w", err)
		}
		drain(patchResp.Body)
	}

	// Scope the rule to this project.
	if existingUUID != "" && projectUUID != "" {
		scopeURL := fmt.Sprintf("%s/api/v1/notification/rule/%s/project/%s",
			d.baseURL, existingUUID, projectUUID)
		scopeResp, err := d.do(ctx, http.MethodPost, scopeURL, "", nil)
		if err == nil {
			drain(scopeResp.Body)
		}
	}
	return nil
}

func (d *DependencyTrackAdapter) do(ctx context.Context, method, rawURL, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", d.apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return d.client.Do(req)
}
