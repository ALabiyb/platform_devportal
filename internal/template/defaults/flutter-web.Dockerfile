# =============================================================================
# Production Dockerfile — React / Next.js / Vite Frontend (multi-stage)
# Build stage compiles static assets; runtime stage serves via nginx.
# Final image: ~25 MB (nginx-unprivileged + built assets — no node_modules).
#
# Best practices: multi-stage (drops node_modules), nginxinc/nginx-unprivileged
# (official non-root nginx, runs as UID 101, listens on 8080), OCI labels.
#
# ADJUST before use:
#   COPY --from=builder line — match your framework's output directory:
#     Vite / React CRA : /app/dist
#     Create React App : /app/build
#     Next.js static   : /app/out  (requires output: 'export' in next.config.js)
# This image also requires nginx.conf in your repo root (committed alongside it).
# =============================================================================

# ── Stage 1: Build ─────────────────────────────────────────────────────────
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files first — better layer caching (only re-runs when deps change)
COPY package.json package-lock.json ./
RUN npm ci --frozen-lockfile

COPY . .
RUN npm run build
# Output: dist/ (Vite), build/ (CRA), out/ (Next.js static export)

# ── Stage 2: Serve ──────────────────────────────────────────────────────────
# nginxinc/nginx-unprivileged = official non-root nginx:
#   - Pre-configured to run as UID 101 (not root)
#   - Listens on 8080 (non-privileged port)
#   - Temp/cache paths already set to /tmp (writable without root)
FROM nginxinc/nginx-unprivileged:1.27-alpine

ARG APP_NAME=app
ARG GIT_AUTHOR=unknown
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown
ARG VERSION=unknown

LABEL org.opencontainers.image.title="${APP_NAME}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.authors="${GIT_AUTHOR}" \
      org.opencontainers.image.vendor="Softnet"

# nginx.conf is committed to the repo alongside this Dockerfile
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy built assets — adjust source path to match your framework (see header)
COPY --from=builder /app/dist /usr/share/nginx/html

# Declare numeric UID explicitly so OPA and Kubernetes runAsNonRoot can verify it
USER 101

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/healthz || exit 1

CMD ["nginx", "-g", "daemon off;"]
