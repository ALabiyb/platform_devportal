# DevPortal — Internal Developer Platform

> Fill one form. Get a fully wired, production-ready project — automatically.

DevPortal is an open-source Internal Developer Platform (IDP) built for engineering teams running a self-hosted DevSecOps stack. A developer submits a single form and DevPortal provisions everything end-to-end in real time:

- GitLab repo wiring (Jenkinsfile, VERSION, Dockerfile committed automatically)
- Jenkins multibranch pipeline job + webhook
- Harbor image registry project
- DefectDojo security product + engagement
- Kubernetes manifests generated and committed (dev / uat / prod)
- ArgoCD Application created per environment
- PostgreSQL database provisioned per environment
- Live provisioning progress streamed to the browser via SSE

No manual steps. No tickets to DevOps. No waiting.

---

## Demo

> Coming in Day 13 — live walkthrough video

---

## Architecture

> **[View full interactive diagram on Eraser →](https://app.eraser.io/workspace/MmusQUEPRU3zvC8sMpsk?diagram=ZX1_s6-X_tAb-sN2gvsR&layout=canvas)**
>
> The diagram covers all five flows: Auth (local bcrypt + OIDC), Provisioning Orchestrator (15 steps), CI/CD Pipeline, GitOps Promotion, and App Runtime.

```
Developer (Browser)
    │
    ▼
DevPortal (Go · chi · pgx · go-oidc)
    │
    ├── Auth ──────────► LOCAL  : POST /auth/login → bcrypt verify → DB session
    │                    OIDC   : Keycloak SSO → id_token → groups → RBAC role
    │                    (AUTH_MODE env var selects the active mode)
    │
    ├── Provisioning ──► GitLab · Jenkins · Harbor · DefectDojo · ArgoCD
    │    (15 steps)      DB Provisioner (CREATE DATABASE per env)
    │
    ├── SSE Hub ───────► Live step-by-step progress streamed to browser
    │
    └── Database ──────► PostgreSQL 16 (own DB)
                         Stores: projects · sessions · audit log · encrypted creds

K8s Namespaces (dev / uat / prod)
    └── App ──────────► pgbouncer ──► App PostgreSQL
                         Traefik (Ingress + TLS)
```

**Build once, promote everywhere** — Jenkins builds the image once with the `:git-sha` tag. ArgoCD promotes the same image from dev → uat → prod by updating the manifest repo tag. No rebuilds per environment.

---

## Supported Providers

| Category | Provider | Status |
|---|---|---|
| Git | GitLab | Day 05 |
| Git | Gitea | Day 05 |
| CI/CD | Jenkins | Day 06 |
| Registry | Harbor | Day 07 |
| Security | DefectDojo | Day 07 |
| GitOps | ArgoCD | Day 08 |
| Auth | Local (bcrypt, built-in) | Day 03 |
| Auth | Keycloak (OIDC / SSO) | Day 03 |
| Database | PostgreSQL | Day 08 |

---

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.23 · chi router · pgx/v5 · go-oidc/v3 |
| Frontend | React 18 · TypeScript · Vite · shadcn/ui · TanStack Query |
| Database | PostgreSQL 16 · pgbouncer |
| Auth | Keycloak OIDC (primary) · GitLab OAuth (fallback) |
| Deploy | Helm chart · Kubernetes · Traefik |
| Security | AES-256-GCM credential encryption · DefectDojo · SonarQube |

---

## Getting Started

### Prerequisites

- Go 1.23+
- Docker 24+
- PostgreSQL 16 (or use the included docker-compose)

### Local development

```bash
# 1. Clone
git clone https://github.com/ALabiyb/platform_devportal.git
cd platform_devportal

# 2. Copy and fill in config
cp .env.example .env

# 3. Create the devportal database
docker exec postgres psql -U postgres -c "
  CREATE DATABASE devportal;
  CREATE USER devportal WITH PASSWORD 'changeme';
  GRANT ALL ON DATABASE devportal TO devportal;
"

# 4. Apply migrations
make migrate

# 5. Run
make run

# 6. Verify
curl http://localhost:8080/healthz
```

Or with Docker Compose (joins existing `traefik-net`):

```bash
docker compose up -d
# Access at http://devportal.localhost
```

### Available make targets

```bash
make build        # compile binary to ./bin/devportal
make run          # run locally (reads .env)
make test         # unit tests with race detector
make lint         # golangci-lint
make migrate      # apply SQL migrations
make docker-build # build production Docker image
make docker-push  # build and push to Harbor
make help         # list all targets
```

---

## Configuration

All configuration is via environment variables. Copy `.env.example` to `.env` and fill in the values marked `REQUIRED`.

| Variable | Required | Default | Description |
|---|---|---|---|
| `DB_PASSWORD` | ✅ | — | DevPortal PostgreSQL password |
| `GITLAB_TOKEN` | ✅ | — | GitLab PAT (scope: api) |
| `JENKINS_TOKEN` | ✅ | — | Jenkins API token |
| `HARBOR_TOKEN` | ✅ | — | Harbor admin password or robot secret |
| `DEFECTDOJO_TOKEN` | ✅ | — | DefectDojo API token |
| `ARGOCD_TOKEN` | ✅ | — | ArgoCD API token |
| `ENCRYPTION_KEY` | ✅ | — | AES-256-GCM key (`openssl rand -base64 32`) |
| `APP_DB_ADMIN_PASSWORD` | ✅ | — | Postgres superuser password for app DB provisioning |
| `AUTH_MODE` | | `oidc` | `oidc` (Keycloak) or `gitlab` (OAuth fallback) |
| `OIDC_CLIENT_SECRET` | ✅ when OIDC | — | Keycloak client secret |

See `.env.example` for the full list with descriptions.

---

## Deployment

DevPortal ships as a Helm chart (Day 16):

```bash
helm upgrade --install devportal ./helm/devportal \
  --namespace devportal \
  --create-namespace \
  -f helm/devportal/values.production.yaml
```

---

## Build Progress

| Day | Deliverable | Status |
|---|---|---|
| Day 01 | Project scaffold — Go module, config, HTTP server, Dockerfile, migrations | ✅ |
| Day 02 | Database layer — pgx pool, typed query helpers for all 10 tables | ✅ |
| Day 03 | Auth — OIDC (Keycloak), GitLab OAuth fallback, DB-backed sessions | ✅ |
| Day 04 | HTTP router — chi, RBAC middleware, rate limiter, all routes | ✅ |
| Day 05 | Plugin interfaces + GitLab adapter | ✅ |
| Day 06 | Jenkins adapter | ⬜ |
| Day 07 | Harbor + DefectDojo adapters | ⬜ |
| Day 08 | ArgoCD adapter + DB provisioner | ⬜ |
| Day 09 | Provisioning orchestrator + SSE hub | ⬜ |
| Day 10 | Jenkinsfile + K8s manifest generator | ⬜ |
| Day 11 | React + Vite frontend scaffold | ⬜ |
| Day 12 | Frontend auth pages | ⬜ |
| Day 13 | Project form + SSE progress view | ⬜ |
| Day 14 | Project list + detail + environment status | ⬜ |
| Day 15 | Credentials + audit log UI | ⬜ |
| Day 16 | Helm chart | ⬜ |
| Day 17 | K8s manifests for DevPortal itself | ⬜ |
| Day 18 | CI/CD for DevPortal | ⬜ |

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions, commit conventions, and the PR process.

---

## License

[MIT](LICENSE) — © 2026 Labiyb M. Said
