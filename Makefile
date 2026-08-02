# ---------------------------------------------------------------------------
# Author: Labiyb M. Said — DevSecOps Engineer
# Contact: saidlabiybm@gmail.com
# ---------------------------------------------------------------------------

BINARY  := devportal
CMD     := ./cmd/devportal
# IMAGE must be set via environment variable or make argument — no domain hardcoded.
# Usage: make docker-build IMAGE=harbor.example.com/devportal/devportal
IMAGE   ?= $(shell echo $${REGISTRY_URL}/devportal/devportal)

# VERSION is the git tag + commit SHA, e.g. "v0.1.0-abc1234".
# Falls back to "dev" when git is not available.
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build run test lint tidy docker-build docker-push migrate clean help

## build: compile the Go binary for the current OS/arch into ./bin/
build:
	@mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) $(CMD)

## run: start devportal locally (reads .env — copy .env.example first)
run:
	@[ -f .env ] || (echo "ERROR: .env not found. Run: cp .env.example .env" && exit 1)
	set -a && . ./.env && set +a && go run $(CMD)

## test: run all unit tests with the race detector enabled
test:
	go test -race -count=1 -timeout=60s ./...

## lint: run golangci-lint (install: brew install golangci-lint)
lint:
	golangci-lint run ./...

## tidy: sync go.mod and go.sum with the actual imports
tidy:
	go mod tidy

## docker-build: build the production Docker image
docker-build:
	docker build \
		--build-arg VERSION=$(VERSION) \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		.

## docker-push: build and push image to Harbor
docker-push: docker-build
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):latest

## migrate: apply SQL migrations to the local dev database
migrate:
	@[ -f .env ] || (echo "ERROR: .env not found" && exit 1)
	@. ./.env && PGPASSWORD=$$DB_PASSWORD psql \
		-h $$DB_HOST -p $$DB_PORT \
		-U $$DB_USER -d $$DB_NAME \
		-f migrations/001_initial.sql
	@echo "Migrations applied."

## clean: remove compiled binaries
clean:
	rm -rf bin/

## help: list all available make targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
