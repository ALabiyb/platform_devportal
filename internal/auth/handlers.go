// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// handlers.go wires auth flows to HTTP endpoints.
//
// Two modes — routes registered differ per AUTH_MODE:
//
//	OIDC mode:
//	  GET  /auth/login    → redirect to Keycloak (HandleLogin)
//	  GET  /auth/callback → exchange code for session (HandleCallback)
//
//	Local mode:
//	  POST /auth/login    → validate email+password, create session (HandleLocalLogin)
//	  POST /auth/register → create first admin account (HandleLocalRegister)
//	  POST /api/v1/users  → admin creates additional accounts (wired in handler pkg)
//
//	Both modes:
//	  POST /auth/logout   → delete session (HandleLogout)
//	  GET  /auth/me       → current user info (HandleMe)

package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ALabiyb/platform_devportal/internal/config"
	"github.com/ALabiyb/platform_devportal/internal/db"
)

// Handler holds the dependencies for all auth HTTP endpoints.
// Exactly one of oidcProvider or localProvider is non-nil, matching AUTH_MODE.
type Handler struct {
	oidcProvider  AuthProvider   // non-nil when AUTH_MODE=oidc
	localProvider *LocalProvider // non-nil when AUTH_MODE=local
	database      *db.DB
	cfg           *config.Config
}

// NewOIDCHandler creates a Handler wired for Keycloak OIDC auth.
func NewOIDCHandler(provider AuthProvider, database *db.DB, cfg *config.Config) *Handler {
	return &Handler{oidcProvider: provider, database: database, cfg: cfg}
}

// NewLocalHandler creates a Handler wired for local username/password auth.
func NewLocalHandler(provider *LocalProvider, database *db.DB, cfg *config.Config) *Handler {
	return &Handler{localProvider: provider, database: database, cfg: cfg}
}

// NewHandler is a convenience constructor that picks the right handler type from cfg.
// Pass the OIDC provider when AUTH_MODE=oidc, or the local provider when AUTH_MODE=local.
// Exactly one of oidc/local must be non-nil.
func NewHandler(oidc AuthProvider, local *LocalProvider, database *db.DB, cfg *config.Config) *Handler {
	return &Handler{oidcProvider: oidc, localProvider: local, database: database, cfg: cfg}
}

// ── OIDC handlers ─────────────────────────────────────────────────────────────

// HandleLogin starts the OIDC login flow — generates state + nonce cookies,
// then redirects the browser to the Keycloak login page.
// GET /auth/login  (OIDC mode only)
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := randomToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	nonce, err := randomToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, shortCookie("auth_state", state))
	http.SetCookie(w, shortCookie("auth_nonce", nonce))
	http.Redirect(w, r, h.oidcProvider.AuthURL(state, nonce), http.StatusFound)
}

// HandleCallback handles the Keycloak redirect back after login.
// Verifies state (CSRF), exchanges code for Claims, upserts user + session.
// GET /auth/callback  (OIDC mode only)
func (h *Handler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("auth_state")
	if err != nil || stateCookie.Value == "" || r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "state mismatch — possible CSRF", http.StatusBadRequest)
		return
	}
	nonceCookie, err := r.Cookie("auth_nonce")
	if err != nil || nonceCookie.Value == "" {
		http.Error(w, "missing nonce cookie", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		if e := r.URL.Query().Get("error"); e != "" {
			http.Error(w, "provider error: "+e, http.StatusBadRequest)
			return
		}
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	claims, err := h.oidcProvider.Exchange(r.Context(), code, nonceCookie.Value)
	if err != nil {
		http.Error(w, "authentication failed: "+err.Error(), http.StatusUnauthorized)
		return
	}

	role := RoleFromClaims(claims, h.cfg.OIDCAdminGroup, h.cfg.OIDCDeveloperGroup, h.cfg.AdminEmail)
	if err := h.finishLogin(w, r, claims, role); err != nil {
		http.Error(w, "login failed", http.StatusInternalServerError)
		return
	}

	// Clear the short-lived CSRF cookies.
	http.SetCookie(w, expireCookie("auth_state", "/auth/callback"))
	http.SetCookie(w, expireCookie("auth_nonce", "/auth/callback"))
	http.Redirect(w, r, "/", http.StatusFound)
}

// ── Local auth handlers ────────────────────────────────────────────────────────

// HandleLocalLogin validates email + password and creates a session.
// POST /auth/login  (local mode only)
func (h *Handler) HandleLocalLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" {
		jsonError(w, "email and password are required", http.StatusBadRequest)
		return
	}

	claims, err := h.localProvider.Authenticate(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrAccountDisabled) {
			jsonError(w, err.Error(), http.StatusForbidden)
			return
		}
		// Always return the same message for wrong email/password to prevent enumeration.
		jsonError(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	// Fetch the stored role from the DB (local users have role in users table).
	dbUser, err := h.database.GetLocalUserByEmail(r.Context(), req.Email)
	if err != nil {
		jsonError(w, "login failed", http.StatusInternalServerError)
		return
	}

	if err := h.finishLogin(w, r, claims, dbUser.Role); err != nil {
		jsonError(w, "login failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleLocalRegister creates the first admin account.
// Succeeds only when no users exist in the org yet (bootstrap only).
// After that, only admins can create users (via POST /api/v1/users).
// POST /auth/register  (local mode only)
func (h *Handler) HandleLocalRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.DisplayName == "" || req.Password == "" {
		jsonError(w, "email, display_name, and password are required", http.StatusBadRequest)
		return
	}

	org, err := h.database.EnsureOrg(r.Context(), h.cfg.OrgName, h.cfg.OrgSlug)
	if err != nil {
		jsonError(w, "failed to initialise organisation", http.StatusInternalServerError)
		return
	}

	// Bootstrap check: only allowed when the org has zero users.
	count, err := h.database.CountUsers(r.Context(), org.ID)
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if count > 0 {
		jsonError(w, "registration is closed — ask your administrator to create an account", http.StatusForbidden)
		return
	}

	// First user always gets admin.
	user, err := h.localProvider.CreateUser(r.Context(), org.ID, req.Email, req.DisplayName, RoleAdmin, req.Password)
	if err != nil {
		jsonError(w, "failed to create account: "+err.Error(), http.StatusConflict)
		return
	}

	claims := &Claims{
		Sub:         user.ID.String(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Provider:    "local",
	}
	if err := h.finishLogin(w, r, claims, RoleAdmin); err != nil {
		jsonError(w, "account created but login failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "admin account created",
		"email":  user.Email,
	})
}

// ── Shared handlers ────────────────────────────────────────────────────────────

// HandleLogout deletes the session from the DB and clears the cookie.
// POST /auth/logout  (both modes)
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("devportal_session"); err == nil && cookie.Value != "" {
		_ = h.database.DeleteSession(r.Context(), cookie.Value)
	}
	http.SetCookie(w, expiredSessionCookie())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "logged out"})
}

// HandleMe returns the authenticated user's profile as JSON.
// GET /auth/me  (both modes, requires RequireAuth middleware)
func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, "not authenticated")
		return
	}
	session, _ := SessionFromContext(r.Context())

	type resp struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Provider    string `json:"provider"`
		IsActive    bool   `json:"is_active"`
		ExpiresAt   string `json:"session_expires_at,omitempty"`
	}

	role := user.Role
	expiresAt := ""
	if session != nil {
		role = session.Role
		expiresAt = session.ExpiresAt.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp{
		ID:          user.ID.String(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Role:        role,
		Provider:    user.Provider,
		IsActive:    user.IsActive,
		ExpiresAt:   expiresAt,
	})
}

// ── Internal helpers ───────────────────────────────────────────────────────────

// finishLogin is shared by both OIDC callback and local login.
// It upserts the org + user in the DB, creates a session, and sets the cookie.
func (h *Handler) finishLogin(w http.ResponseWriter, r *http.Request, claims *Claims, role string) error {
	org, err := h.database.EnsureOrg(r.Context(), h.cfg.OrgName, h.cfg.OrgSlug)
	if err != nil {
		return err
	}

	user, err := h.database.UpsertUser(r.Context(), db.User{
		OrgID:       org.ID,
		Email:       claims.Email,
		DisplayName: claims.DisplayName,
		Provider:    claims.Provider,
		ProviderID:  claims.Sub,
		Role:        role,
	})
	if err != nil {
		return err
	}

	sessionID := uuid.New().String()
	if err := h.database.CreateSession(r.Context(), db.Session{
		ID:        sessionID,
		UserID:    user.ID,
		Role:      role,
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}); err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "devportal_session",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// randomToken generates a cryptographically random 32-byte URL-safe token.
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// shortCookie creates a short-lived (5 min) HttpOnly cookie for CSRF state/nonce.
func shortCookie(name, value string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/auth/callback",
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// expireCookie immediately expires a cookie in the browser.
func expireCookie(name, path string) *http.Cookie {
	return &http.Cookie{Name: name, Value: "", Path: path, MaxAge: -1}
}

// expiredSessionCookie clears the session cookie in the browser.
func expiredSessionCookie() *http.Cookie {
	return &http.Cookie{Name: "devportal_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode}
}

// jsonError writes a JSON error response.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
