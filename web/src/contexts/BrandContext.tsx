// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { createContext, useContext } from "react";
import { type Brand, DEFAULT_BRAND } from "@/lib/branding";

export const BrandContext = createContext<Brand>(DEFAULT_BRAND);

export function useBrand(): Brand {
  return useContext(BrandContext);
}
