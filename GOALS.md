# DevPortal — Daily Build Goals

> One goal per day. Mark done by changing ⬜ to ✅.
> Each code file is fully commented and signed with the author header.

| Day | Deliverable | Status |
|-----|-------------|--------|
| **Day 01** | **Project Scaffold** — `go.mod`, `config` package, HTTP server skeleton, `Makefile`, `Dockerfile`, `docker-compose.yml`, migrations schema | ✅ |
| **Day 02** | **Database Layer** — `pgx` connection pool, `db` package with typed query helpers for all 10 tables | ✅ |
| **Day 03** | **Auth Package** — OIDC provider (Keycloak), GitLab OAuth fallback, DB-backed session store, RBAC middleware | ✅ |
| **Day 04** | **HTTP Router** — `chi` router, RBAC middleware, rate limiter, request logger, all route definitions wired | ✅ |
| **Day 05** | **Plugin Interfaces + GitLab Adapter** — all 6 Go interfaces defined, full GitLab adapter implemented | ✅ |
| **Day 06** | **Jenkins Adapter** — list folders, ensure folder, create multibranch job, trigger branch scan | ✅ |
| **Day 07** | **Harbor + DefectDojo Adapters** — ensure image project, create security product + engagement | ✅ |
| **Day 08** | **ArgoCD Adapter + DB Provisioner** — create ArgoCD Application per env, CREATE DB + USER + GRANT | ✅ |
| **Day 09** | **Provisioning Orchestrator + SSE Hub** — 15-step async flow, per-project broadcast hub, live step updates | ✅ |
| **Day 10** | **Jenkinsfile + K8s Manifest Generator** — per-stack templates (8 stacks), 7 YAMLs × N environments | ✅ |
| **Day 11** | **React + Vite Scaffold** — TypeScript, shadcn/ui, Tailwind CSS, TanStack Query, `go:embed` wiring | ✅ |
| **Day 12** | **Frontend Auth Pages** — login page, OIDC callback, protected routes, user context, nav layout | ⬜ |
| **Day 13** | **Project Form + SSE Progress View** — create-project wizard, live provisioning step stream UI | ⬜ |
| **Day 14** | **Project List + Detail + Environment Status** — dashboard, per-environment status cards, ArgoCD sync state | ⬜ |
| **Day 15** | **Credentials + Audit Log UI** — admin credential manager (encrypted at rest), audit event table | ⬜ |
| **Day 16** | **Helm Chart** — full chart scaffold, `values.yaml` (personal vs production profiles), chart linting | ⬜ |
| **Day 17** | **K8s Manifests for DevPortal Itself** — namespace, deployment, service, ingress, HPA, configmap, secret | ⬜ |
| **Day 18** | **CI/CD for DevPortal** — Jenkinsfile, GitLab webhook, Harbor project, first successful pipeline build | ⬜ |

---

## Architecture (locked 2026-08-02)

- **Backend:** Go 1.23 · chi router · pgx/v5 · go-oidc/v3
- **Frontend:** React 18 · TypeScript · Vite · shadcn/ui · TanStack Query · embedded via `go:embed`
- **Database:** PostgreSQL 16 (devportal's own) + pgbouncer — separate from app DBs
- **Auth:** OIDC via Keycloak (primary) · GitLab OAuth (fallback) · DB-backed sessions
- **GitOps:** ArgoCD watches manifest repo · build once (`:git-sha`) promote dev→uat→prod
- **Security:** AES-256-GCM credential encryption · DefectDojo · SonarQube · Dependency-Track
- **Deploy:** Helm chart · K8s Deployment · Traefik ingress

## Repo Structure

```
devportal/
├── cmd/devportal/main.go       ← entry point
├── internal/
│   ├── config/                 ← env var config (Day 01)
│   ├── db/                     ← pgx pool + query helpers (Day 02)
│   ├── auth/                   ← OIDC + GitLab OAuth + sessions (Day 03)
│   ├── handler/                ← HTTP handlers (Day 04+)
│   ├── plugin/                 ← provider interfaces (Day 05)
│   ├── provisioner/            ← orchestrator + SSE hub (Day 09)
│   └── template/               ← Jenkinsfile + K8s manifest generators (Day 10)
├── migrations/                 ← SQL schema files
├── web/                        ← React SPA (Day 11+)
├── helm/devportal/             ← Helm chart (Day 16)
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── .env.example
```
