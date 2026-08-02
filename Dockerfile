# ---------------------------------------------------------------------------
# Author: Labiyb M. Said — DevSecOps Engineer
# Contact: saidlabiybm@gmail.com
# ---------------------------------------------------------------------------
#
# Multi-stage build:
#   Stage 1 (builder) — compiles a fully static Go binary
#   Stage 2 (final)   — copies only the binary into a minimal distroless image
#
# The final image has no shell, no package manager, and runs as a non-root
# user — smallest possible attack surface for a production container.

# ── Stage 1: Build ──────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

# git is needed so `go mod download` can fetch modules over HTTPS.
# ca-certificates is needed for outbound HTTPS during the build.
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Copy dependency manifests first so Docker can cache the module download
# layer independently of source code changes. A rebuild only re-downloads
# modules when go.mod or go.sum change.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source tree and compile.
# CGO_ENABLED=0 — produces a static binary with no libc dependency.
# -trimpath     — strips local filesystem paths from the binary (reproducibility + security).
# -ldflags -s -w — strips debug info and DWARF tables to shrink binary size.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath \
    -ldflags="-s -w" \
    -o /devportal ./cmd/devportal

# ── Stage 2: Final image ─────────────────────────────────────────────────────
# gcr.io/distroless/static:nonroot — no shell, no root, no unnecessary OS packages.
FROM gcr.io/distroless/static:nonroot AS final

LABEL org.opencontainers.image.title="devportal"
LABEL org.opencontainers.image.description="NexBridge Technologies — Internal Developer Platform"
LABEL org.opencontainers.image.authors="Labiyb M. Said <saidlabiybm@gmail.com>"
LABEL org.opencontainers.image.source="https://github.com/ALabiyb/platform_devportal"

# CA certificates are needed so devportal can make outbound HTTPS calls to
# GitLab, Jenkins, Harbor, DefectDojo, Keycloak, and ArgoCD.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the compiled binary from the builder stage.
COPY --from=builder /devportal /devportal

# UID 65532 is the "nonroot" user baked into distroless images.
# Running as non-root is a K8s security best practice and required by most
# Pod Security Standards (restricted profile).
USER 65532:65532

# Port matches the HTTP_ADDR default of :8080.
EXPOSE 8080

ENTRYPOINT ["/devportal"]
