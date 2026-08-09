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
//	                Then applies SLA config and assigns product members.
//	CreateEngagement — always creates; engagements represent CI/CD scan cycles.

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

// EnsureProduct creates a DefectDojo product if it does not already exist,
// then applies the SLA configuration and assigns product members.
// Returns the product ID for subsequent engagement creation.
func (d *DefectDojoAdapter) EnsureProduct(ctx context.Context, name, description string, cfg ProductConfig) (int, error) {
	productID, err := d.ensureProductRow(ctx, name, description)
	if err != nil {
		return 0, err
	}

	// SLA configuration — best-effort; failure logs but does not abort provisioning.
	if cfg.SLACriticalDays > 0 || cfg.SLAHighDays > 0 || cfg.SLAMediumDays > 0 || cfg.SLALowDays > 0 {
		if slaErr := d.configureSLA(ctx, productID, name, cfg); slaErr != nil {
			slog.Warn("defectdojo: SLA configuration failed (non-fatal)",
				"product_id", productID, "err", slaErr)
		}
	}

	// Member assignment — best-effort; users not in DefectDojo are skipped silently.
	for _, email := range cfg.MemberEmails {
		if memberErr := d.addProductMember(ctx, productID, email); memberErr != nil {
			slog.Warn("defectdojo: add product member failed (non-fatal)",
				"product_id", productID, "email", email, "err", memberErr)
		}
	}

	return productID, nil
}

// ensureProductRow creates the DefectDojo product row if it does not exist
// and returns the product ID in both cases.
func (d *DefectDojoAdapter) ensureProductRow(ctx context.Context, name, description string) (int, error) {
	// Search by exact name.
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

	// prod_type 1 = Research and Development (default in fresh DefectDojo installs).
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

// configureSLA creates a named SLA configuration and links it to the product.
// DefectDojo enforces these deadlines and marks findings as SLA-breached when exceeded.
func (d *DefectDojoAdapter) configureSLA(ctx context.Context, productID int, productName string, cfg ProductConfig) error {
	critical := cfg.SLACriticalDays
	if critical == 0 {
		critical = 7
	}
	high := cfg.SLAHighDays
	if high == 0 {
		high = 30
	}
	medium := cfg.SLAMediumDays
	if medium == 0 {
		medium = 90
	}
	low := cfg.SLALowDays
	if low == 0 {
		low = 180
	}

	// Create the SLA configuration object.
	slaBody, _ := json.Marshal(map[string]any{
		"name":        productName + " SLA",
		"description": fmt.Sprintf("Remediation SLA for %s: Critical=%dd, High=%dd, Medium=%dd, Low=%dd", productName, critical, high, medium, low),
		"critical":    critical,
		"high":        high,
		"medium":      medium,
		"low":         low,
	})
	resp, err := d.do(ctx, http.MethodPost, d.baseURL+"/api/v2/sla_configuration/", "application/json", bytes.NewReader(slaBody))
	if err != nil {
		return fmt.Errorf("defectdojo configureSLA create: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("defectdojo configureSLA: status %d: %s", resp.StatusCode, truncate(b, 256))
	}

	var sla struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sla); err != nil || sla.ID == 0 {
		return fmt.Errorf("defectdojo configureSLA decode: %w", err)
	}

	// Link the SLA configuration to the product via PATCH.
	patchBody, _ := json.Marshal(map[string]any{"sla_configuration": sla.ID})
	patchURL := fmt.Sprintf("%s/api/v2/products/%d/", d.baseURL, productID)
	patchResp, err := d.do(ctx, http.MethodPatch, patchURL, "application/json", bytes.NewReader(patchBody))
	if err != nil {
		return fmt.Errorf("defectdojo configureSLA link: %w", err)
	}
	drain(patchResp.Body)
	return nil
}

// addProductMember finds the DefectDojo user by email and adds them as a
// Writer on the product. Silently skips users not found in DefectDojo.
func (d *DefectDojoAdapter) addProductMember(ctx context.Context, productID int, email string) error {
	// Look up the DefectDojo user by email.
	userURL := fmt.Sprintf("%s/api/v2/users/?email=%s&limit=1", d.baseURL, url.QueryEscape(email))
	resp, err := d.do(ctx, http.MethodGet, userURL, "", nil)
	if err != nil {
		return fmt.Errorf("defectdojo addProductMember lookup %s: %w", email, err)
	}
	defer resp.Body.Close()

	var userResult struct {
		Count   int `json:"count"`
		Results []struct {
			ID int `json:"id"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userResult); err != nil || userResult.Count == 0 {
		// User not in DefectDojo yet — skip silently.
		return nil
	}
	userID := userResult.Results[0].ID

	// Add as product member with Writer role (role ID 2).
	// Roles: 1=Reader, 2=Writer, 3=Maintainer, 4=Owner
	memberBody, _ := json.Marshal(map[string]any{
		"product": productID,
		"user":    userID,
		"role":    2,
	})
	memberResp, err := d.do(ctx, http.MethodPost, d.baseURL+"/api/v2/product_members/", "application/json", bytes.NewReader(memberBody))
	if err != nil {
		return fmt.Errorf("defectdojo addProductMember POST %s: %w", email, err)
	}
	drain(memberResp.Body)

	// 400 can mean the user is already a member — treat as success.
	if memberResp.StatusCode == http.StatusCreated ||
		memberResp.StatusCode == http.StatusOK ||
		memberResp.StatusCode == http.StatusBadRequest {
		return nil
	}
	return fmt.Errorf("defectdojo addProductMember: status %d for %s", memberResp.StatusCode, email)
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
