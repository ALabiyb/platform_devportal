// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// defectdojo.go implements SecurityProvider against the DefectDojo REST API v2.
//
// Auth: Authorization: Token <DEFECTDOJO_TOKEN>
// Idempotency:
//
//	EnsureProduct — GET /api/v2/products/?name=…&limit=1; skip POST if count > 0.
//	CreateEngagement — always creates; engagements represent individual CI/CD runs.

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
	"time"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// DefectDojoAdapter implements SecurityProvider against the DefectDojo v2 API.
type DefectDojoAdapter struct {
	client  *http.Client
	baseURL string // e.g. "https://defectdojo.example.com"
	token   string // DefectDojo API token
}

// NewDefectDojoAdapter constructs a DefectDojoAdapter from config.
func NewDefectDojoAdapter(cfg *config.Config, httpClient *http.Client) *DefectDojoAdapter {
	return &DefectDojoAdapter{
		client:  httpClient,
		baseURL: strings.TrimRight(cfg.DefectDojoURL, "/"),
		token:   cfg.DefectDojoToken,
	}
}

// EnsureProduct creates a DefectDojo product if it does not already exist.
// Returns the product ID for subsequent engagement creation.
// Idempotent — searches by name before creating.
func (d *DefectDojoAdapter) EnsureProduct(ctx context.Context, name, description string) (int, error) {
	// Search by name — DefectDojo supports exact filtering via ?name=
	searchURL := fmt.Sprintf("%s/api/v2/products/?name=%s&limit=1", d.baseURL, url.QueryEscape(name))
	resp, err := d.do(ctx, http.MethodGet, searchURL, "", nil)
	if err != nil {
		return 0, fmt.Errorf("defectdojo EnsureProduct search: %w", err)
	}
	defer resp.Body.Close()

	var searchResult struct {
		Count   int `json:"count"`
		Results []struct {
			ID int `json:"id"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return 0, fmt.Errorf("defectdojo EnsureProduct decode search: %w", err)
	}
	if searchResult.Count > 0 {
		return searchResult.Results[0].ID, nil
	}

	// prod_type 1 = Research and Development (default product type in all fresh DoJo installs).
	body, _ := json.Marshal(map[string]any{
		"name":        name,
		"description": description,
		"prod_type":   1,
	})
	resp2, err := d.do(ctx, http.MethodPost, d.baseURL+"/api/v2/products/", "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("defectdojo EnsureProduct create: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusCreated && resp2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp2.Body)
		return 0, fmt.Errorf("defectdojo EnsureProduct: unexpected status %d: %s", resp2.StatusCode, truncate(b, 256))
	}

	var created struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&created); err != nil {
		return 0, fmt.Errorf("defectdojo EnsureProduct decode created: %w", err)
	}
	return created.ID, nil
}

// CreateEngagement creates a CI/CD engagement under a product.
// Returns the engagement ID.
func (d *DefectDojoAdapter) CreateEngagement(ctx context.Context, input CreateEngagementInput) (int, error) {
	now := time.Now()

	start := input.StartDate
	if start == "" {
		start = now.Format("2006-01-02")
	}
	end := input.EndDate
	if end == "" {
		end = now.AddDate(1, 0, 0).Format("2006-01-02")
	}
	engType := input.EngagementType
	if engType == "" {
		engType = "CI/CD"
	}

	body, _ := json.Marshal(map[string]any{
		"name":                        input.Name,
		"description":                 input.Description,
		"product":                     input.ProductID,
		"engagement_type":             engType,
		"status":                      "In Progress",
		"target_start":                start,
		"target_end":                  end,
		"deduplication_on_engagement": true,
	})
	resp, err := d.do(ctx, http.MethodPost, d.baseURL+"/api/v2/engagements/", "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("defectdojo CreateEngagement: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("defectdojo CreateEngagement: unexpected status %d: %s", resp.StatusCode, truncate(b, 256))
	}

	var created struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return 0, fmt.Errorf("defectdojo CreateEngagement decode: %w", err)
	}
	return created.ID, nil
}

func (d *DefectDojoAdapter) do(ctx context.Context, method, rawURL, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+d.token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return d.client.Do(req)
}
