// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// middleware.go provides HTTP middleware for session-based authentication
// and role-based access control (RBAC).
//
// Usage in a chi router:
//
//	r.Group(func(r chi.Router) {
//	    r.Use(auth.RequireAuth(database, cfg))
//	    r.Use(auth.RequireRole(auth.RoleDeveloper))
//	    r.Get("/projects", handlers.ListProjects)
//	})

package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ALabiyb/platform_devportal/internal/db"
)

// contextKey is an unexported type used for context keys in this package.
// Using a private type prevents key collisions with other packages that
// also store values in context (e.g. chi, tracing).
type contextKey int

const (
	sessionCtxKey contextKey = iota
	userCtxKey
)

// SessionFromContext retrieves the authenticated session from the request context.
// Returns (nil, false) if the request was not authenticated (i.e. RequireAuth
// middleware was not applied or authentication failed).
func SessionFromContext(ctx context.Context) (*db.Session, bool) {
	s, ok := ctx.Value(sessionCtxKey).(*db.Session)
	return s, ok && s != nil
}

// UserFromContext retrieves the authenticated user from the request context.
// Returns (nil, false) if the request was not authenticated.
func UserFromContext(ctx context.Context) (*db.User, bool) {
	u, ok := ctx.Value(userCtxKey).(*db.User)
	return u, ok && u != nil
}

// RequireAuth is a chi-compatible middleware that enforces session authentication.
//
// On each request it:
//  1. Reads the devportal_session cookie
//  2. Loads the session from the database (rejects expired sessions)
//  3. Extends the session expiry by 24 h (sliding window)
//  4. Loads the user record from the database
//  5. Stores both in the request context for downstream handlers
//
// Unauthenticated requests are rejected with 401 JSON. Handlers behind this
// middleware can safely call SessionFromContext and UserFromContext.
func RequireAuth(database *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("devportal_session")
			if err != nil {
				// Cookie absent — not logged in.
				writeUnauthorized(w, "authentication required")
				return
			}

			session, err := database.GetSession(r.Context(), cookie.Value)
			if err != nil || session == nil {
				// Session missing or expired.
				http.SetCookie(w, expiredSessionCookie())
				writeUnauthorized(w, "session expired — please log in again")
				return
			}

			// Slide the expiry window forward on each authenticated request.
			if err := database.TouchSession(r.Context(), session.ID); err != nil {
				// Non-fatal — log and continue. The session is still valid;
				// TouchSession failure just means the window won't slide this tick.
				// Structured logging is handled by the caller's log middleware.
				_ = err
			}

			user, err := database.GetUserByID(r.Context(), session.UserID)
			if err != nil || user == nil {
				writeUnauthorized(w, "user not found")
				return
			}

			// Store session and user in context for downstream handlers.
			ctx := context.WithValue(r.Context(), sessionCtxKey, session)
			ctx = context.WithValue(ctx, userCtxKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns middleware that enforces a minimum RBAC role.
// Must be placed AFTER RequireAuth in the middleware chain.
//
// Returns 403 JSON if the authenticated user's role is below minimum.
func RequireRole(minimum string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := SessionFromContext(r.Context())
			if !ok {
				// RequireAuth was not applied — programming error.
				writeUnauthorized(w, "authentication required")
				return
			}

			if !RoleAtLeast(session.Role, minimum) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "insufficient permissions — required role: " + minimum,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// writeUnauthorized sends a 401 JSON response.
func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// expiredSessionCookie returns a Set-Cookie header that immediately deletes
// the devportal_session cookie in the browser.
func expiredSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     "devportal_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}
