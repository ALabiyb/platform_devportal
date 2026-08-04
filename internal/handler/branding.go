// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// branding.go exposes GET /branding.json — a public endpoint that returns
// the portal's brand config read from environment variables.
//
// The React SPA fetches this once at boot (before first render) and applies
// the values as CSS custom properties and React context. The same Docker image
// ships to every customer; branding is purely env-var config.
//
// Response shape:
//
//	{
//	  "app_name":    "DevPortal",
//	  "company":     "Acme Corp",
//	  "primary_hue": 199,
//	  "logo_url":    "https://cdn.example.com/logo.svg"
//	}

package handler

import (
	"encoding/json"
	"net/http"
)

type brandingResponse struct {
	AppName    string `json:"app_name"`
	Company    string `json:"company"`
	PrimaryHue int    `json:"primary_hue"`
	LogoURL    string `json:"logo_url"`
	// AuthMode tells the React SPA which login UI to render:
	// "local" → email/password form  |  "oidc" → SSO redirect button
	AuthMode string `json:"auth_mode"`
}

func (h *Handler) handleBranding(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// 5-minute public cache — safe because branding only changes on redeploy.
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = json.NewEncoder(w).Encode(brandingResponse{
		AppName:    h.cfg.BrandAppName,
		Company:    h.cfg.BrandCompany,
		PrimaryHue: h.cfg.BrandPrimaryHue,
		LogoURL:    h.cfg.BrandLogoURL,
		AuthMode:   h.cfg.AuthMode,
	})
}
