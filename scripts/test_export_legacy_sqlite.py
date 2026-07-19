#!/usr/bin/env python3

from __future__ import annotations

import json
import sqlite3
import subprocess
import tempfile
import unittest
from pathlib import Path


class LegacySQLiteExportTest(unittest.TestCase):
    def test_exports_deterministic_bundle_without_modifying_source(self) -> None:
        root = Path(__file__).resolve().parents[1]
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            database = directory / "legacy.db"
            output = directory / "bundle"
            self._create_database(database)
            before = database.read_bytes()

            subprocess.run(
                [
                    "python3",
                    str(root / "scripts/export-legacy-sqlite.py"),
                    "--database",
                    str(database),
                    "--output",
                    str(output),
                ],
                check=True,
                cwd=root,
            )

            self.assertEqual(database.read_bytes(), before)
            manifest = json.loads((output / "manifest.json").read_text(encoding="utf-8"))
            self.assertEqual(manifest["format_version"], "gymkhana-legacy-sqlite-v1")
            self.assertEqual(manifest["files"]["contexts.jsonl"]["rows"], 1)
            self.assertEqual(manifest["files"]["columns.jsonl"]["rows"], 1)
            self.assertEqual(manifest["files"]["records.jsonl"]["rows"], 1)
            self.assertEqual(manifest["files"]["attachments.jsonl"]["rows"], 0)
            self.assertRegex(manifest["fingerprint"], r"^[a-f0-9]{64}$")

            record = json.loads((output / "records.jsonl").read_text(encoding="utf-8"))
            self.assertEqual(record["id"], "r1")
            self.assertEqual(record["data"], {"name": "Pessoa Teste"})

    @staticmethod
    def _create_database(path: Path) -> None:
        connection = sqlite3.connect(path)
        connection.executescript(
            '''
            CREATE TABLE "Context" (
                id TEXT PRIMARY KEY, slug TEXT NOT NULL, name TEXT NOT NULL,
                icon TEXT, description TEXT, "order" INTEGER NOT NULL,
                createdAt TEXT NOT NULL, updatedAt TEXT NOT NULL
            );
            CREATE TABLE "ColumnDefinition" (
                id TEXT PRIMARY KEY, contextId TEXT NOT NULL, key TEXT NOT NULL,
                label TEXT NOT NULL, type TEXT NOT NULL, groupName TEXT,
                "order" INTEGER NOT NULL, required INTEGER NOT NULL,
                options TEXT, formula TEXT, isComputed INTEGER NOT NULL,
                visible INTEGER NOT NULL, createdAt TEXT NOT NULL,
                updatedAt TEXT NOT NULL
            );
            CREATE TABLE "Record" (
                id TEXT PRIMARY KEY, contextId TEXT NOT NULL, pessoaId TEXT,
                sourceIntegrationId TEXT, googleFormResponseId TEXT,
                data TEXT NOT NULL, searchText TEXT, usedInGincana INTEGER NOT NULL,
                returned INTEGER NOT NULL, usedAt TEXT, gincanaNote TEXT,
                isDigital INTEGER NOT NULL, createdAt TEXT NOT NULL,
                updatedAt TEXT NOT NULL
            );
            CREATE TABLE "Attachment" (
                id TEXT PRIMARY KEY, recordId TEXT, fileName TEXT NOT NULL,
                mimeType TEXT NOT NULL, filePath TEXT NOT NULL, ocrRawText TEXT,
                ocrStatus TEXT NOT NULL, uploadedById TEXT,
                createdAt TEXT NOT NULL, updatedAt TEXT NOT NULL
            );
            '''
        )
        now = "2026-07-19T00:00:00Z"
        connection.execute(
            'INSERT INTO "Context" VALUES (?,?,?,?,?,?,?,?)',
            ("c1", "dados-pessoais", "Dados pessoais", None, None, 1, now, now),
        )
        connection.execute(
            'INSERT INTO "ColumnDefinition" VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)',
            ("col1", "c1", "name", "Nome", "TEXT", None, 1, 1, None, None, 0, 1, now, now),
        )
        connection.execute(
            'INSERT INTO "Record" VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)',
            (
                "r1",
                "c1",
                None,
                None,
                None,
                json.dumps({"name": "Pessoa Teste"}),
                "pessoa teste",
                0,
                0,
                None,
                None,
                0,
                now,
                now,
            ),
        )
        connection.commit()
        connection.close()


if __name__ == "__main__":
    unittest.main()
