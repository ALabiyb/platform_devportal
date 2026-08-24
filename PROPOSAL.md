<!-- ---------------------------------------------------------------------------
     Author: Labiyb M. Said — DevSecOps Engineer
     Contact: saidlabiybm@gmail.com
     Organisation: NexBridge Technologies
     --------------------------------------------------------------------------- -->

# Proposal: DevPortal — NexBridge Internal Developer Platform

**Prepared for:** Engineering Leadership
**Prepared by:** Labiyb M. Said, DevSecOps Engineer
**Date:** 2026-08-24
**Status of the platform:** Built and running in a full local integration lab. Ready for a supervised pilot on real infrastructure.

---

## 1. Executive Summary

Every new microservice at NexBridge currently requires a developer to manually coordinate five to seven different tools — Git hosting, CI/CD, container registry, two separate security scanners, GitOps, and secrets management — before a single line of business logic ships. That coordination takes days, depends on whoever's free on the platform team that week, and produces inconsistent results: some services get NetworkPolicies, some don't; some get added to the vulnerability scanner, some are forgotten.

**DevPortal** collapses that entire process into one form and 17 automated steps, completing in under two minutes. It is built, it runs against a full local stack of every target integration (Gitea, Jenkins, Harbor, DefectDojo, Dependency-Track, Keycloak), and the code paths for Kubernetes deployment (ArgoCD, Vault, operator-managed databases) are complete and tested to the boundary of "no cluster available in this lab."

**The ask:** access to one real Kubernetes cluster and a Vault instance for a two-week supervised pilot, to take one real service from form submission through to a running, secured pod — closing the only gap between "built" and "proven."

---

## 2. The Problem

| Today, without an IDP | Cost |
|---|---|
| New service onboarding requires a platform engineer's time | Days of lead time, blocks the requesting team |
| Manual setup means inconsistent security posture | Some services skip vulnerability scanning, NetworkPolicies, or SBOM tracking |
| Access management is manual, per-tool | A developer leaving a team doesn't automatically lose Harbor/DefectDojo access |
| No standard for what "production ready" means | Every service reinvents its Jenkinsfile, its manifests, its resource limits |
| Secrets often end up in plaintext env vars or CI variables | Direct compliance and breach-blast-radius risk |

This isn't a hypothetical — it's the standard failure mode industry-wide, which is exactly why Spotify built Backstage, Netflix built Runway, and every scaled engineering org has independently arrived at the same conclusion: **provisioning has to be self-service and policy-enforced by construction, not by review.**

---

## 3. The Solution

A developer fills a 7-step wizard — service name, build tool, runtime port, infrastructure needs (checkboxes: Postgres/Kafka/Redis/RabbitMQ/MinIO), and which other services it talks to. DevPortal then, without any human in the loop:

1. Registers the service in the security scanner (DefectDojo) and SBOM tracker (Dependency-Track)
2. Creates the Git repository with a correct Jenkinsfile, Dockerfile, and branch protection
3. Registers the Jenkins CI/CD pipeline
4. Creates the container registry project and push credentials
5. Generates Kubernetes manifests, including a default-deny NetworkPolicy scoped to exactly the dependencies the developer declared
6. Generates the infrastructure the service asked for — as Kubernetes Custom Resources committed to Git, not manual clicks — a managed Postgres database, Kafka topics, a RabbitMQ vhost, a MinIO bucket, a Redis ACL user
7. Creates the GitOps deployment pipeline (ArgoCD) for dev, UAT, and prod
8. Provisions a Vault secrets path and binds the service's identity to it, so no password ever appears in a config file
9. Sets up CVE email alerts, addressed to exactly the people on that team

Every one of these steps is visible in real time in the browser as it happens.

**What makes this different from a script:** every step is idempotent (safe to re-run), every step degrades gracefully when its target integration isn't configured yet (so the platform is useful in a partial environment during rollout), and every generated file — Jenkinsfile, Kubernetes manifest, resource limits — is centrally editable by the platform team without a code deploy, so a policy change ("all prod services get 3 replicas minimum") propagates to every future service automatically.

---

## 4. Current Status — What Is Actually Built

This is not a prototype or a pitch deck. It is running code, validated against a complete local integration environment containing real instances of every target tool.

| Capability | Status |
|---|---|
| 17-step provisioning orchestrator | ✅ Built, running |
| Git repo + CI/CD + registry + security scanner automation | ✅ Built, validated against real Gitea/Jenkins/Harbor/DefectDojo |
| Operator-managed infrastructure (Postgres, Kafka, RabbitMQ, MinIO, Redis) as GitOps CRs | ✅ Built |
| Auto-generated zero-trust NetworkPolicies | ✅ Built |
| Vault secrets integration (path, policy, K8s auth binding) | ✅ Built — needs a live Vault to prove end-to-end |
| Multi-cluster ArgoCD (cluster registry, not hardcoded) | ✅ Built — needs a live cluster to prove end-to-end |
| Cross-platform access sync (one team-membership change → Git, security scanner, registry, CVE alerts, all updated automatically) | ✅ Built |
| Single sign-on across every tool (Keycloak) | ✅ Built, realm import tested |
| Runtime rebrandable (no rebuild) — supports multi-org / white-label deployment | ✅ Built |
| Kubernetes deployment packaging for the platform itself (Helm chart) | ✅ Built |

**What hasn't happened yet:** no service has gone from "click Provision" all the way to a running pod on a real Kubernetes cluster, because no real cluster has been made available to test against. Everything up to that point — and the code that would run after it — is done.

---

## 5. Business Value

| Dimension | Before | After DevPortal |
|---|---|---|
| Time to a fully wired new service | Days (dependent on platform team availability) | ~90 seconds, self-service |
| Consistency | Manual, varies by who set it up | Identical for every service — enforced by the generator, not a checklist |
| Security posture | Scanning/NetworkPolicy/secrets often skipped under deadline pressure | Present by construction — cannot be skipped without editing the platform |
| Access management overhead | Manual, per-tool, per-person | One change in DevPortal propagates to every tool automatically |
| Platform team load | Provisioning tickets, one by one | Platform team edits templates and policy centrally; provisioning is self-service |

The direct comparison point is Backstage (Spotify), which took Spotify's own platform team roughly two years to build internally before open-sourcing it. This is a leaner, opinionated equivalent purpose-built for NexBridge's actual stack, already functional.

---

## 6. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Untested against a real cluster | This proposal's ask is exactly that — a scoped pilot to close this gap before wider rollout |
| Single point of failure if DevPortal itself goes down | Provisioning is a background job queue (Postgres `SELECT FOR UPDATE SKIP LOCKED`) — a DevPortal restart resumes in-flight jobs, doesn't lose them; already-provisioned services are unaffected since they don't depend on DevPortal staying up |
| Team unfamiliarity with a new self-service tool | Wizard is 7 simple steps with live feedback; no YAML authoring required at any point |
| Vendor/tool lock-in | Every integration sits behind a Go interface (`GitProvider`, `RegistryProvider`, etc.) — swapping Gitea for GitLab, or Harbor for another registry, is an adapter, not a rewrite (GitLab adapter already exists as a fallback) |
| Admission-time policy enforcement (Kyverno) and runtime monitoring (Falco) not yet deployed | Signing (Cosign) and NetworkPolicy generation are done; admission/runtime enforcement is a cluster-side deployment step, not a DevPortal gap — sequenced after the pilot cluster is available |

---

## 7. The Ask

To move from "built and validated in the lab" to "proven in production," this proposal requests:

1. **One Kubernetes cluster** (dev-tier is sufficient) with the CNPG, Strimzi, and RabbitMQ operators installed, and ArgoCD running
2. **One Vault instance** (or a Vault namespace on a shared instance), with the Kubernetes auth method enabled
3. **A two-week window** for a supervised pilot: provision one real service end-to-end, verify the pod runs, connects to its CNPG-managed database, reads its secret from Vault, and is correctly fenced by NetworkPolicy
4. **Sign-off to onboard 2–3 pilot teams** after that validation, before a platform-wide rollout decision

No additional engineering headcount is requested for this phase — the platform is code-complete for it.

---

## 8. Appendix — Technology Stack

| Layer | Technology |
|---|---|
| Portal backend | Go 1.23, chi router, pgx/v5 |
| Portal frontend | React 18, TypeScript, Vite, TanStack Query v5 |
| Database | PostgreSQL 16 |
| Identity | Keycloak (OIDC) |
| SCM | Gitea (default), GitLab (supported) |
| CI/CD | Jenkins + shared library |
| Registry | Harbor + Trivy |
| Security | DefectDojo, Dependency-Track |
| GitOps | ArgoCD |
| Secrets | HashiCorp Vault (VSO or External Secrets Operator) |
| Data infra operators | CloudNativePG, Strimzi, RabbitMQ Operator |
| Image signing | Cosign / Sigstore |
| Packaging | Docker (single container, `go:embed` frontend), Helm chart |

Full technical detail: [`ARCHITECTURE.md`](ARCHITECTURE.md)
