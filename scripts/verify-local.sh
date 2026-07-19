#!/usr/bin/env bash
set -Eeuo pipefail

MODE="${1:-changed}"
shift || true
BASE_REF="${BASE_REF:-origin/main}"
CI_MODE=false
KEEP_DATABASE=false

while (($#)); do
  case "$1" in
    --base)
      BASE_REF="$2"
      shift 2
      ;;
    --ci)
      CI_MODE=true
      shift
      ;;
    --keep-database)
      KEEP_DATABASE=true
      shift
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

log() { printf '\n==> %s\n' "$*"; }
need() { command -v "$1" >/dev/null 2>&1 || { echo "missing required command: $1" >&2; exit 1; }; }

need git
need go

RUN_BACKEND=false
RUN_DATABASE=false
RUN_FRONTEND=false
RUN_CONTRACTS=false
RUN_SECURITY=false

select_changed_scope() {
  local base files
  if git rev-parse --verify "$BASE_REF" >/dev/null 2>&1; then
    base="$(git merge-base HEAD "$BASE_REF")"
  elif git rev-parse --verify HEAD^ >/dev/null 2>&1; then
    base="HEAD^"
  else
    base="HEAD"
  fi
  files="$(git diff --name-only "$base"...HEAD)"
  printf '%s\n' "$files"

  grep -Eq '(^|/).*\.go$|^go\.(mod|sum)$|^cmd/|^internal/|^Makefile$' <<<"$files" && RUN_BACKEND=true || true
  grep -Eq '^database/|^internal/platform/postgres/dbgen/' <<<"$files" && RUN_DATABASE=true || true
  grep -Eq '^apps/|^package\.json$|^pnpm-|^tsconfig|^vitest|^oxlint\.json$' <<<"$files" && RUN_FRONTEND=true || true
  grep -Eq '^api/|^apps/web/src/generated/|^database/(queries|schema\.sql)|^Makefile$' <<<"$files" && RUN_CONTRACTS=true || true
  grep -Eq '^go\.(mod|sum)$|^package\.json$|^pnpm-lock\.yaml$' <<<"$files" && RUN_SECURITY=true || true

  if ! $RUN_BACKEND && ! $RUN_DATABASE && ! $RUN_FRONTEND && ! $RUN_CONTRACTS && ! $RUN_SECURITY; then
    log "No executable verification scope changed"
    exit 0
  fi
}

case "$MODE" in
  changed) select_changed_scope ;;
  quick) RUN_BACKEND=true; RUN_FRONTEND=true ;;
  full) RUN_BACKEND=true; RUN_DATABASE=true; RUN_FRONTEND=true; RUN_CONTRACTS=true ;;
  backend) RUN_BACKEND=true ;;
  database) RUN_DATABASE=true ;;
  frontend) RUN_FRONTEND=true ;;
  contracts) RUN_CONTRACTS=true ;;
  security) RUN_SECURITY=true ;;
  *)
    echo "usage: scripts/verify-local.sh [changed|quick|full|backend|database|frontend|contracts|security] [--base REF] [--ci] [--keep-database]" >&2
    exit 2
    ;;
esac

POSTGRES_CONTAINER=""
cleanup() {
  if [[ -n "$POSTGRES_CONTAINER" ]] && ! $KEEP_DATABASE; then
    docker rm -f "$POSTGRES_CONTAINER" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

ensure_database() {
  if [[ -n "${DATABASE_URL:-}" ]]; then
    if [[ "$DATABASE_URL" != *"localhost"* && "$DATABASE_URL" != *"127.0.0.1"* && "${GYMKHANA_ALLOW_EXTERNAL_DATABASE:-}" != "1" ]]; then
      echo "refusing to run migration verification against a non-local database" >&2
      echo "set GYMKHANA_ALLOW_EXTERNAL_DATABASE=1 only for a disposable database/branch" >&2
      exit 1
    fi
    return
  fi

  need docker
  local port
  port="$(python3 - <<'PY'
import socket
with socket.socket() as sock:
    sock.bind(("127.0.0.1", 0))
    print(sock.getsockname()[1])
PY
)"
  POSTGRES_CONTAINER="gymkhana-local-ci-${USER:-user}-$$"
  log "Starting disposable PostgreSQL on port $port"
  docker run -d --rm \
    --name "$POSTGRES_CONTAINER" \
    -e POSTGRES_DB=gymkhana \
    -e POSTGRES_USER=gymkhana \
    -e POSTGRES_PASSWORD=gymkhana \
    -p "127.0.0.1:${port}:5432" \
    postgres:17-alpine >/dev/null

  for _ in {1..60}; do
    if docker exec "$POSTGRES_CONTAINER" pg_isready -U gymkhana -d gymkhana >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
  docker exec "$POSTGRES_CONTAINER" pg_isready -U gymkhana -d gymkhana >/dev/null
  export DATABASE_URL="postgres://gymkhana:gymkhana@127.0.0.1:${port}/gymkhana?sslmode=disable"
}

if $RUN_BACKEND; then
  log "Backend formatting, vet, tests and build"
  scripts/configure-private-go.sh
  go mod download
  unformatted="$(gofmt -l $(find cmd internal api/generated -type f -name '*.go'))"
  [[ -z "$unformatted" ]] || { echo "$unformatted"; exit 1; }
  go vet ./...
  go test ./...
  go build ./cmd/...
fi

if $RUN_DATABASE; then
  ensure_database
  log "Migrations and PostgreSQL integration"
  scripts/configure-private-go.sh
  make migrate
  make migrate-status
  make migrate-down-one
  make migrate
  make migrate-status
  go test -p=1 -tags=integration \
    ./internal/attachment \
    ./internal/search \
    ./internal/operations \
    ./internal/googleforms \
    ./internal/queryengine \
    ./internal/matching \
    ./internal/aichat \
    ./internal/ocr
fi

if $RUN_CONTRACTS; then
  log "Generated SQL and OpenAPI contracts"
  need node
  corepack enable
  scripts/configure-private-go.sh
  go mod download
  pnpm install --frozen-lockfile
  make generate-sql generate-go generate-ts
  find internal/platform/postgres/dbgen api/generated -type f -name '*.go' -print0 | xargs -0 --no-run-if-empty gofmt -w
  pnpm exec oxfmt \
    apps/web/src/generated/api.ts \
    apps/web/src/generated/attachments-api.ts \
    apps/web/src/generated/search-api.ts \
    apps/web/src/generated/operations-api.ts \
    apps/web/src/generated/google-forms-api.ts \
    apps/web/src/generated/query-api.ts \
    apps/web/src/generated/matching-api.ts \
    apps/web/src/generated/chat-api.ts \
    apps/web/src/generated/ocr-api.ts \
    apps/web/src/generated/tasks-api.ts \
    --write
  git diff --exit-code -- internal/platform/postgres/dbgen api/generated apps/web/src/generated
fi

if $RUN_FRONTEND; then
  log "Frontend typecheck, lint, formatting, tests and build"
  need node
  corepack enable
  pnpm install --frozen-lockfile
  pnpm check:web
fi

if $RUN_SECURITY; then
  log "Dependency security scans"
  need node
  corepack enable
  scripts/configure-private-go.sh
  go mod download
  pnpm install --frozen-lockfile
  go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
  go run github.com/google/osv-scanner/v2/cmd/osv-scanner@v2.4.0 scan source --recursive .
  pnpm audit --audit-level high
fi

if $CI_MODE; then
  log "CI verification completed"
else
  log "Local verification completed"
fi
