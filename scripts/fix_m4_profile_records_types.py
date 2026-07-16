#!/usr/bin/env python3
from pathlib import Path

path = Path("apps/web/src/ProfileRecordsPanel.tsx")
text = path.read_text(encoding="utf-8")
replacements = {
    "onTypes?: () => void;": "onTypes: (() => void) | undefined;",
    "record?: DocumentRecord;": "record: DocumentRecord | undefined;",
    "record?: BillRecord;": "record: BillRecord | undefined;",
    "record?: T;": "record: T | undefined;",
}
for old, new in replacements.items():
    if old not in text:
        raise RuntimeError(f"anchor not found: {old}")
    text = text.replace(old, new)
path.write_text(text, encoding="utf-8")
