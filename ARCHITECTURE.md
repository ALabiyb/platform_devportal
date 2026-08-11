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
4. [The 15-Step Provisioning Orchestrator](#4-the-15-step-provisioning-orchestrator)
5. [Manifest Repository Structure](#5-manifest-repository-structure)
6. [GitOps Flow — Code to Cluster](#6-gitops-flow--code-to-cluster)
7. [Security Layers](#7-security-layers)
8. [Service Dependency Model (Roadmap)](#8-service-dependency-model-roadmap)
9. [Cluster Architecture](#9-cluster-architecture)
10. [IDP Roadmap](#10-idp-roadmap)
11. [Reference — Companies Using IDPs](#11-reference--companies-using-idps)

---

## 1. What is DevPortal?

DevPortal is an **Internal Developer Platform (IDP)** — a self-service layer that lets a developer create a fully wired microservice in one form submission without knowing how Gitea, Jenkins, Harbor, DefectDojo, ArgoCD, or Kubernetes work.

The developer fills in:
- Service name + team
- Build tool (Maven / Go / Node.js / etc.)
- Notification email

DevPortal does the rest — 15 automated steps, live progress in the browser, no waiting for a DevOps engineer.

### Why IDPs exist

Without an IDP, each new microservice requires a developer to:
- Manually create a Git repo and branch protection
- Ask DevOps to set up a Jenkins job
- Ask DevOps to create a Harbor project and robot account
- Register a DefectDojo product and engagement
- Write Kubernetes manifests from scratch
- Configure ArgoCD

This takes days and introduces inconsistency. An IDP makes it take 90 seconds and produces identical, policy-compliant infrastructure every time.

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
| **SCM** | Gitea (default) / GitLab | Source code and manifest repositories |
| **CI/CD** | Jenkins + Shared Library | Build, test, scan, sign, push, promote |
| **Registry** | Harbor | Docker image storage + vulnerability scanning (Trivy) |
| **Security — SAST/SCA** | DefectDojo + Dependency-Track | Centralised vulnerability management + SBOM tracking |
| **Security — DAST** | OWASP ZAP | Dynamic application security testing on staging |
| **GitOps** | ArgoCD | Kubernetes deployment via manifest repo sync |
| **Workloads** | Kubernetes | Container orchestration (dev / uat / prod clusters) |
| **Database (app)** | CloudNative PG (CNPG) | Operator-managed PostgreSQL clusters per service |
| **Policy** | Kyverno | Admission control — blocks unsigned images, missing limits |
| **Runtime Security** | Falco | Syscall-level anomaly detection per pod |
| **Secrets** | HashiCorp Vault | Secret injection into pods via Vault Agent sidecar |
| **Image Signing** | Cosign (Sigstore) | ECDSA P-256 image signatures stored in Harbor |
| **Ingress** | Traefik / Gateway API | TLS termination + routing |

---

## 3. High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Developer's Browser                                 │
│                 Fills form → watches live step progress                     │
└───────────────────────────────┬─────────────────────────────────────────────┘
                                │ HTTPS
                                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                    DevPortal (Go API + React SPA)                           │
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
│                     15-Step Orchestrator                                     │
│                                                                             │
│  Steps 1-2      Steps 3-5     Steps 6-8     Steps 9-10    Steps 11-15      │
│  DefectDojo     SCM Repo      Jenkins        Harbor        Webhook +        │
│  product +      bootstrap +   folder +       project +     Manifest repo +  │
│  engagement     branches +    job +          robot         ArgoCD apps      │
│                 protection    record URL     account                        │
└──────┬──────────────┬──────────────┬──────────────┬──────────────┬──────────┘
       │              │              │              │              │
       ▼              ▼              ▼              ▼              ▼
  DefectDojo       Gitea          Jenkins        Harbor         ArgoCD
  (security hub)   (SCM)          (CI/CD)        (registry)     (GitOps)
       │                                                          │
       ▼                                                          ▼
  Dependency-Track                                         Kubernetes Cluster
  (SCA / SBOM)                                             dev / uat / prod
```

### Component responsibilities

| Component | Responsibility |
|:----------|:--------------|
| **Go API** | REST endpoints, authentication (local or OIDC), SSE streaming |
| **Worker** | Claims provisioning jobs from Postgres queue, runs orchestrator |
| **Orchestrator** | 15 deterministic steps; each step is idempotent and retryable |
| **SSE Hub** | Broadcasts real-time step events to all browser tabs watching a project |
| **Plugin layer** | One interface (`SCMPlugin`, `CIPlugin`, etc.) per integration — swap providers without changing orchestrator |

---

## 4. The 15-Step Provisioning Orchestrator

Each step is **idempotent** — re-running after a failure is always safe. Steps run sequentially. The SSE hub broadcasts `running → done/failed` transitions in real time.

| # | Step | Tool | Why this order |
|:--|:-----|:-----|:---------------|
| 1 | Ensure DefectDojo product | DefectDojo | Security context must exist before any code is written |
| 2 | Create DefectDojo CI/CD engagement | DefectDojo | Engagement ID is baked into Jenkinsfile — must exist before commit |
| 3 | Create source repository | SCM | Repo must exist before any file commits |
| 4 | Commit bootstrap files (Jenkinsfile, Dockerfile, VERSION) + create dev/uat/prod branches | SCM | Bootstrap commit must precede branch protection |
| 5 | Protect main branch | SCM | Blocks force-pushes; CI must be green to merge |
| 6 | Ensure Jenkins team folder | Jenkins | Folder must exist before creating a job inside it |
| 7 | Create Jenkins multibranch pipeline job | Jenkins | Job references the repo created in step 3 |
| 8 | Record Jenkins pipeline job URL | DevPortal DB | Stored for display and webhook use |
| 9 | Ensure Harbor registry project | Harbor | Image project must exist before robot account |
| 10 | Create Harbor robot account | Harbor | Robot credentials used by Jenkins to push images |
| 11 | Configure repository webhook | SCM | Webhook added **after** Jenkins scan settles (~30s) — prevents duplicate builds |
| 12 | Create manifest repository + commit K8s YAMLs | SCM | One repo per Application; service subdirectory per service |
| 13 | Create ArgoCD Application — dev | ArgoCD | Points at `dev` branch of manifest repo |
| 14 | Create ArgoCD Application — uat | ArgoCD | Points at `uat` branch of manifest repo |
| 15 | Create ArgoCD Application — prod + databases | ArgoCD + CNPG | prod env + PostgreSQL DB provisioned with security best practices |

### Why webhook comes last (step 11)

Jenkins performs an initial branch scan when a multibranch job is created. If the webhook is added before the scan completes, each branch push during that window triggers an additional build. By waiting (steps 9-10 add ~30s of buffer), the scan finishes before the webhook is live, ensuring exactly **one build per branch** on every push.

---

## 5. Manifest Repository Structure

### Current structure (flat — per-application repo, per-service folder)

```
<app-slug>-k8s/                    ← one repo per Application
  <service-slug>/                  ← one folder per Service
    namespace.yaml
    deployment.yaml
    service.yaml
    ingress.yaml
    hpa.yaml                       (prod only)
  branches: dev | uat | prod       ← one branch per environment
```

ArgoCD Application points at `path: <service-slug>/`, `targetRevision: dev|uat|prod`.

### Target structure (Kustomize base + overlays)

```
<app-slug>-k8s/
  <service-a>/
    base/
      kustomization.yaml
      namespace.yaml
      deployment.yaml
      service.yaml
      hpa.yaml
      networkpolicy.yaml           ← generated from service dependency declarations
      configmap.yaml
    overlays/
      dev/
        kustomization.yaml
        patch-replicas.yaml        (replicas: 1)
        patch-resources.yaml       (cpu/mem from dev environment profile)
      uat/
        kustomization.yaml
        patch-replicas.yaml
      prod/
        kustomization.yaml
        patch-replicas.yaml        (replicas: 3+)
        patch-resources.yaml       (cpu/mem from prod environment profile)
  <service-b>/
    base/ + overlays/
  infrastructure/
    postgres/
      <service-a>-db.yaml          ← CNPG Cluster CR (if service selected PostgreSQL)
      <service-b>-db.yaml
    kafka/
      <service-a>-topics.yaml      ← KafkaTopic CR (if service selected Kafka)
    rabbitmq/
      <service-a>-queues.yaml      ← RabbitmqQueue CR (if service selected RabbitMQ)
  argocd/
    <service-a>-dev.yaml           ← ArgoCD Application CR
    <service-a>-uat.yaml
    <service-a>-prod.yaml
```

### What are Custom Resources (CRs)?

Kubernetes itself knows `Deployment`, `Service`, `Ingress`, etc. **Custom Resources** extend Kubernetes with new resource types registered by **Operators**:

| Operator | Custom Resource | What Kubernetes does when you apply it |
|:---------|:----------------|:--------------------------------------|
| **CNPG** | `kind: Cluster` | Provisions a full PostgreSQL HA cluster |
| **Strimzi** | `kind: KafkaTopic` | Creates/manages a Kafka topic |
| **RabbitMQ Operator** | `kind: RabbitmqCluster` | Provisions a RabbitMQ broker |
| **ArgoCD** | `kind: Application` | Watches a Git path and syncs it to the cluster |
| **Kyverno** | `kind: ClusterPolicy` | Enforces admission rules on every new Pod |
| **Vault** | `kind: VaultStaticSecret` | Injects Vault secrets as K8s Secrets |

DevPortal generates these CRs as YAML files committed to the manifest repo. ArgoCD syncs them to the cluster. The operators react and do the actual work.

---

## 6. GitOps Flow — Code to Cluster

```
Developer pushes code to dev branch
          │
          ▼
Gitea webhook ──────────────────────→ Jenkins multibranch pipeline
                                               │
                             ┌─────────────────┼─────────────────────┐
                             ▼                 ▼                     ▼
                         Checkout          Build & Test          Security Scan
                         source repo       (Maven/Go/npm)        OWASP DC + SBOM
                                               │                     │
                                               ▼                     ▼
                                       docker build             DefectDojo
                                       docker push              (findings uploaded)
                                       (Harbor)                 Dependency-Track
                                               │                (SBOM uploaded)
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
ArgoCD syncs overlays/dev → Kubernetes
          │
          ▼
Rolling update — zero downtime
Kyverno validates: signed image ✓ resource limits ✓ labels ✓
Falco watches: runtime syscalls
```

### Promotion flow (dev → uat → prod)

```
Jenkins (dev build succeeds)
    │
    ▼  PR: merge dev manifests into uat branch
    │  (or: Jenkins promotes image tag automatically on tag push)
    ▼
ArgoCD syncs uat branch → uat namespace
    │
    ▼  After UAT sign-off
    ▼  PR: merge uat manifests into prod branch
    │
    ▼
ArgoCD syncs prod branch → prod namespace (3 replicas + HPA)
```

---

## 7. Security Layers

DevPortal implements **defence in depth** — multiple overlapping security controls:

```
                    Code                     Build                     Runtime
                      │                        │                          │
                      ▼                        ▼                          ▼
              ┌──────────────┐        ┌──────────────────┐       ┌──────────────────┐
              │  DefectDojo  │        │ Dependency-Track  │       │  Falco           │
              │  SAST/DAST   │        │  SCA / SBOM       │       │  Syscall monitor │
              │  findings hub│        │  CVE tracking     │       │  Anomaly alerts  │
              └──────────────┘        └──────────────────┘       └──────────────────┘
                      │                        │                          │
              ┌──────────────┐        ┌──────────────────┐       ┌──────────────────┐
              │ Harbor Trivy │        │ Cosign signature  │       │  Kyverno policy  │
              │ Image scan   │        │ on every image    │       │  Admission ctrl  │
              └──────────────┘        └──────────────────┘       └──────────────────┘
                                               │                          │
                                      ┌──────────────────┐       ┌──────────────────┐
                                      │ NetworkPolicy     │       │  Vault           │
                                      │ per-service       │       │  Secret inject   │
                                      │ (zero-trust pod   │       │  (no plaintext   │
                                      │  networking)      │       │   env vars)      │
                                      └──────────────────┘       └──────────────────┘
```

| Control | What it prevents |
|:--------|:----------------|
| **DefectDojo** | Centralised findings across SAST, DAST, dependency scans — SLA tracking |
| **Dependency-Track** | Vulnerable open-source libraries; alerts when a new CVE hits a known dependency |
| **Harbor + Trivy** | Vulnerable base images; blocks deployment of critical-severity images |
| **Cosign** | Tampered images; Kyverno rejects any image not signed by NexBridge key |
| **Kyverno** | Missing resource limits, privileged containers, missing required labels |
| **NetworkPolicy** | Lateral movement; pod A cannot reach pod B unless explicitly declared |
| **Vault** | Plaintext secrets in env vars; pods request secrets from Vault at startup |
| **Falco** | Zero-day exploits and insider threats; alerts on unexpected syscalls at runtime |

---

## 8. Service Dependency Model (Roadmap)

The next major capability: when creating a service, the developer declares what it needs. DevPortal generates everything automatically.

### Dependency declaration form (planned)

```
Service: order-service
Team: restaurant-pos

Infrastructure needs:
  [x] PostgreSQL       → CNPG Cluster CR + NetworkPolicy (service → postgres:5432)
  [ ] Redis            → Redis CR + NetworkPolicy
  [x] Kafka            → KafkaTopic CRs + NetworkPolicy (service → kafka-brokers:9092)
  [ ] RabbitMQ         → RabbitmqQueue CR + NetworkPolicy

This service talks to:
  [x] inventory-service  port 8080  → NetworkPolicy (order-service → inventory-service:8080)
  [x] auth-service       port 8080  → NetworkPolicy (order-service → auth-service:8080)

Exposed via ingress: Yes
```

### What DevPortal generates from these declarations

**CNPG PostgreSQL CR** (security best practice):
```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: order-db
  namespace: restaurant-pos-dev
spec:
  instances: 1                  # dev=1, uat=1, prod=3
  storage:
    size: 5Gi
  bootstrap:
    initdb:
      database: order_db
      owner: order_svc
      secret:
        name: order-db-credentials   # Vault-injected or Sealed Secret
  monitoring:
    enablePodMonitor: true
```

**NetworkPolicy** (zero-trust pod networking):
```yaml
# Only order-service can reach its own database
kind: NetworkPolicy
spec:
  podSelector:
    matchLabels:
      app: order-db
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: order-service
      ports:
        - port: 5432
```

**Service-to-service NetworkPolicy**:
```yaml
# order-service is allowed to call inventory-service
kind: NetworkPolicy
spec:
  podSelector:
    matchLabels:
      app: inventory-service
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: order-service
      ports:
        - port: 8080
```

### DB schema additions needed

```sql
-- What infrastructure a service requires
CREATE TABLE service_infra_requirements (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id    UUID REFERENCES projects(id) ON DELETE CASCADE,
  infra_type    TEXT NOT NULL,   -- 'postgres' | 'redis' | 'kafka' | 'rabbitmq'
  config        JSONB,           -- e.g. {"storage": "10Gi", "instances": 3}
  created_at    TIMESTAMPTZ DEFAULT NOW()
);

-- Which services talk to which (drives NetworkPolicy generation)
CREATE TABLE service_dependencies (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  from_project   UUID REFERENCES projects(id) ON DELETE CASCADE,
  to_project     UUID REFERENCES projects(id) ON DELETE CASCADE,
  port           INTEGER NOT NULL DEFAULT 8080,
  protocol       TEXT DEFAULT 'TCP',
  created_at     TIMESTAMPTZ DEFAULT NOW()
);
```

---

## 9. Cluster Architecture

DevPortal registers one Kubernetes cluster per environment in the **Cluster Registry**. ArgoCD runs inside each cluster (or a central ArgoCD manages all clusters via kubeconfig).

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Kubernetes Cluster (dev)                           │
│                                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────────┐  │
│  │  ArgoCD      │  │  Kyverno     │  │  Falco       │  │  Vault Agent   │  │
│  │  (GitOps)    │  │  (policy)    │  │  (runtime)   │  │  (secrets)     │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  └────────────────┘  │
│                                                                             │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │  Namespaces per service × environment                                  │ │
│  │  order-service-dev    payment-service-dev    auth-service-dev          │ │
│  │                                                                        │ │
│  │  Each namespace contains:                                              │ │
│  │  Deployment  Service  Ingress  HPA  NetworkPolicy                      │ │
│  │  CNPG Cluster (if postgres selected)                                   │ │
│  │  KafkaTopic (if kafka selected)                                        │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                                                                             │
│  CNPG Operator    Strimzi Operator    RabbitMQ Operator    Gateway API      │
│  (watches CNPG CRs) (watches KafkaTopic) (watches RMQ CRs)                 │
└─────────────────────────────────────────────────────────────────────────────┘
```

### ArgoCD App-of-Apps pattern

DevPortal commits ArgoCD `Application` CRs into `argocd/` folder of the manifest repo. ArgoCD's root app watches that folder and bootstraps all service apps automatically. Adding a new service → one new YAML file → ArgoCD picks it up without any manual kubectl apply.

```
argocd/
  order-service-dev.yaml    ← ArgoCD Application: path=order-service/overlays/dev
  order-service-uat.yaml    ← ArgoCD Application: path=order-service/overlays/uat
  order-service-prod.yaml   ← ArgoCD Application: path=order-service/overlays/prod
  payment-service-dev.yaml
  ...
```

---

## 10. IDP Roadmap

### Done ✓

- [x] 15-step provisioning orchestrator
- [x] SCM abstraction (Gitea / GitLab / GitHub-ready)
- [x] Jenkins multibranch pipeline + shared library
- [x] Harbor image project + robot account
- [x] DefectDojo product + engagement (SAST/DAST hub)
- [x] Dependency-Track SBOM upload (URL + API key credential)
- [x] Cosign image signing + Kyverno enforcement
- [x] ArgoCD GitOps wiring (dev / uat / prod)
- [x] Live SSE step progress stream in browser
- [x] Platform topology visualization on provisioning page
- [x] Application → Service hierarchy
- [x] Cluster registry + environment profiles
- [x] 15 editable K8s manifest templates
- [x] Edit & Retry on failed provisioning
- [x] Webhook timing fix (1 build per branch per push)

### In Progress / Next

- [ ] Kustomize base + overlays manifest structure
- [ ] Service infrastructure dependency selection (PostgreSQL / Redis / Kafka / RabbitMQ)
- [ ] CNPG Cluster CR generation per service that selects PostgreSQL
- [ ] NetworkPolicy generation from service-to-service dependency declarations
- [ ] Service dependency DB tables (`service_infra_requirements`, `service_dependencies`)
- [ ] App-level manifest repo (ArgoCD Application CRs committed to `argocd/` folder)

### Later

- [ ] Falco alert → DefectDojo finding integration
- [ ] Vault secret injection wiring (VaultAuth + VaultStaticSecret CRs)
- [ ] SonarQube code quality gate in Jenkins pipeline
- [ ] Multi-cluster ArgoCD (central ArgoCD managing dev + uat + prod)
- [ ] Service catalogue UI (searchable list of all services, owners, dependencies)
- [ ] Kafka / RabbitMQ topic and queue CR generation
- [ ] Cost attribution per team (namespace resource usage)

---

## 11. Reference — Companies Using IDPs

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

*Last updated: 2026-08-11 — NexBridge Technologies — Labiyb M. Said*
