#!/usr/bin/env bash
set -euo pipefail

umask 077

: "${TARGET_DATABASE_URL:?TARGET_DATABASE_URL is required}"
: "${CONFIRM_RESTORE_REHEARSAL:?set CONFIRM_RESTORE_REHEARSAL=YES}"

if [[ "$CONFIRM_RESTORE_REHEARSAL" != "YES" ]]; then
  echo "CONFIRM_RESTORE_REHEARSAL must be YES" >&2
  exit 2
fi

backup="${1:?usage: scripts/rehearse-restore.sh BACKUP.pgdump [REPORT.json]}"
report="${2:-recovery-private/restore-rehearsal-$(date -u +%Y%m%dT%H%M%SZ).json}"
metadata="${backup}.json"

for command in pg_restore psql python3 sha256sum; do
  command -v "$command" >/dev/null || { echo "$command is required" >&2; exit 2; }
done

[[ -f "$backup" ]] || { echo "backup does not exist: $backup" >&2; exit 2; }
[[ -f "$metadata" ]] || { echo "backup metadata does not exist: $metadata" >&2; exit 2; }

target_name="$(TARGET_DATABASE_URL="$TARGET_DATABASE_URL" python3 - <<'PY'
import os
from urllib.parse import urlparse
parsed = urlparse(os.environ["TARGET_DATABASE_URL"])
print(parsed.path.lstrip("/").split("?")[0])
PY
)"
case "$target_name" in
  *_rehearsal|*_restore) ;;
  *) echo "target database name must end with _rehearsal or _restore" >&2; exit 2 ;;
esac

expected_checksum="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["sha256"])' "$metadata")"
actual_checksum="$(sha256sum "$backup" | awk '{print $1}')"
[[ "$actual_checksum" == "$expected_checksum" ]] || { echo "backup checksum mismatch" >&2; exit 1; }
pg_restore --list "$backup" >/dev/null

existing_tables="$(PGDATABASE="$TARGET_DATABASE_URL" psql -XAtqc "SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind='r' AND n.nspname NOT IN ('pg_catalog','information_schema')")"
[[ "$existing_tables" == "0" ]] || { echo "restore rehearsal target must be empty" >&2; exit 2; }

PGDATABASE="$TARGET_DATABASE_URL" pg_restore \
  --exit-on-error \
  --single-transaction \
  --no-owner \
  --no-privileges \
  --dbname="$TARGET_DATABASE_URL" \
  "$backup"

restored_tables="$(PGDATABASE="$TARGET_DATABASE_URL" psql -XAtqc "SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind='r' AND n.nspname NOT IN ('pg_catalog','information_schema')")"
[[ "$restored_tables" -gt 0 ]] || { echo "restore completed without user tables" >&2; exit 1; }
mkdir -p "$(dirname "$report")"

RESTORE_REPORT="$report" \
RESTORE_BACKUP_SHA="$actual_checksum" \
RESTORE_TARGET_NAME="$target_name" \
RESTORE_TABLES="$restored_tables" \
python3 - <<'PY'
import json
import os
from datetime import datetime, timezone
from pathlib import Path

path = Path(os.environ["RESTORE_REPORT"])
payload = {
    "format": "restore-rehearsal-v1",
    "completed_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
    "backup_sha256": os.environ["RESTORE_BACKUP_SHA"],
    "target_database": os.environ["RESTORE_TARGET_NAME"],
    "restored_table_count": int(os.environ["RESTORE_TABLES"]),
}
temporary = path.with_suffix(path.suffix + ".tmp")
temporary.write_text(json.dumps(payload, sort_keys=True, separators=(",", ":")) + "\n", encoding="utf-8")
temporary.replace(path)
PY

printf 'restore rehearsal completed: %s tables=%s report=%s\n' "$target_name" "$restored_tables" "$report"
