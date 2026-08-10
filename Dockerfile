# =============================================================================
# Author: Labiyb M. Said — DevSecOps Engineer
# Contact: saidlabiybm@gmail.com
# =============================================================================
# Multi-stage build — three stages:
#   1. frontend  — Node 22 builds the React/Vite SPA into web/dist/
#   2. builder   — Go 1.25 compiles the binary (embeds web/dist at compile time)
#   3. runtime   — minimal Alpine image, binary only (~15 MB total)
#
# No local toolchain required. A plain "docker build ." or
# "docker compose build" works on any machine with Docker installed.
# =============================================================================

# ── Stage 1: Frontend ──────────────────────────────────────────────────────
FROM node:22-alpine AS frontend

WORKDIR /app/web

# Cache npm install — only re-runs when package-lock.json changes
COPY web/package.json web/package-lock.json ./
RUN npm ci --prefer-offline

COPY web/ ./
RUN npm run build


# ── Stage 2: Go binary ─────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Cache module download — only re-runs when go.mod/go.sum change
COPY go.mod go.sum ./
RUN go mod download

# Copy source (includes web/dist placeholder from repo)
COPY . .

# Overwrite web/dist with the real Vite build from Stage 1.
# This is what go:embed picks up at compile time.
COPY --from=frontend /app/web/dist ./web/dist

# CGO_ENABLED=0 → fully static binary, no glibc dependency
# -trimpath   → strip local build paths from stack traces
# -ldflags    → strip debug info + symbol table (smaller binary)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s" \
    -o app ./cmd/devportal


# ── Stage 3: Runtime ───────────────────────────────────────────────────────
FROM alpine:3.19

ARG APP_NAME=devportal
ARG GIT_AUTHOR="Labiyb M. Said"
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown
ARG VERSION=unknown
ARG APP_TIMEZONE=Africa/Dar_es_Salaam

LABEL org.opencontainers.image.title="${APP_NAME}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.authors="${GIT_AUTHOR}" \
      org.opencontainers.image.vendor="NexBridge Technologies"

# ca-certificates: outbound HTTPS calls to Gitea, Jenkins, Harbor, etc.
# tzdata: time.LoadLocation for Africa/Dar_es_Salaam and other timezones
RUN apk add --no-cache ca-certificates tzdata

ENV TZ=${APP_TIMEZONE}
RUN cp /usr/share/zoneinfo/${APP_TIMEZONE} /etc/localtime && \
    echo "${APP_TIMEZONE}" > /etc/timezone

# Numeric UID — satisfies OPA runAsNonRoot and Kubernetes PodSecurity
RUN addgroup -g 10001 -S appgroup && \
    adduser  -u 10001 -S appuser -G appgroup

WORKDIR /app

COPY --from=builder --chown=appuser:appgroup /app/app .

USER 10001

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=20s --retries=3 \
    CMD wget -qO- http://localhost:8080/healthz || exit 1

CMD ["./app"]
