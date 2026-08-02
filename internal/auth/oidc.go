// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// oidc.go implements AuthProvider using Keycloak as the OIDC identity provider.
//
// Flow (Authorization Code + PKCE-equivalent via nonce):
//  1. AuthURL()  → redirect browser to Keycloak login page
//  2. Keycloak   → redirect back to /auth/callback?code=…&state=…
//  3. Exchange() → trade code for tokens, verify id_token signature + nonce,
//                  extract claims (sub, email, name, groups)
//
// The "groups" claim in the id_token must be configured in Keycloak:
//  Client → Client Scopes → groups → Mapper type: Group Membership
//  Token Claim Name: groups, Full group path: OFF

package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// OIDCProvider implements AuthProvider via Keycloak OIDC.
type OIDCProvider struct {
	verifier    *oidc.IDTokenVerifier
	oauth2Cfg   oauth2.Config
	groupsClaim string // name of the claim carrying group membership ("groups" by default)
}

// NewOIDCProvider discovers the Keycloak OIDC configuration from the issuer URL
// (via the /.well-known/openid-configuration endpoint) and returns a ready provider.
// Returns an error if Keycloak is unreachable at startup — this is intentional:
// we want a hard failure at boot, not a silent auth failure at login time.
func NewOIDCProvider(ctx context.Context, cfg *config.Config, httpClient *http.Client) (*OIDCProvider, error) {
	// Inject the shared HTTP client (with TLS skip configured) so the OIDC
	// discovery request respects the same TLS settings as all other outbound calls.
	ctx = oidc.ClientContext(ctx, httpClient)

	provider, err := oidc.NewProvider(ctx, cfg.OIDCIssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc: discover provider at %q: %w", cfg.OIDCIssuerURL, err)
	}

	return &OIDCProvider{
		verifier: provider.Verifier(&oidc.Config{
			ClientID: cfg.OIDCClientID,
		}),
		oauth2Cfg: oauth2.Config{
			ClientID:     cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret,
			RedirectURL:  cfg.OIDCRedirectURL,
			Endpoint:     provider.Endpoint(),
			// "groups" scope is required for Keycloak to include group membership
			// in the id_token. Must be configured as a client scope in Keycloak.
			Scopes: []string{oidc.ScopeOpenID, "profile", "email", "groups"},
		},
		groupsClaim: "groups",
	}, nil
}

// AuthURL returns the Keycloak authorization URL.
// The nonce is embedded in the URL as an OIDC parameter and must be verified
// in Exchange() to prevent token replay attacks.
func (p *OIDCProvider) AuthURL(state, nonce string) string {
	return p.oauth2Cfg.AuthCodeURL(state, oidc.Nonce(nonce))
}

// Exchange trades the authorization code for a validated Claims set.
// It performs all required security checks:
//   - Code exchange with Keycloak token endpoint
//   - id_token signature verification (using Keycloak's JWKS)
//   - Nonce check (prevents replay attacks)
//   - Claims extraction (sub, email, name, groups)
func (p *OIDCProvider) Exchange(ctx context.Context, code, nonce string) (*Claims, error) {
	token, err := p.oauth2Cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oidc: exchange code: %w", err)
	}

	// The id_token is embedded in the OAuth2 token response as an extra field.
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("oidc: no id_token in token response")
	}

	// Verify the id_token: signature (via Keycloak JWKS), issuer, audience, expiry.
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("oidc: verify id_token: %w", err)
	}

	// Nonce check: the nonce in the token must match the one we sent in AuthURL.
	// A mismatch means the token was issued for a different auth request — reject it.
	if idToken.Nonce != nonce {
		return nil, fmt.Errorf("oidc: nonce mismatch — possible token replay attack")
	}

	// Extract all claims as a raw map so we can read standard and custom fields.
	var raw map[string]any
	if err := idToken.Claims(&raw); err != nil {
		return nil, fmt.Errorf("oidc: extract claims: %w", err)
	}

	claims := &Claims{
		Sub:      idToken.Subject,
		Provider: "oidc",
	}

	if v, ok := raw["email"].(string); ok {
		claims.Email = v
	}
	if v, ok := raw["name"].(string); ok {
		claims.DisplayName = v
	}
	if v, ok := raw["preferred_username"].(string); ok {
		claims.Username = v
	}

	// Extract group membership from the configurable groups claim.
	// Keycloak returns groups as a JSON array of strings: ["devportal-admins", "backend-team"]
	if groups, ok := raw[p.groupsClaim].([]any); ok {
		for _, g := range groups {
			if s, ok := g.(string); ok {
				claims.Groups = append(claims.Groups, s)
			}
		}
	}

	return claims, nil
}
