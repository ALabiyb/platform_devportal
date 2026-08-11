# DevPortal — NexBridge Technologies

An Internal Developer Platform (IDP) that provisions the full DevSecOps toolchain for a new service in one form submission — Gitea repository, Jenkinsfile, Harbor image project, DefectDojo engagement, ArgoCD application, and Kubernetes manifests — all wired together automatically, with a live step-by-step progress stream in the browser.

**Maintained by:** Labiyb M. Said — DevSecOps Engineer · saidlabiybm@gmail.com

---

> **Full platform architecture, IDP vision, manifest repo structure, service dependency model, GitOps flow, and roadmap → [`ARCHITECTURE.md`](ARCHITECTURE.md)**

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [What DevPortal Manages](#what-devportal-manages)
3. [Quick Start — New Machine](#quick-start--new-machine)
4. [First Login](#first-login)
5. [Two Compose Files](#two-compose-files)
6. [Migrations — Automatic at Startup](#migrations--automatic-at-startup)
7. [Adding a New Migration](#adding-a-new-migration)
8. [Environment Variables](#environment-variables)
9. [Platform Engineering Admin](#platform-engineering-admin)
10. [Worker Process](#worker-process)
11. [Jenkins Setup](#jenkins-setup)
12. [Image Signing (Cosign)](#image-signing-cosign)
13. [Local Development (No Docker)](#local-development-no-docker)
14. [Deploying to Production](#deploying-to-production)
15. [make help — All Targets](#make-help--all-targets)

---

## Architecture Overview

```
Developer fills form in DevPortal UI
          │
          ▼
  DevPortal (Go API + React SPA)
          │
          │  INSERT provisioning_job
          ▼
  Postgres Job Queue ──── SELECT FOR UPDATE SKIP LOCKED
          │
          ▼
  Worker Pool (embedded in API process, or standalone cmd/worker pod)
          │
          │ 15-step orchestrator
    ┌─────┼──────────────────┬──────────┬────────────┐
    ▼     ▼                  ▼          ▼            ▼
  Gitea  Jenkins          Harbor    DefectDojo    ArgoCD
  repo   multibranch      image     product +     GitOps
  +bot   job + webhook    project   engagement    application
  commits
          │
          ▼
  K8s cluster (via ArgoCD)
```

### Platform Engineering Layer

Admins configure shared infrastructure once; all services inherit it automatically at provision time:

```
clusters  ──── cluster_platform_services  (CNPG / Kafka / MinIO / Redis / RabbitMQ / Vault / Gateway)
     │
     └──── environment_profiles           (CPU / mem / replicas / HPA per dev|uat|prod tier)

manifest_templates                        (15 editable K8s YAML templates)
service_infra_requirements                (per-service infra selections)
```

---

## What DevPortal Manages

| Entity | Description |
|:-------|:------------|
| **Teams** | Groups of developers. Each application belongs to one team |
| **Applications** | A logical product (e.g. "Payment Platform") containing one or more services |
| **Services** | A single deployable unit — one Git repo, one Docker image, one Jenkinsfile, ArgoCD application per environment |
| **Credentials** | Encrypted (AES-256-GCM) secrets stored in DevPortal's DB, used by the provisioner |
| **Clusters** | Kubernetes cluster registry — one row per environment (dev / uat / prod) |
| **Manifest Templates** | 15 K8s YAML templates (Namespace → HPA) editable at runtime via Admin UI |
| **Environment Profiles** | CPU / memory / replica / HPA limits per tier, inherited by every service |
| **Provisioning Jobs** | Async job queue. HTTP API returns immediately; worker provisions in background |

---

## Quick Start — New Machine

Requires: Docker Desktop (or Docker Engine + Compose plugin). Nothing else.

```bash
git clone <your-devportal-repo-url>
cd devportal

make setup                                               # 1. generate secrets + create .env
docker compose -f docker-compose.standalone.yml up -d   # 2. start postgres + devportal
open http://localhost:8080                               # 3. open the portal
```

Then complete first login (see [First Login](#first-login)).

To stop everything:

```bash
docker compose -f docker-compose.standalone.yml down
```

Data is persisted in a named Docker volume (`postgres-data`).

---

## First Login

DevPortal uses built-in local authentication by default (`AUTH_MODE=local`). No external IdP needed. There are no pre-seeded accounts — you create the first admin on first run.

**Option A — via curl:**

```bash
curl -s -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "name":     "Your Name",
    "email":    "admin@example.com",
    "password": "your-secure-password"
  }'
```

**Option B — via UI:**

Go to `http://localhost:8080` → click **Create account** (only visible before any accounts exist).

The first account registered is automatically granted the `admin` role.

---

## Two Compose Files

| File | Use when | What it includes |
|:-----|:---------|:-----------------|
| `docker-compose.standalone.yml` | Any machine — laptop, CI, demo | Own postgres + devportal, port 8080 direct |
| `docker-compose.yml` | Shared lab — existing Traefik + postgres + Gitea + Jenkins | devportal only, joins `traefik-net` |

### Shared lab (docker-compose.yml)

Used when postgres, Traefik, Gitea, Jenkins, DefectDojo, and Harbor are already running as separate compose projects on the same Docker host. DevPortal joins `traefik-net` and is routed via `devportal.localhost`.

Requires the external postgres to have the `devportal` database and user pre-created:

```bash
docker exec postgres psql -U postgres -c "
  CREATE DATABASE devportal;
  CREATE USER devportal WITH PASSWORD 'your-password';
  GRANT ALL PRIVILEGES ON DATABASE devportal TO devportal;
  ALTER DATABASE devportal OWNER TO devportal;
"
```

---

## Migrations — Automatic at Startup

Schema migrations run automatically every time the application starts. No manual `psql` commands or migration tools.

How it works:

1. The runner creates a `schema_migrations` tracking table on first start.
2. All `*.sql` files embedded in the binary (from `internal/db/migrations/`) are sorted by filename.
3. Any file not recorded in `schema_migrations` is applied inside a transaction.
4. After a successful apply, the filename is recorded — it never runs again.
5. If a migration fails, the transaction rolls back and the application exits with a clear error.

Current migration count: **8**

```sql
-- Check applied migrations:
SELECT version, applied_at FROM schema_migrations ORDER BY applied_at;
```

---

## Adding a New Migration

1. Create a new file in `internal/db/migrations/`:

   ```
   009_your_change_description.sql
   ```

2. Write standard SQL — the runner wraps it in a transaction:

   ```sql
   ALTER TABLE projects ADD COLUMN IF NOT EXISTS slack_channel TEXT;
   CREATE INDEX IF NOT EXISTS idx_projects_slug ON projects (slug);
   ```

3. Rebuild:

   ```bash
   make up   # Docker: rebuild image + restart
   ```

**Rules:**
- Always use `IF NOT EXISTS` / `IF EXISTS`.
- Never rename or delete an existing migration file.
- Never modify an already-applied migration — create a new one instead.

---

## Environment Variables

All configuration comes from `.env`. Copy `.env.example` and fill in values. Only two are required to start:

| Variable | Required | Description |
|:---------|:--------:|:------------|
| `DB_PASSWORD` | Yes | PostgreSQL password for the devportal user |
| `ENCRYPTION_KEY` | Yes | AES-256-GCM key. Generate: `openssl rand -base64 32` |

All other variables are optional at startup. The UI and all admin features work without them. Provisioning errors only appear when you trigger a provisioning run and a required variable is missing.

| Category | Variables |
|:---------|:----------|
| HTTP | `HTTP_ADDR` |
| Database | `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`, `DB_SSL_MODE` |
| Auth | `AUTH_MODE` (`local` or `oidc`), `ORG_NAME`, `ORG_SLUG` |
| OIDC / Keycloak | `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URL`, `OIDC_ADMIN_GROUP`, `OIDC_DEVELOPER_GROUP` |
| Gitea (primary SCM) | `GITEA_URL`, `GITEA_TOKEN`, `BOT_NAME`, `BOT_EMAIL` |
| GitLab (fallback SCM) | `GITLAB_URL`, `GITLAB_TOKEN` |
| Jenkins | `JENKINS_URL`, `JENKINS_PUBLIC_URL`, `JENKINS_USER`, `JENKINS_TOKEN`, `GIT_CREDENTIALS_ID` |
| Harbor | `HARBOR_URL`, `HARBOR_USER`, `HARBOR_TOKEN` |
| DefectDojo | `DEFECTDOJO_URL`, `DEFECTDOJO_TOKEN` |
| ArgoCD | `ARGOCD_URL`, `ARGOCD_TOKEN`, `ARGOCD_INSECURE` |
| App DB provisioning | `APP_DB_HOST`, `APP_DB_PORT`, `APP_DB_ADMIN_USER`, `APP_DB_ADMIN_PASSWORD` |
| Jenkinsfile generation | `REGISTRY_URL`, `REGISTRY_CREDENTIALS_ID`, `GIT_CREDENTIALS_ID`, `SHARED_LIBRARY_URL`, `DEPENDENCY_TRACK_URL`, `DEPENDENCY_TRACK_API_KEY_ID`, `K8S_MANIFEST_GROUP`, `INGRESS_BASE_DOMAIN` |
| Worker | `WORKER_CONCURRENCY` (default: 3) |
| Branding | `BRAND_APP_NAME`, `BRAND_COMPANY`, `BRAND_PRIMARY_HUE`, `BRAND_LOGO_URL` |
| TLS | `TLS_SKIP_VERIFY` |

---

## Platform Engineering Admin

The **Platform** page in the sidebar (admin only) is the control plane for the IDP layer:

### Cluster Registry

Register one Kubernetes cluster per environment. DevPortal reads these at provisioning time to know where to create ArgoCD Applications. Three environments are supported: `dev`, `uat`, `prod`.

### Platform Services

Per cluster, configure which managed services are available and their connection details: CloudNativePG, Kafka, MinIO, Redis, RabbitMQ, HashiCorp Vault, Gateway API.

### Manifest Templates

15 Kubernetes YAML templates editable at runtime. DevPortal substitutes `%%%TOKEN%%%` markers at provision time. Templates apply conditionally (e.g. "PostgreSQL only", "Production only").

Default templates (applied in order):
`Namespace → ResourceQuota → LimitRange → ServiceAccount → Role → RoleBinding → NetworkPolicy → VaultAuth → VaultStaticSecret → ConfigMap → Deployment → Service → HTTPRoute → CNPG Database → HPA`

### Environment Profiles

Resource limits and HPA settings per tier — inherited by every service:

| Tier | Typical CPU | Typical Mem | HPA |
|:-----|:------------|:------------|:----|
| dev  | 50m → 200m  | 64Mi → 256Mi | off |
| uat  | 100m → 500m | 128Mi → 512Mi | optional |
| prod | 200m → 1000m | 256Mi → 1Gi | on |

---

## Worker Process

Provisioning is fully async. When a service is created, the API inserts a `provisioning_jobs` row and returns immediately (HTTP 201). The worker claims the job and runs the 15-step orchestrator.

### Embedded (default)

The worker runs as a goroutine pool inside `cmd/devportal`. SSE progress events reach the browser without any IPC bridge because the hub is in-process. `WORKER_CONCURRENCY` controls parallelism (default: 3).

### Standalone (scale-out)

```bash
# Build
go build ./cmd/worker

# Run (needs the same DB and integration env vars as the API)
WORKER_CONCURRENCY=5 ./worker
```

Multiple standalone worker instances can run safely — `SELECT FOR UPDATE SKIP LOCKED` ensures each job is claimed by exactly one worker. Jobs stuck in `running` for more than 15 minutes are automatically reset to `pending` for retry.

---

## Jenkins Setup

Jenkins runs as a Docker container on `traefik-net` and is extended with a custom image (`Dockerfile.jenkins`) that adds Docker CLI and Cosign on top of the official `jenkins/jenkins` base.

### Custom image

The image is built automatically by the compose stack:

```bash
cd /path/to/Docker/traefik
docker compose -f docker-compose.apps.yml build jenkins
docker compose -f docker-compose.apps.yml up -d jenkins
```

What the custom image adds over the official Jenkins LTS:

| Tool | Version | Purpose |
|:-----|:--------|:--------|
| Docker CLI (`docker-ce-cli`) | 29.x | Build and push images from pipeline steps |
| Docker Compose plugin | 5.x | Run multi-container services in pipelines |
| Cosign | v3.1.3 | Sign images after push (see [Image Signing](#image-signing-cosign)) |

The Docker socket is bind-mounted from the host (`//var/run/docker.sock`) so Docker CLI inside Jenkins talks directly to the Docker Desktop daemon. No Docker-in-Docker daemon is needed.

### Jenkins credentials required

Create these credentials in **Manage Jenkins → Credentials → System → Global credentials** before running any pipeline:

| Credential ID | Kind | Value | Purpose |
|:-------------|:-----|:------|:--------|
| `robot-jenkins` | Username with password | User: `robot$devportal` · Password: Harbor robot token | Push images to Harbor |
| `gitea-token` | Username with password | Gitea bot username + token | Checkout and webhook auth |
| `cosign-private-key` | Secret file | Upload `cosign.key` | Sign images with cosign |
| `cosign-password` | Secret text | cosign key passphrase | Decrypt the signing key |
| `dependency-track-api-key` | Secret text | Dependency-Track team API key | Upload SBOMs — get from DT UI: Administration → Teams → API Keys |

### DevPortal env vars that feed into generated Jenkinsfiles

| Variable | Example value | Effect |
|:---------|:-------------|:-------|
| `REGISTRY_URL` | `harbor.docker.localhost` | Base URL for `docker push` |
| `REGISTRY_CREDENTIALS_ID` | `robot-jenkins` | Jenkins credential ID for Harbor login |
| `GIT_CREDENTIALS_ID` | `gitea-token` | Jenkins credential ID for Gitea checkout |
| `SHARED_LIBRARY_URL` | `http://gitea:3000/nexbridge/jenkins-shared-library.git` | Shared library loaded at pipeline start |
| `JENKINS_URL` | `http://jenkins:8080` | Internal URL used by DevPortal to create jobs |
| `JENKINS_PUBLIC_URL` | `https://jenkins.docker.localhost` | Public URL used in Gitea webhooks |

---

## Image Signing (Cosign)

Every Docker image pushed to Harbor is signed using [Cosign](https://docs.sigstore.dev/cosign/overview/) (Sigstore). The signature is stored as an OCI artifact alongside the image in Harbor. Kyverno enforces on each Kubernetes cluster that only signed images can be admitted.

### How it works in the pipeline

```
docker build → docker push to Harbor
                      │
                      ▼
              cosign sign --key cosign.key   (signature + annotations)
              cosign attest --key cosign.key (machine-verifiable predicate)
                      │
                      ▼
              Signature artifacts stored in Harbor
                      │
                      ▼
              Kyverno ClusterPolicy rejects any Pod whose image
              is not signed by the NexBridge cosign key
```

Signing runs as Stage 9 in the shared library (`signImage.groovy`) immediately after the image push. Annotations embedded in each signature:

| Annotation | Value |
|:-----------|:------|
| `company` | `NexBridge` |
| `project` | project name |
| `service` | service/image name |
| `branch` | source Git branch |
| `environment` | `dev` / `uat` / `prod` (derived from branch) |
| `git-commit` | full SHA |
| `builder` | `jenkins` |
| `build-number` | Jenkins build number |

### Key management

The key pair was generated with cosign inside the Jenkins container and is ECDSA P-256. The public key is committed to the repo at `pipeline/cosign.pub`. The private key is stored **only** in Jenkins as a Secret file credential — it is never committed to any repository.

To regenerate the key pair (e.g. key rotation):

```bash
# Inside the Jenkins container
docker exec -it jenkins sh
COSIGN_PASSWORD="your-new-password" cosign generate-key-pair --output-key-prefix nexbridge
```

Then:
1. Update `pipeline/cosign.pub` with the new public key
2. Update the `publicKeys` field in `pipeline/resources/kyverno/01-verify-image-signature.yaml`
3. Replace the `cosign-private-key` and `cosign-password` credentials in Jenkins
4. Re-apply the Kyverno policy on all clusters

### Verify a signed image manually

```bash
# From any machine with cosign installed
COSIGN_INSECURE_SKIP_TLS_VERIFY=true \
cosign verify --key pipeline/cosign.pub \
  harbor.docker.localhost/<project>/<image>:<tag>
```

### Kyverno enforcement

Apply once per cluster:

```bash
kubectl apply -f pipeline/resources/kyverno/01-verify-image-signature.yaml
```

The policy runs in `Enforce` mode — any Pod whose image is not signed by the NexBridge cosign key is blocked at admission. Change `validationFailureAction` to `Audit` first to observe without blocking while rolling out to an existing cluster.

---

## Local Development (No Docker)

```bash
# 1. Create .env and fill in DB_* to point at your local postgres
make setup

# 2. Create the database (one time)
createdb devportal
createuser devportal
psql -c "ALTER USER devportal WITH PASSWORD 'your-password';"
psql -c "GRANT ALL PRIVILEGES ON DATABASE devportal TO devportal;"

# 3. Start the Go API (migrations run automatically)
make dev

# 4. In a second terminal, start the React dev server
cd web && npm ci && npm run dev
```

The API listens on `:8080`. Vite proxies `/api/*` and `/auth/*` to `:8080` automatically — open `http://localhost:5173` for hot-reload UI.

---

## Deploying to Production

### Build the image

```bash
docker build -t devportal:1.0.0 .
```

Multi-stage build:
1. **Node 22-alpine** — `npm ci && npm run build` → `web/dist/`
2. **Go 1.25-alpine** — copies `web/dist/`, compiles binary with `//go:embed` (frontend + migrations baked in)
3. **Alpine 3.19** — runtime only, non-root user (UID 10001), HEALTHCHECK, ~15 MB final image

### Push to Harbor

```bash
export REGISTRY_URL=harbor.docker.localhost  # your local Harbor
make docker-push                             # builds, tags :version + :latest, pushes both
```

### Run in production

```bash
docker run -d \
  --name devportal \
  -p 8080:8080 \
  -e DB_HOST=your-postgres-host \
  -e DB_PASSWORD=your-db-password \
  -e DB_SSL_MODE=require \
  -e ENCRYPTION_KEY=your-base64-key \
  -e AUTH_MODE=local \
  devportal:1.0.0
```

Migrations run automatically on startup. No separate deploy step needed.

---

## make help — All Targets

```
DevPortal — NexBridge Technologies

  setup          create .env from .env.example and generate required secrets
  dev            run devportal locally with live .env (no Docker)
  frontend       build the React SPA into web/dist/
  build          compile the Go binary (run 'make frontend' first)
  build-full     build frontend + Go binary in one step (local, no Docker)
  up             start devportal + postgres using the standalone compose
  down           stop and remove standalone compose containers
  logs           tail devportal logs
  docker-build   build and tag the production image
  docker-push    build and push image to Harbor
  test           run all unit tests with the race detector
  lint           run golangci-lint
  tidy           sync go.mod and go.sum with actual imports
  clean          remove compiled binaries
  help           list all make targets
```

---

**Organisation:** NexBridge Technologies  
**Last Updated:** 2026-08-11 — see [`ARCHITECTURE.md`](ARCHITECTURE.md) for full IDP design and roadmap
