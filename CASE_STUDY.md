<!-- ---------------------------------------------------------------------------
     Author: Labiyb M. Said — DevSecOps Engineer
     Contact: saidlabiybm@gmail.com
     Organisation: NexBridge Technologies
     --------------------------------------------------------------------------- -->

# Building DevPortal: An Internal Developer Platform From Scratch

**A case study in Platform Engineering — NexBridge Technologies**

---

## The problem I set out to solve

At most engineering orgs, spinning up a new microservice looks the same: someone opens a ticket, a platform engineer manually creates a Git repo, wires up a Jenkins job, registers a container registry project, maybe remembers to add it to the vulnerability scanner, writes Kubernetes YAML by hand, and configures a GitOps sync. It takes days. It's inconsistent — every service ends up shaped slightly differently depending on who set it up and how much time pressure they were under.

I wanted to see what it would actually take to build a real Internal Developer Platform — not a slide deck describing one, an actual system that takes a developer from "I need a new service" to a fully provisioned, security-scanned, GitOps-deployed microservice without a human in the loop. Spotify's Backstage was the reference point; I wanted to understand *why* it exists by building an equivalent, purpose-fit to a stack I control end-to-end.

---

## Starting point: what "done" actually means

The temptation with a project like this is to build a form that creates a Git repo and call it an IDP. I decided early that "done" meant something much stricter — a developer should be able to:

1. Fill one form
2. Watch the platform provision a Git repo, a CI/CD pipeline, a container registry, a security scanner entry, Kubernetes manifests, a GitOps deployment, a managed database, and a secrets path
3. Push code and watch it flow through build → scan → sign → deploy automatically
4. See the resulting pod refuse to talk to anything it wasn't explicitly allowed to talk to

That's a materially harder bar than "creates a repo." It meant the platform had to actually understand infrastructure lifecycle — not just call APIs, but reason about ordering, idempotency, and partial failure.

---

## The core design decision: an orchestrator, not a script

The provisioning flow ended up as 17 discrete, ordered steps — DefectDojo product creation, Git repo bootstrap, Jenkins job registration, Harbor project + robot account, manifest generation, ArgoCD application creation, Vault secret path setup, Dependency-Track registration.

Two decisions shaped everything downstream:

**Every step is idempotent.** If step 9 fails, re-running the whole provisioning job doesn't create a duplicate Harbor project or a second Git repo — each step checks whether its target state already exists before acting. This sounds obvious in principle and is genuinely annoying in practice: it means every single integration adapter needs a "get or create" pattern, not just "create," for every resource type across seven different APIs with seven different conventions for "does this already exist."

**Every step that depends on optional infrastructure skips gracefully instead of failing.** Not every deployment of this platform will have a Kubernetes cluster wired up on day one, or a Vault instance ready. Rather than making those hard requirements, each dependent step checks its precondition and logs a skip:

```go
if o.cfg.ArgoCDURL == "" {
    slog.Info("provisioner: ArgoCD not configured — skipping GitOps step")
    return nil
}
```

This turned out to be the decision that made the whole thing testable. I built and validated the entire platform — Git automation, CI/CD, registry, both security tools — against a full local lab running real instances of every integration, with no Kubernetes cluster available. Steps 13 through 16 log "skipped," everything else runs for real, and the manifests get committed to Git exactly as they would in a fully wired deployment, ready for a cluster to pick them up later. The architecture didn't need a "demo mode" — the graceful-skip design *is* the demo mode, and it's identical code to the production path.

---

## The subtle bug that taught me the most: webhook timing

Jenkins performs an initial branch scan the moment a multibranch pipeline job is created. My first version registered the Git webhook immediately after creating the Jenkins job — seemed like the obvious order. In testing, every push during the first ~30 seconds after service creation triggered *two* builds instead of one: Jenkins's own initial scan, plus the webhook firing for the same event.

The fix was to explicitly sequence the webhook registration *after* the Harbor project and robot account steps — not because those steps have anything to do with Jenkins, but because they reliably consume enough wall-clock time for Jenkins's scan to finish first. It's a small thing, but it's the kind of bug that only shows up when you actually run the full sequence against real tools, not when you unit test each adapter in isolation. It's why "built and validated against a real local stack" mattered more to me than "compiles and passes mocked tests."

---

## Where it got genuinely hard: infrastructure as Custom Resources

The easy version of "provision a database for this service" is DevPortal running `CREATE DATABASE` and `CREATE USER` directly. I built that first. It works, but it means DevPortal is now a long-lived holder of database admin credentials, and the database's existence lives only in DevPortal's memory — not in Git, not visible to anyone auditing the manifest repo.

The harder, better version: DevPortal commits a `kind: Database` Custom Resource to the manifest repo. A CloudNativePG operator running in the cluster watches for that CR and does the actual provisioning — including credential rotation, HA failover, backups. DevPortal never touches a database password. The tradeoff is real, though: DevPortal now has to *not* run its direct SQL path when the CR path is selected, or the two would race each other. Getting that branch right — and doing the same thing for Kafka topics (Strimzi), RabbitMQ vhosts, MinIO buckets, and Redis ACL users — was most of the platform's actual complexity. Each of those is a different operator, a different CR shape, a different idempotency story.

---

## The access-control problem nobody templates for

Somewhere past the halfway point I hit a problem that isn't really about provisioning at all: once a service exists, who can see it in each of the five different tools it now lives in? By default, every developer could see every product in the security scanner and every project in the registry, regardless of which team actually owned the service — because none of those tools knew anything about DevPortal's notion of "team."

I built a `membersync` service that treats DevPortal's own membership table as the source of truth and fans changes out to everywhere else — the security scanner's product members, the registry's project members, the Git group, and the CVE-alert recipient list on the SBOM tracker — whenever someone is added to or removed from an application. Add someone to a team in DevPortal, and within seconds they can see their team's findings in the security scanner and start receiving vulnerability emails for their own services, without anyone touching four separate admin panels by hand. This is the piece I'd point to as the actual "platform" part of the platform — the provisioning automation is the more visible half, but the access model is what makes it usable by more than one team without turning into a permissions mess.

---

## Design principle I kept coming back to: nothing is a special case

Branding is one small example of a bigger pattern I tried to hold to throughout. The frontend derives its entire color palette — buttons, badges, panel backgrounds — from three HSL hue values read from a JSON endpoint at boot. Change three environment variables, restart the container, and the portal looks like a different product. It's a small feature, but the discipline behind it — config, not code, drives anything that varies per deployment — is the same discipline behind the cluster registry (add a Kubernetes cluster through a form, not a redeploy), the manifest template system (edit a generated Jenkinsfile's content from the admin UI, not a Git commit to the platform's own source), and the Vault/ExternalSecrets toggle (switch which secrets operator a deployment uses with one flag, not a code branch). Every one of these started as "just hardcode it for now" and got pulled out into config once I noticed the same shape of problem recurring.

---

## What's proven and what isn't — yet

Everything through container registry, CI/CD, and both security tools has run against real software, repeatedly, and works. The Kubernetes-facing half — ArgoCD sync, the operator-managed infrastructure, Vault secret injection — is written, compiles, and is architected the same way as everything that's already validated, but hasn't had a real cluster to run against yet. I'm treating that distinction seriously rather than glossing over it: "the code exists" and "I've watched it work" are different claims, and only one of them is currently true for the last few steps.

That's the honest state of the project — not a demo dressed up as done, and not further along than it actually is.

---

## What I'd do differently

If I were starting over, I'd build the graceful-skip pattern in from step one instead of retrofitting it around step 12 — it turned out to be load-bearing for testability, not an edge case. I'd also introduce the plugin interface abstraction (`GitProvider`, `RegistryProvider`, etc.) before writing the first concrete adapter rather than after the second one, once it became clear I'd need to support both Gitea and GitLab — refactoring three working adapters behind a shared interface after the fact was more churn than designing for it upfront would have been.

---

## Stack

Go 1.23 · React 18 + TypeScript · PostgreSQL 16 · Gitea/GitLab · Jenkins · Harbor · DefectDojo · Dependency-Track · Keycloak · ArgoCD · HashiCorp Vault · CloudNativePG · Strimzi · RabbitMQ Operator · Cosign/Sigstore · Docker · Helm

Full architecture: [`ARCHITECTURE.md`](ARCHITECTURE.md)
