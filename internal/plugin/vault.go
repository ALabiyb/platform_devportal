// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// vault.go implements VaultProvider via Vault's HTTP API.
//
// Auth: X-Vault-Token header. Use a token with:
//   - kv/* write + read
//   - sys/policies/acl/* create + update
//   - auth/<k8s_mount>/role/* create + update
//
// All methods are idempotent. EnsureKVSecret never overwrites non-empty
// values — it reads the existing secret first and only adds absent keys.

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// VaultAdapter implements VaultProvider via Vault's HTTP API.
type VaultAdapter struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewVaultAdapter constructs a VaultAdapter. Returns nil (safe to use — all
// methods return a descriptive "not configured" error) when VAULT_URL is empty.
func NewVaultAdapter(cfg *config.Config, httpClient *http.Client) *VaultAdapter {
	return &VaultAdapter{
		baseURL: strings.TrimRight(cfg.VaultURL, "/"),
		token:   cfg.VaultToken,
		client:  httpClient,
	}
}

// EnsureKVSecret writes placeholder data to a KV v2 path.
// Reads the existing secret first; only adds keys that are absent or empty.
func (v *VaultAdapter) EnsureKVSecret(ctx context.Context, mount, path string, data map[string]string) error {
	if v.baseURL == "" {
		return fmt.Errorf("vault: not configured (VAULT_URL not set)")
	}

	// Read existing secret to avoid clobbering real values on re-provision.
	existing, err := v.kvRead(ctx, mount, path)
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("vault EnsureKVSecret read %s/%s: %w", mount, path, err)
	}

	merged := make(map[string]string, len(data))
	for k, v := range data {
		merged[k] = v
	}
	for k, ev := range existing {
		if ev != "" {
			merged[k] = ev // keep real value; never overwrite with placeholder
		}
	}

	return v.kvWrite(ctx, mount, path, merged)
}

// EnsurePolicy creates or replaces a Vault ACL policy.
func (v *VaultAdapter) EnsurePolicy(ctx context.Context, name, hcl string) error {
	if v.baseURL == "" {
		return fmt.Errorf("vault: not configured (VAULT_URL not set)")
	}
	url := fmt.Sprintf("%s/v1/sys/policies/acl/%s", v.baseURL, name)
	body, _ := json.Marshal(map[string]string{"policy": hcl})
	return v.do(ctx, http.MethodPut, url, body, nil)
}

// EnsureKubernetesRole creates or replaces a K8s auth role in Vault.
func (v *VaultAdapter) EnsureKubernetesRole(ctx context.Context, authMount, roleName, saNamespace, saName string, policies []string) error {
	if v.baseURL == "" {
		return fmt.Errorf("vault: not configured (VAULT_URL not set)")
	}
	url := fmt.Sprintf("%s/v1/auth/%s/role/%s", v.baseURL, authMount, roleName)
	body, _ := json.Marshal(map[string]any{
		"bound_service_account_names":      []string{saName},
		"bound_service_account_namespaces": []string{saNamespace},
		"policies":                         policies,
		"ttl":                              "1h",
		"max_ttl":                          "24h",
	})
	return v.do(ctx, http.MethodPost, url, body, nil)
}

// ── internal helpers ──────────────────────────────────────────────────────────

// kvRead reads the current data map from a KV v2 path.
// Returns nil map (and no error) when the path does not exist yet.
func (v *VaultAdapter) kvRead(ctx context.Context, mount, path string) (map[string]string, error) {
	url := fmt.Sprintf("%s/v1/%s/data/%s", v.baseURL, mount, path)
	var result struct {
		Data struct {
			Data map[string]any `json:"data"`
		} `json:"data"`
	}
	if err := v.do(ctx, http.MethodGet, url, nil, &result); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(result.Data.Data))
	for k, raw := range result.Data.Data {
		if s, ok := raw.(string); ok {
			out[k] = s
		}
	}
	return out, nil
}

// kvWrite writes data to a KV v2 path.
func (v *VaultAdapter) kvWrite(ctx context.Context, mount, path string, data map[string]string) error {
	url := fmt.Sprintf("%s/v1/%s/data/%s", v.baseURL, mount, path)
	payload := map[string]any{"data": data}
	body, _ := json.Marshal(payload)
	return v.do(ctx, http.MethodPut, url, body, nil)
}

// do executes an authenticated Vault HTTP request.
// out is optional — pass nil to discard the response body.
func (v *VaultAdapter) do(ctx context.Context, method, url string, body []byte, out any) error {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("vault: build request: %w", err)
	}
	req.Header.Set("X-Vault-Token", v.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("vault: %s %s: %w", method, url, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return &vaultNotFoundError{url: url}
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("vault: %s %s → %d: %s", method, url, resp.StatusCode, string(respBody))
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("vault: decode response: %w", err)
		}
	}
	return nil
}

type vaultNotFoundError struct{ url string }

func (e *vaultNotFoundError) Error() string { return "vault: 404 " + e.url }

func isNotFound(err error) bool {
	_, ok := err.(*vaultNotFoundError)
	return ok
}
