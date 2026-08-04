// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import { queryClient } from "@/lib/queryClient";
import { fetchBrand, applyBrandCSS } from "@/lib/branding";
import { BrandContext } from "@/contexts/BrandContext";
import { Router } from "@/router";
import "@/index.css";

// Fetch branding before the first render so CSS custom properties are applied
// before any component paints — zero colour flash on load.
async function bootstrap() {
  const brand = await fetchBrand();
  applyBrandCSS(brand);

  ReactDOM.createRoot(document.getElementById("root")!).render(
    <React.StrictMode>
      <BrandContext.Provider value={brand}>
        <QueryClientProvider client={queryClient}>
          <Router />
        </QueryClientProvider>
      </BrandContext.Provider>
    </React.StrictMode>
  );
}

bootstrap();
