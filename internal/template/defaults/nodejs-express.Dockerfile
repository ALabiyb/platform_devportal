# =============================================================================
# Production Dockerfile — Node.js API (Express / NestJS / Fastify)
# Use this for backend API services. For React/Next.js frontends use next.
#
# Best practices: pinned alpine base, numeric non-root user, prod-only deps
# (npm ci --omit=dev), direct node entrypoint for proper SIGTERM handling.
#
# ADJUST before use:
#   CMD — change src/main.js to your actual entry point
#         NestJS: dist/main.js   (run "npm run build" first, uncomment below)
#         Express: src/index.js or src/app.js
# =============================================================================

FROM node:20-alpine

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

# NODE_ENV=production → npm skips devDependencies, enables prod optimisations
ENV NODE_ENV=production

WORKDIR /app

# Numeric UID — required by OPA policy and Kubernetes runAsNonRoot validation
RUN addgroup -g 10001 -S appgroup && adduser -u 10001 -S appuser -G appgroup

# Install production deps only (no test frameworks, build tools, type defs)
COPY --chown=appuser:appgroup package.json package-lock.json ./
RUN npm ci --omit=dev --frozen-lockfile && \
    npm cache clean --force

# For TypeScript/NestJS: uncomment to run build before switching to non-root
# RUN npm ci --frozen-lockfile && npm run build && npm prune --omit=dev

COPY --chown=appuser:appgroup . .

USER 10001

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget -qO- http://localhost:3000/health || exit 1

# Use node directly (not npm start) — npm wraps node and swallows SIGTERM in K8s
CMD ["node", "src/main.js"]
