// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// handlers.go wires the OAuth 2.0 / OIDC login flow to HTTP endpoints.
//
// Mount these with a chi router:
//
//	h := auth.NewHandler(provider, database, cfg)
//	r.Get("/auth/login",    h.HandleLogin)
//	r.Get("/auth/callback", h.HandleCallback)
//	r.Post("/auth/logout",  h.HandleLogout)
//	r.Get("/auth/me",       h.HandleMe)   // requires RequireAuth middleware
//
// State and nonce are stored in short-lived cookies (5-minute TTL, HttpOnly,
// SameSite=Lax) and verified in HandleCallback to prevent CSRF and token replay.

package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ALabiyb/platform_devportal/internal/config"
	"github.com/ALabiyb/platform_devportal/internal/db"
)

// Handler holds the dependencies needed by the auth HTTP handlers.
// Create one with NewHandler and share it for the lifetime of the process.
type Handler struct {
	provider AuthProvider
	database *db.DB
	cfg      *config.Config
}

// NewHandler constructs an auth Handler.
func NewHandler(provider AuthProvider, database *db.DB, cfg *config.Config) *Handler {
	return &Handler{
		provider: provider,
		database: database,
		cfg:      cfg,
	}
}

// HandleLogin starts the OAuth / OIDC login flow.
//
// It generates a cryptographically random state token (CSRF protection) and a
// nonce (OIDC replay protection), stores both in short-lived HttpOnly cookies,
// then redirects the browser to the identity provider's login page.
//
// GET /auth/login
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := randomToken()
	if err != nil {
		http.Error(w, "failed to generate state token", http.StatusInternalServerError)
		return
	}
	nonce, err := randomToken()
	if err != nil {
		http.Error(w, "failed to generate nonce", http.StatusInternalServerError)
		return
	}

	// Short-lived cookies (5 min) — long enough for a human to complete login,
	// short enough to limit the CSRF window.
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_state",
		Value:    state,
		Path:     "/auth/callback",
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_nonce",
		Value:    nonce,
		Path:     "/auth/callback",
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, h.provider.AuthURL(state, nonce), http.StatusFound)
}

// HandleCallback handles the redirect back from the identity provider.
//
// Flow:
//  1. Read and validate the state cookie (CSRF check)
//  2. Read the nonce cookie (OIDC replay check)
//  3. Exchange the authorization code for user Claims
//  4. Upsert the org and user in the database
//  5. Create a new DB-backed session
//  6. Set the session cookie and redirect to /
//
// GET /auth/callback?code=…&state=…
func (h *Handler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	// ── CSRF check ────────────────────────────────────────────────────────────
	stateCookie, err := r.Cookie("auth_state")
	if err != nil || stateCookie.Value == "" {
		http.Error(w, "missing state cookie — possible CSRF", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "state mismatch — possible CSRF attack", http.StatusBadRequest)
		return
	}

	nonceCookie, err := r.Cookie("auth_nonce")
	if err != nil || nonceCookie.Value == "" {
		http.Error(w, "missing nonce cookie", http.StatusBadRequest)
		return
	}

	// ── Exchange the code ─────────────────────────────────────────────────────
	code := r.URL.Query().Get("code")
	if code == "" {
		// The identity provider may send an "error" parameter on failure.
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			http.Error(w, "provider error: "+errParam, http.StatusBadRequest)
			return
		}
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	claims, err := h.provider.Exchange(r.Context(), code, nonceCookie.Value)
	if err != nil {
		http.Error(w, "authentication failed: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// ── Upsert org and user ───────────────────────────────────────────────────
	org, err := h.database.EnsureOrg(r.Context(), h.cfg.OrgName, h.cfg.OrgSlug)
	if err != nil {
		http.Error(w, "failed to initialise organisation", http.StatusInternalServerError)
		return
	}

	role := RoleFromClaims(claims, h.cfg.OIDCAdminGroup, h.cfg.OIDCDeveloperGroup, h.cfg.AdminEmail)

	user, err := h.database.UpsertUser(r.Context(), db.User{
		OrgID:       org.ID,
		Email:       claims.Email,
		DisplayName: claims.DisplayName,
		Provider:    claims.Provider,
		ProviderID:  claims.Sub,
	})
	if err != nil {
		http.Error(w, "failed to upsert user", http.StatusInternalServerError)
		return
	}

	// ── Create session ────────────────────────────────────────────────────────
	sessionID := uuid.New().String()
	session := db.Session{
		ID:        sessionID,
		UserID:    user.ID,
		Role:      role,
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	if err := h.database.CreateSession(r.Context(), session); err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	// Clear the short-lived auth cookies — they're no longer needed.
	http.SetCookie(w, &http.Cookie{Name: "auth_state", Value: "", Path: "/auth/callback", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: "auth_nonce", Value: "", Path: "/auth/callback", MaxAge: -1})

	// Set the long-lived session cookie (24 h, sliding — extended by RequireAuth).
	http.SetCookie(w, &http.Cookie{
		Name:     "devportal_session",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400, // 24 hours
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

// HandleLogout ends the authenticated session.
//
// It deletes the session from the database (so the cookie can't be replayed
// after logout) and clears the session cookie in the browser.
//
// POST /auth/logout
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("devportal_session")
	if err == nil && cookie.Value != "" {
		// Best-effort deletion — if the session is already gone (e.g. expired),
		// we still clear the cookie and return 200.
		_ = h.database.DeleteSession(r.Context(), cookie.Value)
	}

	http.SetCookie(w, expiredSessionCookie())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "logged out"})
}

// HandleMe returns the authenticated user's profile as JSON.
// Requires RequireAuth middleware — callers are guaranteed to have a valid session.
//
// GET /auth/me
func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		// Should never happen if RequireAuth is applied, but be defensive.
		writeUnauthorized(w, "not authenticated")
		return
	}

	session, _ := SessionFromContext(r.Context())

	type meResponse struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Provider    string `json:"provider"`
		ExpiresAt   string `json:"session_expires_at,omitempty"`
	}

	role := ""
	if session != nil {
		role = session.Role
	}

	resp := meResponse{
		ID:          user.ID.String(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Role:        role,
		Provider:    user.Provider,
	}
	if session != nil {
		resp.ExpiresAt = session.ExpiresAt.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// randomToken generates a cryptographically random 32-byte URL-safe token.
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
