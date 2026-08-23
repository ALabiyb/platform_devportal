# DevPortal — NexBridge Technologies

Internal Developer Platform (IDP). A developer fills one 7-step form and gets a fully provisioned, production-ready service in ~90 seconds — Git repo, CI/CD pipeline, container registry project, security scanning product, Kubernetes manifests, GitOps applications, Vault secrets path, and Dependency-Track CVE alerts — all wired together, zero manual steps.

**Author:** Labiyb M. Said — DevSecOps Engineer · saidlabiybm@gmail.com  
**Company:** NexBridge Technologies

> Full IDP vision, manifest repo structure, GitOps flow, and roadmap → [`ARCHITECTURE.md`](ARCHITECTURE.md)  
> Build goals and phase tracker → [`GOALS.md`](GOALS.md)

---

## Table of Contents

1. [What DevPortal Provisions](#1-what-devportal-provisions)
2. [Local Lab Stack](#2-local-lab-stack)
3. [Keycloak Realm Setup](#3-keycloak-realm-setup)
4. [Configure Each Tool for SSO](#4-configure-each-tool-for-sso)
5. [DevPortal Configuration](#5-devportal-configuration)
6. [Running DevPortal Locally](#6-running-devportal-locally)
7. [First Login and Initial Setup](#7-first-login-and-initial-setup)
8. [Testing End-to-End](#8-testing-end-to-end)
9. [Database Migrations](#9-database-migrations)
10. [Jenkins Credentials Setup](#10-jenkins-credentials-setup)
11. [Image Signing (Cosign)](#11-image-signing-cosign)
12. [Deploying to Kubernetes (Helm)](#12-deploying-to-kubernetes-helm)
13. [Deploying to Another Environment](#13-deploying-to-another-environment)
14. [Environment Variables Reference](#14-environment-variables-reference)
15. [make help](#15-make-help)

---

## 1. What DevPortal Provisions

One form → 17 automated steps:

| Step | What happens |
|------|-------------|
| 1 | DefectDojo product created with SLA deadlines (Critical / High / Medium / Low days) |
| 2 | DefectDojo CI/CD engagement created — Jenkins will import SBOM scans here |
| 3 | Gitea repository created (private, team namespace) |
| 4 | Jenkinsfile + Dockerfile + VERSION committed in one atomic commit; dev / uat / prod branches created |
| 5 | `main` branch protected — no force-push |
| 6 | Jenkins team folder created |
| 7 | Jenkins multibranch pipeline job created; Jenkins scans immediately |
| 8 | Pipeline URL saved |
| 9 | Harbor image project created (Trivy auto-scan on push enabled) |
| 10 | Harbor robot account created for Jenkins push/pull |
| 11 | Gitea webhook registered → Jenkins; all future pushes trigger CI |
| 12 | Manifest repo `<app>-k8s` gets Kustomize base + dev / uat / prod overlays; NetworkPolicy egress rules auto-generated from declared service dependencies and infra |
| 13–15 | ArgoCD Applications created per environment — skipped gracefully when no cluster configured |
| 16 | Vault KV path + ACL policy + K8s auth role created — skipped when `VAULT_URL` not set |
| 17 | Dependency-Track project created; email notification rule set up — all application members receive CVE alerts via Mailpit (local) or real SMTP (prod) |

**Infra selections** (PostgreSQL / Kafka / Redis / RabbitMQ / MinIO) produce committed operator CRs in a separate `-platform` ArgoCD Application — CNPG Database, Strimzi KafkaTopic, RabbitMQ Vhost+User, MinIO bucket init Job, Redis ACL init Job.

**Member sync** — when a lead adds or removes a team member in the DevPortal UI, the change propagates in the background to Gitea (group), DefectDojo (product member), Harbor (project member), and Dependency-Track (CVE notification emails). Keycloak is the single IdP — users need only one login.

---

## 2. Local Lab Stack

All services run behind **Traefik v3.6** on `*.docker.localhost`.

| Service | URL | Role |
|---------|-----|------|
| **DevPortal** | https://devportal.localhost | This platform |
| **Keycloak** | https://keycloak.docker.localhost | IdP — SSO for all tools |
| **Gitea** | https://git.docker.localhost | Source control |
| **Jenkins** | https://jenkins.docker.localhost | CI/CD |
| **Harbor** | https://harbor.docker.localhost | Container registry |
| **DefectDojo** | https://defectdojo.docker.localhost | Vulnerability management |
| **Dependency-Track** | https://dtrack.docker.localhost | SCA + CVE tracking |
| **SonarQube** | https://sonarqube.docker.localhost | Static analysis |
| **Mailpit** | https://mail.docker.localhost | SMTP catcher — all email lands here |
| **Traefik** | https://traefik.docker.localhost | Edge router |
| **pgAdmin** | https://pgadmin.docker.localhost | DB management |

> **No k8s cluster?** Steps 13–15 (ArgoCD) and 16 (Vault) are skipped gracefully. Steps 1–12 and 17 all run and produce real output.

---

## 3. Keycloak Realm Setup

A ready-to-import realm export is at `keycloak/realm-export.json`. It creates:
- Realm `nexbridge`
- OIDC clients for every tool with correct redirect URIs
- Groups: `devportal-admins`, `devportal-developers`
- Groups claim mapper (groups appear in access tokens)

### Import

1. Go to **https://keycloak.docker.localhost/admin**
2. Top-left dropdown → **Create realm**
3. Click **Browse** → select `keycloak/realm-export.json` → **Create**

The realm `nexbridge` appears immediately with all clients pre-configured.

### Change client secrets

The export contains placeholder secrets (e.g. `devportal-secret-change-me`). Change each one:

**Keycloak admin → nexbridge realm → Clients → `<client>` → Credentials → Regenerate**

Update the corresponding value in your `.env.local` and in each tool's OIDC configuration.

### Create users

**Keycloak admin → nexbridge → Users → Add user**

For DevPortal admin access: after creating the user, go to **Groups** tab → join `devportal-admins`.

---

## 4. Configure Each Tool for SSO

Replace `<secret>` with the actual client secret from Keycloak.  
Issuer URL for all tools: `https://keycloak.docker.localhost/realms/nexbridge`

### Gitea

**Site Administration → Authentication Sources → Add Authentication Source**

| Field | Value |
|-------|-------|
| Authentication Type | OAuth2 |
| Authentication Name | `keycloak` |
| OAuth2 Provider | OpenID Connect |
| Client ID | `gitea` |
| Client Secret | `<gitea secret>` |
| OpenID Connect Auto Discovery URL | `https://keycloak.docker.localhost/realms/nexbridge/.well-known/openid-configuration` |

### Jenkins

Install plugin: **OpenID Connect Authentication** (`oic-auth`)

**Manage Jenkins → Security → Security Realm → Login with OpenID Connect**

| Field | Value |
|-------|-------|
| Client id | `jenkins` |
| Client secret | `<jenkins secret>` |
| Well Known Configuration URL | `https://keycloak.docker.localhost/realms/nexbridge/.well-known/openid-configuration` |
| User name field | `preferred_username` |

### Harbor

**Administration → Configuration → Authentication**

| Field | Value |
|-------|-------|
| Auth Mode | OIDC |
| OIDC Provider Name | Keycloak |
| OIDC Endpoint | `https://keycloak.docker.localhost/realms/nexbridge` |
| OIDC Client ID | `harbor` |
| OIDC Client Secret | `<harbor secret>` |
| OIDC Scope | `openid,profile,email` |
| Verify Certificate | unchecked (self-signed) |
| Automatic onboarding | checked |
| Username Claim | `preferred_username` |

### DefectDojo

**System Settings → Social Authentication**

| Field | Value |
|-------|-------|
| Enable OIDC | ✓ |
| OIDC_OP_JWKS_ENDPOINT | `https://keycloak.docker.localhost/realms/nexbridge/protocol/openid-connect/certs` |
| OIDC_OP_AUTHORIZATION_ENDPOINT | `https://keycloak.docker.localhost/realms/nexbridge/protocol/openid-connect/auth` |
| OIDC_OP_TOKEN_ENDPOINT | `https://keycloak.docker.localhost/realms/nexbridge/protocol/openid-connect/token` |
| OIDC_OP_USER_ENDPOINT | `https://keycloak.docker.localhost/realms/nexbridge/protocol/openid-connect/userinfo` |
| OIDC_RP_CLIENT_ID | `defectdojo` |
| OIDC_RP_CLIENT_SECRET | `<defectdojo secret>` |

Also add to DefectDojo `.env`:
```
DD_SOCIAL_AUTH_OIDC_OIDC_ENDPOINT=https://keycloak.docker.localhost/realms/nexbridge
```

### SonarQube

**Administration → Configuration → General Settings → Authentication → OpenID Connect**

| Field | Value |
|-------|-------|
| Enabled | true |
| Client ID | `sonarqube` |
| Client Secret | `<sonarqube secret>` |
| Issuer URI | `https://keycloak.docker.localhost/realms/nexbridge` |

### Dependency-Track

**Administration → Configuration → OpenID Connect**

| Field | Value |
|-------|-------|
| Enable OpenID Connect | ✓ |
| Issuer | `https://keycloak.docker.localhost/realms/nexbridge` |
| Client ID | `dependency-track` |

**SMTP (Mailpit) — for CVE email testing:**

**Administration → Notifications → Alert Configuration**

| Field | Value |
|-------|-------|
| SMTP Server Hostname | `mail` (Docker network name) or `mail.docker.localhost` |
| SMTP Server Port | `1025` |
| SMTP TLS | disabled |
| From address | `dtrack@nexbridge.local` |

All notification emails land in **https://mail.docker.localhost** — no real SMTP needed.

---

## 5. DevPortal Configuration

Copy `.env.local` to `.env` and fill in the token/password values:

```bash
cp .env.local .env
```

Values to fill in (get these from each tool):

| Variable | Where to get it |
|----------|----------------|
| `DB_PASSWORD` | your Postgres password for the `devportal` user |
| `ENCRYPTION_KEY` | `openssl rand -base64 32` |
| `GITEA_TOKEN` | Gitea → Settings → Applications → Generate Token (scope: all) |
| `JENKINS_TOKEN` | Jenkins → your user → Configure → API Token → Add new Token |
| `HARBOR_TOKEN` | Harbor admin password (or robot account secret) |
| `DEFECTDOJO_TOKEN` | DefectDojo → API v2 → `/api/v2/` → Auth Token endpoint |
| `DEPENDENCY_TRACK_API_KEY` | DT → Administration → Teams → create team → API Keys → Generate |
| `OIDC_CLIENT_SECRET` | Keycloak → nexbridge → Clients → devportal → Credentials |

Create the DevPortal database (one time):

```bash
docker exec postgres psql -U postgres -c "
  CREATE DATABASE devportal;
  CREATE USER devportal WITH PASSWORD 'devportal';
  GRANT ALL PRIVILEGES ON DATABASE devportal TO devportal;
  ALTER DATABASE devportal OWNER TO devportal;
"
```

---

## 6. Running DevPortal Locally

### Option A — Go binary (fastest for development)

```bash
# Build frontend
cd web && npm ci && npm run build && cd ..

# Run API (migrations run automatically on startup)
make dev
# or: go run ./cmd/devportal
```

DevPortal listens on `:8080`. With `AUTH_MODE=local`, no Keycloak needed for first login.

### Option B — Docker (matches production)

```bash
# Build image
docker build -t devportal:dev .

# Run (joined to same Docker network as other tools)
docker run -d \
  --name devportal \
  --network traefik-net \
  -l "traefik.enable=true" \
  -l "traefik.http.routers.devportal.rule=Host(\`devportal.localhost\`)" \
  -l "traefik.http.routers.devportal.tls=true" \
  --env-file .env \
  devportal:dev
```

Or use the compose file (joins `traefik-net` automatically):

```bash
docker compose up -d
```

---

## 7. First Login and Initial Setup

### Local auth (AUTH_MODE=local)

Register the first admin account — only available before any user exists:

```bash
curl -X POST https://devportal.localhost/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "name":     "Labiyb Said",
    "email":    "admin@nexbridge.local",
    "password": "your-password"
  }'
```

The first account is automatically granted `admin` role.

### OIDC (AUTH_MODE=oidc)

1. Import the Keycloak realm (see §3)
2. Create a user in the `nexbridge` realm and add them to `devportal-admins`
3. Set `AUTH_MODE=oidc` and fill `OIDC_CLIENT_SECRET` in `.env`
4. Go to **https://devportal.localhost** → you are redirected to Keycloak → login → redirected back

### Platform Engineering — Admin setup (do this before first provisioning)

Log in as admin → **Platform** in the sidebar:

1. **Clusters** → Add a cluster per environment (or leave empty — steps 13–15 skip gracefully)
2. **Platform Services** → For each cluster, configure CNPG / Kafka / Redis / RabbitMQ / MinIO namespaces
3. **Environment Profiles** → Set CPU / memory / replica limits per tier
4. **Manifest Templates** → Review defaults (auto-seeded on startup); edit if needed

---

## 8. Testing End-to-End

### What will work (no k8s cluster needed)

Steps 1–12 and 17 all call real tools:

| Step | Expected result |
|------|----------------|
| 1–2 | DefectDojo product + engagement visible at `defectdojo.docker.localhost` |
| 3–5 | Repository appears in Gitea at `git.docker.localhost/<team>/<service>` with Jenkinsfile committed |
| 6–8 | Jenkins job appears at `jenkins.docker.localhost/<team>/<service>` |
| 9–10 | Harbor project + robot account at `harbor.docker.localhost` |
| 11 | Gitea webhook registered — push to any branch triggers Jenkins |
| 12 | `<app>-k8s` manifest repo created in Gitea with Kustomize base + overlays |
| 13–15 | Logged as skipped (ArgoCD not configured) — not a failure |
| 16 | Logged as skipped (Vault not configured) — not a failure |
| 17 | DT project created at `dtrack.docker.localhost`; check `mail.docker.localhost` for test notification |

### Step by step

1. **Create a team** — Admin → Teams → New Team
2. **Create an application** — Applications → New Application → assign to team
3. **Add members** — Application → Members → add a second user; check DefectDojo + Harbor for updated membership
4. **Create a service** — Applications → `<app>` → New Service → fill the wizard → Provision
5. **Watch live progress** — the step stream shows real-time results; steps 13–15 show "skipped"
6. **Verify in each tool**:
   - Gitea: repo + Jenkinsfile exist
   - Jenkins: job runs DEVPORTAL_BOOTSTRAP scan
   - Harbor: project + robot account exist
   - DefectDojo: product + engagement exist, member is listed
   - Dependency-Track: project exists; check Mailpit for notification email
7. **Push code** — `git clone` the new repo, push a commit, watch Jenkins build → Harbor image push

---

## 9. Database Migrations

Schema migrations run automatically every time DevPortal starts. No manual SQL.

Current migration count: **13**

```sql
-- Check what has run:
SELECT version, applied_at FROM schema_migrations ORDER BY applied_at;
```

### Adding a migration

1. Create `internal/db/migrations/014_your_change.sql`
2. Use `IF NOT EXISTS` / `IF EXISTS` — migrations run inside a transaction and are never replayed
3. `make dev` or `docker compose up -d --build` — migration runs on next startup

---

## 10. Jenkins Credentials Setup

Create these in **Manage Jenkins → Credentials → System → Global → Add credentials** before any pipeline runs:

| Credential ID | Kind | Value |
|--------------|------|-------|
| `robot-jenkins` | Username with password | User: `robot$devportal` · Password: Harbor robot token from step 10 |
| `gitea-token` | Username with password | Gitea bot username + `GITEA_TOKEN` from `.env` |
| `cosign-private-key` | Secret file | Upload `cosign.key` |
| `cosign-password` | Secret text | cosign key passphrase |
| `dependency-track-api-key` | Secret text | DT team API key (`DEPENDENCY_TRACK_API_KEY`) |
| `sonarqube-token` | Secret text | SonarQube user token (Administration → Security → Users → Tokens) |

**SonarQube in Jenkins** — also configure the SonarQube server:

**Manage Jenkins → System → SonarQube servers → Add**
- Name: `SonarQube`
- Server URL: `https://sonarqube.docker.localhost`
- Server authentication token: `sonarqube-token` credential

---

## 11. Image Signing (Cosign)

Every image pushed to Harbor is signed with [Cosign](https://docs.sigstore.dev/cosign/overview/). The signature is stored as an OCI artifact in Harbor. On a k8s cluster, Kyverno enforces that only signed images are admitted.

The cosign key pair lives in:
- `cosign.pub` — committed to this repo (public key, safe to share)
- `cosign.key` — stored only in Jenkins as a Secret file credential (`cosign-private-key`)

To verify a signed image manually:

```bash
COSIGN_INSECURE_SKIP_TLS_VERIFY=true \
cosign verify --key cosign.pub \
  harbor.docker.localhost/<project>/<image>:<tag>
```

To rotate the key pair:

```bash
docker exec -it jenkins sh
COSIGN_PASSWORD="new-password" cosign generate-key-pair --output-key-prefix nexbridge
```

Then update `cosign.pub` in the repo, update the Kyverno policy public key, and replace the Jenkins credential.

---

## 12. Deploying to Kubernetes (Helm)

The Helm chart at `chart/` packages DevPortal for production Kubernetes deployments.

### Prerequisites

```bash
# Create the secrets before installing
kubectl create secret generic devportal-secrets \
  --from-literal=DB_PASSWORD=xxx \
  --from-literal=ENCRYPTION_KEY=xxx \
  --from-literal=GITEA_TOKEN=xxx \
  --from-literal=JENKINS_TOKEN=xxx \
  --from-literal=HARBOR_TOKEN=xxx \
  --from-literal=DEFECTDOJO_TOKEN=xxx \
  --from-literal=OIDC_CLIENT_SECRET=xxx \
  --from-literal=APP_DB_ADMIN_PASSWORD=xxx \
  --from-literal=DEPENDENCY_TRACK_API_KEY=xxx
```

### Install

```bash
# Personal / homelab profile
helm install devportal ./chart \
  -f chart/profiles/personal.yaml \
  --set config.authMode=oidc \
  --set ingress.host=devportal.example.com \
  --set ingress.traefik.enabled=true

# Production profile
helm install devportal ./chart \
  -f chart/profiles/production.yaml \
  --set config.authMode=oidc \
  --set ingress.host=devportal.example.com \
  --set ingress.traefik.enabled=true \
  --set ingress.traefik.certResolver=letsencrypt
```

### Upgrade

```bash
helm upgrade devportal ./chart -f chart/profiles/personal.yaml
```

### Values

| Key | Default | Description |
|-----|---------|-------------|
| `replicaCount` | `1` | Pod count (ignored when HPA enabled) |
| `autoscaling.enabled` | `false` | Enable HPA |
| `config.authMode` | `local` | `local` or `oidc` |
| `config.argoCDURL` | `""` | Leave empty when no k8s cluster — steps 13–15 skipped |
| `config.vaultURL` | `""` | Leave empty when no Vault — step 16 skipped |
| `ingress.traefik.enabled` | `false` | Use Traefik IngressRoute instead of Ingress |
| `existingSecret` | `devportal-secrets` | Name of the K8s Secret holding all sensitive values |

---

## 13. Deploying to Another Environment

DevPortal is a single Docker image. Every environment (homelab, staging, production) runs the **same image** — only `.env` changes. No rebuild needed for a new environment or a new customer.

### What changes between environments

| What | Local lab | Staging / Production |
|------|-----------|----------------------|
| All tool URLs | `*.docker.localhost` | your actual domain, e.g. `*.example.com` |
| `OIDC_ISSUER_URL` | `https://keycloak.docker.localhost/realms/nexbridge` | update domain + realm name if you rename it |
| `OIDC_REDIRECT_URL` | `https://devportal.localhost/auth/callback` | `https://devportal.example.com/auth/callback` |
| `INGRESS_BASE_DOMAIN` | `docker.localhost` | `example.com` — baked into generated K8s Ingress manifests |
| `REGISTRY_URL` | `harbor.docker.localhost` | `harbor.example.com` — baked into generated Jenkinsfiles |
| `SHARED_LIBRARY_URL` | Gitea URL on the local network | Gitea URL on the new network |
| `TLS_SKIP_VERIFY` | `true` (self-signed Traefik certs) | `false` (valid certs from cert-manager / Let's Encrypt) |
| `DB_SSL_MODE` | `disable` | `require` |
| `AUTH_MODE` | `local` (no Keycloak needed) | `oidc` (Keycloak backed by AD/LDAP) |
| `ARGOCD_URL` / `VAULT_URL` | empty (graceful skip) | set when cluster is available |

### Step-by-step for a new environment

1. **Copy the config template**
   ```bash
   cp .env.example .env
   ```

2. **Set all URLs** — replace every `docker.localhost` occurrence with your new domain.

3. **Generate required secrets**
   ```bash
   # Encryption key (one-time per environment — never reuse across envs)
   openssl rand -base64 32    # → ENCRYPTION_KEY

   # DB password
   openssl rand -hex 16       # → DB_PASSWORD
   ```

4. **SCM provider** — if you are using GitLab instead of Gitea:
   ```bash
   GIT_PROVIDER=gitlab
   GITLAB_URL=https://gitlab.example.com
   GITLAB_TOKEN=glpat-xxx
   ```
   If staying on Gitea, leave `GIT_PROVIDER=gitea` (the default) and fill `GITEA_URL`/`GITEA_TOKEN`.

5. **Import the Keycloak realm** (see §3) — but update redirect URIs in `keycloak/realm-export.json` to use the new domain before importing, or edit them in the Keycloak admin UI after import.

6. **Keycloak client secrets** — regenerate each client secret in Keycloak and copy the new values into each tool's OIDC config AND into your `.env`.

7. **Branding** — no rebuild needed. All three hues drive the complete portal palette. Change `BRAND_*` vars in `.env` and restart:
   - `BRAND_APP_NAME` — portal title in nav and login page
   - `BRAND_COMPANY` — organisation name under the logo
   - `BRAND_PRIMARY_HUE` — main CTAs, buttons, links, focus ring (`199`=sky · `142`=green · `221`=indigo · `38`=amber)
   - `BRAND_SECONDARY_HUE` — badges, tag chips, secondary highlights (`262`=violet · `300`=pink · `180`=teal)
   - `BRAND_SURFACE_HUE` — panel/card/border tint (`215`=cool gray · `20`=warm gray · `222`=deep navy)
   - `BRAND_LOGO_URL` — URL to a hosted SVG or PNG logo

   React writes all three as CSS custom properties (`--brand-hue`, `--brand-secondary-hue`, `--brand-surface-hue`) before first render. Every colour token derives from these — the same image ships to every customer, branded via env vars only.

8. **Database** — create the devportal DB on the target Postgres instance (same `CREATE DATABASE` command as §5), then set `DB_HOST`, `DB_USER`, `DB_PASSWORD` in `.env`.

9. **Start** — `docker compose up -d` or `helm install` (see §12).

### Things baked into generated files at provisioning time

These values are written into Jenkinsfiles and K8s manifest repos when a service is first provisioned. Changing them in `.env` afterwards does NOT retroactively update already-provisioned services — only new services pick up the new values.

| Variable | Baked into |
|----------|-----------|
| `REGISTRY_URL` | every generated `Jenkinsfile` (image push/pull URL) |
| `INGRESS_BASE_DOMAIN` | every generated K8s `Ingress` / `IngressRoute` host rule |
| `SHARED_LIBRARY_URL` | every generated `Jenkinsfile` `@Library` annotation |
| `DEPENDENCY_TRACK_API_KEY_ID` | every generated `Jenkinsfile` DT upload step |

If you need to migrate already-provisioned services to a new domain, re-run provisioning or manually update the committed Jenkinsfiles in each service's Git repo.

---

## 14. Environment Variables Reference

All config comes from environment variables. Only `DB_PASSWORD` and `ENCRYPTION_KEY` are required at startup — everything else is optional and only fails at provisioning time when actually used.

| Variable | Required | Default | Description |
|----------|:--------:|---------|-------------|
| `DB_PASSWORD` | **yes** | — | DevPortal's own DB password |
| `ENCRYPTION_KEY` | **yes** | — | AES-256-GCM key. `openssl rand -base64 32` |
| `HTTP_ADDR` | no | `:8080` | Listen address |
| `AUTH_MODE` | no | `local` | `local` or `oidc` |
| `OIDC_ISSUER_URL` | oidc | — | `https://keycloak.docker.localhost/realms/nexbridge` |
| `OIDC_CLIENT_ID` | oidc | `devportal` | Keycloak client ID |
| `OIDC_CLIENT_SECRET` | oidc | — | Keycloak client secret |
| `OIDC_REDIRECT_URL` | oidc | — | `https://devportal.localhost/auth/callback` |
| `OIDC_ADMIN_GROUP` | no | `devportal-admins` | Keycloak group → admin role |
| `OIDC_DEVELOPER_GROUP` | no | `devportal-developers` | Keycloak group → developer role |
| `GIT_PROVIDER` | no | `gitea` | `gitea` or `gitlab` |
| `GITEA_URL` | no | — | `https://git.docker.localhost` |
| `GITEA_TOKEN` | no | — | Gitea PAT (scope: all) |
| `JENKINS_URL` | no | — | `https://jenkins.docker.localhost` |
| `JENKINS_PUBLIC_URL` | no | — | Public Jenkins URL (used in webhooks) |
| `JENKINS_TOKEN` | no | — | Jenkins API token |
| `HARBOR_URL` | no | — | `https://harbor.docker.localhost` |
| `HARBOR_TOKEN` | no | — | Harbor admin password |
| `DEFECTDOJO_URL` | no | — | `https://defectdojo.docker.localhost` |
| `DEFECTDOJO_TOKEN` | no | — | DefectDojo API token |
| `DEPENDENCY_TRACK_URL` | no | — | `https://dtrack.docker.localhost` |
| `DEPENDENCY_TRACK_API_KEY` | no | — | DT team API key (DevPortal direct calls) |
| `DEPENDENCY_TRACK_API_KEY_ID` | no | `dependency-track-api-key` | Jenkins credential ID for Jenkinsfile |
| `ARGOCD_URL` | no | — | Leave empty without k8s cluster |
| `ARGOCD_TOKEN` | no | — | ArgoCD account token |
| `VAULT_URL` | no | — | Leave empty without Vault |
| `VAULT_TOKEN` | no | — | Vault provisioner token |
| `VAULT_KV_MOUNT` | no | `secret` | KV v2 mount path |
| `VAULT_K8S_AUTH_MOUNT` | no | `kubernetes` | K8s auth mount path |
| `VAULT_USE_VSO` | no | `false` | `true` = VSO CRs, `false` = ESO ExternalSecret |
| `REGISTRY_URL` | no | — | `harbor.docker.localhost` (no https://) |
| `REGISTRY_CREDENTIALS_ID` | no | `robot-jenkins` | Jenkins credential ID |
| `GIT_CREDENTIALS_ID` | no | — | Jenkins credential ID for Gitea |
| `SHARED_LIBRARY_URL` | no | — | Jenkins shared library Git URL |
| `K8S_MANIFEST_GROUP` | no | `kubernetes-manifest` | Gitea org for manifest repos |
| `INGRESS_BASE_DOMAIN` | no | — | `docker.localhost` |
| `WORKER_CONCURRENCY` | no | `3` | Parallel provisioning jobs |
| `TLS_SKIP_VERIFY` | no | `true` | Disable TLS cert verification for local tools |
| `BRAND_APP_NAME` | no | `DevPortal` | Portal title in nav bar and login page |
| `BRAND_COMPANY` | no | — | Organisation name shown under the logo |
| `BRAND_PRIMARY_HUE` | no | `199` | HSL hue (0–360) — primary: CTAs, buttons, links, focus ring |
| `BRAND_SECONDARY_HUE` | no | `262` | HSL hue (0–360) — secondary: badges, tag chips, secondary highlights |
| `BRAND_SURFACE_HUE` | no | `215` | HSL hue (0–360) — panel/card/border tint (lower sat = more neutral) |
| `BRAND_LOGO_URL` | no | — | URL to a hosted logo image (SVG or PNG) |

> **Branding is fully runtime — all three hues drive the complete palette.** React fetches `GET /branding.json` at boot and writes `--brand-hue`, `--brand-secondary-hue`, `--brand-surface-hue` as CSS custom properties before the first render. Every colour token in the portal derives from these three. Change any `BRAND_*` var, restart — new palette applied instantly, no image rebuild.

---

## 15. make help

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
**Last updated:** 2026-08-23 — sections 13–15 added (environment migration guide, branding docs, env var table updated)
