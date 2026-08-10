// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// credentials.go — HTTP handlers for /api/v1/credentials (admin-only).
// Values are encrypted at rest with AES-256-GCM using the ENCRYPTION_KEY
// from config. The raw token never reaches the database.

package handler

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	authpkg "github.com/ALabiyb/platform_devportal/internal/auth"
	"github.com/ALabiyb/platform_devportal/internal/db"
	"github.com/jackc/pgx/v5"
)

// ── Crypto helpers ────────────────────────────────────────────────────────────

func (h *Handler) encryptionKey() ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(h.cfg.EncryptionKey)
	if err != nil {
		return nil, errors.New("invalid ENCRYPTION_KEY: must be base64-encoded 32 bytes")
	}
	if len(key) != 32 {
		return nil, errors.New("invalid ENCRYPTION_KEY: must decode to exactly 32 bytes")
	}
	return key, nil
}

func (h *Handler) encryptToken(plaintext string) ([]byte, error) {
	key, err := h.encryptionKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// ── Credential response type ─────────────────────────────────────────────────

type credentialResponse struct {
	ID           string `json:"id"`
	ProviderType string `json:"provider_type"`
	Label        string `json:"label"`
	CreatedAt    string `json:"created_at"`
}

func toCredResp(c db.WorkspaceCredential) credentialResponse {
	return credentialResponse{
		ID:           c.ID.String(),
		ProviderType: c.ProviderType,
		Label:        c.Label,
		CreatedAt:    c.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// ListCredentials returns all workspace credentials for the org (no raw values).
func (h *Handler) ListCredentials(w http.ResponseWriter, r *http.Request) {
	user, _ := authpkg.UserFromContext(r.Context())
	creds, err := h.db.ListCredentials(r.Context(), user.OrgID)
	if err != nil {
		jsonError(w, "list credentials: "+err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]credentialResponse, len(creds))
	for i, c := range creds {
		out[i] = toCredResp(c)
	}
	jsonOK(w, out)
}

// CreateCredential encrypts the provided token and stores it.
// Body: {"provider_type":"gitlab","label":"service account","token":"glpat-…"}
func (h *Handler) CreateCredential(w http.ResponseWriter, r *http.Request) {
	user, _ := authpkg.UserFromContext(r.Context())

	var req struct {
		ProviderType string `json:"provider_type"`
		Label        string `json:"label"`
		Token        string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.ProviderType == "" || req.Label == "" || req.Token == "" {
		jsonError(w, "provider_type, label, and token are required", http.StatusBadRequest)
		return
	}

	ciphertext, err := h.encryptToken(req.Token)
	if err != nil {
		jsonError(w, "encryption failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	cred, err := h.db.CreateCredential(r.Context(), db.WorkspaceCredential{
		OrgID:          user.OrgID,
		ProviderType:   req.ProviderType,
		Label:          req.Label,
		EncryptedValue: ciphertext,
	})
	if err != nil {
		jsonError(w, "create credential: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toCredResp(*cred))
}

// DeleteCredential removes a credential from the vault.
// DELETE /api/v1/credentials/{credentialID}
func (h *Handler) DeleteCredential(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "credentialID")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, _ := authpkg.UserFromContext(r.Context())
	if err := h.db.DeleteCredential(r.Context(), id, user.OrgID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, "credential not found", http.StatusNotFound)
			return
		}
		jsonError(w, "delete credential: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
