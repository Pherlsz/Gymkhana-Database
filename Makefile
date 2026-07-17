SHELL := /bin/sh

GO ?= go
PNPM ?= pnpm
BIN_DIR := $(CURDIR)/bin

OAPI_CODEGEN_VERSION := v2.7.2
SQLC_VERSION := v1.31.1
TERN_VERSION := v2.4.1
STATICCHECK_VERSION := v0.7.0
GOVULNCHECK_VERSION := v1.6.0

.PHONY: setup dev dev-api dev-web build build-backend build-frontend generate generate-go generate-ts generate-sql format format-check lint lint-backend lint-frontend test test-backend test-frontend test-race vuln check check-backend check-frontend check-config services-up services-down migrate migrate-down-one migrate-status reset-db clean

setup:
	@corepack enable
	@$(PNPM) install --frozen-lockfile
	@$(GO) mod download

services-up:
	@docker compose up -d db

services-down:
	@docker compose down

migrate:
	@$(GO) run github.com/jackc/tern/v2@$(TERN_VERSION) migrate --migrations database/migrations --config database/tern.conf

migrate-down-one:
	@$(GO) run github.com/jackc/tern/v2@$(TERN_VERSION) migrate --destination -1 --migrations database/migrations --config database/tern.conf

migrate-status:
	@$(GO) run github.com/jackc/tern/v2@$(TERN_VERSION) status --migrations database/migrations --config database/tern.conf

reset-db:
	@docker compose down -v
	@docker compose up -d db
	@$(MAKE) migrate

dev:
	@echo "Run 'make dev-api' and 'make dev-web' in separate terminals."

dev-api:
	@$(GO) run ./cmd/api

dev-web:
	@$(PNPM) dev:web

check-config:
	@$(GO) run ./cmd/configcheck

build: build-backend build-frontend

build-backend:
	@$(GO) build ./cmd/...

build-frontend:
	@$(PNPM) build:web

generate: generate-go generate-ts generate-sql

generate-go:
	@mkdir -p api/generated/attachments
	@mkdir -p api/generated/search
	@$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) --config api/oapi-codegen.yaml api/openapi.yaml
	@$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) --config api/oapi-attachments-codegen.yaml api/attachments.openapi.yaml
	@$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) --config api/oapi-search-codegen.yaml api/search.openapi.yaml

generate-ts:
	@$(PNPM) generate:openapi:ts

generate-sql:
	@$(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate -f database/sqlc.yaml

format:
	@gofmt -w $$(find cmd internal api/generated -type f -name '*.go' 2>/dev/null)
	@$(PNPM) format

format-check:
	@unformatted="$$(gofmt -l $$(find cmd internal api/generated -type f -name '*.go' 2>/dev/null))"; \
	if [ -n "$$unformatted" ]; then echo "Unformatted Go files:"; echo "$$unformatted"; exit 1; fi
	@$(PNPM) format:check

lint: lint-backend lint-frontend

lint-backend:
	@GOBIN="$(BIN_DIR)" $(GO) install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
	@"$(BIN_DIR)/staticcheck" ./...
	@$(GO) vet ./...

lint-frontend:
	@$(PNPM) lint:web

test: test-backend test-frontend

test-backend:
	@$(GO) test ./...

test-frontend:
	@$(PNPM) test:web

test-race:
	@$(GO) test -race ./...

vuln:
	@$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

check-backend: format-check lint-backend test-backend test-race build-backend vuln

check-frontend:
	@$(PNPM) check:web

check: check-backend check-frontend

clean:
	@rm -rf "$(BIN_DIR)" coverage coverage-web apps/web/dist
