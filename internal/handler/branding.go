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
//	  "app_name":       "DevPortal",
//	  "company":        "Acme Corp",
//	  "primary_hue":    199,
//	  "secondary_hue":  262,
//	  "surface_hue":    215,
//	  "logo_url":       "https://cdn.example.com/logo.svg",
//	  "auth_mode":      "oidc"
//	}
//
// React reads all three hues and writes them as CSS custom properties before
// the first render — --brand-hue, --brand-secondary-hue, --brand-surface-hue.
// Every color token in index.css is derived from these three properties via
// hsl(var(--brand-*) ...) so the full palette shifts with a single env-var change.

package handler

import (
	"encoding/json"
	"net/http"
)

type brandingResponse struct {
	AppName      string `json:"app_name"`
	Company      string `json:"company"`
	PrimaryHue   int    `json:"primary_hue"`
	SecondaryHue int    `json:"secondary_hue"`
	SurfaceHue   int    `json:"surface_hue"`
	LogoURL      string `json:"logo_url"`
	// AuthMode tells the React SPA which login UI to render:
	// "local" → email/password form  |  "oidc" → SSO redirect button
	AuthMode string `json:"auth_mode"`
}

func (h *Handler) handleBranding(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// 5-minute public cache — safe because branding only changes on redeploy.
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = json.NewEncoder(w).Encode(brandingResponse{
		AppName:      h.cfg.BrandAppName,
		Company:      h.cfg.BrandCompany,
		PrimaryHue:   h.cfg.BrandPrimaryHue,
		SecondaryHue: h.cfg.BrandSecondaryHue,
		SurfaceHue:   h.cfg.BrandSurfaceHue,
		LogoURL:      h.cfg.BrandLogoURL,
		AuthMode:     h.cfg.AuthMode,
	})
}
