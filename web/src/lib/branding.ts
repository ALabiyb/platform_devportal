// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com

export interface Brand {
  app_name: string;
  company: string;
  primary_hue: number;    // 0-360 — main CTAs, buttons, links, focus ring
  secondary_hue: number;  // 0-360 — badges, tags, secondary highlights
  surface_hue: number;    // 0-360 — subtle tint for panel/card/border surfaces
  logo_url: string;
  // "local" → email/password form  |  "oidc" → SSO redirect button
  auth_mode: "local" | "oidc";
}

export const DEFAULT_BRAND: Brand = {
  app_name: "DevPortal",
  company: "",
  primary_hue: 199,    // sky blue
  secondary_hue: 262,  // violet
  surface_hue: 215,    // cool gray-blue
  logo_url: "",
  auth_mode: "local",
};

// fetchBrand loads branding config from the Go server.
// Falls back to DEFAULT_BRAND on any network or parse error so the app
// always boots even if the endpoint is temporarily unavailable.
export async function fetchBrand(): Promise<Brand> {
  try {
    const res = await fetch("/branding.json");
    if (!res.ok) return DEFAULT_BRAND;
    const data = await res.json() as Brand;
    return { ...DEFAULT_BRAND, ...data };
  } catch {
    return DEFAULT_BRAND;
  }
}

// applyBrandCSS writes all three brand hues as CSS custom properties.
// Called once before React renders so there is no colour flash on load.
// index.css derives every colour token from these three properties via
// hsl(var(--brand-hue) ...) — the full palette shifts from a single call.
export function applyBrandCSS(brand: Brand): void {
  const root = document.documentElement;
  const clamp = (h: number) => Math.max(0, Math.min(360, Math.round(h)));
  root.style.setProperty("--brand-hue",           String(clamp(brand.primary_hue)));
  root.style.setProperty("--brand-secondary-hue", String(clamp(brand.secondary_hue)));
  root.style.setProperty("--brand-surface-hue",   String(clamp(brand.surface_hue)));
}
