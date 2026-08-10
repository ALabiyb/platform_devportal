# =============================================================================
# Production Dockerfile — Spring Boot / Maven
# JAR is built by Jenkins (mvn package -DskipTests -B) before this runs.
# This stage is runtime-only — no JDK, no Maven, no source code in the image.
#
# Best practices: pinned JRE-only base, numeric non-root user, OCI labels,
# container-aware JVM flags, Alpine wget healthcheck.
# =============================================================================

FROM eclipse-temurin:21-jre-alpine

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

WORKDIR /app

# Numeric UID — required by OPA policy and Kubernetes runAsNonRoot validation
RUN addgroup -g 10001 -S appgroup && adduser -u 10001 -S appuser -G appgroup

# JAR produced by: mvn clean package -DskipTests=true -B
COPY --chown=appuser:appgroup target/*.jar app.jar

RUN mkdir -p /app/logs && chown -R appuser:appgroup /app

USER 10001

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD wget -qO- http://localhost:8080/actuator/health || exit 1

# -XX:+UseContainerSupport  → respect container CPU/memory limits
# -XX:MaxRAMPercentage=75.0 → use 75% of container memory for heap
ENTRYPOINT ["java", \
    "-XX:+UseContainerSupport", \
    "-XX:MaxRAMPercentage=75.0", \
    "-Djava.security.egd=file:/dev/./urandom", \
    "-Dspring.profiles.active=prod", \
    "-jar", "app.jar"]
