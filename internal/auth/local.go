// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// local.go implements built-in username/password authentication.
//
// Unlike OIDC, there is no external redirect — the user submits email + password
// directly to POST /auth/login. devportal hashes and verifies passwords with
// bcrypt (cost 12) and stores the hash in users.password_hash.
//
// Bootstrap rule:
//   If no users exist in the org yet, the first POST /auth/register creates
//   an admin account without requiring an existing admin to approve it.
//   Once at least one user exists, only an admin can create new accounts.
//
// This provider does NOT implement the AuthProvider interface (which is built
// for redirect-based OAuth flows). It exposes its own Authenticate() method,
// called directly by HandleLocalLogin instead of the Exchange() path.

package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/ALabiyb/platform_devportal/internal/config"
	"github.com/ALabiyb/platform_devportal/internal/db"
)

// bcryptCost is the work factor for bcrypt password hashing.
// Cost 12 requires ~300ms to hash on a modern CPU — slow enough to make
// brute-force attacks impractical, fast enough for interactive login.
const bcryptCost = 12

// ErrInvalidCredentials is returned by Authenticate when the email/password
// combination is wrong. Use this sentinel to avoid leaking whether the email
// exists or the password is wrong (use the same message for both).
var ErrInvalidCredentials = errors.New("invalid email or password")

// ErrAccountDisabled is returned when the user exists but is_active = false.
var ErrAccountDisabled = errors.New("account is disabled — contact your administrator")

// LocalProvider manages local user account authentication.
// It does not implement AuthProvider (no redirect flow).
type LocalProvider struct {
	db  *db.DB
	cfg *config.Config
}

// NewLocalProvider constructs a LocalProvider.
func NewLocalProvider(database *db.DB, cfg *config.Config) *LocalProvider {
	return &LocalProvider{db: database, cfg: cfg}
}

// Authenticate validates email + plaintext password against the stored bcrypt hash.
// Returns a Claims set on success, ErrInvalidCredentials on wrong email/password,
// or ErrAccountDisabled if the account has been deactivated.
func (p *LocalProvider) Authenticate(ctx context.Context, email, password string) (*Claims, error) {
	user, err := p.db.GetLocalUserByEmail(ctx, email)
	if err != nil {
		// Treat "not found" the same as "wrong password" to prevent email enumeration.
		return nil, ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, ErrAccountDisabled
	}

	if user.PasswordHash == nil || *user.PasswordHash == "" {
		// User was created by OIDC and has no local password — reject.
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return &Claims{
		Sub:         user.ID.String(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Groups:      nil, // local users have no group claims
		Provider:    "local",
	}, nil
}

// CreateUser hashes the password with bcrypt and persists a new local user.
// The caller is responsible for enforcing the admin-only restriction
// (or the first-user bootstrap exception).
func (p *LocalProvider) CreateUser(ctx context.Context, orgID uuid.UUID, email, displayName, role, password string) (*db.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("local auth: hash password: %w", err)
	}
	return p.db.CreateLocalUser(ctx, orgID, email, displayName, role, string(hash))
}

// SetPassword replaces the password hash for a local user.
// Used by the admin "reset password" endpoint.
func (p *LocalProvider) SetPassword(ctx context.Context, userID uuid.UUID, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("local auth: hash password: %w", err)
	}
	return p.db.SetUserPasswordHash(ctx, userID, string(hash))
}
