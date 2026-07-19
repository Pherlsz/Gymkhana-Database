#!/usr/bin/env python3
"""Export the legacy Prisma SQLite database into a deterministic, read-only bundle."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import sqlite3
import tempfile
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Iterable
from urllib.parse import quote

FORMAT_VERSION = "gymkhana-legacy-sqlite-v1"
FILES = ("contexts.jsonl", "columns.jsonl", "records.jsonl", "attachments.jsonl")


def canonical(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def normalize_json(value: Any) -> Any:
    if value is None or isinstance(value, (dict, list, bool, int, float)):
        return value
    if isinstance(value, str):
        try:
            return json.loads(value)
        except json.JSONDecodeError as exc:
            raise ValueError("legacy Record.data is not valid JSON") from exc
    raise TypeError(f"unsupported JSON value type: {type(value).__name__}")


def rows(connection: sqlite3.Connection, query: str) -> Iterable[dict[str, Any]]:
    for row in connection.execute(query):
        yield dict(row)


def write_jsonl(path: Path, values: Iterable[dict[str, Any]]) -> tuple[int, str]:
    digest = hashlib.sha256()
    count = 0
    with path.open("wb") as output:
        for value in values:
            encoded = (canonical(value) + "\n").encode("utf-8")
            output.write(encoded)
            digest.update(encoded)
            count += 1
    return count, digest.hexdigest()


def table_names(connection: sqlite3.Connection) -> set[str]:
    return {row[0] for row in connection.execute("SELECT name FROM sqlite_master WHERE type = 'table'")}


def export(database: Path, output: Path) -> None:
    if output.exists():
        if not output.is_dir() or any(output.iterdir()):
            raise SystemExit(f"output directory must not exist or must be empty: {output}")
        output.rmdir()
    output.parent.mkdir(parents=True, exist_ok=True)
    temporary = Path(tempfile.mkdtemp(prefix=f".{output.name}-", dir=output.parent))
    try:
        uri = f"file:{quote(str(database.resolve()))}?mode=ro"
        connection = sqlite3.connect(uri, uri=True)
        connection.row_factory = sqlite3.Row
        try:
            connection.execute("PRAGMA query_only = ON")
            connection.execute("BEGIN")
            required = {"Context", "ColumnDefinition", "Record", "Attachment"}
            missing = required - table_names(connection)
            if missing:
                raise SystemExit(f"legacy database is missing tables: {', '.join(sorted(missing))}")

            datasets: dict[str, Iterable[dict[str, Any]]] = {
                "contexts.jsonl": rows(
                    connection,
                    'SELECT id, slug, name, icon, description, "order" AS sort_order, '
                    'createdAt AS created_at, updatedAt AS updated_at FROM "Context" ORDER BY id',
                ),
                "columns.jsonl": rows(
                    connection,
                    'SELECT id, contextId AS context_id, key, label, type, groupName AS group_name, '
                    '"order" AS sort_order, required, options, formula, isComputed AS is_computed, '
                    'visible, createdAt AS created_at, updatedAt AS updated_at '
                    'FROM "ColumnDefinition" ORDER BY id',
                ),
                "records.jsonl": (
                    {**record, "data": normalize_json(record["data"])}
                    for record in rows(
                        connection,
                        'SELECT id, contextId AS context_id, pessoaId AS person_id, '
                        'sourceIntegrationId AS source_integration_id, '
                        'googleFormResponseId AS google_form_response_id, data, searchText AS search_text, '
                        'usedInGincana AS used_in_gymkhana, returned, usedAt AS used_at, '
                        'gincanaNote AS gymkhana_note, isDigital AS is_digital, '
                        'createdAt AS created_at, updatedAt AS updated_at FROM "Record" ORDER BY id',
                    )
                ),
                "attachments.jsonl": rows(
                    connection,
                    'SELECT id, recordId AS record_id, fileName AS file_name, mimeType AS mime_type, '
                    'filePath AS file_path, ocrRawText AS ocr_raw_text, ocrStatus AS ocr_status, '
                    'uploadedById AS uploaded_by_id, createdAt AS created_at, updatedAt AS updated_at '
                    'FROM "Attachment" ORDER BY id',
                ),
            }

            manifest_files: dict[str, dict[str, Any]] = {}
            for name in FILES:
                count, digest = write_jsonl(temporary / name, datasets[name])
                manifest_files[name] = {"rows": count, "sha256": digest}

            fingerprint_input = "".join(
                f"{name}:{manifest_files[name]['sha256']}\n" for name in sorted(manifest_files)
            ).encode("utf-8")
            manifest = {
                "format_version": FORMAT_VERSION,
                "source_schema": "prisma-sqlite-record-v1",
                "exported_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
                "files": manifest_files,
                "fingerprint": hashlib.sha256(fingerprint_input).hexdigest(),
            }
            (temporary / "manifest.json").write_text(canonical(manifest) + "\n", encoding="utf-8")
            connection.rollback()
        finally:
            connection.close()
        os.replace(temporary, output)
    except BaseException:
        shutil.rmtree(temporary, ignore_errors=True)
        raise


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--database", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    arguments = parser.parse_args()
    if not arguments.database.is_file():
        raise SystemExit(f"database file does not exist: {arguments.database}")
    export(arguments.database, arguments.output)


if __name__ == "__main__":
    main()
