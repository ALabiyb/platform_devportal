# =============================================================================
# Production Dockerfile — .NET ASP.NET Core (multi-stage)
# Build stage uses the full SDK; runtime stage has only the ASP.NET runtime.
# Final image: ~200 MB (aspnet:alpine — no SDK, no compiler, no source).
#
# Best practices: multi-stage, layer-cached restore, numeric non-root user,
# non-privileged port 8080, Alpine wget healthcheck.
#
# ADJUST before use:
#   ENTRYPOINT — replace YourApp.dll with your actual project DLL name
#                (matches your .csproj AssemblyName, usually the project folder name)
# =============================================================================

# ── Stage 1: Build ─────────────────────────────────────────────────────────
FROM mcr.microsoft.com/dotnet/sdk:8.0-alpine AS builder

WORKDIR /app

# Restore dependencies first — only re-runs when .csproj changes
COPY *.csproj ./
RUN dotnet restore

COPY . .
RUN dotnet publish \
    --configuration Release \
    --output /app/publish \
    --no-restore \
    /p:UseAppHost=false

# ── Stage 2: Runtime ────────────────────────────────────────────────────────
FROM mcr.microsoft.com/dotnet/aspnet:8.0-alpine

ARG APP_NAME=app
ARG GIT_AUTHOR=unknown
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown
ARG VERSION=unknown
ARG APP_TIMEZONE=Africa/Dar_es_Salaam

LABEL org.opencontainers.image.title="${APP_NAME}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.authors="${GIT_AUTHOR}" \
      org.opencontainers.image.vendor="Softnet"

ENV TZ=${APP_TIMEZONE}
RUN apk add --no-cache tzdata && \
    cp /usr/share/zoneinfo/${APP_TIMEZONE} /etc/localtime && \
    echo "${APP_TIMEZONE}" > /etc/timezone && \
    apk del tzdata

# Port 8080 — non-root processes cannot bind to ports below 1024
ENV ASPNETCORE_URLS=http://+:8080
ENV ASPNETCORE_ENVIRONMENT=Production
ENV DOTNET_RUNNING_IN_CONTAINER=true
ENV DOTNET_SYSTEM_GLOBALIZATION_INVARIANT=false

WORKDIR /app

# Numeric UID — required by OPA policy and Kubernetes runAsNonRoot validation
RUN addgroup -g 10001 -S appgroup && adduser -u 10001 -S appuser -G appgroup

COPY --from=builder --chown=appuser:appgroup /app/publish .

USER 10001

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

# Replace YourApp.dll with your project DLL name (matches .csproj AssemblyName)
ENTRYPOINT ["dotnet", "YourApp.dll"]
