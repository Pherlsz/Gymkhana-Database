SHELL := /bin/sh

GO ?= go
PNPM ?= pnpm
BIN_DIR := $(CURDIR)/bin

OAPI_CODEGEN_VERSION := v2.7.2
SQLC_VERSION := v1.31.1
TERN_VERSION := v2.4.1
STATICCHECK_VERSION := v0.7.0
GOVULNCHECK_VERSION := v1.6.0
OSV_SCANNER_VERSION := v2.4.0

.PHONY: setup dev dev-api dev-web build build-backend build-frontend generate generate-go generate-ts generate-sql format format-check lint lint-backend lint-frontend test test-backend test-frontend test-race vuln scan check check-backend check-frontend check-config services-up services-down require-database-url migrate migrate-down-one migrate-status reset-db clean

setup:
	@corepack enable
	@$(PNPM) install --frozen-lockfile
	@$(GO) mod download

services-up:
	@echo "services-up is retired: the rebuild uses Neon, not local containers. Inject DATABASE_URL with lokeys."
	@exit 1

services-down:
	@echo "services-down is retired: the rebuild uses Neon, not local containers."
	@exit 1

require-database-url:
	@if [ -z "$$DATABASE_URL" ]; then \
		echo "DATABASE_URL is required. Inject Neon with: lokeys run -p gymkhana --env dev -- make migrate"; \
		exit 1; \
	fi

migrate: require-database-url
	@$(GO) run github.com/jackc/tern/v2@$(TERN_VERSION) migrate --migrations database/migrations --conn-string "$$DATABASE_URL"
	@$(GO) run ./cmd/river-migrate -action migrate

migrate-down-one: require-database-url
	@$(GO) run github.com/jackc/tern/v2@$(TERN_VERSION) migrate --destination -1 --migrations database/migrations --conn-string "$$DATABASE_URL"

migrate-status: require-database-url
	@$(GO) run github.com/jackc/tern/v2@$(TERN_VERSION) status --migrations database/migrations --conn-string "$$DATABASE_URL"
	@$(GO) run ./cmd/river-migrate -action validate

reset-db:
	@echo "reset-db is retired: the rebuild uses Neon only. There is no local database to reset."
	@echo "Do not drop Gymkhana-Database-Dev-18 from Make."
	@exit 1

dev:
	@echo "Inject secrets with lokeys, then run API and web in separate terminals:"
	@echo "  lokeys run -p gymkhana --env dev -- make dev-api"
	@echo "  lokeys run -p gymkhana --env dev -- make dev-web"

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
	@mkdir -p api/generated/operations
	@mkdir -p api/generated/googleforms
	@mkdir -p api/generated/query
	@mkdir -p api/generated/matching
	@mkdir -p api/generated/chat
	@mkdir -p api/generated/ocr
	@$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) --config api/oapi-codegen.yaml api/openapi.yaml
	@$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) --config api/oapi-attachments-codegen.yaml api/attachments.openapi.yaml
	@$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) --config api/oapi-search-codegen.yaml api/search.openapi.yaml
	@$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) --config api/oapi-operations-codegen.yaml api/operations.openapi.yaml
	@$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) --config api/oapi-google-forms-codegen.yaml api/google-forms.openapi.yaml
	@$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) --config api/oapi-query-codegen.yaml api/query.openapi.yaml
	@$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) --config api/oapi-matching-codegen.yaml api/matching.openapi.yaml
	@$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) --config api/oapi-chat-codegen.yaml api/chat.openapi.yaml
	@$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) --config api/oapi-ocr-codegen.yaml api/ocr.openapi.yaml

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

scan: vuln
	@$(GO) run github.com/google/osv-scanner/v2/cmd/osv-scanner@$(OSV_SCANNER_VERSION) scan source --recursive .
	@$(PNPM) audit --audit-level high

check-backend: format-check lint-backend test-backend test-race build-backend scan

check-frontend:
	@$(PNPM) check:web

check: check-backend check-frontend

clean:
	@rm -rf "$(BIN_DIR)" coverage coverage-web apps/web/dist
