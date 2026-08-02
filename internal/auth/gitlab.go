// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// gitlab.go implements AuthProvider using GitLab (or Gitea) OAuth 2.0.
//
// This is the fallback provider for deployments that do not have Keycloak.
// Set AUTH_MODE=gitlab and provide GITLAB_OAUTH_CLIENT_ID / SECRET.
//
// Differences from OIDC:
//   - No id_token — user info comes from a separate API call to /api/v4/user
//   - No groups claim — RBAC role is determined only by adminEmail match
//     (all other GitLab users get "developer" by default)
//   - nonce parameter is accepted but not used (GitLab OAuth is not OIDC)

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"golang.org/x/oauth2"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// GitLabProvider implements AuthProvider using GitLab / Gitea OAuth 2.0.
type GitLabProvider struct {
	oauth2Cfg  oauth2.Config
	gitlabURL  string
	httpClient *http.Client
}

// NewGitLabProvider builds a GitLab OAuth provider from config.
// Works with both GitLab CE/EE and Gitea — just set GITLAB_URL to your instance.
func NewGitLabProvider(cfg *config.Config, httpClient *http.Client) *GitLabProvider {
	return &GitLabProvider{
		oauth2Cfg: oauth2.Config{
			ClientID:     cfg.GitLabOAuthClientID,
			ClientSecret: cfg.GitLabOAuthClientSecret,
			RedirectURL:  cfg.GitLabOAuthRedirectURL,
			Endpoint: oauth2.Endpoint{
				// GitLab and Gitea share the same OAuth path conventions.
				AuthURL:  cfg.GitLabURL + "/oauth/authorize",
				TokenURL: cfg.GitLabURL + "/oauth/token",
			},
			Scopes: []string{"read_user"},
		},
		gitlabURL:  cfg.GitLabURL,
		httpClient: httpClient,
	}
}

// AuthURL returns the GitLab OAuth authorization URL.
// The nonce parameter is intentionally ignored — GitLab OAuth is not OIDC
// and has no nonce concept. It is accepted only for interface compatibility.
func (p *GitLabProvider) AuthURL(state, _ string) string {
	return p.oauth2Cfg.AuthCodeURL(state)
}

// Exchange trades the authorization code for a Claims set by:
//  1. Exchanging the code for an access token with the GitLab token endpoint
//  2. Calling /api/v4/user with the access token to fetch user info
//
// Unlike OIDC, there is no id_token to verify — we trust GitLab's API response.
func (p *GitLabProvider) Exchange(ctx context.Context, code, _ string) (*Claims, error) {
	// Inject the shared HTTP client so TLS settings are respected.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)

	token, err := p.oauth2Cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("gitlab oauth: exchange code: %w", err)
	}

	// Call the GitLab user API to get the authenticated user's profile.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		p.gitlabURL+"/api/v4/user", nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab oauth: build user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab oauth: call user API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab oauth: user API returned HTTP %d", resp.StatusCode)
	}

	var u struct {
		ID       int    `json:"id"`
		Email    string `json:"email"`
		Name     string `json:"name"`
		Username string `json:"username"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("gitlab oauth: decode user response: %w", err)
	}

	return &Claims{
		Sub:         strconv.Itoa(u.ID), // GitLab numeric ID as stable string identifier
		Email:       u.Email,
		DisplayName: u.Name,
		Username:    u.Username,
		Groups:      nil,      // GitLab OAuth does not expose group membership — role comes from adminEmail
		Provider:    "gitlab",
	}, nil
}
