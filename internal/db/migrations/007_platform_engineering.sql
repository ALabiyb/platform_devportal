-- ---------------------------------------------------------------------------
-- Author: Labiyb M. Said — DevSecOps Engineer
-- Contact: saidlabiybm@gmail.com
-- ---------------------------------------------------------------------------
-- Migration 007 — Platform Engineering: Cluster Registry, Manifest Templates,
--                 Environment Profiles, Infra Requirements
--
-- Adds the five tables that underpin the full IDP target architecture:
--
--   clusters                    — registered K8s clusters (dev/uat/prod)
--   cluster_platform_services   — per-cluster shared services (CNPG, Kafka, etc.)
--   manifest_templates          — 15 K8s YAML templates, admin-editable
--   environment_profiles        — resource limits + replica counts per tier
--   service_infra_requirements  — what each service selected at registration
--
-- Also extends projects (port, health_path) and environments (cluster_id).
-- ---------------------------------------------------------------------------

-- ── clusters ──────────────────────────────────────────────────────────────────
-- One row per Kubernetes cluster registered with DevPortal.
-- Each environment tier (dev/uat/prod) maps to exactly one cluster per org.
-- Credentials are referenced from workspace_credentials (AES-256-GCM encrypted).
CREATE TABLE IF NOT EXISTS clusters (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name         TEXT        NOT NULL,           -- e.g. "dev-cluster"
    display_name TEXT        NOT NULL,           -- e.g. "Development Cluster"
    -- environment is the tier this cluster serves: "dev" | "uat" | "prod"
    environment  TEXT        NOT NULL,
    -- api_endpoint is the K8s API server address, e.g. "https://192.168.200.10:6443"
    api_endpoint TEXT        NOT NULL,
    -- argocd_url is the ArgoCD API server, e.g. "https://argocd.dev.softnethq.co.tz"
    argocd_url   TEXT        NOT NULL,
    -- kubeconfig_credential_id references the encrypted kubeconfig in workspace_credentials.
    -- DevPortal uses this to call the ArgoCD API on the correct cluster.
    kubeconfig_credential_id UUID REFERENCES workspace_credentials(id) ON DELETE SET NULL,
    -- argocd_credential_id references the encrypted ArgoCD API token.
    argocd_credential_id     UUID REFERENCES workspace_credentials(id) ON DELETE SET NULL,
    -- status: "active" | "inactive" — inactive clusters are skipped by the orchestrator.
    status       TEXT        NOT NULL DEFAULT 'active',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by   UUID        REFERENCES users(id) ON DELETE SET NULL,
    -- one cluster per environment per org (dev/uat/prod are exclusive per org)
    UNIQUE (org_id, environment),
    UNIQUE (org_id, name)
);

-- ── cluster_platform_services ──────────────────────────────────────────────────
-- Per-cluster configuration for each shared platform service.
-- Platform team fills this once via the admin UI.
-- DevPortal reads these rows when generating manifests and provisioning resources.
--
-- service_type values and their config JSONB shapes:
--
--   "cnpg"     — { "cluster_name": "postgres-cluster", "namespace": "postgres",
--                  "superuser_secret": "postgres-cluster-superuser" }
--
--   "kafka"    — { "brokers": "kafka.kafka.svc.cluster.local:9092",
--                  "admin_secret_ref": "kafka-admin-creds",
--                  "admin_secret_namespace": "kafka" }
--
--   "minio"    — { "endpoint": "minio.minio.svc.cluster.local:9000",
--                  "admin_secret_ref": "minio-root-creds",
--                  "admin_secret_namespace": "minio" }
--
--   "redis"    — { "host": "redis-master.redis.svc.cluster.local",
--                  "port": "6379",
--                  "secret_ref": "redis-secret",
--                  "secret_namespace": "redis" }
--
--   "rabbitmq" — { "host": "rabbitmq.rabbitmq.svc.cluster.local",
--                  "port": "5672",
--                  "secret_ref": "rabbitmq-creds",
--                  "secret_namespace": "rabbitmq" }
--
--   "vault"    — { "addr": "http://vault.vault.svc.cluster.local:8200",
--                  "mount": "secret",
--                  "auth_mount": "kubernetes",
--                  "namespace": "vault" }
--
--   "gateway"  — { "name": "main-gateway", "namespace": "istio-system",
--                  "section_name": "https",
--                  "tls_secret": "dev-wildcard-tls",
--                  "domain": "dev.softnethq.co.tz" }
CREATE TABLE IF NOT EXISTS cluster_platform_services (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id   UUID        NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    service_type TEXT        NOT NULL,
    -- enabled: false means this service is not available to developers on this cluster.
    enabled      BOOLEAN     NOT NULL DEFAULT true,
    config       JSONB       NOT NULL DEFAULT '{}',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by   UUID        REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE (cluster_id, service_type)
);

-- ── manifest_templates ─────────────────────────────────────────────────────────
-- Standard K8s YAML templates for every resource DevPortal generates per service.
-- Same design as pipeline_templates: seeded with defaults at startup,
-- admin-editable at runtime, rendered with %%%MARKER%%% substitution.
--
-- Numbered naming convention mirrors the k8s-dashboard pattern:
--   01-namespace, 02-resourcequota, 03-limitrange, 04-serviceaccount,
--   05-role, 06-rolebinding, 07-networkpolicy, 08-vaultauth,
--   09-vaultstaticsecret, 10-configmap, 11-deployment, 12-service,
--   13-httproute, 14-cnpg-database, 15-hpa
CREATE TABLE IF NOT EXISTS manifest_templates (
    name         TEXT        PRIMARY KEY,    -- e.g. "07-networkpolicy"
    display_name TEXT        NOT NULL,       -- e.g. "Network Policy"
    -- conditional controls when this template is rendered:
    --   ""          → always render (namespace, deployment, service, httproute, etc.)
    --   "cnpg"      → only if service selected PostgreSQL
    --   "kafka"     → only if service selected Kafka
    --   "minio"     → only if service selected MinIO
    --   "redis"     → only if service selected Redis
    --   "rabbitmq"  → only if service selected RabbitMQ
    --   "prod"      → only for prod environment (hpa)
    conditional  TEXT        NOT NULL DEFAULT '',
    content      TEXT        NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by   UUID        REFERENCES users(id) ON DELETE SET NULL
);

-- ── environment_profiles ───────────────────────────────────────────────────────
-- Resource requests, limits, and replica counts per environment tier.
-- DevPortal substitutes these into the Deployment template via %%%MARKERS%%%.
-- Platform team updates these once; all services automatically inherit the change.
CREATE TABLE IF NOT EXISTS environment_profiles (
    name          TEXT        PRIMARY KEY,   -- "dev" | "uat" | "prod"
    -- CPU/memory requests and limits follow K8s resource quantity format.
    cpu_request   TEXT        NOT NULL,      -- e.g. "50m"
    mem_request   TEXT        NOT NULL,      -- e.g. "64Mi"
    cpu_limit     TEXT        NOT NULL,      -- e.g. "200m"
    mem_limit     TEXT        NOT NULL,      -- e.g. "256Mi"
    replicas      INT         NOT NULL,      -- initial replica count
    -- storage_class is the K8s StorageClass name for PVCs on this cluster tier.
    storage_class TEXT        NOT NULL DEFAULT 'rook-ceph-block',
    -- HPA fields — used when hpa_enabled is true (rendered into 15-hpa.yaml).
    hpa_enabled   BOOLEAN     NOT NULL DEFAULT false,
    hpa_min       INT         NOT NULL DEFAULT 2,
    hpa_max       INT         NOT NULL DEFAULT 10,
    -- cpu_threshold / mem_threshold are the HPA averageUtilization targets (%).
    cpu_threshold INT         NOT NULL DEFAULT 70,
    mem_threshold INT         NOT NULL DEFAULT 80,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    UUID        REFERENCES users(id) ON DELETE SET NULL
);

-- Seed default profiles. Admins can update these via the admin UI.
-- ON CONFLICT DO NOTHING preserves any admin edits on re-run.
INSERT INTO environment_profiles
    (name, cpu_request, mem_request, cpu_limit, mem_limit, replicas, storage_class, hpa_enabled, hpa_min, hpa_max, cpu_threshold, mem_threshold)
VALUES
    ('dev',  '50m',  '64Mi',  '200m',  '256Mi', 1, 'rook-ceph-block', false, 2, 5,  70, 80),
    ('uat',  '100m', '128Mi', '500m',  '512Mi', 1, 'rook-ceph-block', false, 2, 5,  70, 80),
    ('prod', '200m', '256Mi', '1000m', '1Gi',   2, 'rook-ceph-block', true,  2, 10, 70, 80)
ON CONFLICT (name) DO NOTHING;

-- ── service_infra_requirements ─────────────────────────────────────────────────
-- Records what shared infrastructure each service selected at registration time.
-- One row per service per infrastructure type (e.g. order-service → cnpg, kafka).
--
-- DevPortal reads these rows at provision time to:
--   1. Provision the physical resource (CNPG Database CR, Kafka topics, MinIO bucket)
--   2. Conditionally render the right manifest templates
--   3. Populate ConfigMap with non-sensitive connection details
--   4. Write per-service credentials to Vault
CREATE TABLE IF NOT EXISTS service_infra_requirements (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    -- service_type: "cnpg" | "kafka" | "minio" | "redis" | "rabbitmq"
    service_type TEXT        NOT NULL,
    -- config holds service-specific options set at registration time.
    --   cnpg:     {}  (DB name auto-derived: {slug}_{env})
    --   kafka:    { "topics": ["order-service.events", "order-service.commands"] }
    --   minio:    { "bucket_prefix": "order-service" }
    --   redis:    {}
    --   rabbitmq: { "vhost": "order-service" }
    config       JSONB       NOT NULL DEFAULT '{}',
    -- provisioned is set true after DevPortal successfully creates the resource.
    provisioned  BOOLEAN     NOT NULL DEFAULT false,
    -- vault_path records where credentials for this infra type were written in Vault.
    -- e.g. "secret/data/dev/order-service"
    vault_path   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- one infra type per service (e.g. order-service can only select cnpg once)
    UNIQUE (project_id, service_type)
);

-- ── projects: extend for IDP ───────────────────────────────────────────────────
-- port:        the container's HTTP port — used in Service, HTTPRoute, and health probes.
-- health_path: the liveness + readiness probe HTTP path.
ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS port        INT  NOT NULL DEFAULT 8080,
    ADD COLUMN IF NOT EXISTS health_path TEXT NOT NULL DEFAULT '/healthz';

-- ── environments: link to cluster ─────────────────────────────────────────────
-- Each environment row now records which registered cluster it was deployed to.
-- Nullable so existing environments created before migration 007 still work.
ALTER TABLE environments
    ADD COLUMN IF NOT EXISTS cluster_id UUID REFERENCES clusters(id) ON DELETE SET NULL;

-- ── indexes ───────────────────────────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_clusters_org_env
    ON clusters (org_id, environment);

CREATE INDEX IF NOT EXISTS idx_cluster_platform_services_cluster
    ON cluster_platform_services (cluster_id, service_type);

CREATE INDEX IF NOT EXISTS idx_service_infra_project
    ON service_infra_requirements (project_id);

CREATE INDEX IF NOT EXISTS idx_environments_cluster
    ON environments (cluster_id);
