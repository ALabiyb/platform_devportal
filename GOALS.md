# DevPortal — Build Goals & Status

> NexBridge Technologies — Internal Developer Platform
> Author: Labiyb M. Said — DevSecOps Engineer

---

## Completed

| Phase | Deliverable | Status |
|-------|-------------|--------|
| **01** | **Project Scaffold** — `go.mod`, config package, HTTP server skeleton, Makefile, Dockerfile, docker-compose | ✅ |
| **02** | **Database Layer** — pgx connection pool, typed query helpers, embedded migration runner | ✅ |
| **03** | **Auth Package** — local username/password (primary), OIDC/Keycloak (optional), DB-backed sessions, RBAC middleware | ✅ |
| **04** | **HTTP Router** — chi router, RBAC middleware, per-IP rate limiter, structured request logger | ✅ |
| **05** | **Plugin System** — 6 provider interfaces, Gitea adapter (primary), GitLab adapter (fallback) | ✅ |
| **06** | **Jenkins Adapter** — ensure folder, create multibranch job, trigger branch scan, shared library wiring | ✅ |
| **07** | **Harbor + DefectDojo Adapters** — ensure image project, create product + engagement | ✅ |
| **08** | **ArgoCD Adapter + App DB Provisioner** — create ArgoCD Application per env, CREATE DB/USER/GRANT | ✅ |
| **09** | **Provisioning Orchestrator + SSE Hub** — 15-step async flow, per-project broadcast hub, live step updates | ✅ |
| **10** | **Jenkinsfile + Manifest Generator** — 8 stack templates, K8s YAML per environment | ✅ |
| **11** | **React + Vite Scaffold** — TypeScript, shadcn/ui, TailwindCSS, TanStack Query v5, `go:embed` wiring | ✅ |
| **12** | **Frontend Auth** — local login + register, protected routes, user context, idle timeout auto-logout | ✅ |
| **13** | **Teams + Applications Model** — Teams > Applications > Services hierarchy replaces flat Projects model | ✅ |
| **14** | **Service Provisioning UI** — create service wizard, live SSE progress stream, step-by-step status view | ✅ |
| **15** | **Admin CRUD UI** — Credentials vault, Audit log, User management, Pipeline template editor | ✅ |
| **16** | **Gitea Adapter + Bot Commits** — repo create, branch protect, Jenkinsfile/Dockerfile/manifest push via bot | ✅ |
| **17** | **Modular Monolith + Worker Split** — Postgres job queue (`provisioning_jobs`), embedded worker goroutine pool, standalone `cmd/worker` binary for scale-out | ✅ |
| **18** | **Platform Engineering IDP Layer** — 5 new DB tables: cluster registry, per-cluster platform services (CNPG/Kafka/MinIO/Redis/RabbitMQ/Vault/Gateway), manifest templates, environment profiles, service infra requirements | ✅ |
| **19** | **Platform Admin UI** — Cluster Registry tab, Manifest Templates editor (15 K8s YAMLs), Environment Profiles (CPU/mem/HPA per tier) | ✅ |

---

## In Progress / Next

| Phase | Deliverable | Priority |
|-------|-------------|----------|
| **20** | **Provisioner reads cluster registry** — replace hardcoded ArgoCD URL with cluster row, read env profile for resource limits | High |
| **21** | **Infra selection on service create** — UI form to select CNPG/Kafka/MinIO/Redis/RabbitMQ/Vault, stored in `service_infra_requirements` | High |
| **22** | **Manifest template seed** — seed 15 default K8s YAML templates at startup using `SeedManifestTemplates` (ON CONFLICT DO NOTHING) | Medium |
| **23** | **Helm Chart** — full chart scaffold, `values.yaml` (personal vs production profiles), chart linting, CI pipeline for the chart | Medium |
| **24** | **DevPortal self-hosting** — Jenkins pipeline for DevPortal itself, Harbor project, ArgoCD application, first successful self-deploy | Medium |
| **25** | **SonarQube + Dependency-Track integration** — SAST gate in Jenkinsfile, SBOM upload, CVE dashboard link in service detail | Low |
| **26** | **OIDC / Keycloak wiring for production** — group-based RBAC from OIDC claims, Keycloak realm export, production auth guide | Low |

---

## Architecture (current)

```
┌─────────────────────────────────────────────────────────────┐
│                      DevPortal                              │
│                                                             │
│  ┌──────────────┐  ┌─────────────────────────────────────┐ │
│  │  React SPA   │  │         Go HTTP API (chi)           │ │
│  │  (Vite/TS)   │◄─│  auth · projects · teams · admin   │ │
│  │  go:embed    │  └────────────────┬────────────────────┘ │
│  └──────────────┘                   │                       │
│                                     │ EnqueueProvisioningJob│
│                           ┌─────────▼─────────┐            │
│                           │  provisioning_jobs │            │
│                           │  (Postgres queue)  │            │
│                           └─────────┬──────────┘            │
│                                     │ SELECT FOR UPDATE     │
│                           ┌─────────▼──────────┐           │
│                           │   Worker Pool (3)   │           │
│                           │  cmd/devportal      │           │
│                           │  (embedded) or      │           │
│                           │  cmd/worker (pod)   │           │
│                           └─────────┬───────────┘           │
└─────────────────────────────────────│─────────────────────┘
                                      │ 15-step orchestrator
           ┌──────────┬───────────────┼──────────┬──────────┐
           ▼          ▼               ▼          ▼          ▼
         Gitea     Jenkins          Harbor    DefectDojo  ArgoCD
        (repo +   (multibranch    (image     (product +  (GitOps
        commits)   job)           project)   engagement) app)

Platform Engineering layer (Admin UI):
  clusters → cluster_platform_services → manifest_templates
  environment_profiles → service_infra_requirements
```

**Stack:**
- Backend: Go 1.23 · chi · pgx/v5
- Frontend: React 18 · TypeScript · Vite · TailwindCSS · TanStack Query v5 · shadcn/ui
- Database: PostgreSQL 16 · 8 migrations applied
- Auth: Local (primary) · OIDC/Keycloak (optional)
- GitOps: ArgoCD · build-once tag `:git-sha` · promote dev→uat→prod
- Security: AES-256-GCM credential encryption · DefectDojo · Kyverno image signing
- Infra: Docker Compose (dev) · Helm chart (production, in progress)
