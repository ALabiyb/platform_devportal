// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// Package auth handles all authentication and authorisation for devportal.
//
// Providers:
//   - OIDC (Keycloak)    — primary, recommended for production
//   - GitLab OAuth        — fallback for personal deployments without Keycloak
//
// Both providers implement AuthProvider so the rest of the app never needs
// to know which backend is active — it just calls AuthURL and Exchange.
//
// RBAC roles (least → most privileged):
//   viewer    — read-only: view projects, environments, generated files
//   developer — create projects, update pipeline config
//   admin     — everything: manage credentials, view audit log, all orgs/teams
package auth

import "context"

// AuthProvider is the interface implemented by both the OIDC and GitLab providers.
// Swapping auth backends requires only changing which concrete type is wired
// in main.go — no handler or middleware code changes needed.
type AuthProvider interface {
	// AuthURL returns the URL to redirect the browser to for authentication.
	// state is a CSRF token stored in a short-lived cookie.
	// nonce is OIDC replay protection stored in a short-lived cookie.
	// GitLab OAuth ignores nonce — it is accepted for interface compatibility.
	AuthURL(state, nonce string) string

	// Exchange trades the authorization code (from the callback query param)
	// for a validated, normalised Claims set. Returns an error if the code
	// is invalid, expired, or the id_token fails verification.
	Exchange(ctx context.Context, code, nonce string) (*Claims, error)
}

// Claims holds normalised user information from the identity provider.
// Both OIDC and GitLab OAuth populate this struct so downstream code is provider-agnostic.
type Claims struct {
	// Sub is the stable unique user identifier from the identity provider.
	// For Keycloak OIDC: the "sub" JWT claim.
	// For GitLab OAuth: the numeric user ID converted to string.
	Sub string

	Email       string
	DisplayName string
	Username    string

	// Groups contains Keycloak group memberships from the "groups" id_token claim.
	// Empty for GitLab OAuth (GitLab OAuth does not expose group membership).
	Groups []string

	// Provider is "oidc" or "gitlab" — stored in the users table so we know
	// which backend authenticated this user on future logins.
	Provider string
}

// Role constants — the only valid values for sessions.role and team_members.role.
const (
	RoleAdmin     = "admin"
	RoleDeveloper = "developer"
	RoleViewer    = "viewer"
)

// RoleFromClaims maps the identity provider's group claims to a devportal role.
//
// Priority order:
//  1. adminEmail match   → admin  (bootstrap: used before Keycloak groups exist)
//  2. adminGroup match   → admin
//  3. developerGroup match → developer
//  4. no match           → viewer (least-privilege default)
func RoleFromClaims(claims *Claims, adminGroup, developerGroup, adminEmail string) string {
	if adminEmail != "" && claims.Email == adminEmail {
		return RoleAdmin
	}
	for _, g := range claims.Groups {
		if g == adminGroup {
			return RoleAdmin
		}
	}
	for _, g := range claims.Groups {
		if g == developerGroup {
			return RoleDeveloper
		}
	}
	return RoleViewer
}

// RoleAtLeast returns true when role is at least as privileged as minimum.
// Used by middleware to enforce route-level access control.
func RoleAtLeast(role, minimum string) bool {
	rank := map[string]int{RoleViewer: 1, RoleDeveloper: 2, RoleAdmin: 3}
	return rank[role] >= rank[minimum]
}
