<!-- ---------------------------------------------------------------------------
     Author: Labiyb M. Said — DevSecOps Engineer
     Contact: saidlabiybm@gmail.com
     Organisation: NexBridge Technologies
     --------------------------------------------------------------------------- -->

# DevPortal — Platform Architecture

This document is the authoritative design record for DevPortal, NexBridge's Internal Developer Platform (IDP). It describes what has been built, why each decision was made, and where the platform is heading.

---

## Table of Contents

1. [What is DevPortal?](#1-what-is-devportal)
2. [Platform Stack](#2-platform-stack)
3. [High-Level Architecture](#3-high-level-architecture)
4. [The 17-Step Provisioning Orchestrator](#4-the-17-step-provisioning-orchestrator)
5. [Manifest Repository Structure](#5-manifest-repository-structure)
6. [Custom Resources Generated Per Service](#6-custom-resources-generated-per-service)
7. [GitOps Flow — Code to Cluster](#7-gitops-flow--code-to-cluster)
8. [Cross-Platform Identity & Access](#8-cross-platform-identity--access)
9. [Security Layers](#9-security-layers)
10. [Cluster Architecture](#10-cluster-architecture)
11. [Platform Self-Service — Branding & Multi-Tenancy](#11-platform-self-service--branding--multi-tenancy)
12. [What's Left to Production](#12-whats-left-to-production)
13. [Reference — Companies Using IDPs](#13-reference--companies-using-idps)

---

## 1. What is DevPortal?

DevPortal is an **Internal Developer Platform (IDP)** — a self-service layer that lets a developer create a fully wired microservice in one form submission without knowing how Gitea, Jenkins, Harbor, DefectDojo, Dependency-Track, ArgoCD, Vault, or Kubernetes work underneath.

The developer fills a 7-step wizard:
1. **Identity** — service name, application, team
2. **Build tool** — Maven / Gradle / Go / Node.js / Python / .NET / Flutter
3. **Runtime** — port, health check paths, resource tier
4. **Infra** — PostgreSQL / Kafka / Redis / RabbitMQ / MinIO (checkboxes)
5. **Deps** — which other services in the application this one talks to
6. **Review** — summary of everything about to be created
7. **Provision** — live step-by-step progress, streamed over SSE

DevPortal does the rest — **17 automated steps**, live progress in the browser, no waiting for a DevOps engineer.

### Why IDPs exist

Without an IDP, each new microservice requires a developer to:
- Manually create a Git repo and branch protection
- Ask DevOps to set up a Jenkins job
- Ask DevOps to create a Harbor project and robot account
- Register a DefectDojo product and engagement, and a Dependency-Track project
- Write Kubernetes manifests, NetworkPolicies, and operator CRs from scratch
- Configure ArgoCD and Vault secret paths by hand
- Manually add every teammate to five different tools

This takes days and introduces inconsistency — every service ends up slightly different, and security controls get skipped under deadline pressure. An IDP makes it take **~90 seconds** and produces identical, policy-compliant infrastructure every time.

**Companies that operate IDPs at scale:**
- **Netflix** — internal platform called *Runway*; handles 1,000+ microservices
- **Spotify** — built *Backstage* (open-sourced in 2020); now the most widely adopted IDP framework
- **Airbnb** — internal platform team manages service lifecycle for 3,000+ services
- **Uber** — *uDeploy* and *devpod* for container lifecycle management
- **LinkedIn** — LPS (LinkedIn Platform Services) orchestrates Java and Python services across regions

---

## 2. Platform Stack

| Layer | Tool | Role |
|:------|:-----|:-----|
| **Portal** | DevPortal (Go + React) | Self-service UI + provisioning orchestrator |
| **Identity** | Keycloak | Single OIDC IdP — SSO across every tool below |
| **SCM** | Gitea (default) / GitLab | Source code and manifest repositories |
| **CI/CD** | Jenkins + Shared Library | Build, test, scan, sign, push, promote |
| **Registry** | Harbor | Docker image storage + vulnerability scanning (Trivy) |
| **Security — SAST/DAST hub** | DefectDojo | Centralised findings, SLA tracking, per-application member access |
| **Security — SCA / SBOM** | Dependency-Track | CVE tracking on dependencies, CVE email alerts per application member |
| **GitOps** | ArgoCD | Kubernetes deployment via manifest repo sync, per-cluster registry |
| **Workloads** | Kubernetes | Container orchestration (dev / uat / prod clusters, one registry entry per cluster) |
| **Database (app)** | CloudNativePG (CNPG) | Operator-managed PostgreSQL clusters per service, committed as CRs |
| **Streaming** | Strimzi | Operator-managed Kafka topics per service |
| **Messaging** | RabbitMQ Operator | Vhost + user CRs per service |
| **Object storage** | MinIO | Bucket init Jobs per service |
| **Cache** | Redis | ACL user init Jobs, key-namespace isolation per service |
| **Secrets** | HashiCorp Vault (VSO or ESO) | KV path + policy + K8s auth role per service; VaultStaticSecret or ExternalSecret CR |
| **Image Signing** | Cosign (Sigstore) | ECDSA signatures stored in Harbor |
| **Ingress** | Traefik | TLS termination + routing (`*.docker.localhost` in the lab, real domains in prod) |

---

## 3. High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Developer's Browser                                 │
│                 Fills 7-step wizard → watches live step progress            │
└───────────────────────────────┬─────────────────────────────────────────────┘
                                │ HTTPS (Keycloak SSO or local auth)
                                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                    DevPortal (Go API + React SPA, single container)         │
│                                                                             │
│   ┌──────────────┐   ┌───────────────┐   ┌───────────────────────────────┐ │
│   │  React SPA   │   │  Go REST API  │   │  SSE Hub (live step stream)   │ │
│   │  (Vite)      │   │  (chi router) │   │  EventSource → browser        │ │
│   └──────────────┘   └───────┬───────┘   └───────────────────────────────┘ │
│                              │ INSERT provisioning_job                      │
└──────────────────────────────┼──────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                 PostgreSQL Job Queue                                         │
│          SELECT FOR UPDATE SKIP LOCKED — safe multi-worker                  │
└──────────────────────────────┬──────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│              Worker Pool (embedded goroutines or standalone pod)             │
│                     17-Step Orchestrator                                     │
│                                                                             │
│  Steps 1-2      Steps 3-5     Steps 6-8     Steps 9-11    Step 12          │
│  DefectDojo     Gitea repo    Jenkins        Harbor +      Manifest repo    │
│  product +      bootstrap +   folder +       webhook       + operator CRs  │
│  engagement     branches +    job                          + NetworkPolicy │
│                 protection                                                 │
│                                                                             │
│  Steps 13-15                Step 16              Step 17                   │
│  ArgoCD apps                Vault KV path +      Dependency-Track          │
│  dev/uat/prod                policy + K8s role    project + CVE email      │
│  (per-cluster registry,     (VSO or ESO CR)       notification rule        │
│   graceful skip if unset)                                                  │
└──────┬──────────────┬──────────────┬──────────────┬──────────────┬──────────┘
       │              │              │              │              │
       ▼              ▼              ▼              ▼              ▼
  DefectDojo       Gitea          Jenkins        Harbor         ArgoCD
  Dependency-      (SCM)          (CI/CD)        (registry)     (GitOps)
  Track                                                            │
                                                                    ▼
                                                          Kubernetes Cluster(s)
                                                          dev / uat / prod
                                                    (CNPG, Strimzi, RabbitMQ,
                                                     MinIO, Redis, Vault/VSO)
```

### Component responsibilities

| Component | Responsibility |
|:----------|:--------------|
| **Go API** | REST endpoints, authentication (local bcrypt or Keycloak OIDC), SSE streaming, branding config |
| **Worker** | Claims provisioning jobs from Postgres queue (`SELECT FOR UPDATE SKIP LOCKED`), runs orchestrator |
| **Orchestrator** | 17 deterministic steps; each step is idempotent and retryable; steps 13–16 skip gracefully when the corresponding integration is unconfigured |
| **SSE Hub** | Broadcasts real-time step events to every browser tab watching a project |
| **membersync.Service** | Fans out application member changes to DefectDojo, Harbor, and Dependency-Track as a background goroutine |
| **Plugin layer** | One interface per integration (`GitProvider`, `CIProvider`, `RegistryProvider`, `SecurityProvider`, `ArgoCDProvider`, `VaultProvider`, `DependencyTrackProvider`) — swap providers without touching the orchestrator |

---

## 4. The 17-Step Provisioning Orchestrator

Each step is **idempotent** — re-running after a failure is always safe. Steps run sequentially. The SSE hub broadcasts `running → done/failed` transitions in real time.

| # | Step | Tool | Why this order |
|:--|:-----|:-----|:---------------|
| 1 | Ensure DefectDojo product | DefectDojo | Security context must exist before any code is written |
| 2 | Create DefectDojo CI/CD engagement | DefectDojo | Engagement ID is baked into Jenkinsfile — must exist before commit |
| 3 | Create source repository | Gitea/GitLab | Repo must exist before any file commits |
| 4 | Commit bootstrap files (Jenkinsfile, Dockerfile, VERSION) + create dev/uat/prod branches | SCM | Bootstrap commit must precede branch protection |
| 5 | Protect main branch | SCM | Blocks force-pushes; CI must be green to merge |
| 6 | Ensure Jenkins team folder | Jenkins | Folder must exist before creating a job inside it |
| 7 | Create Jenkins multibranch pipeline job | Jenkins | Job references the repo created in step 3; triggers initial branch scan |
| 8 | Record Jenkins pipeline job URL | DevPortal DB | Stored for display and later reference |
| 9 | Ensure Harbor registry project | Harbor | Image project must exist before robot account |
| 10 | Create Harbor robot account | Harbor | Robot credentials used by Jenkins to push images |
| 11 | Configure repository webhook | SCM | Added **after** the Jenkins scan settles (~30s buffer from steps 9–10) — prevents duplicate builds |
| 12 | Commit manifest repo — Kustomize overlays, operator CRs, NetworkPolicy | SCM | Renders from DB-editable templates + cluster registry + environment profiles + infra selections |
| 13 | Create ArgoCD Application — dev | ArgoCD | Destination server resolved from the cluster registry; skipped gracefully if no ArgoCD configured |
| 14 | Create ArgoCD Application — uat | ArgoCD | Same, `uat` environment |
| 15 | Create ArgoCD Application — prod + `-platform` app | ArgoCD | prod env + separate ArgoCD Application syncing the `infra/` directory (CNPG, Strimzi, RabbitMQ, MinIO, Redis CRs) |
| 16 | Configure Vault secrets path | Vault | KV path + ACL policy + K8s auth role per environment; skipped gracefully if `VAULT_URL` unset or no infra selected |
| 17 | Register Dependency-Track project + CVE notifications | Dependency-Track | Project created; email notification rule scoped to it, addressed to every application member |

### Why webhook comes last (step 11)

Jenkins performs an initial branch scan when a multibranch job is created. If the webhook is added before the scan completes, each branch push during that window triggers an additional build. Waiting for steps 9–10 to finish (~30s of buffer) lets the scan finish before the webhook goes live, guaranteeing exactly **one build per branch** on every push.

### Graceful skip design

Steps 13–16 depend on infrastructure that may not exist in every deployment (a Kubernetes cluster, a Vault instance). Rather than failing the whole run, each of these steps checks its precondition first and logs a skip:

```go
if o.cfg.ArgoCDURL == "" {
    slog.Info("provisioner: ArgoCD not configured — skipping GitOps step")
    return nil
}
```

This means the platform is useful in three deployment modes without any code branching:
- **Local lab** (no cluster) — steps 1–12 and 17 run for real; 13–16 log skipped; manifests are still committed to git, ready for a cluster to sync them later
- **Staging** (cluster, no Vault) — steps 1–15 and 17 run for real; 16 skips
- **Production** (everything wired) — all 17 steps run for real

---

## 5. Manifest Repository Structure

```
<app-slug>-k8s/                          ← one repo per Application
  <service-slug>/
    base/
      kustomization.yaml
      namespace.yaml
      deployment.yaml
      service.yaml
      hpa.yaml
      networkpolicy.yaml                 ← generated from infra selections + service dependencies
      configmap.yaml
    overlays/
      dev/    kustomization.yaml, patch-replicas.yaml, patch-resources.yaml
      uat/    kustomization.yaml, patch-replicas.yaml, patch-resources.yaml
      prod/   kustomization.yaml, patch-replicas.yaml, patch-resources.yaml
    infra/
      dev/    cnpg-database.yaml   kafkatopic.yaml   rabbitmq-vhost.yaml
              rabbitmq-user.yaml   minio-job.yaml     redis-acl-job.yaml
              vault-auth.yaml      vault-static-secret.yaml   (or external-secret.yaml)
      uat/    (same set, uat-scoped)
      prod/   (same set, prod-scoped)
  argocd/
    <service-slug>-dev.yaml               ← ArgoCD Application (workload)
    <service-slug>-uat.yaml
    <service-slug>-prod.yaml
    <service-slug>-platform-dev.yaml      ← ArgoCD Application (infra/ CRs only)
    <service-slug>-platform-uat.yaml
    <service-slug>-platform-prod.yaml
  branches: dev | uat | prod              ← ArgoCD targetRevision per environment
```

**cpu/mem/replicas** in `patch-resources.yaml` and `patch-replicas.yaml` come from the `environment_profiles` table — set once by the platform team, applied to every service in that tier automatically.

Every manifest file's *content* — not just its existence — is admin-editable. `manifest_templates` in Postgres holds 19 templates (base resources, overlay patches, and every operator CR listed above). The generator merges any admin override on top of the built-in default; if no override exists, the default template renders unchanged. This means platform engineers can adjust the Deployment probe timings, add a company-wide label, or change a resource default without a code deploy.

---

## 6. Custom Resources Generated Per Service

Kubernetes itself knows `Deployment`, `Service`, `Ingress`. **Custom Resources** extend it with new types registered by **Operators**. DevPortal generates these CRs as YAML in `infra/<env>/`; a dedicated `-platform` ArgoCD Application syncs them separately from the workload Application, so an operator CR change never triggers a workload restart and vice versa.

| Trigger (infra checkbox) | CR generated | Operator | What happens on apply |
|:--------------------------|:--------------|:---------|:-----------------------|
| PostgreSQL | `kind: Database` | CNPG | Operator provisions the DB + user inside the shared CNPG cluster — no manual SQL, no plaintext password in the manifest repo |
| Kafka | `kind: KafkaTopic` (one per declared topic) | Strimzi | Creates the topic — 3 partitions, replication factor 1, 7-day retention by default |
| RabbitMQ | `kind: Vhost` + `kind: User` | RabbitMQ Operator | Provisions an isolated vhost and a scoped user, credentials referenced by secret name |
| MinIO | `kind: Job` (ArgoCD sync hook) | — | Runs `mc mb --ignore-existing` against the shared MinIO tenant; `BeforeHookCreation` delete policy makes re-syncs idempotent |
| Redis | `kind: Job` (ArgoCD sync hook) | — | Runs `ACL SETUSER <service>-<env> ... ~<service>-<env>:*` — per-service key-namespace isolation on a shared Redis instance |
| (always, if any infra/deps declared) | `kind: NetworkPolicy` | — | Default-deny; explicit ingress/egress rules per selected infra namespace and per declared service-to-service dependency |
| (always) | `kind: VaultAuth` + `kind: VaultStaticSecret` **or** `kind: ExternalSecret` | VSO or ESO (`VAULT_USE_VSO` toggle) | Binds the service's K8s ServiceAccount to its Vault policy; syncs the Vault KV path into a native K8s Secret the pod reads via `envFrom` |

**Why CRs instead of DevPortal calling `CREATE DATABASE` directly:** the CR is declarative and lives in Git — anyone can see exactly what infrastructure a service has by reading its manifest repo, and the operator (not DevPortal) owns the actual provisioning, retries, and lifecycle. DevPortal explicitly skips its own direct SQL path (`EnsureDatabase`/`EnsureUser`) whenever CNPG is selected, to avoid racing the operator.

---

## 7. GitOps Flow — Code to Cluster

```
Developer pushes code to dev branch
          │
          ▼
Gitea webhook ──────────────────────→ Jenkins multibranch pipeline
                                               │
                             ┌─────────────────┼─────────────────────┐
                             ▼                 ▼                     ▼
                         Checkout          Build & Test          Security Scan
                         source repo       (per build-tool       SBOM + SAST
                                            template)                 │
                                               │                     ▼
                                               ▼                 DefectDojo +
                                       docker build             Dependency-Track
                                       docker push               (findings + SBOM
                                       (Harbor, Trivy scan)       uploaded)
                                               │
                                               ▼
                                       cosign sign
                                       (Sigstore ECDSA)
                                               │
                                               ▼
                                    Update image tag in
                                    manifest repo (dev branch)
                                               │
          ┌────────────────────────────────────┘
          │  Gitea webhook
          ▼
ArgoCD detects manifest change
          │
          ▼
ArgoCD syncs overlays/dev + infra/dev → Kubernetes (per cluster registry entry)
          │
          ▼
Rolling update — zero downtime
NetworkPolicy already in place — pod can only reach declared dependencies
Pod reads its secrets from the Vault-synced K8s Secret (no plaintext env vars)
```

### Promotion flow (dev → uat → prod)

```
Jenkins (dev build succeeds)
    │
    ▼  PR: merge dev manifests into uat branch (or automated tag promotion)
    ▼
ArgoCD syncs uat branch → uat namespace, on the uat cluster from the registry
    │
    ▼  After UAT sign-off
    ▼  PR: merge uat manifests into prod branch
    ▼
ArgoCD syncs prod branch → prod namespace (higher replica count + HPA from environment_profiles)
```

---

## 8. Cross-Platform Identity & Access

Keycloak is the single IdP for every tool in the stack — one login, one set of groups (`devportal-admins`, `devportal-developers`), federated into Gitea, Jenkins, Harbor, DefectDojo, SonarQube, and Dependency-Track via OIDC.

DevPortal's own `project_members` table is the **source of truth for application-level access**. When a lead adds or removes a member on an application in the DevPortal UI, `membersync.Service` fans that change out in the background to every other tool:

```
AddApplicationMember (DevPortal DB + Gitea group)
          │
          ▼  go h.syncer.SyncApplicationMembers(ctx, appID)   ← background goroutine, non-fatal errors
          │
   ┌──────┼──────────────────────┬──────────────────────────┐
   ▼      ▼                      ▼                          ▼
Gitea   DefectDojo            Harbor                 Dependency-Track
group   SyncProductMembers    SyncProjectMembers      EnsureEmailNotification
        (adds/removes         (adds/removes           (rewrites the CVE alert
         product members,      project members,         rule's recipient list)
         reconciles to          role_id: 2/Developer)
         exact wanted set)
```

**Effect:** a developer added to an application in DevPortal automatically sees only their own products in DefectDojo, only their own projects in Harbor, and starts receiving CVE emails for their own services in Dependency-Track — without a platform engineer touching four separate admin consoles.

---

## 9. Security Layers

DevPortal implements **defence in depth** — overlapping controls at code, build, and runtime:

```
                    Code                     Build                     Runtime
                      │                        │                          │
                      ▼                        ▼                          ▼
              ┌──────────────┐        ┌──────────────────┐       ┌──────────────────┐
              │  DefectDojo  │        │ Dependency-Track  │       │  NetworkPolicy   │
              │  SAST/DAST   │        │  SCA / SBOM       │       │  zero-trust pod  │
              │  findings hub│        │  CVE tracking     │       │  networking      │
              └──────────────┘        └──────────────────┘       └──────────────────┘
                      │                        │                          │
              ┌──────────────┐        ┌──────────────────┐       ┌──────────────────┐
              │ Harbor Trivy │        │ Cosign signature  │       │  Vault / VSO     │
              │ Image scan   │        │ on every image     │       │  no plaintext    │
              └──────────────┘        └──────────────────┘       │  secrets in pods  │
                                                                   └──────────────────┘
```

| Control | What it prevents | Status |
|:--------|:----------------|:------:|
| **DefectDojo** | Findings scattered across teams — centralised triage with SLA tracking, scoped per application member | ✅ built |
| **Dependency-Track** | Silent CVE drift in dependencies — email alert the moment a new CVE hits a known library | ✅ built |
| **Harbor + Trivy** | Vulnerable base images shipping to prod | ✅ built |
| **Cosign** | Tampered or unauthorised images | ✅ built (signing step; admission enforcement is cluster-side Kyverno, not yet wired) |
| **NetworkPolicy** | Lateral movement — a pod can't reach another pod unless explicitly declared as a dependency or selected infra | ✅ built, generated automatically |
| **Vault (VSO/ESO)** | Plaintext secrets in env vars or Git — pods get secrets synced from Vault at deploy time | ✅ built |
| **Keycloak SSO + group RBAC** | Credential sprawl, orphaned accounts across 7 tools | ✅ built |
| **Cross-platform member sync** | Developers seeing findings/registries for services they don't own | ✅ built |

---

## 10. Cluster Architecture

DevPortal registers **one Kubernetes cluster per environment** in the Cluster Registry (`clusters` + `cluster_platform_services` tables) — no more flat, single-cluster env vars. Each registry row carries the environment's ArgoCD destination server and the namespace/cluster-name of each shared platform service (CNPG cluster, Kafka broker namespace, MinIO tenant, Redis instance).

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Kubernetes Cluster (per env, from registry)         │
│                                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────────┐  │
│  │  ArgoCD      │  │  Kyverno     │  │  Falco       │  │  Vault Agent / │  │
│  │  (GitOps)    │  │  (policy —   │  │  (runtime —  │  │  VSO           │  │
│  │              │  │   planned)   │  │   planned)   │  │  (secrets)     │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  └────────────────┘  │
│                                                                             │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │  Namespace per service × environment                                   │ │
│  │  order-service-dev    payment-service-dev    auth-service-dev          │ │
│  │                                                                        │ │
│  │  Each namespace: Deployment · Service · Ingress · HPA · NetworkPolicy  │ │
│  │  + whichever CRs the service selected (CNPG / KafkaTopic / Vhost /     │ │
│  │    MinIO Job / Redis Job / VaultAuth+StaticSecret)                     │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                                                                             │
│  CNPG Operator    Strimzi Operator    RabbitMQ Operator    (shared,        │
│  (watches CNPG CRs) (watches KafkaTopic) (watches Vhost/User)  cluster-wide)│
└─────────────────────────────────────────────────────────────────────────────┘
```

Adding a new cluster is a form submission in **Platform → Clusters**, not a code change — the orchestrator reads the registry at provision time and resolves the correct destination server, CNPG cluster name, and namespace conventions per environment automatically.

---

## 11. Platform Self-Service — Branding & Multi-Tenancy

The Docker image is identical for every deployment; branding is entirely runtime config, no rebuild:

| Var | Controls |
|-----|----------|
| `BRAND_APP_NAME`, `BRAND_COMPANY` | Portal title and org name |
| `BRAND_PRIMARY_HUE` | CTAs, buttons, links, focus ring |
| `BRAND_SECONDARY_HUE` | Badges, tag chips, secondary highlights |
| `BRAND_SURFACE_HUE` | Panel/card/border tint |
| `BRAND_LOGO_URL` | Hosted logo image |

`GET /branding.json` serves these values; React writes them as CSS custom properties before first paint, and every color token in the stylesheet derives from `hsl(var(--brand-hue) ...)`. This means the same binary can be deployed for multiple internal orgs or white-labelled for a customer, distinguished only by environment variables.

The Helm chart (`chart/`) ships with `personal` and `production` value profiles and supports both a standard Kubernetes `Ingress` and a Traefik `IngressRoute`, so the platform team itself can run DevPortal on the same Kubernetes clusters it provisions services into.

---

## 12. What's Left to Production

Everything described above is implemented and compiles clean. What remains is **operational validation, not code**:

| Gap | What's actually missing |
|-----|--------------------------|
| **Real Kubernetes cluster(s)** | Steps 13–16 are code-complete but currently skip gracefully — no cluster is registered yet. Registering one dev cluster proves the ArgoCD + CNPG + NetworkPolicy path end-to-end. |
| **Real Vault instance** | Step 16's wiring (KV write, policy, K8s auth role) is done; it needs a running Vault + real secret values seeded once (DB passwords, MinIO keys) |
| **Kyverno admission enforcement** | Cosign signing happens; nothing yet rejects an unsigned image at admission time — this is a cluster-side policy, not a DevPortal change |
| **Falco runtime monitoring** | Not yet deployed to any cluster — operational, not code |
| **DevPortal self-hosting** | The platform hasn't provisioned itself through its own pipeline yet (dogfooding) |
| **SonarQube quality gate** | SonarQube runs standalone; not yet wired as a required Jenkinsfile gate |
| **One real end-to-end test** | No service has been taken from "click Provision" through to a running, Vault-backed, NetworkPolicy-fenced pod on a real cluster yet |

None of this requires new features. It requires infrastructure (a cluster, a Vault instance) and one supervised test run.

---

## 13. Reference — Companies Using IDPs

| Company | Platform | Public resource |
|:--------|:---------|:----------------|
| Spotify | Backstage (open source) | backstage.io |
| Netflix | Runway (internal) | netflixtechblog.com — search "paved road" |
| Airbnb | Internal Platform Team | medium.com/airbnb-engineering |
| Uber | uDeploy / devpod | eng.uber.com |
| LinkedIn | LPS | engineering.linkedin.com |

**Learning resources:**

- [CNCF Platform Engineering Whitepaper](https://tag-app-delivery.cncf.io/whitepapers/platforms/) — the definitive practitioner reference, free
- *Team Topologies* — Matthew Skelton & Manuel Pais — explains **why** platforms exist
- *Platform Engineering* — Luca Galante — the **how**, written by Humanitec's CEO
- [Humanitec blog](https://humanitec.com/blog) — resource dependency graph patterns
- [platformengineering.org](https://platformengineering.org) — community hub, Slack, case studies

---

*Last updated: 2026-08-24 — NexBridge Technologies — Labiyb M. Said*
