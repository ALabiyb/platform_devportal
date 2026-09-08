// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com

// Per-language accent colour, backed by the --lang-* tokens in index.css.
// Single source of truth for every build-tool dot / label in the app.
export const LANG_COLOR: Record<string, string> = {
  maven:  "var(--lang-maven)",
  java:   "var(--lang-maven)",
  gradle: "var(--lang-gradle)",
  go:     "var(--lang-go)",
  node:   "var(--lang-node)",
  npm:    "var(--lang-node)",
  "nodejs-express": "var(--lang-node)",
  nextjs: "var(--lang-node)",
  python: "var(--lang-python)",
  pip:    "var(--lang-python)",
  "python-fastapi": "var(--lang-python)",
  docker: "var(--lang-docker)",
  dotnet: "var(--lang-dotnet)",
};

export function langColor(buildTool: string | undefined): string {
  return (buildTool && LANG_COLOR[buildTool]) || "var(--faint)";
}
