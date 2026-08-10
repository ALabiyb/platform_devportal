# =============================================================================
# Production Dockerfile — Python (FastAPI / Flask / Django)
# Runs as a numeric non-root user — satisfies OPA + Kubernetes runAsNonRoot.
#
# Best practices: pinned alpine base, numeric non-root user, layer-cached
# pip install (only re-runs when requirements.txt changes), no .pyc files.
#
# ADJUST before use:
#   CMD — change to match your framework and entry point:
#     FastAPI  : uvicorn app.main:app --host 0.0.0.0 --port 8000 --workers 2
#     Flask    : gunicorn "app:create_app()" --workers 2 --bind 0.0.0.0:8000
#     Django   : gunicorn project.wsgi:application --workers 2 --bind 0.0.0.0:8000
# =============================================================================

FROM python:3.12-alpine

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

# Cleaner containers and real-time log streaming
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

WORKDIR /app

# Numeric UID — required by OPA policy and Kubernetes runAsNonRoot validation
RUN addgroup -g 10001 -S appgroup && adduser -u 10001 -S appuser -G appgroup

# Install deps as root — pip needs write access to site-packages
# Copy requirements first for better layer caching
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY --chown=appuser:appgroup . .

USER 10001

EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget -qO- http://localhost:8000/health || exit 1

# Adjust entrypoint to your framework (see header)
CMD ["uvicorn", "app.main:app", \
     "--host", "0.0.0.0", \
     "--port", "8000", \
     "--workers", "2", \
     "--no-access-log"]
