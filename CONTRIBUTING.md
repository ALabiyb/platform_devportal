# Contributing to DevPortal

Thank you for your interest in contributing to DevPortal — an open-source Internal Developer Platform that provisions production-ready projects end-to-end.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [How to Contribute](#how-to-contribute)
- [Development Setup](#development-setup)
- [Commit Convention](#commit-convention)
- [Pull Request Process](#pull-request-process)
- [Reporting Bugs](#reporting-bugs)
- [Suggesting Features](#suggesting-features)

---

## Code of Conduct

Be respectful, constructive, and inclusive. We welcome contributors of all experience levels.

---

## Getting Started

1. **Fork** the repository on GitHub
2. **Clone** your fork locally:
   ```bash
   git clone https://github.com/<your-username>/platform_devportal.git
   cd platform_devportal
   ```
3. **Add the upstream remote** so you can pull future updates:
   ```bash
   git remote add upstream https://github.com/ALabiyb/platform_devportal.git
   ```

---

## Development Setup

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.23+ | Backend |
| Docker | 24+ | Local dev stack |
| Node.js | 20+ | Frontend (Day 11+) |
| make | any | Build shortcuts |

### Running locally

```bash
# 1. Copy environment config
cp .env.example .env
# Fill in the required values (see .env.example comments)

# 2. Create the devportal database in your local PostgreSQL
docker exec postgres psql -U postgres -c "
  CREATE DATABASE devportal;
  CREATE USER devportal WITH PASSWORD 'changeme';
  GRANT ALL ON DATABASE devportal TO devportal;
"

# 3. Apply migrations
make migrate

# 4. Run the server
make run

# 5. Verify it's up
curl http://localhost:8080/healthz
```

Or with Docker Compose (joins the existing `traefik-net`):

```bash
docker compose up -d
# Access at http://devportal.localhost
```

### Running tests

```bash
make test          # unit tests with race detector
make lint          # golangci-lint (install: brew install golangci-lint)
```

---

## How to Contribute

### Good first issues

Look for issues labelled **`good first issue`** — these are well-scoped tasks that don't require deep knowledge of the whole codebase.

### Areas we welcome contributions in

- **New provider adapters** — GitLab self-hosted, GitHub, Bitbucket, ArgoCD, Vault
- **Build tool templates** — new Dockerfiles and Jenkinsfiles for additional stacks
- **Frontend components** — React/TypeScript UI (shadcn/ui, TanStack Query)
- **Documentation** — setup guides, architecture explanations, video walkthroughs
- **Tests** — unit tests for adapters, integration tests for the provisioning flow
- **Helm chart** — improvements to the chart values and K8s manifests

---

## Commit Convention

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short description>

[optional body]
```

| Type | When to use |
|------|-------------|
| `feat` | A new feature |
| `fix` | A bug fix |
| `docs` | Documentation only |
| `refactor` | Code change that is neither a fix nor a feature |
| `test` | Adding or updating tests |
| `chore` | Build process, dependency updates, tooling |

**Examples:**
```
feat(gitlab): add webhook creation to GitLab adapter
fix(auth): handle expired OIDC token on session refresh
docs(contributing): add development setup section
```

---

## Pull Request Process

1. **Branch** off `main` with a descriptive name:
   ```bash
   git checkout -b feat/argocd-adapter
   ```
2. **Write code** — every file must include the author header block (see existing files for the format)
3. **Test** — `make test` must pass before submitting
4. **Lint** — `make lint` must pass (no new warnings)
5. **Open a PR** against `main` with:
   - A clear title following the commit convention
   - A description of what changed and why
   - Screenshots or logs if it's a UI or behaviour change
6. **Address review feedback** — maintainers may request changes; please respond within a week
7. **Do not squash** — keep individual commits so the history is readable

PRs that add a new provider adapter **must** include:
- The adapter implementation under `internal/plugin/<provider>/`
- At least one unit test
- A section in the README under "Supported Providers"

---

## Reporting Bugs

Open a GitHub Issue with:

- **Title**: short, specific description of the bug
- **Steps to reproduce**: numbered list, minimal and complete
- **Expected behaviour**: what should happen
- **Actual behaviour**: what actually happens (include logs / error messages)
- **Environment**: OS, Go version, Docker version, devportal version

---

## Suggesting Features

Open a GitHub Issue with the label `enhancement` and describe:

- The problem you are trying to solve
- Your proposed solution
- Any alternatives you considered

Large features should be discussed in an issue **before** opening a PR so we can align on the design.

---

## Author

**Labiyb M. Said** — DevSecOps Engineer  
Contact: saidlabiybm@gmail.com  
GitHub: [@ALabiyb](https://github.com/ALabiyb)

---

*Licensed under the [MIT License](LICENSE).*
