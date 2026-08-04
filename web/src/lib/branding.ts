// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com

export interface Brand {
  app_name: string;
  company: string;
  primary_hue: number;
  logo_url: string;
}

export const DEFAULT_BRAND: Brand = {
  app_name: "DevPortal",
  company: "",
  primary_hue: 199,
  logo_url: "",
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

// applyBrandCSS writes the brand hue into the CSS custom property that
// drives every primary/ring colour token in the dark theme.
// Called once before React renders so there is no colour flash on load.
export function applyBrandCSS(brand: Brand): void {
  const hue = Math.max(0, Math.min(360, brand.primary_hue));
  document.documentElement.style.setProperty("--brand-hue", String(hue));
}
