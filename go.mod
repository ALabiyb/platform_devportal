module github.com/ALabiyb/platform_devportal

go 1.23

require (
	// HTTP router — lightweight, idiomatic, stdlib-compatible
	github.com/go-chi/chi/v5 v5.1.0

	// OIDC token validation for Keycloak auth
	github.com/coreos/go-oidc/v3 v3.11.0

	// OAuth2 (required by go-oidc)
	golang.org/x/oauth2 v0.24.0

	// PostgreSQL driver — pure Go, no CGO dependency
	github.com/jackc/pgx/v5 v5.7.1

	// UUID generation for all primary keys
	github.com/google/uuid v1.6.0

	// Rate limiting middleware
	golang.org/x/time v0.8.0
)
