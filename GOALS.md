# DevPortal — Build Goals & Status

> NexBridge Technologies — Internal Developer Platform
> Author: Labiyb M. Said — DevSecOps Engineer
> Last updated: 2026-08-23 (P5-partial: cross-platform member sync + DT integration)

---

## What we are building

**DevPortal** is NexBridge's Internal Developer Platform (IDP). A developer fills one 7-step form — service name, language profile, ports, infrastructure (PostgreSQL / Kafka / Redis / RabbitMQ / MinIO), dependencies — and the platform does everything else:

- Creates a Git repository with the correct Jenkinsfile, Dockerfile, and VERSION
- Registers a Jenkins multibranch pipeline
- Creates a Harbor image project and robot account
- Registers a DefectDojo security product and CI/CD engagement
- Commits Kustomize manifests (base + dev/uat/prod overlays) to a dedicated manifest repo
- Creates ArgoCD Applications that sync to Kubernetes for all three environments
- Provisions operator Custom Resources (CNPG Database, Strimzi KafkaTopic, RabbitMQ Vhost+User) as committed YAML
- Creates a MinIO bucket and Redis ACL user per service via GitOps-managed init Jobs
- Generates per-service NetworkPolicies so only declared callers can reach each pod

Everything is automated. Nothing requires a DevOps engineer. 90 seconds from form submit to running pod.

---

## Architecture at a glance

```
Browser (React SPA)
       │ HTTPS
       ▼
DevPortal Go API  ──→  provisioning_jobs (Postgres queue)
       │                        │
       │                        ▼
   SSE Hub  ◄──────────  Worker Pool (15-step orchestrator)
   (live progress)              │
                    ┌───────────┼────────────┬──────────────┐
                    ▼           ▼            ▼              ▼
                  Gitea      Jenkins      Harbor         ArgoCD
               (repo +     (pipeline)  (registry)    (GitOps sync)
               manifests)                                   │
                                                    Kubernetes
                                                  dev / uat / prod
                                                  (CNPG, Strimzi,
                                                   RabbitMQ, Vault)
```

Stack: **Go 1.23 + chi + pgx/v5** backend · **React 18 + TypeScript + Vite + TanStack Query v5** frontend · **PostgreSQL 16** · **Docker Compose** (dev) · **Helm** (production, planned).

---

## Completed phases

| # | Deliverable | Notes |
|---|-------------|-------|
| 01 | Project scaffold | `go.mod`, config, HTTP server, Makefile, Dockerfile, docker-compose |
| 02 | Database layer | pgx pool, typed query helpers, embedded migration runner (13 migrations) |
| 03 | Auth | Local bcrypt (primary) · OIDC/Keycloak (optional) · DB-backed sessions · RBAC middleware |
| 04 | HTTP router | chi router · RBAC · per-IP rate limiter · structured request logger |
| 05 | Plugin system | 6 provider interfaces · Gitea adapter (primary) · GitLab adapter (fallback) |
| 06 | Jenkins adapter | Ensure folder · create multibranch job · trigger scan · shared library wiring |
| 07 | Harbor + DefectDojo adapters | Ensure image project · create product + engagement |
| 08 | ArgoCD adapter | Create ArgoCD Application per env · `CREATE DATABASE / USER / GRANT` |
| 09 | 15-step orchestrator + SSE hub | Async flow · per-project broadcast hub · live step updates |
| 10 | Jenkinsfile + manifest generator | 8 stack templates · K8s YAML per environment |
| 11 | React + Vite scaffold | TypeScript · TanStack Query v5 · `go:embed` |
| 12 | Frontend auth | Local login/register · protected routes · user context · idle auto-logout |
| 13 | Teams → Applications → Services model | Replaced flat Projects model |
| 14 | Service provisioning UI | 7-step wizard · live SSE progress · step-by-step status view |
| 15 | Admin CRUD UI | Credentials vault · Audit log · Users · Pipeline template editor |
| 16 | Gitea adapter + bot commits | Repo create · branch protect · Jenkinsfile/Dockerfile/manifests via bot |
| 17 | Modular monolith + worker split | Postgres job queue · goroutine worker pool · standalone `cmd/worker` |
| 18 | Platform Engineering IDP layer | 5 new DB tables: clusters · cluster_platform_services · manifest_templates · environment_profiles · service_infra_requirements |
| 19 | Platform admin UI (all tabs) | Cluster registry · Language profiles · Manifest templates · Environment profiles — all with modals, fully wired |
| 20 | All UI dead buttons wired | Users · Teams · Credentials · Service wizard with real API data |
| 21 | **P1 — Wizard wired to real API** | `POST /api/v1/applications/{id}/services` called on "Provision service" · real SSE stream replaces fake timer · infra selections saved to `service_infra_requirements` · deps list from real services in same application |
| 22 | **P1 — Audit events** | `service.created` written in `CreateService` handler · `service.provisioning.complete/failed` written in worker after Provision · Dashboard Recent Activity shows real events with descriptive labels |
| 23 | **P1 — Dashboard Applications card** | 5th KPI card · clickable · consistent height with siblings · active/archived breakdown |
| 24 | **P1 — ApplicationsPage step drawer** | Failed/deploying services expand inline to show all 15 provisioning steps with status icons |
| 25 | **P2 — Cluster registry → orchestrator** | `clusterPlatform()` resolves `PlatformRefs` (CNPG/Kafka/RabbitMQ/Redis/MinIO cluster names + namespaces) and k8s `apiServer` per environment from `cluster_platform_services` JSONB rows · ArgoCD `CreateApplication` now passes cluster-specific `Server` destination · graceful fallback to flat config when no cluster registered |
| 26 | **P2 — Environment profiles → manifests** | `GetEnvironmentProfile` called per env in step 12 · `cpu_request`, `mem_request`, `cpu_limit`, `mem_limit`, `replicas` injected into Kustomize overlay patches |
| 27 | **P2 — Manifest template seed** | `SeedManifestTemplates` runs at startup · all 19 templates exist in DB on first boot · admins can edit any template; edits survive restarts |
| 28 | **P3 — CNPG Database CR** | `kind: Database` CR committed to `<service>/infra/<env>/cnpg-database.yaml` · separate `-platform` ArgoCD Application syncs infra dir · `EnsureDatabase`/`EnsureUser` SQL skipped when CNPG selected (operator handles it) |
| 29 | **P3 — Strimzi KafkaTopic CR** | `kind: KafkaTopic` CR committed per topic derived from infra req config · topics: field split on comma · partitions 3 / replicas 1 / 7-day retention |
| 30 | **P3 — RabbitMQ Vhost + User CRs** | `kind: Vhost` + `kind: User` CRs committed · vhost path from infra req config or defaults to `/<appname>` · user secret ref `<appname>-<env>-rabbit-creds` |
| 31 | **P3 — NetworkPolicy egress rules** | Egress rules generated from `service_infra_requirements` + `service_dependencies` · per-infra namespace rules for CNPG (5432), Kafka (9092), RabbitMQ (5672), Redis (6379), MinIO (9000) · per-dep namespace+pod rules for service-to-service calls |
| 32 | **P3 — MinIO bucket init Job** | `kind: Job` ArgoCD sync hook committed to `<service>/infra/<env>/minio-job.yaml` · runs `mc mb --ignore-existing` · `BeforeHookCreation` delete policy makes re-syncs idempotent · admin credentials from `MinIOAdminSecret` (default `minio-root-credentials`) |
| 33 | **P3 — Redis ACL init Job** | `kind: Job` ArgoCD sync hook committed to `<service>/infra/<env>/redis-acl-job.yaml` · `ACL SETUSER <appname>-<env> reset on nopass ~<appname>-<env>:* &* +@all` + `ACL SAVE` · key-namespace isolation per service · `REDIS_USERNAME` injected into infra ConfigMap |
| 34 | **P4 — Vault config + VaultProvider** | `VAULT_URL` / `VAULT_TOKEN` / `VAULT_KV_MOUNT` / `VAULT_K8S_AUTH_MOUNT` / `VAULT_USE_VSO` config fields · `VaultProvider` interface + HTTP adapter (no external SDK) · idempotent: reads existing KV secret before writing, never clobbers real passwords |
| 35 | **P4 — Step 16: Configure Vault secrets path** | Per env: `EnsureKVSecret` writes placeholder keys (`DB_PASSWORD`, `KAFKA_SASL_PASSWORD`, etc.) to `devportal/<appname>/<env>` · `EnsurePolicy` writes ACL policy `devportal-<appname>-<env>` · `EnsureKubernetesRole` binds service ServiceAccount to the policy via K8s auth method · skipped gracefully when `VAULT_URL` not set or no infra requirements |
| 36 | **P4 — VSO CRs in overlay** | `VAULT_USE_VSO=true` renders `kind: VaultAuth` + `kind: VaultStaticSecret` CRs instead of `ExternalSecret` (ESO) · `VaultAuth` binds the service SA to the Vault K8s auth role · `VaultStaticSecret` syncs the flat KV path to `<appname>-secret` K8s Secret · pods pick it up via existing `envFrom: secretRef` · admins can edit both templates in the admin UI |
| 37 | **P5 — Cross-platform member sync** | `SecurityProvider.SyncProductMembers` + `RegistryProvider.SyncProjectMembers` added to plugin interfaces · `DependencyTrackProvider` interface + `DependencyTrackAdapter` (no SDK — pure `net/http`) · `internal/membersync.Service` fans out member changes to DefectDojo, Harbor, and DT in one best-effort pass · `AddApplicationMember`/`RemoveApplicationMember` handlers call syncer as a background goroutine after DB + Gitea update · `DEPENDENCY_TRACK_API_KEY` config field for DevPortal → DT direct API calls |
| 38 | **P5 — Dependency-Track step 17** | Step 17 added to provisioner: creates DT project (idempotent GET then PUT) + `EnsureEmailNotification` writes/updates a named notification rule with all application member emails · scopes rule to the DT project via `POST /api/v1/notification/rule/{uuid}/project/{projectUUID}` · skipped gracefully when `DEPENDENCY_TRACK_API_KEY` not set |

---

## Where we are now (2026-08-23)

**P1, P2, P3, P4, and the core of P5 (cross-platform RBAC + CVE notifications) are complete.** The provisioner is fully GitOps-driven end-to-end:

- The 7-step wizard calls the real API, streams real SSE progress, and persists infra + dependency selections
- Manifests are generated with CPU/mem/replica values from the `environment_profiles` table (editable in admin UI)
- ArgoCD Applications are pointed at the correct Kubernetes cluster per environment via the cluster registry
- Platform service refs (CNPG cluster name, Kafka broker namespace, etc.) come from `cluster_platform_services` JSONB rows — no hardcoded environment variables
- All manifest templates live in DB and are editable by admins; provisioner uses them as source of truth
- Audit events record service creation and provisioning outcome; Recent Activity on the dashboard is live
- Every infra selection produces committed operator CRs — CNPG `Database`, Strimzi `KafkaTopic`, RabbitMQ `Vhost`+`User`, MinIO bucket init `Job`, Redis ACL init `Job` — all applied by a separate `-platform` ArgoCD Application per environment
- NetworkPolicy egress rules are generated from declared infra requirements and service dependencies; no manual firewall rules needed
- Services that select CNPG no longer use raw SQL (`EnsureDatabase`) — the CNPG operator provisions the database and user from the committed CR

DevPortal is now the single source of truth for access across all platform tools. When a lead adds or removes a member on an application, the change propagates in the background to DefectDojo (product member list), Harbor (project member), Gitea (group), and Dependency-Track (CVE email notification rule). Developers only see findings for their own services.

The remaining gap is the P5 operational items (Helm chart, self-hosting, SonarQube gate).

---

## Open gaps — ordered by priority

### P4 — Vault secret injection

| Gap | What needs to happen |
|-----|---------------------|
| **Vault path + AppRole per service** | At provision time: `vault secrets enable -path=secret/<app>/<service>`, write policy, create AppRole or K8s auth role bound to the service's ServiceAccount. |
| **VaultStaticSecret CR** | Commit `kind: VaultStaticSecret` (VSO) CR so VSO syncs the Vault path to a K8s Secret. Pods reference the Secret via `envFrom`. |
| **VaultAuth CR** | Commit `kind: VaultAuth` binding the service's ServiceAccount to its Vault role. |

### P5 — Production packaging

| Gap | What needs to happen |
|-----|---------------------|
| **Helm chart** | Full chart scaffold. `values.yaml` with personal (single-node) and production profiles. Chart linting + CI pipeline. |
| **DevPortal self-hosting** | Jenkins pipeline for DevPortal itself (dogfood). Harbor project, ArgoCD app. First successful self-deploy via the platform. |
| **SonarQube gate** | SAST gate in Jenkinsfile (quality gate fails build). Code quality dashboard link in service detail page. |
| **Multi-cluster ArgoCD** | Central ArgoCD managing dev + uat + prod via separate kubeconfig contexts. Cluster registry `api_endpoint` already drives the ArgoCD `destination.server` — this gap is the operational setup, not code. |

---

## Milestone map

```
NOW ──────────────────────────────────────────────────────────────► PRODUCTION

  [P1 ✅] Wizard      [P2 ✅] Cluster       [P3 ✅] Infra CRs  [P4 ✅] Vault
  wired to API        registry drives        CNPG / Kafka /    KV path + policy
                      manifests + ArgoCD     RabbitMQ /        K8s auth role
                                             MinIO / Redis     VaultAuth CR
                                             NetworkPolicy     VaultStaticSecret
                                             auto-generated    zero-plaintext pods

                                                               [P5] Helm + self-hosting
```

---

## Definition of "done"

The platform is considered complete when a developer can:

1. Log in to DevPortal
2. Fill the 7-step service creation form
3. Click **Provision service**
4. Watch all 15 steps complete in real time
5. Push a commit to the new repo
6. Watch Jenkins build → Harbor push → ArgoCD sync → pod running in Kubernetes
7. See the pod connect to its Postgres DB (via CNPG) and read its secrets from Vault (via VSO)
8. See the NetworkPolicy prevent any other pod from reaching it that was not declared as a dependency

Steps 1–6 work today. Steps 7 and 8 require the platform team to seed real secret values into the Vault KV paths created by step 16 (DB passwords from CNPG operator, MinIO access keys, etc.) — the wiring and policy setup is done, only the values remain.
