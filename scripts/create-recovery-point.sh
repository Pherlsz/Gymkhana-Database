#!/usr/bin/env bash
set -euo pipefail

umask 077

: "${SOURCE_DATABASE_URL:?SOURCE_DATABASE_URL is required}"

output="${1:-recovery-private/gymkhana-$(date -u +%Y%m%dT%H%M%SZ).pgdump}"
case "$output" in
  *.pgdump) ;;
  *) echo "backup output must end with .pgdump" >&2; exit 2 ;;
esac

for command in pg_dump pg_restore psql python3 sha256sum; do
  command -v "$command" >/dev/null || { echo "$command is required" >&2; exit 2; }
done

mkdir -p "$(dirname "$output")"
repository_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [[ -n "$repository_root" ]]; then
  absolute_output="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$output")"
  case "$absolute_output" in
    "$repository_root"/*)
      git check-ignore -q "$absolute_output" || {
        echo "refusing to write a database backup to a tracked repository path" >&2
        exit 2
      }
      ;;
  esac
fi

if [[ -e "$output" || -e "$output.json" ]]; then
  echo "recovery point already exists: $output" >&2
  exit 2
fi

temporary="$(mktemp "${output}.tmp.XXXXXX")"
trap 'rm -f "$temporary"' EXIT

PGDATABASE="$SOURCE_DATABASE_URL" pg_dump \
  --format=custom \
  --compress=9 \
  --no-owner \
  --no-privileges \
  --lock-wait-timeout=5000 \
  --file="$temporary"

pg_restore --list "$temporary" >/dev/null
checksum="$(sha256sum "$temporary" | awk '{print $1}')"
bytes="$(wc -c < "$temporary" | tr -d ' ')"
server_version="$(PGDATABASE="$SOURCE_DATABASE_URL" psql -XAtqc 'SHOW server_version')"
created_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

mv "$temporary" "$output"
trap - EXIT

RECOVERY_FILE="$output" \
RECOVERY_CHECKSUM="$checksum" \
RECOVERY_BYTES="$bytes" \
RECOVERY_SERVER_VERSION="$server_version" \
RECOVERY_CREATED_AT="$created_at" \
python3 - <<'PY'
import json
import os
from pathlib import Path

output = Path(os.environ["RECOVERY_FILE"] + ".json")
payload = {
    "format": "postgresql-custom-v1",
    "created_at": os.environ["RECOVERY_CREATED_AT"],
    "sha256": os.environ["RECOVERY_CHECKSUM"],
    "bytes": int(os.environ["RECOVERY_BYTES"]),
    "server_version": os.environ["RECOVERY_SERVER_VERSION"],
}
temporary = output.with_suffix(output.suffix + ".tmp")
temporary.write_text(json.dumps(payload, sort_keys=True, separators=(",", ":")) + "\n", encoding="utf-8")
temporary.replace(output)
PY

printf 'recovery point created: %s\nsha256: %s\n' "$output" "$checksum"
