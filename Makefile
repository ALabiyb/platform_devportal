# ---------------------------------------------------------------------------
# Author: Labiyb M. Said — DevSecOps Engineer
# Contact: saidlabiybm@gmail.com
# ---------------------------------------------------------------------------

BINARY  := devportal
CMD     := ./cmd/devportal
IMAGE   ?= $(shell echo $${REGISTRY_URL}/devportal/devportal)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: setup dev build frontend build-full test lint tidy \
        docker-build docker-push up down logs clean help

# ── First-time setup ────────────────────────────────────────────────────────

## setup: create .env from .env.example and generate required secrets
setup:
	@if [ -f .env ]; then \
		echo ".env already exists — skipping. Delete it and re-run to regenerate."; \
	else \
		cp .env.example .env; \
		echo "Created .env from .env.example"; \
	fi
	@# Generate ENCRYPTION_KEY if still empty
	@if grep -q 'ENCRYPTION_KEY=$$' .env || grep -q 'ENCRYPTION_KEY= *$$' .env; then \
		KEY=$$(openssl rand -base64 32); \
		if [ "$$(uname)" = "Darwin" ]; then \
			sed -i '' "s|^ENCRYPTION_KEY=.*|ENCRYPTION_KEY=$$KEY|" .env; \
		else \
			sed -i "s|^ENCRYPTION_KEY=.*|ENCRYPTION_KEY=$$KEY|" .env; \
		fi; \
		echo "Generated ENCRYPTION_KEY"; \
	fi
	@# Generate DB_PASSWORD if still empty
	@if grep -q 'DB_PASSWORD=$$' .env || grep -q 'DB_PASSWORD= *$$' .env; then \
		PASS=$$(openssl rand -hex 16); \
		if [ "$$(uname)" = "Darwin" ]; then \
			sed -i '' "s|^DB_PASSWORD=.*|DB_PASSWORD=$$PASS|" .env; \
		else \
			sed -i "s|^DB_PASSWORD=.*|DB_PASSWORD=$$PASS|" .env; \
		fi; \
		echo "Generated DB_PASSWORD"; \
	fi
	@echo ""
	@echo "Next steps:"
	@echo "  Standalone (any machine):  docker compose -f docker-compose.standalone.yml up -d"
	@echo "  Shared infra (local lab):  docker compose up -d"
	@echo "  Local Go run:              make dev"
	@echo ""
	@echo "Then open http://localhost:8080 and POST /auth/register to create your admin account."

# ── Local development ───────────────────────────────────────────────────────

## dev: run devportal locally with live .env (no Docker)
dev:
	@[ -f .env ] || (echo "ERROR: .env not found. Run: make setup" && exit 1)
	set -a && . ./.env && set +a && go run $(CMD)

## frontend: build the React SPA into web/dist/ (required before 'make build')
frontend:
	cd web && npm ci && npm run build

## build: compile the Go binary (run 'make frontend' first if not using Docker)
build:
	@mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) $(CMD)

## build-full: build frontend then Go binary in one step (local, no Docker)
build-full: frontend build

# ── Docker ──────────────────────────────────────────────────────────────────

## up: start devportal + postgres using the standalone compose (no shared infra needed)
up:
	@[ -f .env ] || (echo "ERROR: .env not found. Run: make setup" && exit 1)
	docker compose -f docker-compose.standalone.yml up -d --build

## down: stop and remove standalone compose containers
down:
	docker compose -f docker-compose.standalone.yml down

## logs: tail devportal logs
logs:
	docker compose -f docker-compose.standalone.yml logs -f devportal

## docker-build: build and tag the production image
docker-build:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		.

## docker-push: build and push image to Harbor
docker-push: docker-build
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):latest

# ── Quality ─────────────────────────────────────────────────────────────────

## test: run all unit tests with the race detector
test:
	go test -race -count=1 -timeout=60s ./...

## lint: run golangci-lint (install: brew install golangci-lint)
lint:
	golangci-lint run ./...

## tidy: sync go.mod and go.sum with actual imports
tidy:
	go mod tidy

# ── Housekeeping ────────────────────────────────────────────────────────────

## clean: remove compiled binaries
clean:
	rm -rf bin/

## help: list all available make targets with descriptions
help:
	@echo "DevPortal — NexBridge Technologies"
	@echo ""
	@grep -E '^##' Makefile | sed 's/## /  /' | column -t -s ':'
